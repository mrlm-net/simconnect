package world

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Taxiing round an aircraft in the way: a departure cleared to push, pushing
// or pushed back waiting for its taxi takes its taxiway (and the push path)
// for a while. One taxiing on a route that passes it too near for the two
// wings to clear is given another route that keeps clear, when there is one
// it fits (any airport: the span check of the route search; at LKPR a
// pushback onto JB leaves J beside it too narrow, a CRJ goes by JO). Without
// one it keeps its route and gives way as before.

// keepClearEvery: how often the places taken are looked at.
const keepClearEvery = time.Second

// keepClear re-plans the taxiing aircraft round the places taken when they
// change. On the connection's goroutine (tick).
func (cc *controlCenter) keepClear() {
	if now := cc.clock.Now(); now.Sub(cc.keepClearAt) < keepClearEvery {
		return
	} else {
		cc.keepClearAt = now
	}
	items := cc.snapshotItems()
	type taken struct {
		it *controlled
		o  airport.Occupied
	}
	var occ []taken
	var key strings.Builder
	for _, it := range items {
		if it.dep == nil {
			continue
		}
		if o, ok := it.dep.Occupies(); ok {
			occ = append(occ, taken{it, o})
			// Its state only: a push's place shrinks every frame as it goes,
			// and re-planned for each, every taxiing aircraft searched its
			// route every second (live, LKPR: 80–90 ms each time).
			fmt.Fprintf(&key, "%s/%d;", it.Tail, it.dep.State())
		}
	}
	cc.keepClearOfOncoming(items)
	if key.String() == cc.keepClearKey {
		return
	}
	cc.keepClearKey = key.String()
	if len(occ) == 0 {
		return
	}
	for _, it := range items {
		if it.dep == nil && it.arr == nil {
			continue
		}
		it.mu.Lock()
		manual := it.view.Manual
		it.mu.Unlock()
		if manual {
			continue // the user's routes stay as given
		}
		var mine []airport.Occupied
		for _, t := range occ {
			if t.it != it && t.it.ICAO == it.ICAO {
				mine = append(mine, t.o)
			}
		}
		if len(mine) == 0 {
			continue
		}
		// The route search off the connection's goroutine; the new route
		// put in place on it (cc.do).
		go cc.reroute(it, mine, "traffic in the way", nil)
	}
}

// reroute searches it a route round occ off the connection's goroutine and
// puts it in place on it, said by ground when the taxiways changed; done,
// when set, is told whether it was.
func (cc *controlCenter) reroute(it *controlled, occ []airport.Occupied, why string, done func(bool)) {
	apply := it.planAvoidOccupied(occ)
	if apply == nil {
		if done != nil {
			done(false)
		}
		return
	}
	cc.do(func() error {
		it.mu.Lock()
		before := it.rerouteSaid().Text
		it.mu.Unlock()
		ok := apply()
		if done != nil {
			done(ok)
		}
		if !ok {
			return nil
		}
		it.mu.Lock()
		it.setRoute()
		tx := it.rerouteSaid()
		it.view.Instruction, it.view.InstructionAt = "taxi round traffic", nil
		it.mu.Unlock()
		tlog.printf("%-6s ground: new route round %s: %s", it.Tail, why, tx.Text)
		// Said only when the taxiways changed: the same "via J, H, A"
		// again is no new clearance (live, SWR1216 heard it twice in 19 s).
		if tx.Text != "" && tx.Text != before {
			it.say(tx)
		}
		return nil
	})
}

// Two of ours on routes along the same taxiway the opposite way: ground
// gives one another route round the shared stretch when there is one —
// first the one further from it, then the other (live, LKPR: TVS681 in
// along F to C24 met CSA398 out along F from C28; TVS681 had D). Without
// one, both keep their routes and the one not yet on the stretch holds
// short of it (traffic's head-on check). Generic: any airport, any two.
const (
	// opposedMinMeters: a shared stretch shorter than this is left to the
	// hold (a junction crossed the other way, a few metres of a turn).
	opposedMinMeters = 150.0
	// opposedForget: a pair looked at on its routes then is forgotten
	// after this (a new route of either is looked at at once).
	opposedForget = 10 * time.Minute
	// opposedEndMeters: the part of a shared stretch this near the end of
	// a route is not kept off in a re-plan (awayFrom).
	opposedEndMeters = 250.0
)

// keepClearOfOncoming is that look over ours on the ground (keepClear).
func (cc *controlCenter) keepClearOfOncoming(items []*controlled) {
	type onGround struct {
		it      *controlled
		route   *airport.Route
		way     []airport.LatLon
		half    float64
		movable bool
	}
	var ours []onGround
	for _, it := range items {
		route, way, movable, ok := it.groundWay()
		if !ok {
			continue
		}
		it.mu.Lock()
		half, manual := traffic.MotionProfileFor(it.view.Model).SpanMeters/2, it.view.Manual
		it.mu.Unlock()
		ours = append(ours, onGround{it, route, way, half, movable && !manual})
	}
	now := cc.clock.Now()
	if cc.opposedTried == nil {
		cc.opposedTried = map[string]time.Time{}
	}
	for k, at := range cc.opposedTried {
		if now.Sub(at) > opposedForget {
			delete(cc.opposedTried, k)
		}
	}
	for i, a := range ours {
		for _, b := range ours[i+1:] {
			if a.it.ICAO != b.it.ICAO || (!a.movable && !b.movable) {
				continue
			}
			// Once per pair of routes: what is ahead of each only shrinks
			// along the same route (live, the same pair logged every 30 s).
			ka, kb := fmt.Sprintf("%s %p", a.it.Tail, a.route), fmt.Sprintf("%s %p", b.it.Tail, b.route)
			key := min(ka, kb) + "|" + max(ka, kb)
			if _, tried := cc.opposedTried[key]; tried {
				continue
			}
			cc.opposedTried[key] = now
			reach := a.half + b.half + traffic.HeadOnMarginMeters
			fromA, fromB, length, alongB, ok := traffic.OpposedStretch(a.way, b.way, reach)
			if !ok || length < opposedMinMeters {
				continue
			}
			_, _, _, alongA, _ := traffic.OpposedStretch(b.way, a.way, reach)
			// The one further from the stretch first: it would hold.
			first, second := a, b
			// Each keeps off the other's way except near its own end: the
			// last stretch to its stand or runway has no way round (LKPR:
			// C24 only by H1 at 894, where CSA398 came out of C28), held
			// for there instead.
			firstOcc := airport.Occupied{Points: thinned(awayFrom(alongB, a.way[len(a.way)-1])), HalfSpan: b.half}
			secondOcc := airport.Occupied{Points: thinned(awayFrom(alongA, b.way[len(b.way)-1])), HalfSpan: a.half}
			if fromB > fromA {
				first, second, firstOcc, secondOcc = b, a, secondOcc, firstOcc
			}
			cc.log.printf("%-6s ground: %s comes the other way along %.0f m of its route", first.it.Tail, second.it.Tail, length)
			why := "traffic coming the other way"
			try := func(o onGround, occ airport.Occupied, next func(bool)) {
				if !o.movable {
					next(false)
					return
				}
				cc.reroute(o.it, []airport.Occupied{occ}, why, next)
			}
			go try(first, firstOcc, func(done bool) {
				if !done {
					go try(second, secondOcc, func(bool) {})
				}
			})
		}
	}
}

// awayFrom is pts without those within opposedEndMeters of end.
func awayFrom(pts []airport.LatLon, end airport.LatLon) []airport.LatLon {
	var out []airport.LatLon
	for _, p := range pts {
		if calc.HaversineMeters(p.Lat, p.Lon, end.Lat, end.Lon) > opposedEndMeters {
			out = append(out, p)
		}
	}
	return out
}

// thinned is every fourth point of pts (and the last): a route search
// checks each edge against them all.
func thinned(pts []airport.LatLon) []airport.LatLon {
	var out []airport.LatLon
	for i := 0; i < len(pts); i += 4 {
		out = append(out, pts[i])
	}
	if len(pts) > 0 && (len(pts)-1)%4 != 0 {
		out = append(out, pts[len(pts)-1])
	}
	return out
}

// turningInMeters: an arrival with this little of its taxi-in left is
// turning in to its stand (turningIn).
const turningInMeters = 150.0

// turningIn reports whether an arrival taxiing in has turningInMeters or
// less of its way left: at its stand, not passing by.
func (it *controlled) turningIn() bool {
	_, way, _, ok := it.groundWay()
	if !ok {
		return false
	}
	left := 0.0
	for i := 1; i < len(way) && left <= turningInMeters; i++ {
		left += calc.HaversineMeters(way[i-1].Lat, way[i-1].Lon, way[i].Lat, way[i].Lon)
	}
	return left <= turningInMeters
}

// groundWay is its taxi route and the way ahead on it from where it is,
// and whether ground may re-plan it now (an arrival waiting for or on its
// taxi-in, a departure taxiing); ok false when it is not on the ground
// with a route.
func (it *controlled) groundWay() (r *airport.Route, way []airport.LatLon, movable, ok bool) {
	switch {
	case it.dep != nil:
		switch it.dep.State() {
		case traffic.TaxiAwaitingPushback, traffic.TaxiPushback, traffic.TaxiAwaitingTaxi:
		case traffic.TaxiTaxiing:
			movable = true
		default:
			return nil, nil, false, false
		}
		r = it.dep.Route()
	case it.arr != nil:
		switch it.arr.State() {
		case traffic.ArrivalVacating:
		case traffic.ArrivalAwaitingTaxi, traffic.ArrivalTaxiing:
			movable = true
		default:
			return nil, nil, false, false
		}
		if p := it.arr.Plan(); p != nil {
			r = p.Route
		}
	}
	if r == nil || len(r.Points) < 2 {
		return nil, nil, false, false
	}
	it.mu.Lock()
	pos := it.view.Position
	it.mu.Unlock()
	near, best := 0, math.Inf(1)
	for i, p := range r.Points {
		if d := (p.Lat-pos.Lat)*(p.Lat-pos.Lat) + (p.Lon-pos.Lon)*(p.Lon-pos.Lon); d < best {
			near, best = i, d
		}
	}
	if near >= len(r.Points)-1 {
		return nil, nil, false, false
	}
	return r, r.Points[near:], movable, true
}

// occupiedFor is the places other departures at its airport take now.
func (cc *controlCenter) occupiedFor(it *controlled) []airport.Occupied {
	var out []airport.Occupied
	for _, o := range cc.snapshotItems() {
		if o == it || o.dep == nil || o.ICAO != it.ICAO {
			continue
		}
		if p, ok := o.dep.Occupies(); ok {
			out = append(out, p)
		}
	}
	return out
}

// avoidOccupied is the controller's AvoidOccupied.
func (it *controlled) avoidOccupied(occ []airport.Occupied) bool {
	if it.dep != nil {
		return it.dep.AvoidOccupied(occ)
	}
	return it.arr.AvoidOccupied(occ)
}

// planAvoidOccupied is avoidOccupied's search (the controller's
// PlanAvoidOccupied): nil none, else the step putting it in place.
// A controller over the wire (an actuator's) has no such step: its
// search runs as before, put in place at once.
func (it *controlled) planAvoidOccupied(occ []airport.Occupied) func() bool {
	type planner interface {
		PlanAvoidOccupied([]airport.Occupied) func() bool
	}
	var c any = it.arr
	if it.dep != nil {
		c = it.dep
	}
	if p, ok := c.(planner); ok {
		return p.PlanAvoidOccupied(occ)
	}
	return func() bool { return it.avoidOccupied(occ) }
}

// rerouteSaid is ground's new taxi instruction for the route from where the
// aircraft is (it.mu held).
func (it *controlled) rerouteSaid() traffic.Transmission {
	var r *airport.Route
	if it.dep != nil {
		r = it.dep.Route()
	} else if p := it.arr.Plan(); p != nil {
		r = p.Route
	}
	if r == nil || len(r.Points) == 0 {
		return traffic.Transmission{}
	}
	near, best := 0, math.Inf(1)
	for i, p := range r.Points {
		if d := (p.Lat-it.view.Position.Lat)*(p.Lat-it.view.Position.Lat) + (p.Lon-it.view.Position.Lon)*(p.Lon-it.view.Position.Lon); d < best {
			near, best = i, d
		}
	}
	ahead, err := it.graph.RouteFromNodes(r.Nodes[near:])
	if err != nil {
		return traffic.Transmission{}
	}
	via := ahead.SpokenTaxiways(-1)
	if it.dep != nil {
		return traffic.ClearedTaxiToRunway(it.Tail, it.view.Runway, r.Entry, via)
	}
	return traffic.ClearedTaxiToStand(it.Tail, it.view.Stand, via)
}

// snapshotItems is the controlled aircraft now.
func (cc *controlCenter) snapshotItems() []*controlled {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	out := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		out = append(out, it)
	}
	return out
}

// climbOn has a departure handed to MSFS AI climb on to the level
// departure cleared it to (radarFt), past the top of its SID: said alone,
// the climb waypoints still ended there (live, EZY516 level at FL100).
func (it *controlled) climbOn() {
	it.mu.Lock()
	ft := it.radarFt
	it.mu.Unlock()
	if ft <= 0 {
		ft = departureClimbFt
	}
	it.climbOnTo(ft)
}

// climbOnTo has a departure handed to MSFS AI climb on to ft and level
// there: climbOn's level, or on a staffed departure the level of its
// clearance until the player clears it higher (#1096).
func (it *controlled) climbOnTo(ft float64) {
	it.mu.Lock()
	pos, alt, id := it.view.Position, it.heightFt, it.objectID
	it.mu.Unlock()
	if it.graph != nil {
		alt += it.graph.Layout.Altitude / 0.3048
	}
	for _, a := range it.cc.world.Aircraft() { // where it is now, if seen
		if a.ObjectID == id && id != 0 {
			pos, alt = a.Position, a.AltFt
		}
	}
	if err := it.cc.do(func() error { return it.dep.ClimbTo(pos, alt, ft) }); err != nil {
		it.cc.log.printf("%-6s climb to %.0f ft not applied: %v", it.Tail, ft, err)
	}
}
