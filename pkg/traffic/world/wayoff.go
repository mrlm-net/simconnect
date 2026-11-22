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

// keepWayOffClear plans each backtracking arrival's way off the runway,
// until it touches down, clear of where departures hold or will hold
// short (#1640): by another exit, else to another stand. A player's
// arrival is never moved (#1096). On the connection's goroutine
// (keepClear); the searches off it (planWayOff).
func (cc *controlCenter) keepWayOffClear(items []*controlled) {
	for _, it := range items {
		if it.arr == nil || it.gates.Load() || !beforeTouchdown(it) {
			continue
		}
		p, ok := it.arr.(wayOffPlanner)
		if !ok {
			continue
		}
		// Only one that rolls to the end has a way off to keep clear: for
		// the others nothing is gathered on the frames' goroutine.
		if plan := it.arr.Plan(); plan == nil || !plan.EndTurn {
			it.wayOffBlocked.Store(false)
			continue
		}
		occ := wayOffTaken(it, items)
		if p.WayOffClear(occ) {
			it.wayOffBlocked.Store(false)
			continue
		}
		// None found a moment ago: each exit to each stand is a route search.
		if it.wayOffBlocked.Load() && cc.clock.Now().UnixNano() < it.wayOffRetryAt.Load() {
			continue
		}
		if it.wayOffPlanning.Load() || cc.wayOffSearching.Swap(true) {
			continue // one search at a time; the next looks a second later
		}
		it.wayOffPlanning.Store(true)
		go cc.planWayOff(it, p, occ)
	}
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
// ends later than it began. On the connection's goroutine, as it is put in
// place.
func (cc *controlCenter) wayOffStale(it *controlled, from int, occ []airport.Occupied) string {
	if it.gates.Load() {
		return "a player's now"
	}
	if !beforeTouchdown(it) {
		return "down or gone"
	}
	it.mu.Lock()
	stand := it.stand
	it.mu.Unlock()
	if stand != from {
		return "given another stand"
	}
	if !sameTaken(occ, wayOffTaken(it, cc.snapshotItems())) {
		return "the departures have moved on"
	}
	return ""
}

// planWayOff finds it a way off clear of occ and puts it in place, if that
// still holds (wayOffStale): another exit to its stand, else the nearest
// free stand of its kind that fits and has one. The stand is named once, in
// the taxi clearance after landing.
func (cc *controlCenter) planWayOff(it *controlled, p wayOffPlanner, occ []airport.Occupied) {
	defer cc.wayOffSearching.Store(false)
	defer it.wayOffPlanning.Store(false)
	it.mu.Lock()
	old, tail, model := it.stand, it.Tail, it.view.Model
	it.mu.Unlock()
	to, apply := old, p.PlanWayOff(-1, occ)
	half := traffic.MotionProfileFor(model).SpanMeters / 2
	if apply == nil && it.stands != nil && old >= 0 && old < len(it.graph.Layout.Parking) {
		l := it.graph.Layout
		was := l.Parking[old]
		near := func(i int) float64 {
			q := l.Parking[i].Position
			return calc.HaversineMeters(was.Position.Lat, was.Position.Lon, q.Lat, q.Lon)
		}
		stands := slices.DeleteFunc(slices.Clone(l.SuitableStands(half)), func(i int) bool {
			return i == old || l.Parking[i].Type != was.Type || !it.stands.Free(i, half) || it.stands.TakenFrom(i, tail, 0) != ""
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
			if apply = p.PlanWayOff(i, occ); apply != nil {
				to = i
				break
			}
		}
	}
	err := cc.do(func() error {
		if cc.wayOffStale(it, old, occ) != "" {
			return nil // dropped: looked at afresh a second later
		}
		if apply == nil {
			it.wayOffRetryAt.Store(cc.clock.Now().Add(wayOffRetry).UnixNano())
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
		if !apply() {
			if to != old {
				it.stands.Release(to)
			}
			return nil
		}
		it.wayOffBlocked.Store(false)
		it.mu.Lock()
		it.setRoute()
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
		return nil
	})
	if err != nil {
		cc.log.printf("%-6s ground: way off the runway: %v", tail, err)
	}
}
