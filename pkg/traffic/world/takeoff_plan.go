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
		if who := t.cc.takeoffMeets(g, it); who != "" {
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
