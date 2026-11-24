package traffic

import (
	"math"
	"slices"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Rolling to the end (#1618: TFFJ 10, 642 m, its one exit C 271 m in, the
// others behind the touchdown): when the injected rollout cannot slow to
// its exit's turn-off speed by the exit, braking at its BrakeDecel, and no
// later exit can be made either, the arrival rolls on down the centreline
// slowing evenly, turns round near the runway's far end within its width
// (the teardrop of a departure's backtrack, backtrack.go) and backtracks
// to the first exit of the opposite direction that reaches its stand.
// Before, it swung round into the exit behind or beside it at 60 kt. The
// runway stays taken until it is off it by that exit (ArrivalPlan
// Backtrack, ClearIndex).
//
// Planned up front (#1631): PlanArrival plans it from the start when no
// exit ahead is in reach of the type's touchdown speed and braking
// (exitReach.makes), so the runway time the tower counts on includes the
// roll to the end and back (ArrivalPlan.EndTurnMeters).

// endTurnLeadM: the exit back is this far at least past the end of the
// turn round, so it leaves along the centreline.
const endTurnLeadM = 20.0

// endTurnRadius is the radius of the turn round on runway rwy, and a0 how
// far before the far threshold it is centred.
func endTurnRadius(rwy airport.Runway) (r, a0 float64) {
	r = math.Min(math.Max(rwy.Width/2-backtrackEdge, backtrackMinRadius), backtrackMaxRadius)
	return r, r + 5
}

// oppositeEnd is the end of rwy facing land.
func oppositeEnd(rwy airport.Runway, land airport.RunwayEnd) airport.RunwayEnd {
	if rwy.Primary.Name == land.Name {
		return rwy.Secondary
	}
	return rwy.Primary
}

// endTurnExits are the exits of the direction opposite land on rwy far
// enough from its threshold to leave after the turn round, nearest first.
func endTurnExits(g *airport.Graph, rwy airport.Runway, land airport.RunwayEnd) []airport.RunwayExit {
	opposite := oppositeEnd(rwy, land)
	exits, err := g.RunwayExits(opposite.Name)
	if err != nil {
		return nil
	}
	r, a0 := endTurnRadius(rwy)
	exits = slices.DeleteFunc(slices.Clone(exits), func(x airport.RunwayExit) bool {
		return landingAlong(g, opposite.Name, x) < a0+2.5*r+endTurnLeadM
	})
	slices.SortStableFunc(exits, func(a, b airport.RunwayExit) int { return int(math.Copysign(1, a.Along-b.Along)) })
	return exits
}

// endTurnPlan is PlanArrival's plan for an arrival on runwayEnd that rolls
// to the far end, turns round and backtracks to an exit (above), planned
// as the landing on runwayEnd; false when the runway is too short to slow
// to TurnSpeedKts by the turn, or no exit back reaches the stand.
func endTurnPlan(g *airport.Graph, runwayEnd string, parking int, o ArrivalOptions, reach exitReach) (airport.RunwayExit, *airport.Route, float64, bool) {
	rwy, land, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return airport.RunwayExit{}, nil, 0, false
	}
	r, a0 := endTurnRadius(rwy)
	if reach.firmRollout(TurnSpeedKts) > rwy.Length-land.DisplacedM-(a0+2*r) {
		return airport.RunwayExit{}, nil, 0, false
	}
	for _, x := range endTurnExits(g, rwy, land) {
		route, err := g.RouteFromRunway(x, parking, o.Route)
		if err != nil || g.TurnsBack(route) {
			continue
		}
		return x, route, endTurnMeters(rwy, x.Along), true
	}
	return airport.RunwayExit{}, nil, 0, false
}

// endTurnMeters is the way on runway rwy past an exit back, fromFar before
// the far threshold: to the turn, round it and back.
func endTurnMeters(rwy airport.Runway, fromFar float64) float64 {
	r, a0 := endTurnRadius(rwy)
	return 2*math.Max(0, fromFar-(a0+2*r)) + (math.Pi+0.5)*r
}

// endTurnoffInReach replans the arrival, its nose gear at nose rolling at
// kts too fast for its exit, onto a turnoff leading back in the runway's
// last endTurnoffM that it slows to at its ExitSpeed (endTurnoff, #1674)
// and whose way on reaches the stand without a U-turn, one the aircraft
// fits, bending back up to MaxEntryAngle (exitTightTurning). False none,
// the plan as it was. c.mu held.
func (c *ArrivalController) endTurnoffInReach(kts float64, nose airport.LatLon) bool {
	g := c.req.Graph
	turnoffs, err := g.RunwayTurnoffs(c.plan.End.Name, airport.MaxEntryAngle)
	if err != nil {
		return false
	}
	length := c.plan.Runway.Length
	plan, track, exitAlong, vacateAlong := c.plan, c.track, c.exitAlong, c.vacateAlong
	for _, x := range turnoffs {
		if !leadsBack(x) || x.Along < length-endTurnoffM || c.exitTightTurning(x, airport.MaxEntryAngle) != "" {
			continue
		}
		if c.planExit(x, false, false) == nil && !g.TurnsBack(c.plan.Route) && c.exitTurnReach(nose) >= kts*ktsToMS {
			c.seq.add(c.now(), "too fast for "+plan.Exit.Taxiway+": rolling on to "+x.Taxiway+" at the end", 0, kts)
			return true
		}
		c.plan, c.track, c.exitAlong, c.vacateAlong = plan, track, exitAlong, vacateAlong
	}
	return false
}

// endTurn replans the arrival, its nose gear at nose rolling at kts, to
// turn round at the far end when its exit is out of reach (above); it
// returns the turn's points, from where it leaves the centreline to where
// it is back on it heading the other way, nil when it makes its exit or
// there is no exit back that reaches the stand. Planned so already
// (ArrivalPlan.EndTurn), only the points.
func (c *ArrivalController) endTurn(kts float64, nose airport.LatLon) []airport.LatLon {
	g := c.req.Graph
	if g == nil {
		return nil
	}
	rwy, land := c.plan.Runway, c.plan.End
	opposite := oppositeEnd(rwy, land)
	r, a0 := endTurnRadius(rwy)
	thr, hdg := opposite.Threshold, opposite.Heading
	at := func(x, y float64) airport.LatLon { return offsetHeading(offsetHeading(thr, hdg, x), hdg+90, y) }
	if alongHeading(thr, hdg, nose) < a0+3*r {
		return nil // at the end already
	}
	turn := func() []airport.LatLon {
		pts := []airport.LatLon{at(a0+2*r, 0)}
		for deg := 90.0; deg <= 270; deg += 15 {
			phi := deg * math.Pi / 180
			pts = append(pts, at(a0+r*math.Cos(phi), r*math.Sin(phi)))
		}
		return append(pts, at(a0+2.5*r, 0), at(a0+2.5*r+15, 0))
	}
	if c.plan.EndTurn {
		return turn()
	}
	if c.plan.Backtrack || c.exitTurnReach(nose) >= kts*ktsToMS {
		return nil
	}
	req := c.req
	// A turnoff at the far end it still slows to at taxi speed, as planned
	// up front (#1674); not in place of an exit named for it (#1096).
	if !c.exitNamed && req.Exit == nil && c.endTurnoffInReach(kts, nose) {
		return nil
	}
	for _, x := range endTurnExits(g, rwy, land) {
		plan, err := PlanArrival(g, opposite.Name, req.Parking, ArrivalOptions{
			SpawnNm: c.plan.SpawnNm, Exit: &x, Route: req.Options, GroundAGL: req.GroundAGL, NoseOffset: req.NoseOffset, StandSide: req.StandSide,
			TouchdownKts: approachProfileOf(req).TouchdownKts, BrakeDecel: req.Rollout.BrakeDecel,
		})
		if err != nil || g.TurnsBack(plan.Route) {
			continue
		}
		plan.Spawn, plan.End = c.plan.Spawn, land
		plan.Backtrack, plan.ClearIndex = true, len(x.Path)-1
		plan.EndTurn, plan.EndTurnMeters = true, 2*math.Max(0, x.Along-(a0+2*r))+(math.Pi+0.5)*r
		plan.Exit.Along = rwy.Length - x.Along
		c.seq.add(c.now(), "no exit in reach: rolling to the end, back to "+x.Taxiway, 0, kts)
		c.plan = plan
		c.track = newRouteTracker(plan.Route)
		c.exitAlong = c.track.cum[len(plan.Exit.Path)-1]
		c.vacateAlong = c.track.cum[plan.VacateIndex] - plan.VacateBackMeters
		return turn()
	}
	return nil
}
