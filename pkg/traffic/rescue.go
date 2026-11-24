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
	// Search runs its route searches (#1670): nil, at once, on the
	// goroutine that asks; else as Search runs them (off the goroutine
	// that drives it), what one finds put in place by the first frame
	// (Update, Follow) after it ends. Meanwhile it stands, or drives on as
	// it was.
	Search func(func())

	// found is its search running, or ended and not yet put in place.
	found *rescueFound

	mu       sync.Mutex
	pending  bool // waiting at the depot for After to move off
	init     types.SIMCONNECT_DATA_INITPOSITION
	objectID uint32
	// aircraft is where the one it was entering to was last (Follow).
	aircraft *airport.LatLon
	// convoy is the crew it left with (After). laneStop: where it stops
	// behind one on its way (KeepBehind), applied with the convoy's.
	convoy   *rescueConvoy
	laneStop float64
	// homeAtStop: sent home rolling, following or with no way home from
	// where it can brake to; it brakes to a stand at homeDecel and goes
	// home, or is taken away, from standing.
	homeAtStop bool
	homeDecel  float64
	// depotKnown: no way from its depot out, so it appeared at its standby
	// point; it still looks for one home (#1656).
	depotKnown bool
	// enterWay: sent in as it drove, it takes this way once it has braked
	// to a stand (Enter). runwayStop: where it brakes to held off a runway
	// nearer than HoldAt can stop it.
	enterWay   *RescueWay
	runwayStop float64
	// body is the aircraft it attends, of bodyProf (Attend).
	body     *GroundPose
	bodyProf MotionProfile

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
	return &SimObjectRescue{client: client, inj: inj, title: title, reqID: reqID, block: math.Inf(1), laneStop: math.Inf(1), runwayStop: math.Inf(1)}
}

// Attend tells it where the aircraft it attends is and how big (zero: the
// default profile): its way home keeps off that aircraft's body (#1643).
// Without it, it relies on RescueGapMeters.
func (r *SimObjectRescue) Attend(aircraft GroundPose, p MotionProfile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p == (MotionProfile{}) {
		p = DefaultMotionProfile()
	}
	if r.body == nil {
		r.body = new(GroundPose)
	}
	*r.body, r.bodyProf = aircraft, p
}

// Standby creates it at the nearest depot and sends it to at, beside the
// runway, where it waits.
func (r *SimObjectRescue) Standby(at GroundPose) error {
	return r.PlanStandby(at)()
}

// PlanStandby is Standby in two (#1670): it looks for its way from the
// depot to at without its lock held, so on any goroutine, and returns the
// step that creates it and sends it on that way, for the goroutine that
// drives it. Layout and Depot are set before; After is read by the step.
func (r *SimObjectRescue) PlanStandby(at GroundPose) func() error {
	r.mu.Lock()
	l, depot := r.Layout, r.Depot
	r.mu.Unlock()
	d, ok := airport.LatLon{}, false
	var mover *GroundMover
	if l != nil {
		if depot != nil {
			d, ok = *depot, true
		} else {
			d, ok = nearestDepot(l, at.Position)
		}
		if ok {
			route, found := RescueStandbyWay(l, d, at.Position)
			if !found {
				route, found = rescueStandbyOffWay(l, d, at.Position)
			}
			if found {
				if path, err := rescuePath(cleanWay(route), fuelRoadProfile()); err == nil {
					mover = NewGroundMoverFrom(path, rescueRoadProfile(), localBearing(path.PointAt(0), path.PointAt(math.Min(5, path.Length()))), 0)
				}
			}
		}
	}
	return func() error {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.standby(at, d, ok, mover)
	}
}

// standby creates it, its depot d (ok: one) and its way from there mover
// (nil: none, it appears at at). r.mu held.
func (r *SimObjectRescue) standby(at GroundPose, d airport.LatLon, ok bool, mover *GroundMover) error {
	r.pose = at
	if r.After != nil {
		r.convoy = r.After.crew()
	}
	if r.convoy == nil {
		r.convoy = &rescueConvoy{}
	}
	if ok {
		r.depot, r.depotKnown = d, true
		if mover != nil {
			r.mover = mover
			r.pose, r.hasDepot, r.depotKnown = r.mover.Pose(), true, false
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
	return r.drive(dt)
}

// drive is Update. r.mu held.
func (r *SimObjectRescue) drive(dt float64) error {
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
	if err := r.settle(); err != nil {
		return err
	}
	if r.mover != nil && r.enterWay != nil {
		r.enterBraking(dt)
		return r.place()
	}
	if r.mover != nil {
		if !r.homing {
			r.holdOffRunway()
		}
		if r.homeAtStop {
			r.brake(dt, r.homeDecel)
		} else {
			r.applyStop()
			r.pose = r.mover.Step(dt)
		}
		if r.homeAtStop {
			if r.mover.v == 0 {
				r.homeAtStop, r.mover = false, nil
				if err := r.homeStanding(); err != nil || r.done {
					return err
				}
			}
		} else if r.pose.Arrived {
			r.mover = nil
			// Home: unless at the end of the way it was on as it was sent
			// home, its way home still looked for (#1670).
			if r.homing && r.found == nil {
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
	if err := r.settle(); err != nil {
		return err
	}
	if r.found != nil && (r.mover != nil || r.pose.GroundSpeedKts == 0) {
		// Its way in still looked for: on as it was, or standing (#1670).
		// Following already, it follows on.
		return r.drive(dt)
	}
	if r.mover != nil && r.enterWay != nil {
		r.enterBraking(dt)
		return r.place()
	}
	if r.mover != nil && r.entering {
		if r.aircraft == nil {
			r.aircraft = new(airport.LatLon)
		}
		*r.aircraft = pose.Position
		// On its way in by the taxiways and the exit: to its end, or
		// until it is near the aircraft.
		r.holdOffRunway()
		r.applyStop()
		m := r.mover
		// As it comes near, no faster than it stops in before its gap.
		room := localDist(r.pose.Position, pose.Position) - RescueGapMeters
		if m.v > math.Max(0, kts)*knot+math.Sqrt(2*rescueAccel*math.Max(0, room)) {
			r.brake(dt, rescueAccel)
		} else {
			r.pose = m.Step(dt)
		}
		held := r.rwyHold != nil && r.holdFor == m && r.holdAt >= 0 && m.s < r.holdAt
		if !r.pose.Arrived && (held || localDist(r.pose.Position, pose.Position) > RescueGapMeters+RescueJoinMeters) {
			return r.place()
		}
		// The stop it had on its way goes on as its room ahead.
		r.block = math.Min(r.block, r.laneStop-m.s)
		// Moved on its way this frame: following from the next. Both in
		// one frame, it jumped twice its step.
		r.mover = nil
		r.kts = r.pose.GroundSpeedKts
		return r.place()
	}
	r.mover = nil // off its standby: following now
	// From the speed it has, whatever drove it last (#1643).
	r.kts = r.pose.GroundSpeedKts
	target := offsetHeading(pose.Position, pose.Heading+180, RescueGapMeters)
	d := localDist(r.pose.Position, target)
	off := headingDiff(r.pose.Heading, localBearing(r.pose.Position, target))
	// At its place, or just past it (the place moved): it brakes straight
	// on and waits rather than turn back round to it.
	there := d <= RescueStopMeters || d < RescueGapMeters && math.Abs(off) > 90
	if there {
		off = 0
		if r.kts == 0 {
			return r.place()
		}
	}
	want := math.Min(RescueFollowKts, math.Max(kts+5, 10))
	if kts < 1 && d > RescueJoinMeters {
		// To one standing, not at its place yet: there at its speed on
		// the roads, not at 10 kt down the runway (LROP 08L: 1.6 km from
		// its way in, still 265 m off after five minutes). Following one
		// rolling, no faster than above.
		want = math.Max(want, rescueRoadProfile().CruiseKts)
	}
	// Slowing down toward the target, or behind a vehicle that goes
	// first: no faster than it stops in.
	free := math.Min(d-RescueStopMeters, r.block)
	if r.entering {
		free = math.Min(free, r.runwayRoom())
	}
	if there {
		free = 0
	}
	stop := math.Sqrt(2*rescueAccel*math.Max(0, free)) / knot
	want = math.Min(want, stop)
	if r.kts < want {
		r.kts = math.Min(want, r.kts+rescueAccel*dt/knot)
	} else {
		r.kts = math.Max(want, r.kts-rescueAccel*dt/knot)
	}
	if r.kts == 0 {
		// Standing (held, or behind one ahead): no turning on the spot.
		r.pose.GroundSpeedKts = 0
		return r.place()
	}
	step := r.kts * knot * dt
	// Turning toward it no faster than rescueTurnDegS: no snap of its
	// heading as the aircraft turns off.
	turn := math.Max(-rescueTurnDegS*dt, math.Min(rescueTurnDegS*dt, off))
	hdg := math.Mod(r.pose.Heading+turn+360, 360)
	r.pose = GroundPose{Position: offsetHeading(r.pose.Position, hdg, step), Heading: hdg, GroundSpeedKts: r.kts}
	return r.place()
}

// Home sends it back to its depot (removed there), or removes it where it
// comes to a stand without one.
func (r *SimObjectRescue) Home() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.homing {
		return nil
	}
	r.homing = true
	r.enterWay = nil
	r.laneStop, r.runwayStop = math.Inf(1), math.Inf(1)
	if r.objectID == 0 {
		r.done = true
		r.convoy.leave(r)
		return nil
	}
	switch {
	case r.mover != nil && r.mover.v > 0:
		r.homeRolling()
		r.tell()
		return nil
	case r.mover == nil && r.pose.GroundSpeedKts > 0 && r.homeFromFollowing():
		r.tell()
		return nil
	}
	return r.homeStanding()
}

// homeStanding sets it off home from where it stands, or takes it away
// without a way there. r.mu held.
func (r *SimObjectRescue) homeStanding() error {
	if r.Layout == nil || !r.hasDepot && !r.depotKnown {
		return r.finish()
	}
	a, from := r.ask(), r.pose
	return r.search(func() func() error {
		path := a.homePath(from)
		return func() error {
			if r.done || !r.homing || r.mover != nil {
				return nil
			}
			if !samePlace(r.pose, from) {
				return r.homeStanding() // moved meanwhile: from where it is
			}
			if path == nil {
				return r.finish()
			}
			r.mover = NewGroundMoverFrom(path, rescueRoadProfile(), from.Heading, 0)
			r.tell()
			return nil
		}
	})
}

// homePath is the way home of one standing at from; nil without one.
func (a rescueAsk) homePath(from GroundPose) *GroundPath {
	// From its nose, where the mover takes a path to start: from its
	// reference point it went back a wheelbase in a frame (worldtest,
	// the MyCrew app: 5 m as it left the stopped aircraft).
	nose := NoseGear(from.Position, from.Heading, rescueRoadProfile())
	route, ok := a.routeHome(nose)
	if !ok {
		return nil
	}
	// Without the spikes of its join and the graph's spurs (#1630
	// (d)), so it turns out where the road truly lies behind it.
	way := cleanWay(append([]airport.LatLon{nose}, route...))
	// From standby or from following alike, off forwards (#1630).
	way = a.homeWayFrom(nose, from.Heading, way)
	path, err := rescuePath(way, fuelRoadProfile())
	if err != nil {
		return nil
	}
	return path
}

// homeWayFrom is the way home from nose, facing h, given the direct
// one: that, unless it sets off backwards or runs over one of its crew
// standing; then out round (homeOut).
func (a rescueAsk) homeWayFrom(nose airport.LatLon, h float64, direct []airport.LatLon) []airport.LatLon {
	direct = a.roundBody(direct)
	back := startsBackwards(direct, h)
	if !back && a.inTheWay(direct) == 0 {
		return direct
	}
	out := a.homeOut(nose, h, direct)
	if !back && a.inTheWay(out) > 0 {
		return direct
	}
	return out
}

// routeHome is the way to its depot from p: by the node in reach, else
// back along the runway, or the nearest path, to one (#1656).
func (a rescueAsk) routeHome(p airport.LatLon) ([]airport.LatLon, bool) {
	route, err := a.layout.VehicleRouteJoin(p, a.depot, true, false)
	if err != nil || len(route) < 2 {
		route, err = a.layout.VehicleRouteBack(p, a.depot, a.keepShortOf()...)
	}
	return route, err == nil && len(route) >= 2
}

// keepShortOf is the aircraft it attends, for a way along the runway.
func (a rescueAsk) keepShortOf() []airport.LatLon {
	if a.body == nil {
		return nil
	}
	return []airport.LatLon{a.body.Position}
}

// roundBody is way round the aircraft it attends.
func (a rescueAsk) roundBody(way []airport.LatLon) []airport.LatLon {
	if a.body == nil || len(way) < 2 {
		return way
	}
	return roundRouteOn(a.layout, way, *a.body, a.bodyProf)
}

// inTheWay is what way runs over: crew standing, the aircraft attended.
func (a rescueAsk) inTheWay(way []airport.LatLon) int {
	n := a.overCrew(way)
	if a.body == nil {
		return n
	}
	f := aircraftFrame{nose: NoseGear(a.body.Position, a.body.Heading, a.bodyProf), hdg: a.body.Heading, prof: a.bodyProf}
	for i := 1; i < len(way); i++ {
		if !f.clear(way[i-1], way[i]) {
			return n + 1
		}
	}
	return n
}

// homeRolling sends it home as it drives, its speed carried on (#1630: it
// set off again from 0 kt, a stop in one frame). It keeps to its way for
// as far as it takes to brake from that speed, no further than that way
// goes, the runway it is held off, the aircraft it was driving to or one
// of its crew ahead (braking harder then, at a rate), and goes home from
// there; without a way from there it stops on the way it is on first
// (homeWhenStopped). r.mu held.
func (r *SimObjectRescue) homeRolling() {
	old := r.mover
	path, s0, v := old.Path(), old.Pose().Distance, old.v
	prof := rescueRoadProfile()
	on := v*v/(2*prof.Decel) + v*SpeedResponseSeconds
	room := path.Length() - s0
	if r.holding && r.holdAt >= 0 {
		room = math.Min(room, r.holdAt-rescueRunwayStopMeters-s0)
	}
	if r.entering && r.aircraft != nil {
		room = math.Min(room, localDist(r.pose.Position, *r.aircraft)-(RescueGapMeters-RescueJoinMeters))
	}
	for _, o := range r.convoy.others(r) {
		// Ahead on its way, as far on as that one needs to stop itself.
		if along, off := path.DistanceTo(o.pos); off < rescueLaneMeters && along > s0 {
			room = math.Min(room, along-s0-rescueBodyRoomMeters+o.v*o.v/(2*prof.Decel))
		}
	}
	road := fuelRoadProfile()
	if room < on {
		on = math.Max(room, 0)
		decel := math.Max(prof.Decel, math.Min(rescueAccel, v*v/(2*math.Max(on, 1))))
		prof.Decel, road.Decel = decel, decel
	}
	if r.Layout == nil || !r.hasDepot && !r.depotKnown {
		r.homeWhenStopped(s0+on, prof.Decel)
		return
	}
	// Where it may leave its way, the first that has a join in reach:
	// with none from there (the middle of a long taxiway), it keeps to
	// its way on to where one is. Read here, searched off the goroutine.
	cuts := []homeCut{homeCutAt(path, s0, on, r.pose.Heading)}
	if r.hasDepot {
		for d := on; d >= cleanWayMeters && d <= room; d += rescueHomeLookMeters {
			cuts = append(cuts, homeCutAt(path, s0, d, r.pose.Heading))
		}
	}
	a := r.ask()
	_ = r.search(func() func() error {
		cut, rest := a.homeFrom(cuts, road)
		return func() error {
			if r.done || !r.homing {
				return nil
			}
			if r.mover != old {
				if r.mover == nil && r.objectID != 0 {
					return r.homeStanding() // at the end of its way meanwhile
				}
				return nil
			}
			end := s0 + cut.on
			if rest == nil {
				r.homeWhenStopped(end, prof.Decel)
				return nil
			}
			// From where it is now, on the way it was on (#1670).
			s := old.Pose().Distance
			kept := []airport.LatLon{path.PointAt(s)}
			if cut.on >= cleanWayMeters {
				if s > end-cleanWayMeters {
					r.homeRolling() // past where it was to leave: looked for again
					return nil
				}
				for i, p := range path.pts {
					if c := path.cum[i]; c > s+0.1 && c < end-0.1 {
						kept = append(kept, p)
					}
				}
				kept = append(kept, cut.at)
			} else if s > s0+0.01 {
				if s-s0 >= cleanWayMeters {
					r.homeRolling()
					return nil
				}
				rest = rest[1:] // not back to where it was
			}
			// Rounded again where the two meet.
			next, err := rescueRounded(append(kept, rest...), road)
			if err != nil {
				r.homeWhenStopped(end, prof.Decel)
				return nil
			}
			r.mover = NewGroundMoverFrom(next, prof, r.pose.Heading, old.v/ktsToMS)
			r.mover.a = old.a
			r.tell()
			return nil
		}
	})
}

// homeCut is where one sent home driving leaves its way: on metres on, at
// facing h, from (on a little, so the corner home is rounded) where its
// way home starts.
type homeCut struct {
	on       float64
	at, from airport.LatLon
	h        float64
}

// homeCutAt is the cut on metres on from s0 along path, facing h there
// when that is less than cleanWayMeters.
func homeCutAt(path *GroundPath, s0, on, h float64) homeCut {
	at := path.PointAt(s0)
	if on >= cleanWayMeters {
		end := s0 + on
		h = localBearing(path.PointAt(end-1), path.PointAt(end))
		at = path.PointAt(end)
	}
	return homeCut{on: on, at: at, from: offsetHeading(at, h, wayTurnRadiusM), h: h}
}

// homeFrom is the way home of one sent home driving: from the first of
// cuts after the first with a join in reach, else the first. nil without
// a way (homeRolling).
func (a rescueAsk) homeFrom(cuts []homeCut, road MotionProfile) (homeCut, []airport.LatLon) {
	cut := cuts[0]
	for _, c := range cuts[1:] {
		if route, err := a.layout.VehicleRouteJoin(c.from, a.depot, true, false); err == nil && len(route) >= 2 {
			cut = c
			break
		}
	}
	route, ok := a.routeHome(cut.from)
	if !ok {
		return cut, nil
	}
	way := a.roundBody(cleanWay(append([]airport.LatLon{cut.from}, route...)))
	if startsBackwards(way, cut.h) || a.inTheWay(way) > 0 {
		way = a.homeOut(cut.at, cut.h, way)
	} else {
		way = append([]airport.LatLon{cut.at}, way...)
	}
	rest, err := NewArcPath(way, road, 8)
	if err != nil {
		return cut, nil
	}
	return cut, rest.pts
}

// homeWhenStopped has it brake to a stand by stop on the way it is on,
// at decel or as much harder as that takes up to rescueAccel, to go home
// from there (homeStanding). r.mu held.
func (r *SimObjectRescue) homeWhenStopped(stop, decel float64) {
	m := r.mover
	if d := stop - m.s; d > 0 {
		decel = math.Max(decel, m.v*m.v/(2*d))
	} else {
		decel = rescueAccel
	}
	r.homeAtStop, r.homeDecel = true, math.Min(decel, rescueAccel)
}

// homeFromFollowing sends it home as it follows: straight on, braking to
// a stand at rescueAccel (Follow drives it no faster than it stops in so
// before its place), then out forwards as from standby (homeStanding).
// False standing. r.mu held.
func (r *SimObjectRescue) homeFromFollowing() bool {
	v, prof := r.pose.GroundSpeedKts*ktsToMS, rescueRoadProfile()
	if v < minRollMS {
		return false
	}
	nose := NoseGear(r.pose.Position, r.pose.Heading, prof)
	far := offsetHeading(nose, r.pose.Heading, v*v/(2*rescueAccel)+prof.WheelbaseMeters)
	path, err := NewGroundPath([]airport.LatLon{nose, far}, prof)
	if err != nil {
		return false
	}
	r.mover, r.entering = NewGroundMoverFrom(path, prof, r.pose.Heading, r.pose.GroundSpeedKts), false
	r.homeAtStop, r.homeDecel = true, rescueAccel
	return true
}

// brake moves it on by dt along its way, slowing at decel: no stop point
// halts it from speed (it is taken away from a stand only). r.mu held.
func (r *SimObjectRescue) brake(dt, decel float64) {
	m := r.mover
	next := math.Max(0, m.v-decel*dt)
	m.s = math.Min(m.path.Length(), m.s+(m.v+next)/2*dt)
	if m.s >= m.path.Length() {
		next = 0
	}
	m.v, m.a = next, 0
	m.place()
	r.pose = m.pose
}

// rescuePath is a rescue vehicle's way as a path, its corners rounded.
func rescuePath(way []airport.LatLon, p MotionProfile) (*GroundPath, error) {
	arcs, err := NewArcPath(way, p, 8)
	if err != nil {
		return nil, err
	}
	return rescueRounded(arcs.pts, p)
}

// rescueRounded is pts with every bend eased over rescueRoundMeters each
// way, twice: a corner between short legs, where an arc has no room, is no
// tighter than the wheelbase (#1630: the nose turned square and the
// vehicle behind it stood in one step). The ends stay, straights too.
func rescueRounded(pts []airport.LatLon, p MotionProfile) (*GroundPath, error) {
	g, err := NewSmoothPath(pts, p)
	if err != nil {
		return nil, err
	}
	n := max(1, int(math.Ceil(g.Length())))
	at := make([]airport.LatLon, n+1)
	for i := range at {
		at[i] = g.PointAt(g.Length() * float64(i) / float64(n))
	}
	o := at[0]
	for range 2 {
		lat, lon := make([]float64, n+2), make([]float64, n+2)
		for i, q := range at {
			lat[i+1], lon[i+1] = lat[i]+q.Lat-o.Lat, lon[i]+q.Lon-o.Lon
		}
		for i := 1; i < n; i++ {
			k := min(rescueRoundMeters, i, n-i)
			w := float64(2*k + 1)
			at[i] = airport.LatLon{Lat: o.Lat + (lat[i+k+1]-lat[i-k])/w, Lon: o.Lon + (lon[i+k+1]-lon[i-k])/w}
		}
	}
	return NewSmoothPath(at, p)
}

// rescueHomeLookMeters: sent home driving, it looks this far apart along
// its way for a join.
const rescueHomeLookMeters = 25.0

// rescueRoundMeters: how far each way a bend of its way is eased.
const rescueRoundMeters = 6

// overCrew is how many of its crew standing the first rescueCrewLookMeters
// of way run over.
func (a rescueAsk) overCrew(way []airport.LatLon) int {
	n := 0
	for _, o := range a.convoy.others(a.r) {
		if o.v > 0 {
			continue
		}
		run := 0.0
		for i := 1; i < len(way) && run < rescueCrewLookMeters; i++ {
			a, b := way[i-1], way[i]
			d := localDist(a, b)
			if left := rescueCrewLookMeters - run; d > left {
				b = offsetHeading(a, localBearing(a, b), left)
			}
			run += d
			if pointSegDist(o.pos, a, b) < rescueLaneMeters {
				n++
				break
			}
		}
	}
	return n
}

// Standing reports where it is when it stands at its standby point: in
// the simulator, not driving, following, entering nor going home.
func (r *SimObjectRescue) Standing() (GroundPose, uint32, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.standing() {
		return GroundPose{}, 0, false
	}
	return r.pose, r.objectID, true
}

// standing is whether it stands at its standby point. r.mu held.
func (r *SimObjectRescue) standing() bool {
	return r.objectID != 0 && !r.done && !r.pending && !r.homing && !r.entering && r.mover == nil
}

// PlanMove plans its drive, standing (Standing), to another standby point
// at by the roads and taxiways (#1654: off the way an arrival vacates by,
// planned after it took its point), without its lock held during the
// search, so on any goroutine: nil when it is not standing or there is no
// way, or none without turning back on itself (with at behind it, it turns
// out to at's side first), else the step that sends it, for the goroutine
// that drives it (false: it set off or was moved meanwhile).
func (r *SimObjectRescue) PlanMove(at GroundPose) func() bool {
	r.mu.Lock()
	if !r.standing() || r.Layout == nil {
		r.mu.Unlock()
		return nil
	}
	from, l := r.pose, r.Layout
	r.mu.Unlock()
	// From its nose the way it faces (no turn back on the spot), ending
	// where the way first passes the point, as on its way out.
	nose := NoseGear(from.Position, from.Heading, rescueRoadProfile())
	route, err := l.VehicleRoutePose(nose, from.Heading, at.Position, noHeading)
	if err != nil || len(route) < 2 {
		return nil
	}
	for i := 1; i < len(route); i++ {
		if pointSegDist(at.Position, route[i-1], route[i]) <= rescueOnWayMeters {
			route = route[:i:i]
			break
		}
	}
	// Its nose on past at by its length there: the vehicle itself stands
	// at it, not short of it, back on the way it left.
	way := cleanWay(append(append([]airport.LatLon{nose}, route...), at.Position))
	h := from.Heading
	if len(way) > 1 && startsBackwards(way, h) {
		// Behind it: out to its side (turnOut) and straight to it.
		if localDist(nose, at.Position) > rescueMoveDirectMeters {
			return nil
		}
		side := math.Copysign(90, headingDiff(h, localBearing(nose, at.Position)))
		way = append(turnOut(nose, h, side), at.Position)
	}
	if len(way) < 2 || wayTurnsBack(way) {
		return nil
	}
	n := len(way)
	way[n-1] = offsetHeading(at.Position, localBearing(way[n-2], at.Position), localDist(nose, from.Position))
	path, err := rescuePath(way, fuelRoadProfile())
	if err != nil {
		return nil
	}
	return func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.standing() || r.pose.Position != from.Position || r.pose.Heading != from.Heading {
			return false
		}
		r.mover = NewGroundMoverFrom(path, rescueRoadProfile(), from.Heading, 0)
		return true
	}
}

// startsBackwards reports whether way sets off more than a quarter turn
// from heading h: a mover put on it reverses, then swings round.
func startsBackwards(way []airport.LatLon, h float64) bool {
	for _, p := range way[1:] {
		if localDist(way[0], p) >= cleanWayMeters {
			return math.Abs(headingDiff(h, localBearing(way[0], p))) > 90
		}
	}
	return false
}

// turnOut is a vehicle's way out to its side (90 right, -90 left) from
// nose, facing h: a little ahead only, it may stand nose to nose with an
// aircraft, then across.
func turnOut(nose airport.LatLon, h, side float64) []airport.LatLon {
	a := offsetHeading(nose, h, rescueTurnOutAheadMeters)
	return []airport.LatLon{nose, a, offsetHeading(a, h+side, 2*wayTurnRadiusM)}
}

// rescueTurnOutAheadMeters: turning out, it drives this far ahead at most.
const rescueTurnOutAheadMeters = 5.0

// homeOut is the way home from standby at nose, facing h, where the road
// to the depot lies behind it (#1630: it reversed a few metres, then
// swung round). It turns out to the side and, if need be, on round, and
// joins the road from there, by the shortest way that does
// not turn back. direct is the way it could not take.
func (a rescueAsk) homeOut(nose airport.LatLon, h float64, direct []airport.LatLon) []airport.LatLon {
	side := turnSide(nose, h, direct)
	var best, round []airport.LatLon
	bestC, roundHits := math.Inf(1), math.MaxInt
	for _, s := range []float64{side, -side} {
		for _, loop := range []bool{false, true} {
			pts := turnOut(nose, h, s)
			endH := h + s
			if loop {
				endH = h + 180
				pts = append(pts, offsetHeading(pts[2], endH, 2*wayTurnRadiusM))
			}
			on, err := a.layout.VehicleRoutePose(pts[len(pts)-1], endH, a.depot, noHeading)
			if err != nil || len(on) < 2 {
				on, err = a.layout.VehicleRouteBack(pts[len(pts)-1], a.depot, a.keepShortOf()...)
			}
			if err != nil || len(on) < 2 {
				continue
			}
			way := a.roundBody(append(pts[:len(pts)-1:len(pts)-1], cleanWay(on)...))
			hits := a.inTheWay(way)
			if loop && hits < roundHits {
				round, roundHits = way, hits
			}
			if c := wayCost(way, noHeading) + float64(hits)*rescueOverCrewCost; c < bestC && !wayTurnsBack(way) {
				best, bestC = way, c
			}
		}
	}
	if best != nil {
		return best
	}
	if round != nil {
		// Every one turns back further on: still never off backwards.
		return round
	}
	on := direct[1:]
	for len(on) > 1 && localDist(nose, on[0]) < cleanWayMeters {
		on = on[1:]
	}
	pts := turnOut(nose, h, side)
	return a.roundBody(append(append(pts, offsetHeading(pts[2], h+180, 2*wayTurnRadiusM)), on...))
}

// turnSide is the side (90 right, -90 left) of nose, facing h, that way
// sets off to.
func turnSide(nose airport.LatLon, h float64, way []airport.LatLon) float64 {
	for _, p := range way[1:] {
		if localDist(nose, p) >= cleanWayMeters {
			if headingDiff(h, localBearing(nose, p)) < 0 {
				return -90
			}
			break
		}
	}
	return 90
}

// turnRound is way from nose, facing h, setting off behind it, turned out
// forwards: to its side and, the way still behind, on round.
func turnRound(nose airport.LatLon, h float64, way []airport.LatLon) []airport.LatLon {
	side := turnSide(nose, h, way)
	pts := turnOut(nose, h, side)
	on := way[1:]
	for len(on) > 1 && localDist(nose, on[0]) < 2*wayTurnRadiusM {
		on = on[1:]
	}
	if startsBackwards(append([]airport.LatLon{pts[2]}, on...), h+side) {
		pts = append(pts, offsetHeading(pts[2], h+180, 2*wayTurnRadiusM))
	}
	return append(pts, on...)
}

// rescueMoveDirectMeters: turned out, it drives straight over the
// pavement to a standby point this near at most (as far as one may join
// the roads and taxiways).
const rescueMoveDirectMeters = 150.0

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
	r.tell()
	err := r.inj.PlaceMoving(r.objectID, r.pose)
	if errors.Is(err, ErrGroundUnknown) {
		return nil
	}
	return err
}

func (r *SimObjectRescue) finish() error {
	r.done = true
	r.convoy.leave(r)
	obj := r.objectID
	r.inj.Forget(obj)
	return r.client.AIRemoveObject(obj, r.reqID)
}

// RescueBodyProfile is a rescue vehicle's body, for the ground picture
// (#1632: one standing on an arrival's way off the runway).
func RescueBodyProfile() MotionProfile { return rescueRoadProfile() }

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
// Follow drives it straight, from the speed it has. Driving, it first
// brakes to a stand on the way it is on (#1643).
func (r *SimObjectRescue) Enter(way RescueWay) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.homing {
		return nil
	}
	r.entering, r.enterWay = true, nil
	r.enterFrom(way)
	return nil
}

// enterFrom looks for its way in by way from where it is (Enter). r.mu
// held.
func (r *SimObjectRescue) enterFrom(way RescueWay) {
	l, from, m := r.Layout, r.pose, r.mover
	rolling := m != nil && m.v > 0
	cut, h, run := airport.LatLon{}, 0.0, 0.0
	if rolling {
		// Straight on past its way's end as far as braking at rescueAccel
		// takes before it turns: turned at speed, it lost speed in a step.
		cut, h = rescueWayEnd(m.Path())
		run = m.v*m.v/(2*rescueAccel) - (m.path.Length() - m.s)
	}
	_ = r.search(func() func() error {
		next := rescueEnterMover(l, from, way)
		var rest []airport.LatLon
		if rolling && next != nil {
			rest = rescueEnterRest(l, cut, h, run, way)
		}
		return func() error {
			if r.done || r.homing || !r.entering {
				return nil
			}
			switch {
			case next == nil:
				// No way by the taxiways: Follow drives it straight.
				r.mover = nil
			case r.mover != nil && r.mover.v > 0 && !r.canStop():
				// Too fast to stop on what is left of its way: in from its
				// end, rolling; with no way on from there, Follow drives
				// it straight.
				if r.mover == m {
					r.mover = rescueRollIn(m, rest, r.pose.Heading)
				} else {
					r.mover = nil
				}
			case r.mover != nil && r.mover.v > 0:
				r.enterWay = &way
			case r.mover == nil && r.pose.GroundSpeedKts > 0:
				// Following already: Follow takes it on.
			case !samePlace(r.pose, from):
				r.enterFrom(way) // came to a stand meanwhile: from there
			default:
				r.mover = next
			}
			return nil
		}
	})
}

// rescueEnterMover drives one at pose on l by way, its corners rounded
// and off forwards as its other ways (#1643: a corner tighter than the
// wheelbase was a step in its speed); nil without a way. No lock.
func rescueEnterMover(l *airport.Layout, pose GroundPose, way RescueWay) *GroundMover {
	if l == nil {
		return nil
	}
	h := pose.Heading
	nose := NoseGear(pose.Position, h, rescueRoadProfile())
	route, err := l.VehicleRouteJoin(nose, way.Node, true, true)
	if err != nil || len(route) < 1 {
		return nil
	}
	pts := cleanWay(append(append([]airport.LatLon{nose}, route...), way.Onto...))
	if startsBackwards(pts, h) {
		pts = turnRound(nose, h, pts)
	}
	// At its speed on the roads to an emergency (rescueRoadProfile), not
	// a fuel truck's 12 kt: LKPR 12's 2 km way in took over five minutes.
	path, err := rescuePath(pts, rescueRoadProfile())
	if err != nil || path.Length() <= 1 {
		return nil
	}
	return NewGroundMoverFrom(path, rescueRoadProfile(), h, 0)
}

// enterBraking brakes it on the way it was on, then takes the way in
// (Enter). r.mu held.
func (r *SimObjectRescue) enterBraking(dt float64) {
	m := r.mover
	// At what it can brake (Enter took the way in rolling where that is
	// too little for what is left of its way): no stop in a frame.
	r.brake(dt, rescueAccel)
	if m.v == 0 && r.found == nil {
		l, from, way := r.Layout, r.pose, *r.enterWay
		_ = r.search(func() func() error {
			next := rescueEnterMover(l, from, way)
			return func() error {
				if r.done || r.homing || r.enterWay == nil || r.mover != m {
					return nil
				}
				r.mover, r.enterWay = next, nil
				return nil
			}
		})
	}
}

// canStop reports whether it stops on what is left of its way braking at
// rescueAccel. r.mu held.
func (r *SimObjectRescue) canStop() bool {
	m := r.mover
	return m.v*m.v/(2*rescueAccel) <= m.path.Length()-m.s
}

// rescueWayEnd is the end of path and the heading there.
func rescueWayEnd(path *GroundPath) (airport.LatLon, float64) {
	end := path.Length()
	return path.PointAt(end), localBearing(path.PointAt(math.Max(0, end-1)), path.PointAt(end))
}

// rescueEnterRest is the way in by way from cut, facing h, at the end of
// the way one drives on: straight on for run metres (none at 0 or less),
// then by the taxiways from a little on, round first where they lie
// behind; nil without one. No lock.
func rescueEnterRest(l *airport.Layout, cut airport.LatLon, h, run float64, way RescueWay) []airport.LatLon {
	if l == nil {
		return nil
	}
	from := cut
	if run > 0 {
		from = offsetHeading(cut, h, run)
	}
	route, err := l.VehicleRouteJoin(offsetHeading(from, h, wayTurnRadiusM), way.Node, true, true)
	if err != nil || len(route) < 1 {
		return nil
	}
	pts := cleanWay(append(append([]airport.LatLon{from}, route...), way.Onto...))
	if rescueBacksOff(pts, h) {
		pts = turnRound(from, h, pts)
	}
	if run > 0 {
		pts = append([]airport.LatLon{cut}, pts...)
	}
	arcs, err := NewArcPath(pts, fuelRoadProfile(), 8)
	if err != nil {
		return nil
	}
	return arcs.pts
}

// rescueBacksOff reports whether way, facing h at its start, lies behind
// it or turns back (wayTurnsBack) within its first wayTurnBackM: rolling
// in, a hairpin there is no way to take (LKPR 12: 10 m on, then 142° back
// up the road it came down, a cusp its speed fell in).
func rescueBacksOff(way []airport.LatLon, h float64) bool {
	if startsBackwards(way, h) {
		return true
	}
	n, d := 1, 0.0
	for n < len(way) && d < wayTurnBackM {
		d += localDist(way[n-1], way[n])
		n++
	}
	return wayTurnsBack(way[:n])
}

// rescueRollInJerk (m/s³): rolling in, it is braking at rescueAccel within
// half a second.
const rescueRollInJerk = 2 * rescueAccel

// rescueRollIn drives one on m, facing h, on along m's way to its end and
// by rest from there, the two rounded where they meet, from its speed and
// braking no harder than rescueAccel (as homeRolling); nil without rest.
func rescueRollIn(m *GroundMover, rest []airport.LatLon, h float64) *GroundMover {
	if len(rest) == 0 {
		return nil
	}
	path, s := m.Path(), m.Pose().Distance
	kept := []airport.LatLon{path.PointAt(s)}
	for i, p := range path.pts {
		if c := path.cum[i]; c > s+0.1 && c < path.Length()-0.1 {
			kept = append(kept, p)
		}
	}
	road, prof := rescueRoadProfile(), rescueRoadProfile()
	road.Decel, prof.Decel = rescueAccel, rescueAccel
	// Into its braking at once, as its run on past the way's end counts
	// on: at the road jerk it braked at 1 m/s² by the turn, still at
	// 12 m/s, and the turn took its speed in a step.
	prof.Jerk = math.Max(prof.Jerk, rescueRollInJerk)
	next, err := rescueRounded(append(kept, rest...), road)
	if err != nil {
		return nil
	}
	mv := NewGroundMoverFrom(next, prof, h, m.v/ktsToMS)
	mv.a = m.a
	return mv
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
		// Found again when held again.
		r.holdFor, r.runwayStop = nil, math.Inf(1)
		return
	}
	s := r.mover.Pose().Distance
	if r.holdFor != r.mover {
		r.holdFor, r.holdAt, r.runwayStop = r.mover, -1, math.Inf(1)
		r.holding = false
		path := r.mover.Path()
		for d := s; d <= path.Length(); d += 2 {
			if rw, on := r.Layout.RunwayAt(path.PointAt(d), RescueRunwayMarginMeters); on && rw.Index == r.rwyHold.Index {
				r.holdAt = d
				break
			}
		}
	}
	if r.holdAt < 0 || !math.IsInf(r.runwayStop, 1) {
		return
	}
	at, v := r.holdAt-rescueRunwayStopMeters, r.mover.v
	switch {
	case r.holding:
	case at >= s+v*v/(2*r.mover.p.Decel)+v*SpeedResponseSeconds:
		r.mover.HoldAt(at)
		r.holding = true
	case s < r.holdAt:
		// Too near for HoldAt, which halts dead reached at speed: braked to
		// as to a stop behind one ahead (applyStop, #1643).
		r.runwayStop = at
	}
}

// runwayRoom is how far it may go straight on before the runway it is
// held off (entering with no way by the taxiways). r.mu held.
func (r *SimObjectRescue) runwayRoom() float64 {
	if r.rwyHold == nil || r.Layout == nil {
		return math.Inf(1)
	}
	held := func(d float64) bool {
		rw, on := r.Layout.RunwayAt(offsetHeading(r.pose.Position, r.pose.Heading, d), RescueRunwayMarginMeters)
		return on && rw.Index == r.rwyHold.Index
	}
	if held(0) {
		return math.Inf(1)
	}
	v := r.kts * knot
	reach := v*v/(2*rescueAccel) + 2*rescueRunwayStopMeters
	for d := rescueRunwayLookMeters; d <= reach; d += rescueRunwayLookMeters {
		if held(d) {
			return d - rescueRunwayStopMeters
		}
	}
	return math.Inf(1)
}

// rescueRunwayLookMeters: following, it looks this far apart for a runway
// it is held off.
const rescueRunwayLookMeters = 5.0

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
		var sameHome []rescueMember
		if r.homing && !r.homeAtStop {
			for _, o := range r.convoy.others(r) {
				if o.homing && localDist(o.depot, r.depot) < RescueDepotGapMeters {
					sameHome = append(sameHome, o)
				}
			}
		}
	next:
		for _, q := range ahead {
			// Going home to one depot they go in their order there
			// (homeStop), not the crew's, or each waits for the other.
			for _, o := range sameHome {
				if localDist(o.pos, q) < 0.5 {
					continue next
				}
			}
			if along, off := path.DistanceTo(q); off < rescueLaneMeters && along > s {
				stop = math.Min(stop, along-RescueSpacingMeters)
			}
		}
		r.laneStop = stop
		r.applyStop()
		r.block = math.Inf(1)
		return
	}
	r.laneStop = math.Inf(1)
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

// applyStop sets the mover's stop behind the vehicles going first: one on
// its way (KeepBehind), one of its crew ahead of it home (homeStop). Found
// too near to stop there it brakes at a rate, the stop moved on to what
// braking at rescueAccel takes, for one on its way no nearer than
// rescueBodyRoomMeters. r.mu held.
func (r *SimObjectRescue) applyStop() {
	if r.mover == nil {
		return
	}
	home := r.homeStop()
	stop := math.Min(math.Min(r.laneStop, home), r.runwayStop)
	if math.IsInf(stop, 1) {
		r.mover.ClearTrafficStop()
		return
	}
	m := r.mover
	// The mover halts dead TrafficOverrunMeters past a stop set too near.
	if to := m.s + m.v*m.v/(2*rescueAccel) - TrafficOverrunMeters; to > stop {
		if !math.IsInf(r.laneStop, 1) {
			to = math.Min(to, r.laneStop+RescueSpacingMeters-rescueBodyRoomMeters-TrafficOverrunMeters)
		}
		stop = math.Max(stop, to)
	}
	m.SetTrafficStop(stop)
}

// homeStop is how far it may go home: the crew go to their depot in the
// order of what is left of their ways there, RescueSpacingMeters apart
// along them, so where the ways join they come one after another (two
// setting off together drove into each other). +Inf: nobody before it.
// r.mu held.
func (r *SimObjectRescue) homeStop() float64 {
	if !r.homing || r.homeAtStop || r.mover == nil || r.convoy == nil {
		return math.Inf(1)
	}
	m, me := r.mover, r.convoy.member(r)
	left := m.path.Length() - m.s
	stop := math.Inf(1)
	for _, o := range r.convoy.others(r) {
		if !o.homing || localDist(o.depot, r.depot) >= RescueDepotGapMeters {
			continue
		}
		if o.left < me.left || o.left == me.left && o.n < me.n {
			stop = math.Min(stop, m.s+left-o.left-RescueSpacingMeters)
		}
	}
	return stop
}

// Going home: room left to one ahead (its tail, this one's nose and some),
// how far ahead a way is looked at for one of the crew standing on it and
// what running over one costs a way (metres).
const (
	rescueBodyRoomMeters = 15.0
	rescueCrewLookMeters = 120.0
	rescueOverCrewCost   = 10000.0
)

// rescueConvoy is what a crew's vehicles know of each other (those joined
// by After). Its lock is taken with a vehicle's held, never the other way.
type rescueConvoy struct {
	mu sync.Mutex
	by map[*SimObjectRescue]rescueMember
	n  int
}

// rescueMember is one as it last was: where, its speed (m/s) and, homing,
// what is left of its way to depot. n: the order it joined in.
type rescueMember struct {
	pos    airport.LatLon
	depot  airport.LatLon
	v      float64
	left   float64
	homing bool
	n      int
}

// crew is the convoy it is in.
func (r *SimObjectRescue) crew() *rescueConvoy {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.convoy
}

// tell records it in its convoy as it is now. r.mu held.
func (r *SimObjectRescue) tell() {
	c := r.convoy
	if c == nil {
		return
	}
	e := rescueMember{pos: r.pose.Position, depot: r.depot, v: r.pose.GroundSpeedKts * ktsToMS}
	if r.homing && !r.homeAtStop && r.mover != nil {
		e.homing, e.left = true, r.mover.path.Length()-r.mover.s
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.by == nil {
		c.by = map[*SimObjectRescue]rescueMember{}
	}
	if was, ok := c.by[r]; ok {
		e.n = was.n
	} else {
		c.n++
		e.n = c.n
	}
	c.by[r] = e
}

func (c *rescueConvoy) member(r *SimObjectRescue) rescueMember {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.by[r]
}

func (c *rescueConvoy) others(r *SimObjectRescue) []rescueMember {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []rescueMember
	for o, e := range c.by {
		if o != r {
			out = append(out, e)
		}
	}
	return out
}

func (c *rescueConvoy) leave(r *SimObjectRescue) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.by, r)
}
