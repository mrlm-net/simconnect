package traffic

import (
	"errors"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Rescue vehicles (the emergency add-on, part 1): fire and rescue trucks
// for an aircraft in an emergency. Each drives from the nearest vehicle
// depot (the fire station's stand-in) along the vehicle roads to its
// standby point beside the landing runway and waits; once the aircraft is
// down it follows it, RescueGapMeters behind, down the runway and to where
// it stops; then it drives home and is removed. Driven by the Injector,
// like SimObjectFuelTruck.
type SimObjectRescue struct {
	client engine.Client
	inj    *Injector
	title  string
	reqID  uint32
	// Layout is the airport, for the roads from the depot and back; nil or
	// no road: it appears at its standby point and leaves from where it is.
	Layout *airport.Layout
	// Depot is the fire station it leaves from and goes home to; nil: the
	// vehicle depot nearest its standby point (#1587).
	Depot *airport.LatLon
	// After is the vehicle leaving before it: from the same depot it
	// appears only once After is RescueDepotGapMeters out, so they never
	// stand on one another there; nil or another depot, at once.
	After *SimObjectRescue

	mu       sync.Mutex
	pending  bool // waiting at the depot for After to move off
	init     types.SIMCONNECT_DATA_INITPOSITION
	objectID uint32
	pose     GroundPose
	mover    *GroundMover // driving to standby or home; nil waiting or following
	homing   bool
	depot    airport.LatLon
	hasDepot bool
	kts      float64 // following: its speed now
	err      error
	done     bool
	entering bool // driving its way onto the runway to the aircraft (Enter)
	// rwyHold is the runway it must not drive onto (HoldOffRunway);
	// holdFor the mover holdAt was found on (-1: no runway on its way).
	rwyHold *airport.Runway
	holdFor *GroundMover
	holdAt  float64
	holding bool
	block   float64 // following: free way ahead before a vehicle that goes first (KeepBehind)
}

// Rescue vehicle figures: how far behind the aircraft it follows, its top
// speed following (on the runway after the touchdown), and how near it
// stops.
const (
	RescueGapMeters  = 80.0
	RescueFollowKts  = 50.0
	RescueStopMeters = 5.0
	rescueAccel      = 2.5  // m/s², up to speed after the aircraft
	rescueTurnDegS   = 30.0 // its quickest turn following
)

// NewSimObjectRescue creates a rescue vehicle of the given ground vehicle
// title; reqID is the SimConnect request ID of its creation.
func NewSimObjectRescue(client engine.Client, inj *Injector, title string, reqID uint32) *SimObjectRescue {
	return &SimObjectRescue{client: client, inj: inj, title: title, reqID: reqID, block: math.Inf(1)}
}

// Standby creates it at the nearest depot and sends it to at, beside the
// runway, where it waits.
func (r *SimObjectRescue) Standby(at GroundPose) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pose = at
	if r.Layout != nil {
		d, ok := airport.LatLon{}, false
		if r.Depot != nil {
			d, ok = *r.Depot, true
		} else {
			d, ok = nearestDepot(r.Layout, at.Position)
		}
		if ok {
			if route, ok := RescueStandbyWay(r.Layout, d, at.Position); ok {
				if path, err := NewArcPath(route, fuelRoadProfile(), 8); err == nil {
					r.mover = NewGroundMoverFrom(path, rescueRoadProfile(), localBearing(path.PointAt(0), path.PointAt(math.Min(5, path.Length()))), 0)
					r.pose, r.depot, r.hasDepot = r.mover.Pose(), d, true
				}
			}
		}
	}
	r.init = types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: r.pose.Position.Lat, Longitude: r.pose.Position.Lon, Heading: r.pose.Heading, OnGround: 1,
	}
	if r.hasDepot && r.After != nil {
		if d, ok := r.After.leftFrom(); ok && localDist(d, r.depot) < RescueDepotGapMeters {
			r.pending = true
			return nil
		}
	}
	return r.client.AICreateSimulatedObject(r.title, r.init, r.reqID)
}

// RescueDepotGapMeters: a vehicle leaving a depot after another appears
// once that one is this far from it.
const RescueDepotGapMeters = 30.0

// leftFrom is the depot it left from, false without one.
func (r *SimObjectRescue) leftFrom() (airport.LatLon, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.depot, r.hasDepot
}

// clearOfDepot reports whether it is out of the way of the next vehicle
// from its depot: RescueDepotGapMeters off where it appeared, or gone.
func (r *SimObjectRescue) clearOfDepot() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done {
		return true
	}
	start := airport.LatLon{Lat: r.init.Latitude, Lon: r.init.Longitude}
	return r.objectID != 0 && !r.pending && localDist(r.pose.Position, start) >= RescueDepotGapMeters
}

// Waiting reports that it waits at its depot for the vehicle ahead to move
// off, not yet in the simulator.
func (r *SimObjectRescue) Waiting() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending && !r.done
}

// Handle takes its creation's answer: the object is driven from then on.
func (r *SimObjectRescue) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return false
	}
	a := msg.AsAssignedObjectID()
	if a == nil || uint32(a.DwRequestID) != r.reqID {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.objectID = uint32(a.DwObjectID)
	r.err = r.inj.Takeover(r.objectID)
	return true
}

// Update drives it on its way (to standby or home) by dt seconds; waiting
// it stays put.
func (r *SimObjectRescue) Update(dt float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending && !r.done && r.After.clearOfDepot() {
		r.pending = false
		return r.client.AICreateSimulatedObject(r.title, r.init, r.reqID)
	}
	if r.done || r.objectID == 0 {
		return nil
	}
	if err := r.err; err != nil {
		r.err = nil
		return err
	}
	if r.mover != nil {
		if !r.entering && !r.homing {
			r.holdOffRunway()
		}
		r.pose = r.mover.Step(dt)
		if r.pose.Arrived {
			r.mover = nil
			if r.homing {
				return r.finish()
			}
		}
	}
	return r.place()
}

// Follow drives it after the aircraft at pose, moving at kts:
// RescueGapMeters behind it, no faster than RescueFollowKts, stopping when
// it stops.
func (r *SimObjectRescue) Follow(pose GroundPose, kts, dt float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending && !r.done && r.After.clearOfDepot() {
		r.pending = false
		return r.client.AICreateSimulatedObject(r.title, r.init, r.reqID)
	}
	if r.done || r.objectID == 0 || r.homing {
		return nil
	}
	if r.mover != nil && r.entering {
		// On its way in by the taxiways and the exit: to its end, or
		// until it is near the aircraft.
		r.pose = r.mover.Step(dt)
		if !r.pose.Arrived && localDist(r.pose.Position, pose.Position) > RescueGapMeters+RescueJoinMeters {
			return r.place()
		}
		r.kts = r.pose.GroundSpeedKts
	}
	r.mover = nil // off its standby: following now
	target := offsetHeading(pose.Position, pose.Heading+180, RescueGapMeters)
	d := localDist(r.pose.Position, target)
	if d <= RescueStopMeters {
		r.kts = 0
		return r.place()
	}
	want := math.Min(RescueFollowKts, math.Max(kts+5, 10))
	// Slowing down toward the target, or behind a vehicle that goes
	// first: no faster than it stops in.
	free := math.Min(d-RescueStopMeters, r.block)
	stop := math.Sqrt(2*rescueAccel*math.Max(0, free)) / knot
	want = math.Min(want, stop)
	if r.kts < want {
		r.kts = math.Min(want, r.kts+rescueAccel*dt/knot)
	} else {
		r.kts = want
	}
	step := math.Min(d, r.kts*knot*dt)
	// Turning toward it no faster than rescueTurnDegS: no snap of its
	// heading as the aircraft turns off.
	turn := math.Max(-rescueTurnDegS*dt, math.Min(rescueTurnDegS*dt, headingDiff(r.pose.Heading, localBearing(r.pose.Position, target))))
	hdg := math.Mod(r.pose.Heading+turn+360, 360)
	r.pose = GroundPose{Position: offsetHeading(r.pose.Position, hdg, step), Heading: hdg, GroundSpeedKts: r.kts}
	return r.place()
}

// Home sends it back to its depot (removed there), or removes it where it
// is without one.
func (r *SimObjectRescue) Home() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.homing {
		return nil
	}
	r.homing = true
	if r.objectID == 0 {
		r.done = true
		return nil
	}
	if r.Layout != nil && r.hasDepot {
		// From its nose, where the mover takes a path to start: from its
		// reference point it went back a wheelbase in a frame (worldtest,
		// the MyCrew app: 5 m as it left the stopped aircraft).
		nose := NoseGear(r.pose.Position, r.pose.Heading, rescueRoadProfile())
		if route, err := r.Layout.VehicleRouteJoin(nose, r.depot, true, false); err == nil && len(route) >= 2 {
			if path, err := NewArcPath(append([]airport.LatLon{nose}, route...), fuelRoadProfile(), 8); err == nil {
				r.mover = NewGroundMoverFrom(path, rescueRoadProfile(), r.pose.Heading, 0)
				return nil
			}
		}
	}
	return r.finish()
}

// Track is where it is and, driving to its standby or home, the way still
// ahead: the path's points past the mover's distance along it (#1622: not
// past the path point nearest it, which on a way doubling back is one
// near its end); false before it is in the simulator or once gone.
func (r *SimObjectRescue) Track() (pose GroundPose, route []airport.LatLon, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.objectID == 0 || r.done {
		return GroundPose{}, nil, false
	}
	if r.mover != nil {
		path, s := r.mover.Path(), r.mover.Pose().Distance
		route = []airport.LatLon{r.pose.Position}
		for i, p := range path.pts {
			if path.cum[i] > s {
				route = append(route, p)
			}
		}
	}
	return r.pose, route, true
}

// Done reports that it has gone.
func (r *SimObjectRescue) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}

// Remove takes it away at once.
func (r *SimObjectRescue) Remove() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.objectID == 0 {
		r.done = true
		return nil
	}
	return r.finish()
}

func (r *SimObjectRescue) place() error {
	err := r.inj.PlaceMoving(r.objectID, r.pose)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

func (r *SimObjectRescue) finish() error {
	r.done = true
	obj := r.objectID
	r.inj.Forget(obj)
	return r.client.AIRemoveObject(obj, r.reqID)
}

// rescueRoadProfile drives it on the roads, faster than a fuel truck: on
// its way to an emergency.
func rescueRoadProfile() MotionProfile {
	p := fuelRoadProfile()
	p.CruiseKts = 35
	return p
}

// RescueJoinMeters: on its way in, a vehicle this much farther than
// RescueGapMeters from the aircraft leaves its way and drives to its
// place by it.
const RescueJoinMeters = 20.0

// Enter sends it from where it is (at standby) to the aircraft by way:
// along the taxiways to the exit or entry, onto the runway by it, then on
// as Follow drives it (#1597). Without a layout or a way by the taxiways,
// Follow drives it straight.
func (r *SimObjectRescue) Enter(way RescueWay) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.homing {
		return nil
	}
	r.entering, r.mover = true, nil
	if r.Layout == nil {
		return nil
	}
	nose := NoseGear(r.pose.Position, r.pose.Heading, rescueRoadProfile())
	route, err := r.Layout.VehicleRouteJoin(nose, way.Node, true, true)
	if err != nil || len(route) < 1 {
		return nil
	}
	pts := append(append([]airport.LatLon{nose}, route...), way.Onto...)
	if path, err := NewArcPath(dedupeClose(pts), fuelRoadProfile(), 8); err == nil && path.Length() > 1 {
		r.mover = NewGroundMoverFrom(path, rescueRoadProfile(), r.pose.Heading, r.pose.GroundSpeedKts)
	}
	return nil
}

// dedupeClose drops points within half a metre of the one before.
func dedupeClose(pts []airport.LatLon) []airport.LatLon {
	out := pts[:0:0]
	for _, p := range pts {
		if len(out) > 0 && localDist(out[len(out)-1], p) < 0.5 {
			continue
		}
		out = append(out, p)
	}
	return out
}

// HoldOffRunway keeps it off rwy on its way to standby or home (the
// aircraft is landing on it): it stops rescueRunwayStopMeters short of
// the runway, RescueRunwayMarginMeters beside its edge, where its way
// crosses it, unless already on it. nil lets it go on.
func (r *SimObjectRescue) HoldOffRunway(rwy *airport.Runway) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rwy == nil {
		r.rwyHold = nil
		return
	}
	c := *rwy
	r.rwyHold = &c
}

// Runway hold figures: the runway's edge widened by
// RescueRunwayMarginMeters counts as on it; a vehicle stops
// rescueRunwayStopMeters before that.
const (
	RescueRunwayMarginMeters = 10.0
	rescueRunwayStopMeters   = 15.0
)

// holdOffRunway applies HoldOffRunway to the mover. r.mu held.
func (r *SimObjectRescue) holdOffRunway() {
	if r.rwyHold == nil || r.Layout == nil {
		if r.holding {
			r.mover.ClearHold()
			r.holding = false
		}
		return
	}
	s := r.mover.Pose().Distance
	if r.holdFor != r.mover {
		r.holdFor, r.holdAt = r.mover, -1
		path := r.mover.Path()
		for d := s; d <= path.Length(); d += 2 {
			if rw, on := r.Layout.RunwayAt(path.PointAt(d), RescueRunwayMarginMeters); on && rw.Index == r.rwyHold.Index {
				r.holdAt = d
				break
			}
		}
	}
	if r.holdAt >= 0 && r.holdAt-rescueRunwayStopMeters > s {
		r.mover.HoldAt(r.holdAt - rescueRunwayStopMeters)
		r.holding = true
	}
}

// KeepBehind keeps it RescueSpacingMeters behind the vehicles at ahead
// (those that go first): on its way, stopping behind one standing on it;
// following, slowing behind one ahead of it.
func (r *SimObjectRescue) KeepBehind(ahead []airport.LatLon) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.mover != nil {
		path := r.mover.Path()
		s := r.mover.Pose().Distance
		stop := math.Inf(1)
		for _, q := range ahead {
			if along, off := path.DistanceTo(q); off < rescueLaneMeters && along > s {
				stop = math.Min(stop, along-RescueSpacingMeters)
			}
		}
		if math.IsInf(stop, 1) {
			r.mover.ClearTrafficStop()
		} else {
			r.mover.SetTrafficStop(stop)
		}
		r.block = math.Inf(1)
		return
	}
	r.block = math.Inf(1)
	for _, q := range ahead {
		d := localDist(r.pose.Position, q)
		if math.Abs(headingDiff(r.pose.Heading, localBearing(r.pose.Position, q))) < rescueAheadDeg {
			r.block = math.Min(r.block, d-RescueSpacingMeters)
		}
	}
}

// rescueLaneMeters: a vehicle this close to the way of another is on it;
// rescueAheadDeg: following, one within this of its heading is ahead.
const (
	rescueLaneMeters = 8.0
	rescueAheadDeg   = 45.0
)
