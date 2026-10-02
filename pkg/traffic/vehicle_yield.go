//go:build windows
// +build windows

package traffic

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Service vehicles give way to aircraft: a tug or fuel truck driving to or
// from its depot looks VehicleLookMeters ahead on its way and stops
// VehicleStopShortMeters short of where a moving aircraft's body, or the
// next VehicleAircraftLookMeters of its path, comes within both half
// widths plus VehicleClearMeters. It waits there until the aircraft has
// passed. A vehicle already in an aircraft's path (within
// VehicleCommitMeters) drives on to clear it. Parked aircraft do not count
// (the roads pass them), nor the vehicle's own aircraft. Aircraft do not
// see the vehicles: they keep their way.
const (
	VehicleLookMeters         = 40.0
	VehicleStopShortMeters    = 3.0
	VehicleAircraftLookMeters = 120.0
	VehicleClearMeters        = 8.0
	VehicleCommitMeters       = 4.0
	vehicleHalfMeters         = 1.5
	vehicleLookStep           = 2.0
)

// Service vehicles among themselves: on the roads they keep
// VehicleLaneMeters right of the centreline (blended in over
// vehicleLaneBlendMeters after the start and before the end of their way),
// so oncoming ones pass; one stops VehicleStopShortMeters short of another
// vehicle's body within both half widths plus VehicleGapMeters of its way:
// always behind one it follows; at a crossing, or two starting on the
// same spot, the one with the higher object ID waits. Waiting
// VehicleWaitMax for vehicles, it drives on regardless for
// vehicleIgnoreFor (one parked on its way would hold it for ever).
const (
	VehicleLaneMeters      = 2.0
	VehicleGapMeters       = 3.0
	VehicleWaitMax         = time.Minute
	vehicleLaneBlendMeters = 15.0
	vehicleIgnoreFor       = 20 * time.Second
)

// VehicleTraffic is the traffic a service vehicle gives way to:
// GroundPicture.
type VehicleTraffic interface {
	// VehicleConflict returns how far along ahead (points every
	// vehicleLookStep meters from the vehicle's front) the first conflict
	// is, with a moving aircraft other than own or (unless noVehicles)
	// another vehicle than self; vehicle reports which; ok false when
	// none.
	VehicleConflict(own, self uint32, ahead []airport.LatLon, half float64, noVehicles bool, now time.Time) (at float64, vehicle, ok bool)
	// ReportVehicle records vehicle id where it is (its reference point,
	// heading, length); ForgetVehicle drops it.
	ReportVehicle(id uint32, pos airport.LatLon, hdg, length float64, now time.Time)
	ForgetVehicle(id uint32)
}

// vehicleEntry is a service vehicle in the ground picture.
type vehicleEntry struct {
	pos         airport.LatLon
	hdg, length float64
	at          time.Time
}

func (p *GroundPicture) ReportVehicle(id uint32, pos airport.LatLon, hdg, length float64, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.vehicles == nil {
		p.vehicles = map[uint32]vehicleEntry{}
	}
	p.vehicles[id] = vehicleEntry{pos: pos, hdg: hdg, length: length, at: now}
}

func (p *GroundPicture) ForgetVehicle(id uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.vehicles, id)
}

// trafficAware is a vehicle that gives way (SimObjectTug,
// SimObjectFuelTruck): the departure hands it its ground picture, its own
// aircraft and its clock.
type trafficAware interface {
	SetTraffic(t VehicleTraffic, own uint32, now func() time.Time)
}

// VehicleConflict implements VehicleTraffic: the first point of ahead near
// a moving aircraft's body (always) or near its path ahead (beyond
// VehicleCommitMeters).
func (p *GroundPicture) VehicleConflict(own, self uint32, ahead []airport.LatLon, half float64, noVehicles bool, now time.Time) (float64, bool, bool) {
	p.mu.Lock()
	var others []groundEntry
	for id, e := range p.aircraft {
		moving := e.pushing || len(e.ahead) > 0 && !e.waiting
		if id != own && moving && now.Sub(e.at) <= TrafficStaleAfter {
			others = append(others, e)
		}
	}
	type veh struct {
		id uint32
		e  vehicleEntry
	}
	var vehicles []veh
	for id, e := range p.vehicles {
		if id != self && !noVehicles && now.Sub(e.at) <= TrafficStaleAfter {
			vehicles = append(vehicles, veh{id, e})
		}
	}
	p.mu.Unlock()
	mine := 0.0 // the vehicle's heading: along its way
	if len(ahead) > 1 {
		mine = localBearing(ahead[0], ahead[1])
	}
	for i, q := range ahead {
		d := float64(i) * vehicleLookStep
		for _, v := range vehicles {
			rel := math.Abs(headingDiff(mine, v.e.hdg))
			if rel > 120 {
				continue // oncoming: in the other lane
			}
			reach := 2*half + VehicleGapMeters
			hit := false
			for b := -v.e.length / 2; b <= v.e.length/2+0.01; b += math.Max(1, v.e.length/4) {
				if localDist(q, offsetHeading(v.e.pos, v.e.hdg, b)) <= reach {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			// Where it is along my way from my front: behind me it waits
			// for me; side by side (two starting on the same spot) or
			// crossing, the higher ID waits, so two never wait for each
			// other; ahead, I follow.
			along := localDist(ahead[0], v.e.pos) * math.Cos((localBearing(ahead[0], v.e.pos)-mine)*math.Pi/180)
			if along < -v.e.length/2-1 {
				continue
			}
			if (rel > 45 || along <= v.e.length/2+1) && self < v.id {
				continue
			}
			return d, true, true
		}
		for _, e := range others {
			oh := e.half
			if oh <= 0 {
				oh = DefaultHalfSpanMeters
			}
			reach := half + oh + VehicleClearMeters
			for b := -e.tail; b <= e.nose+0.01; b += trafficBodyStep {
				if localDist(q, offsetHeading(e.pos, e.hdg, b)) <= reach {
					return d, false, true
				}
			}
			if d <= VehicleCommitMeters {
				continue // in its path already: clear it
			}
			run, prev := 0.0, e.pos
			for _, r := range e.ahead {
				run += localDist(prev, r)
				prev = r
				if run > VehicleAircraftLookMeters {
					break
				}
				if localDist(q, r) <= reach {
					return d, false, true
				}
			}
		}
	}
	return 0, false, false
}

// vehicleYield holds a vehicle's mover short of the first conflict ahead
// on its way, or lets it go on.
type vehicleYield struct {
	traffic VehicleTraffic
	own     uint32
	now     func() time.Time
	waiting bool
	// self is the vehicle's object; waitFrom when it began waiting for a
	// vehicle; ignoreUntil: it drives on past vehicles until then.
	self        uint32
	waitFrom    time.Time
	ignoreUntil time.Time
}

func (y *vehicleYield) SetTraffic(t VehicleTraffic, own uint32, now func() time.Time) {
	y.traffic, y.own, y.now = t, own, now
}

// check sets or clears m's traffic stop; it reports whether the vehicle is
// waiting for an aircraft.
func (y *vehicleYield) check(m *GroundMover) bool {
	if y.traffic == nil || m == nil || y.now == nil {
		return false
	}
	path, s := m.Path(), m.Pose().Distance
	var ahead []airport.LatLon
	for d := s; d <= math.Min(path.Length(), s+VehicleLookMeters); d += vehicleLookStep {
		ahead = append(ahead, path.PointAt(d))
	}
	now := y.now()
	at, vehicle, ok := y.traffic.VehicleConflict(y.own, y.self, ahead, vehicleHalfMeters, now.Before(y.ignoreUntil), now)
	if ok && vehicle {
		if y.waitFrom.IsZero() {
			y.waitFrom = now
		}
		if now.Sub(y.waitFrom) >= VehicleWaitMax {
			y.ignoreUntil, y.waitFrom = now.Add(vehicleIgnoreFor), time.Time{}
			at, _, ok = y.traffic.VehicleConflict(y.own, y.self, ahead, vehicleHalfMeters, true, now)
		}
	} else {
		y.waitFrom = time.Time{}
	}
	if ok {
		m.SetTrafficStop(s + math.Max(0, at-VehicleStopShortMeters))
		y.waiting = true
		return true
	}
	m.ClearTrafficStop()
	y.waiting = false
	return false
}

// lane is where a vehicle on mover m (nil: not on its way) is shown:
// VehicleLaneMeters right of its way, blended in over
// vehicleLaneBlendMeters at both ends.
func lane(pose GroundPose, m *GroundMover) GroundPose {
	if m == nil {
		return pose
	}
	s, l := m.Pose().Distance, m.Path().Length()
	k := math.Max(0, math.Min(1, math.Min(s, l-s)/vehicleLaneBlendMeters))
	if k > 0 {
		pose.Position = offsetHeading(pose.Position, pose.Heading+90, VehicleLaneMeters*k)
	}
	return pose
}

// report puts the vehicle, shown at pose, into the ground picture.
func (y *vehicleYield) report(id uint32, pose GroundPose, length float64) {
	if y.traffic == nil || y.now == nil || id == 0 {
		return
	}
	y.self = id
	y.traffic.ReportVehicle(id, pose.Position, pose.Heading, length, y.now())
}

// forget takes the vehicle out of the ground picture.
func (y *vehicleYield) forget(id uint32) {
	if y.traffic != nil {
		y.traffic.ForgetVehicle(id)
	}
}
