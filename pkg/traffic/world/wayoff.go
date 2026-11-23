package world

import (
	"slices"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// wayOffRetry is how long after finding no way off clear the next search
// waits; wayOffStands how many other stands one search tries at most, the
// nearest (each is a route search from every exit back); wayOffMovedM how
// far a departure searched round may have moved and still count as there.
const (
	wayOffRetry  = 10 * time.Second
	wayOffStands = 8
	wayOffMovedM = 2.0
)

// wayOffPlanner is an arrival controller that plans its way off
// (traffic.ArrivalController, #1640).
type wayOffPlanner interface {
	WayOffClear([]airport.Occupied) bool
	PlanWayOff(parking int, occ []airport.Occupied) func() bool
}

// keepWayOffClear plans each backtracking arrival's way off the runway and
// in to its stand, until it touches down, clear of where departures hold or
// will hold short (#1640) and, with its stand, of who stands on the apron
// and their ways out (#1641, apron_pair.go): by another exit, else to
// another stand. A player's arrival is never moved (#1096). On the
// connection's goroutine (keepClear); the searches off it (planWayOff), and
// with the director split from the actuator the look too (lookOverLink):
// nothing is asked over the link here.
func (cc *controlCenter) keepWayOffClear(items []*controlled) {
	for _, it := range items {
		if it.arr == nil || it.gates.Load() {
			continue
		}
		p, ok := wayOffPlannerOf(it.arr) // an actuator's too (#1655)
		if !ok {
			continue
		}
		if r, linked := p.(*remotePlanner); linked {
			if it.wayOffLooking.Swap(true) {
				it.wayOffBlocked.Store(false) // not answered yet: not "no way off"
				continue
			}
			go cc.lookOverLink(it, r, items)
			continue
		}
		if beforeTouchdown(it) {
			cc.takeLook(it, p, cc.lookWayOff(it, p, items))
		}
	}
}

// wayOffLook is one look at an arrival's way off.
type wayOffLook struct {
	stand      int // it was on
	backtracks bool
	occ        []airport.Occupied // where departures hold or will
	clear      bool               // of occ
	apronClear bool               // of the apron's places too
	apron      string             // apronPlaces.key
}

// lookWayOff asks whether it's way off is clear; it changes nothing.
func (cc *controlCenter) lookWayOff(it *controlled, p wayOffPlanner, items []*controlled) wayOffLook {
	it.mu.Lock()
	l, model := wayOffLook{stand: it.stand}, it.view.Model
	it.mu.Unlock()
	// Only one backtracking has a way in to keep clear: for the others
	// nothing is gathered.
	plan := it.arr.Plan()
	if plan == nil || !(plan.EndTurn || plan.Backtrack) {
		return l
	}
	l.backtracks, l.occ = true, wayOffTaken(it, items)
	if l.clear = p.WayOffClear(l.occ); l.clear {
		ap := apronPlacesFor(it, items)
		l.apron = ap.key
		l.apronClear = ap.standClear(cc, it.graph, l.stand, model) && (len(ap.bodies) == 0 || p.WayOffClear(slices.Concat(l.occ, ap.bodies)))
	}
	return l
}

// takeLook puts what look l found in place and starts the search it asks
// for; it asks the controller nothing. On the connection's goroutine.
func (cc *controlCenter) takeLook(it *controlled, p wayOffPlanner, l wayOffLook) {
	if !l.backtracks {
		it.wayOffBlocked.Store(false)
		it.apronBlocked.Store(false)
		return
	}
	none := &it.wayOffBlocked // the search that found none
	if l.clear {
		none = &it.apronBlocked
		it.wayOffBlocked.Store(false)
		if l.apronClear {
			it.apronBlocked.Store(false)
			return
		}
	}
	// None found a moment ago: each exit to each stand is a route search.
	if none.Load() && cc.clock.Now().UnixNano() < it.wayOffRetryAt.Load() {
		return
	}
	if it.wayOffPlanning.Load() || cc.wayOffSearching.Swap(true) {
		return // one search at a time; the next looks a second later
	}
	it.wayOffPlanning.Store(true)
	go cc.planWayOff(it, p, l.occ)
}

// standNow is the stand it has now.
func (it *controlled) standNow() int {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.stand
}

// beforeTouchdown is whether arrival it is still to touch down: not on its
// landing roll or past its exit, cleared to taxi in, or gone.
func beforeTouchdown(it *controlled) bool {
	st := it.arr.State()
	return st == traffic.ArrivalApproaching || st == traffic.ArrivalLanding
}

// wayOffTaken are the places departures at it's airport hold short or,
// taxiing out, will.
func wayOffTaken(it *controlled, items []*controlled) []airport.Occupied {
	var occ []airport.Occupied
	for _, o := range items {
		if o == it || o.dep == nil || o.ICAO != it.ICAO {
			continue
		}
		var at []airport.LatLon
		switch o.dep.State() {
		case traffic.TaxiHoldingShort:
			o.mu.Lock()
			if pos := o.view.Position; pos != (airport.LatLon{}) {
				at = []airport.LatLon{pos}
			}
			o.mu.Unlock()
		case traffic.TaxiTaxiing:
			if r := o.dep.Route(); r != nil && len(r.Points) > 0 {
				at = r.Points[len(r.Points)-1:]
			}
		}
		if len(at) == 0 {
			continue
		}
		o.mu.Lock()
		model := o.view.Model
		o.mu.Unlock()
		occ = append(occ, airport.Occupied{Points: at, HalfSpan: traffic.MotionProfileFor(model).SpanMeters / 2})
	}
	return occ
}

// sameTaken is whether a and b are the same places taken, in any order.
func sameTaken(a, b []airport.Occupied) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		if !slices.ContainsFunc(b, func(y airport.Occupied) bool {
			return x.HalfSpan == y.HalfSpan && slices.EqualFunc(x.Points, y.Points, func(p, q airport.LatLon) bool {
				return calc.HaversineMeters(p.Lat, p.Lon, q.Lat, q.Lon) <= wayOffMovedM
			})
		}) {
			return false
		}
	}
	return true
}

// wayOffStale is why what a search found, begun with the arrival on stand
// from and the departures at occ, no longer holds ("": it does): the search
// ends later than it began. On the connection's goroutine as it is put in
// place; over a link just before, off it: it asks the actuator.
func (cc *controlCenter) wayOffStale(it *controlled, from int, occ []airport.Occupied) string {
	if it.gates.Load() {
		return "a player's now"
	}
	if !beforeTouchdown(it) {
		return "down or gone"
	}
	if it.standNow() != from {
		return "given another stand"
	}
	if !sameTaken(occ, wayOffTaken(it, cc.snapshotItems())) {
		return "the departures have moved on"
	}
	return ""
}

// planWayOff finds it a way off clear of occ and of the apron's places and
// puts it in place, if that still holds (wayOffStale): another exit to its
// stand, else the nearest free stand of its kind that fits and has one.
// None clear of the apron too: one clear of occ alone, and the departure on
// the apron goes first (pairedArrival). Only with none clear of occ does it
// wait for the runway (#1640). The stand is named once, in the taxi
// clearance after landing.
func (cc *controlCenter) planWayOff(it *controlled, p wayOffPlanner, occ []airport.Occupied) {
	defer cc.wayOffSearching.Store(false)
	defer it.wayOffPlanning.Store(false)
	it.mu.Lock()
	old, tail, model := it.stand, it.Tail, it.view.Model
	it.mu.Unlock()
	ap := apronPlacesFor(it, cc.snapshotItems())
	half := traffic.MotionProfileFor(model).SpanMeters / 2
	search := func(taken []airport.Occupied, fits func(stand int) bool) (int, func() bool) {
		if fits(old) {
			if apply := p.PlanWayOff(-1, taken); apply != nil {
				return old, apply
			}
		}
		if it.stands == nil || old < 0 || old >= len(it.graph.Layout.Parking) {
			return old, nil
		}
		l := it.graph.Layout
		was := l.Parking[old]
		near := func(i int) float64 {
			q := l.Parking[i].Position
			return calc.HaversineMeters(was.Position.Lat, was.Position.Lon, q.Lat, q.Lon)
		}
		stands := slices.DeleteFunc(slices.Clone(l.SuitableStands(half)), func(i int) bool {
			return i == old || l.Parking[i].Type != was.Type || !fits(i) || !it.stands.Free(i, half) || it.stands.TakenFrom(i, tail, 0) != ""
		})
		slices.SortStableFunc(stands, func(a, b int) int {
			switch da, db := near(a), near(b); {
			case da < db:
				return -1
			case da > db:
				return 1
			}
			return 0
		})
		for _, i := range stands[:min(len(stands), wayOffStands)] {
			if apply := p.PlanWayOff(i, taken); apply != nil {
				return i, apply
			}
		}
		return old, nil
	}
	to, apply := search(slices.Concat(occ, ap.bodies), func(i int) bool { return ap.standClear(cc, it.graph, i, model) })
	// apronOnly: clear of occ, not of the apron; kept: as planned is.
	apronOnly, kept := false, false
	if apply == nil && (len(ap.bodies) > 0 || len(ap.out) > 0) {
		if p.WayOffClear(occ) {
			apronOnly, kept = true, true
		} else if to, apply = search(occ, func(int) bool { return true }); apply != nil {
			apronOnly = true
		}
	}
	// holds: the departures and it are where they were, and nobody else is
	// on the apron or pushing.
	holds := func() bool {
		return cc.wayOffStale(it, old, occ) == "" && ap.key == apronPlacesFor(it, cc.snapshotItems()).key
	}
	endTurns := func() bool {
		plan := it.arr.Plan()
		return plan != nil && plan.EndTurn
	}
	r, linked := p.(*remotePlanner)
	var rolls bool
	if linked {
		// A call to the actuator lost (#1655) is not "none clear", which
		// would send it round: looked at afresh.
		if r.lost() {
			it.wayOffBlocked.Store(false)
			return
		}
		// Asked here: on the director's loop only the take goes over the link.
		if !holds() {
			return
		}
		plan := it.arr.Plan()
		if plan == nil {
			it.wayOffBlocked.Store(false) // not answered: no guess
			return
		}
		rolls = plan.EndTurn
	}
	// take: over the link, the stand kept for it and the take still to come,
	// off the director's loop (#1673): it and the plan read after it are a
	// round trip each.
	var take bool
	err := cc.do(func() error {
		if linked {
			if it.gates.Load() || it.standNow() != old {
				return nil
			}
		} else if !holds() {
			return nil // dropped: looked at afresh a second later
		}
		if apronOnly {
			it.wayOffRetryAt.Store(cc.clock.Now().Add(wayOffRetry).UnixNano())
			if !it.apronBlocked.Swap(true) {
				cc.log.printf("%-6s ground: no way in clear of who stands on the apron, the departure first", tail)
			}
			if kept {
				it.wayOffBlocked.Store(false)
				return nil
			}
		}
		if apply == nil {
			it.wayOffRetryAt.Store(cc.clock.Now().Add(wayOffRetry).UnixNano())
			// Sent round for a departure only where it rolls to the end
			// (#1640); elsewhere it keeps its exit and is held short of.
			if !linked {
				rolls = endTurns()
			}
			if !rolls {
				it.apronBlocked.Store(true)
				it.wayOffBlocked.Store(false)
				return nil
			}
			if !it.wayOffBlocked.Swap(true) {
				cc.log.printf("%-6s ground: no way off the runway clear of departures", tail)
			}
			return nil
		}
		if to != old {
			// Free and fitting still, by the allocator's own rule.
			if !it.stands.Free(to, half) || it.stands.TakenFrom(to, tail, 0) != "" {
				return nil
			}
			if err := it.stands.Occupy(to, tail, half); err != nil {
				return err
			}
		}
		if linked {
			take = true
			return nil
		}
		if !apply() {
			if to != old {
				it.stands.Release(to)
			}
			return nil
		}
		cc.tookWayOff(it, old, to, apronOnly, model, it.setRoute)
		return nil
	})
	if err != nil {
		cc.log.printf("%-6s ground: way off the runway: %v", tail, err)
	}
	if take {
		cc.takeOverLink(it, old, to, apronOnly, model, apply)
	}
}

// takeOverLink takes a way off found over the link, stand to kept for it
// (from old), off the director's loop (#1673): the take and the plan read
// after it are a round trip each. Never a player's (#1096), which it may
// have become since the loop looked. Put in place on the loop.
func (cc *controlCenter) takeOverLink(it *controlled, old, to int, apronOnly bool, model string, apply func() bool) {
	took := !it.gates.Load() && apply()
	var route *airport.Route
	if took {
		if plan := it.arr.Plan(); plan != nil {
			route = plan.Route
		}
	}
	err := cc.do(func() error {
		if !took {
			if to != old {
				it.stands.Release(to)
			}
			return nil
		}
		cc.tookWayOff(it, old, to, apronOnly, model, func() { it.setRouteTo(route) })
		return nil
	})
	if err != nil {
		cc.log.printf("%-6s ground: way off the runway: %v", it.Tail, err)
	}
}

// tookWayOff puts in place arrival it's way off, taken: from stand old to
// stand to (kept for it already), its route by setRoute (it.mu held). On
// the connection's goroutine.
func (cc *controlCenter) tookWayOff(it *controlled, old, to int, apronOnly bool, model string, setRoute func()) {
	tail := it.Tail
	it.wayOffBlocked.Store(false)
	if !apronOnly {
		it.apronBlocked.Store(false)
	}
	it.mu.Lock()
	setRoute()
	was := ""
	if to != old {
		was = it.view.Stand
		it.stand, it.view.Stand = to, it.graph.Layout.Parking[to].Label()
		it.tugStand.Store(int32(to))
	}
	mgr, label, rwy := it.managed, it.view.Stand, it.view.Runway
	it.mu.Unlock()
	if to != old {
		it.stands.Release(old)
		if mgr != nil {
			mgr.Describe(tail, model, label, rwy)
		}
		cc.log.printf("%-6s ground: way off the runway planned clear of departures, stand %s (was %s)", tail, label, was)
	} else {
		cc.log.printf("%-6s ground: way off the runway planned clear of departures", tail)
	}
	cc.changed("control")
}
