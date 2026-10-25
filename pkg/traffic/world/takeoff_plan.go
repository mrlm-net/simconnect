package world

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A take-off is cleared only with the departure's way out clear of the
// arrivals' ways in: its SID flown from the runway at the climb it can
// make, against their STARs and approaches as planned. The runway rules
// alone count the arrivals' time to the threshold; a long procedure or a
// SID across the arrival flow met them in the air (the user: "14 NM when
// the last one takes off, with a big IFR procedure a close call 100%";
// live, ENT1744 off 06 into TVS1113 on the 24 approach, held under TVS122
// at 3700 ft).

// takeoffLiftoffNM: the departure taken airborne this far down the runway.
const takeoffLiftoffNM = 1.0

// takeoffPlan is the departure off end as predicted: from lift-off along
// its SID (DepartureWaypoints, its altitude constraints and top), at
// climbRateFpm from the field; straight out with no SID.
func takeoffPlan(end airport.RunwayEnd, fieldFt float64, sid []airport.NavPoint) (traffic.TrackedAircraft, []traffic.RoutePoint) {
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, takeoffLiftoffNM*1852)
	off := airport.LatLon{Lat: lat, Lon: lon}
	me := traffic.TrackedAircraft{Observation: traffic.Observation{Position: off, AltFt: fieldFt, GroundKts: 160, Heading: end.Heading}, Phase: traffic.PhaseDeparting, Ours: true}
	var route []traffic.RoutePoint
	for _, w := range traffic.DepartureWaypoints(off, end.Heading, fieldFt, sid) {
		route = append(route, traffic.RoutePoint{Position: airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}, AltFt: w.Altitude, Kts: w.KtsSpeed})
	}
	return me, reachable(me, route)
}

// takeoffMeets names an arrival of ours that departure it, taking off from
// its runway now, would lose separation with on its way out (within
// conflictLookAhead); "" none. The arrivals established on the final to
// its own runway end are the runway rules' (it climbs away from them).
func (cc *controlCenter) takeoffMeets(g *airport.Graph, it *controlled) string {
	return cc.takeoffMeetsCapped(g, it, 0)
}

// takeoffMeetsCapped is takeoffMeets with the climb restricted to capFt
// after departure (0 none).
func (cc *controlCenter) takeoffMeetsCapped(g *airport.Graph, it *controlled, capFt float64) string {
	if it.dep == nil {
		return ""
	}
	it.mu.Lock()
	rwy := it.view.Runway
	it.mu.Unlock()
	_, end, ok := g.Layout.RunwayEnd(rwy)
	if !ok {
		return ""
	}
	me, plan := takeoffPlan(end, g.Layout.Altitude/0.3048, it.dep.DepartureRoute())
	me.Tail = it.Tail
	if capFt > 0 {
		for i := range plan {
			plan[i].AltFt = math.Min(plan[i].AltFt, capFt)
		}
	}
	plans := map[string][]traffic.RoutePoint{me.Tail: plan}
	list := []traffic.TrackedAircraft{me}
	for _, a := range cc.world.Aircraft() {
		if !a.Ours || a.Grounded() || a.Tail == it.Tail {
			continue
		}
		o := cc.byTail(a.Tail)
		if o == nil || o.arr == nil || o.ICAO != it.ICAO || o.object() != a.ObjectID || o.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		if o.circuit != nil {
			// VFR in the circuit: the tower gives traffic information, it
			// does not hold an IFR departure for it; its landing is the
			// runway rules' (live, DLH555 lined up for 5 minutes for
			// OKGKF joining downwind).
			continue
		}
		o.mu.Lock()
		same := o.view.Runway == rwy
		o.mu.Unlock()
		proc := o.arr.ProcedurePlan()
		if same && len(proc) == 0 && math.Abs(headingDiff(a.Heading, end.Heading)) < 30 {
			continue // on its final to our runway end: the runway rules
		}
		plans[a.Tail] = heldPlan(proc, a)
		list = append(list, a)
	}
	return takeoffConflict(list, plans)
}

// takeoffConflict names who list[0], the departure as takeoffPlan has it,
// meets flown along plans; "" none.
func takeoffConflict(list []traffic.TrackedAircraft, plans map[string][]traffic.RoutePoint) string {
	if len(list) < 2 {
		return ""
	}
	opts := climbOpts
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint { return plans[a.Tail] }
	me := list[0].Tail
	for _, c := range traffic.PredictConflicts(list, opts) {
		switch me {
		case c.A:
			return c.B
		case c.B:
			return c.A
		}
	}
	return ""
}

// takeoffFilter is tails cleared for take-off less those whose way out
// meets an arrival's way in, said waiting in c.
func (t *towers) takeoffFilter(g *airport.Graph, tails []string, c *traffic.RunwayClearances, ours map[string]*controlled) []string {
	var keep []string
	for _, tail := range tails {
		it := ours[tail]
		if it == nil || t.givenTakeoff(tail) {
			keep = append(keep, tail) // not ours, or cleared already: it goes
			continue
		}
		if who := t.cc.priorityClose(g.Layout.ICAO); who != "" {
			if c.Waiting == nil {
				c.Waiting = map[string]string{}
			}
			c.Waiting[tail] = "priority traffic " + who // an emergency on its way in (emergency.go)
			continue
		}
		if who := t.cc.takeoffMeets(g, it); who != "" {
			// A restriction first, as tower amends the clearance: "after
			// departure climb to altitude 4000 feet", the lowest that keeps
			// it clear; held only when none does (the user: restrictions up
			// front, not a level-off later).
			if ft := t.cc.afterDepartureCap(g, it); ft > 0 {
				keep = append(keep, tail)
				continue
			}
			if c.Waiting == nil {
				c.Waiting = map[string]string{}
			}
			c.Waiting[tail] = who + " on its way in"
			continue
		}
		keep = append(keep, tail)
	}
	return keep
}

// afterDepartureSteps are the restrictions tried after departure, in
// thousands of feet above the field rounded up (lowest first).
var afterDepartureSteps = []float64{3000, 4000, 5000}

// afterDepartureCap is the lowest climb restriction after departure that
// keeps departure it clear of the arrivals' ways in (feet MSL), given now
// (said once, and set on its controller); 0 none clears it.
func (cc *controlCenter) afterDepartureCap(g *airport.Graph, it *controlled) float64 {
	it.mu.Lock()
	had := it.afterDepFt
	it.mu.Unlock()
	if had > 0 {
		if cc.takeoffMeetsCapped(g, it, had) == "" {
			return had // given already, still clear
		}
		return 0
	}
	field := g.Layout.Altitude / 0.3048
	for _, step := range afterDepartureSteps {
		ft := math.Ceil((field+step)/1000) * 1000
		if cc.takeoffMeetsCapped(g, it, ft) != "" {
			continue
		}
		if !it.dep.LimitClimb(ft) {
			return 0
		}
		it.mu.Lock()
		it.afterDepFt = ft
		it.mu.Unlock()
		cc.log.printf("%-6s take-off: after departure climb to %.0f ft, clear of the arrivals", it.Tail, ft)
		it.say(traffic.AfterDepartureClimb(it.Tail, ft))
		return ft
	}
	return 0
}
