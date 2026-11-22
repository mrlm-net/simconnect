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
	pose       GroundPose
	mover      *GroundMover // driving to standby or home; nil waiting or following
	homing     bool
	depot      airport.LatLon
	hasDepot   bool
	kts        float64 // following: its speed now
	err        error
	done       bool
	entering   bool // driving its way onto the runway to the aircraft (Enter)
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
	return &SimObjectRescue{client: client, inj: inj, title: title, reqID: reqID, block: math.Inf(1), laneStop: math.Inf(1)}
}

// Standby creates it at the nearest depot and sends it to at, beside the
// runway, where it waits.
func (r *SimObjectRescue) Standby(at GroundPose) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pose = at
	if r.After != nil {
		r.convoy = r.After.crew()
	}
	if r.convoy == nil {
		r.convoy = &rescueConvoy{}
	}
	if r.Layout != nil {
		d, ok := airport.LatLon{}, false
		if r.Depot != nil {
			d, ok = *r.Depot, true
		} else {
			d, ok = nearestDepot(r.Layout, at.Position)
		}
		if ok {
			if route, ok := RescueStandbyWay(r.Layout, d, at.Position); ok {
				if path, err := rescuePath(cleanWay(route), fuelRoadProfile()); err == nil {
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
		if r.homeAtStop {
			r.homeBrake(dt)
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
		if r.aircraft == nil {
			r.aircraft = new(airport.LatLon)
		}
		*r.aircraft = pose.Position
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
		r.kts, r.pose.GroundSpeedKts = 0, 0
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
// comes to a stand without one.
func (r *SimObjectRescue) Home() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || r.homing {
		return nil
	}
	r.homing = true
	r.laneStop = math.Inf(1)
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
	if r.Layout != nil && r.hasDepot {
		// From its nose, where the mover takes a path to start: from its
		// reference point it went back a wheelbase in a frame (worldtest,
		// the MyCrew app: 5 m as it left the stopped aircraft).
		nose := NoseGear(r.pose.Position, r.pose.Heading, rescueRoadProfile())
		if route, err := r.Layout.VehicleRouteJoin(nose, r.depot, true, false); err == nil && len(route) >= 2 {
			// Without the spikes of its join and the graph's spurs (#1630
			// (d)), so it turns out where the road truly lies behind it.
			way := cleanWay(append([]airport.LatLon{nose}, route...))
			// From standby or from following alike, off forwards (#1630).
			way = r.homeWayFrom(nose, r.pose.Heading, way)
			if path, err := rescuePath(way, fuelRoadProfile()); err == nil {
				r.mover = NewGroundMoverFrom(path, rescueRoadProfile(), r.pose.Heading, 0)
				r.tell()
				return nil
			}
		}
	}
	return r.finish()
}

// homeWayFrom is the way home from nose, facing h, given the direct
// one: that, unless it sets off backwards or runs over one of its crew
// standing; then out round (homeOut).
func (r *SimObjectRescue) homeWayFrom(nose airport.LatLon, h float64, direct []airport.LatLon) []airport.LatLon {
	back := startsBackwards(direct, h)
	if !back && r.overCrew(direct) == 0 {
		return direct
	}
	out := r.homeOut(nose, h, direct)
	if !back && r.overCrew(out) > 0 {
		return direct
	}
	return out
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
	if r.Layout != nil && r.hasDepot {
		// With no join in reach from there (the middle of a long taxiway),
		// it keeps to its way on to where one is.
		for d := on; d >= cleanWayMeters && d <= room; d += rescueHomeLookMeters {
			at := path.PointAt(s0 + d)
			from := offsetHeading(at, localBearing(path.PointAt(s0+d-1), at), wayTurnRadiusM)
			if route, err := r.Layout.VehicleRouteJoin(from, r.depot, true, false); err == nil && len(route) >= 2 {
				on = d
				break
			}
		}
	}
	cut, h := path.PointAt(s0), r.pose.Heading
	kept := []airport.LatLon{cut}
	if on >= cleanWayMeters {
		end := s0 + on
		for i, p := range path.pts {
			if c := path.cum[i]; c > s0+0.1 && c < end-0.1 {
				kept = append(kept, p)
			}
		}
		h = localBearing(path.PointAt(end-1), path.PointAt(end))
		cut = path.PointAt(end)
		kept = append(kept, cut)
	}
	if r.Layout == nil || !r.hasDepot {
		r.homeWhenStopped(s0+on, prof.Decel)
		return
	}
	// Straight on from there a little, so the corner home is rounded.
	a := offsetHeading(cut, h, wayTurnRadiusM)
	route, err := r.Layout.VehicleRouteJoin(a, r.depot, true, false)
	if err != nil || len(route) < 2 {
		r.homeWhenStopped(s0+on, prof.Decel)
		return
	}
	way := cleanWay(append([]airport.LatLon{a}, route...))
	if startsBackwards(way, h) || r.overCrew(way) > 0 {
		way = r.homeOut(cut, h, way)
	} else {
		way = append([]airport.LatLon{cut}, way...)
	}
	rest, err := NewArcPath(way, road, 8)
	if err != nil {
		r.homeWhenStopped(s0+on, prof.Decel)
		return
	}
	// Rounded again where the two meet.
	next, err := rescueRounded(append(kept, rest.pts...), road)
	if err != nil {
		r.homeWhenStopped(s0+on, prof.Decel)
		return
	}
	r.mover = NewGroundMoverFrom(next, prof, r.pose.Heading, v/ktsToMS)
	r.mover.a = old.a
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

// homeBrake moves it on by dt along its way, slowing at homeDecel: no
// stop point halts it from speed (it is taken away from a stand only).
// r.mu held.
func (r *SimObjectRescue) homeBrake(dt float64) {
	m := r.mover
	next := math.Max(0, m.v-r.homeDecel*dt)
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
func (r *SimObjectRescue) overCrew(way []airport.LatLon) int {
	n := 0
	for _, o := range r.convoy.others(r) {
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
func (r *SimObjectRescue) homeOut(nose airport.LatLon, h float64, direct []airport.LatLon) []airport.LatLon {
	side := 90.0
	for _, p := range direct[1:] {
		if localDist(nose, p) >= cleanWayMeters && headingDiff(h, localBearing(nose, p)) < 0 {
			side = -90
		}
		if localDist(nose, p) >= cleanWayMeters {
			break
		}
	}
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
			on, err := r.Layout.VehicleRoutePose(pts[len(pts)-1], endH, r.depot, noHeading)
			if err != nil || len(on) < 2 {
				continue
			}
			way := append(pts[:len(pts)-1:len(pts)-1], cleanWay(on)...)
			hits := r.overCrew(way)
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
	return append(append(pts, offsetHeading(pts[2], h+180, 2*wayTurnRadiusM)), on...)
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
	stop := math.Min(r.laneStop, home)
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
