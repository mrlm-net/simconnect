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

// VehicleTraffic is the aircraft a service vehicle gives way to:
// GroundPicture.
type VehicleTraffic interface {
	// VehicleConflict returns how far along ahead (points every
	// vehicleLookStep meters from the vehicle's front) the first conflict
	// with a moving aircraft other than own is; ok false when none.
	VehicleConflict(own uint32, ahead []airport.LatLon, half float64, now time.Time) (at float64, ok bool)
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
func (p *GroundPicture) VehicleConflict(own uint32, ahead []airport.LatLon, half float64, now time.Time) (float64, bool) {
	p.mu.Lock()
	var others []groundEntry
	for id, e := range p.aircraft {
		moving := e.pushing || len(e.ahead) > 0 && !e.waiting
		if id != own && moving && now.Sub(e.at) <= TrafficStaleAfter {
			others = append(others, e)
		}
	}
	p.mu.Unlock()
	for i, q := range ahead {
		d := float64(i) * vehicleLookStep
		for _, e := range others {
			oh := e.half
			if oh <= 0 {
				oh = DefaultHalfSpanMeters
			}
			reach := half + oh + VehicleClearMeters
			for b := -e.tail; b <= e.nose+0.01; b += trafficBodyStep {
				if localDist(q, offsetHeading(e.pos, e.hdg, b)) <= reach {
					return d, true
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
					return d, true
				}
			}
		}
	}
	return 0, false
}

// vehicleYield holds a vehicle's mover short of the first conflict ahead
// on its way, or lets it go on.
type vehicleYield struct {
	traffic VehicleTraffic
	own     uint32
	now     func() time.Time
	waiting bool
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
	if at, ok := y.traffic.VehicleConflict(y.own, ahead, vehicleHalfMeters, y.now()); ok {
		m.SetTrafficStop(s + math.Max(0, at-VehicleStopShortMeters))
		y.waiting = true
		return true
	}
	m.ClearTrafficStop()
	y.waiting = false
	return false
}
