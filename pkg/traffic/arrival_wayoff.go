package traffic

import (
	"math"
	"slices"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// WayOffClear reports whether the way back off the runway of an arrival
// backtracking (ArrivalPlan.Backtrack, EndTurn: rolling to the end first),
// and on to its stand, passes none of occ: where others stand or will hold
// short (#1640: at TFFJ it stood on the runway nose to nose with the
// departure holding short on its exit, each waiting for the other). True
// for any other arrival.
func (c *ArrivalController) WayOffClear(occ []airport.Occupied) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.plan == nil || !(c.plan.EndTurn || c.plan.Backtrack) || c.plan.Route == nil || len(occ) == 0 {
		return true
	}
	opts := withSpan(c.req.Options, c.profile())
	opts.Occupied = occ
	return !c.req.Graph.PassesOccupied(c.plan.Route, 0, opts)
}

// PlanWayOff plans that way clear of occ to stand parking (negative: its
// own) by the nearest exit back that has one, without the controller's lock
// held during the search: nil when none is clear or it is committed, else
// the step that puts the plan in place, for the connection's goroutine
// (false: it touched down, or was planned afresh, meanwhile). Before the
// rollout only, flown by injection. One backtracking without the turn at
// the end keeps its exit (#1641: only the way on to a stand differs).
func (c *ArrivalController) PlanWayOff(parking int, occ []airport.Occupied) func() bool {
	c.mu.Lock()
	if c.plan == nil || !(c.plan.EndTurn || c.plan.Backtrack) || c.state >= ArrivalRollout || !c.req.InjectApproach && !c.flyingProc {
		c.mu.Unlock()
		return nil
	}
	old, req, opts := c.plan, c.req, withSpan(c.req.Options, c.profile())
	c.mu.Unlock()
	g := req.Graph
	if parking < 0 {
		parking = req.Parking
	}
	if parking >= len(g.Layout.Parking) {
		return nil
	}
	opts.Occupied = occ
	from := oppositeEnd(old.Runway, old.End).Name
	r, a0 := endTurnRadius(old.Runway)
	var exits []airport.RunwayExit
	if old.EndTurn {
		exits = endTurnExits(g, old.Runway, old.End)
	} else {
		from, exits = old.End.Name, []airport.RunwayExit{old.Exit}
		opts.UseRunwayPaths = true // back along it, as it was planned (backtrackExit)
	}
	for _, x := range exits {
		plan, err := PlanArrival(g, from, parking, ArrivalOptions{
			SpawnNm: old.SpawnNm, Exit: &x, Route: opts, GroundAGL: req.GroundAGL, NoseOffset: req.NoseOffset, StandSide: req.StandSide,
			TouchdownKts: approachProfileOf(req).TouchdownKts, BrakeDecel: req.Rollout.BrakeDecel,
		})
		if err != nil || g.TurnsBack(plan.Route) || g.PassesOccupied(plan.Route, 0, opts) || !old.EndTurn && !plan.Backtrack {
			continue
		}
		// Landing as planned; only the way back differs (endTurnPlan).
		plan.Spawn, plan.End, plan.Waypoints = old.Spawn, old.End, slices.Clone(old.Waypoints)
		if old.EndTurn {
			plan.Backtrack, plan.ClearIndex = true, len(x.Path)-1
			plan.EndTurn, plan.EndTurnMeters = true, 2*math.Max(0, x.Along-(a0+2*r))+(math.Pi+0.5)*r
			plan.Exit.Along = old.Runway.Length - x.Along
		}
		return func() bool {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.state >= ArrivalRollout || c.plan != old {
				return false
			}
			c.req.Parking = parking
			c.standHeading = g.Layout.Parking[parking].Heading
			c.plan = plan
			c.track = newRouteTracker(plan.Route)
			c.exitAlong = c.track.cum[len(plan.Exit.Path)-1]
			c.vacateAlong = c.track.cum[plan.VacateIndex] - plan.VacateBackMeters
			c.seq.add(c.now(), "way off taken: back to "+x.Taxiway+", stand "+g.Layout.Parking[parking].Label(), 0, 0)
			return true
		}
	}
	return nil
}
