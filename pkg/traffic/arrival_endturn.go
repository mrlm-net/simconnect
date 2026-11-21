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

// endTurnLeadM: the exit back is this far at least past the end of the
// turn round, so it leaves along the centreline.
const endTurnLeadM = 20.0

// endTurn replans the arrival, its nose gear at nose rolling at kts, to
// turn round at the far end when its exit is out of reach (above); it
// returns the turn's points, from where it leaves the centreline to where
// it is back on it heading the other way, nil when it makes its exit or
// there is no exit back that reaches the stand.
func (c *ArrivalController) endTurn(kts float64, nose airport.LatLon) []airport.LatLon {
	g := c.req.Graph
	if g == nil || c.plan.Backtrack || c.exitTurnReach(nose) >= kts*ktsToMS {
		return nil
	}
	rwy, land := c.plan.Runway, c.plan.End
	opposite := rwy.Primary
	if opposite.Name == land.Name {
		opposite = rwy.Secondary
	}
	r := math.Min(math.Max(rwy.Width/2-backtrackEdge, backtrackMinRadius), backtrackMaxRadius)
	a0 := r + 5
	thr, hdg := opposite.Threshold, opposite.Heading
	at := func(x, y float64) airport.LatLon { return offsetHeading(offsetHeading(thr, hdg, x), hdg+90, y) }
	if alongHeading(thr, hdg, nose) < a0+3*r {
		return nil // at the end already
	}
	exits, err := g.RunwayExits(opposite.Name)
	if err != nil {
		return nil
	}
	exits = slices.Clone(exits)
	slices.SortStableFunc(exits, func(a, b airport.RunwayExit) int { return int(math.Copysign(1, a.Along-b.Along)) })
	req := c.req
	for _, x := range exits {
		if landingAlong(g, opposite.Name, x) < a0+2.5*r+endTurnLeadM {
			continue
		}
		plan, err := PlanArrival(g, opposite.Name, req.Parking, ArrivalOptions{
			SpawnNm: c.plan.SpawnNm, Exit: &x, Route: req.Options, GroundAGL: req.GroundAGL, NoseOffset: req.NoseOffset, StandSide: req.StandSide,
			TouchdownKts: approachProfileOf(req).TouchdownKts, BrakeDecel: req.Rollout.BrakeDecel,
		})
		if err != nil || g.TurnsBack(plan.Route) {
			continue
		}
		plan.Spawn, plan.End = c.plan.Spawn, land
		plan.Backtrack, plan.ClearIndex = true, len(x.Path)-1
		c.seq.add(c.now(), "no exit in reach: rolling to the end, back to "+x.Taxiway, 0, kts)
		c.plan = plan
		c.track = newRouteTracker(plan.Route)
		c.exitAlong = c.track.cum[len(plan.Exit.Path)-1]
		c.vacateAlong = c.track.cum[plan.VacateIndex] - plan.VacateBackMeters
		pts := []airport.LatLon{at(a0+2*r, 0)}
		for deg := 90.0; deg <= 270; deg += 15 {
			phi := deg * math.Pi / 180
			pts = append(pts, at(a0+r*math.Cos(phi), r*math.Sin(phi)))
		}
		return append(pts, at(a0+2.5*r, 0), at(a0+2.5*r+15, 0))
	}
	return nil
}
