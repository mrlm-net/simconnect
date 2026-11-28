package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// wayOffReportNM: an arrival on its approach keeps its way off the runway
// (GroundPicture.ReportWayOff) from this far from its threshold, and on
// until it has vacated (#1719). Farther out a departure taxiing to hold
// short on its exit is let go on: it may well be gone before.
const wayOffReportNM = 6.0

// wayOffInMeters: of an arrival's taxi-in past its way off the runway,
// this much is kept with it (GroundPicture.ReportWayOff): where an
// aircraft stopping short for it would stand in its way.
const wayOffInMeters = 400.0

// reportWayOff tells the ground picture the way off the runway this
// arrival keeps (wayOffNow), or that it keeps none any more. c.mu held.
func (c *ArrivalController) reportWayOff(pos airport.LatLon) {
	if c.picture == nil || c.objectID == 0 {
		return
	}
	way, in := c.wayOffNow(pos)
	if len(way) == 0 && !c.wayOffSent {
		return
	}
	c.wayOffSent = len(way) > 0
	c.picture.ReportWayOff(c.objectID, way, in, c.profile().SpanMeters/2, c.now())
}

// wayOffNow is the way off the runway this arrival keeps, at pos: its
// planned exit up to the first junction past its vacate stop, and its
// taxi-in on from there (wayOffInMeters of it); nil before
// wayOffReportNM out, once vacated, and for one planned back along the
// runway (its way off is kept clear by PlanWayOff, #1640). c.mu held.
func (c *ArrivalController) wayOffNow(pos airport.LatLon) (way, in []airport.LatLon) {
	if c.plan == nil || c.plan.Route == nil || c.plan.EndTurn || c.plan.Backtrack || c.req.Graph == nil {
		return nil, nil
	}
	switch c.state {
	case ArrivalLanding, ArrivalRollout, ArrivalVacating:
	case ArrivalApproaching:
		t := c.plan.End.LandingThreshold()
		if calc.HaversineNM(pos.Lat, pos.Lon, t.Lat, t.Lon) > wayOffReportNM {
			return nil, nil
		}
	default:
		return nil, nil
	}
	if c.wayOffPlan != c.plan {
		c.wayOffPlan = c.plan
		c.wayOffWay, c.wayOffIn = exitWay(c.req.Graph, c.plan.Route, c.plan.VacateIndex)
	}
	return c.wayOffWay, c.wayOffIn
}

// exitWay is route r's way off the runway, from its start up to the first
// junction at or past point vacate, and its way on from there
// (wayOffInMeters), each sampled every trafficBodyStep meters.
func exitWay(g *airport.Graph, r *airport.Route, vacate int) (way, in []airport.LatLon) {
	j := exitJunction(g, r, vacate)
	if j < 0 {
		return nil, nil
	}
	return sampleEvery(r.Points[:j+1], trafficBodyStep, math.Inf(1)), sampleEvery(r.Points[j:], trafficBodyStep, wayOffInMeters)
}

// exitJunction is the point of route r where its way off the runway ends:
// the first junction at or past point vacate (its last point if none); -1
// for a route of no way.
func exitJunction(g *airport.Graph, r *airport.Route, vacate int) int {
	if r == nil || len(r.Points) < 2 || len(r.Nodes) != len(r.Points) || len(r.Edges) != len(r.Nodes)-1 {
		return -1
	}
	for i := max(vacate, 0); i < len(r.Nodes); i++ {
		if len(g.Adj[r.Nodes[i]]) >= 3 {
			return i
		}
	}
	return len(r.Points) - 1
}

// exitStretch is route r up to its exitJunction; nil for a route of no
// way.
func exitStretch(g *airport.Graph, r *airport.Route, vacate int) *airport.Route {
	j := exitJunction(g, r, vacate)
	if j < 0 {
		return nil
	}
	return &airport.Route{Nodes: r.Nodes[:j+1], Points: r.Points[:j+1], Edges: r.Edges[:j]}
}

// sampleEvery is the polyline pts sampled every step meters from its start
// up to upTo meters along it (or its end), both ends included.
func sampleEvery(pts []airport.LatLon, step, upTo float64) []airport.LatLon {
	if len(pts) == 0 {
		return nil
	}
	out := []airport.LatLon{pts[0]}
	done, next := 0.0, step
	for i := 1; i < len(pts) && done < upTo; i++ {
		a, b := pts[i-1], pts[i]
		l := localDist(a, b)
		for next <= done+l && next <= upTo {
			f := (next - done) / l
			out = append(out, airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: a.Lon + (b.Lon-a.Lon)*f})
			next += step
		}
		done += l
		if done <= upTo {
			out = append(out, b)
		}
	}
	return out
}

// planExitClear is PlanWayOff for an arrival planned off by an exit
// ahead (#1719): its way off clear of occ, where departures hold short.
// Another exit still ahead that it makes braking as planned (RolloutDecel
// on the roll, its planned rollout before touchdown), the first such;
// with none, before touchdown, to the end and back to one (EndTurn). Its
// own stand only (parking negative or its own); nil when none is clear,
// it is past its rollout, or it is not flown by injection. Before
// touchdown and on the roll (VacateAt plans the braking again).
func (c *ArrivalController) planExitClear(parking int, occ []airport.Occupied) func() bool {
	c.mu.Lock()
	if c.plan == nil || c.plan.EndTurn || c.plan.Backtrack || c.plan.Route == nil ||
		c.state < ArrivalApproaching || c.state > ArrivalRollout ||
		parking >= 0 && parking != c.req.Parking || !c.req.InjectApproach && !c.flyingProc {
		c.mu.Unlock()
		return nil
	}
	old, req, opts, state := c.plan, c.req, withSpan(c.req.Options, c.profile()), c.state
	along, kts := 0.0, 0.0
	if state == ArrivalRollout {
		var pos airport.LatLon
		var hdg float64
		switch {
		case c.mover != nil:
			p := c.mover.Pose()
			pos, hdg, kts = p.Position, p.Heading, p.GroundSpeedKts
		case c.approach != nil:
			p := c.approach.Pose()
			pos, hdg, kts = p.Position, p.Heading, p.GroundSpeedKts
		default:
			pos, hdg, kts = c.last.Position, c.last.Heading, c.last.GroundSpeed
		}
		along = alongHeading(old.End.Threshold, old.End.Heading, NoseGear(pos, hdg, c.profile()))
	}
	c.mu.Unlock()
	g := req.Graph
	opts.Occupied = occ
	exits, err := g.RunwayExits(old.End.Name)
	if err != nil {
		return nil
	}
	reach := exitReach{approachProfileOf(req).TouchdownKts, req.Rollout.BrakeDecel, shortFieldRun(req)}
	makes := func(x airport.RunwayExit) bool {
		if state == ArrivalRollout {
			v, vx := kts*ktsToMS, ExitSpeed(x)*ktsToMS
			return x.Along > along && x.Along-along >= (v*v-vx*vx)/(2*RolloutDecel)+rolloutExitLeadM
		}
		return landingAlong(g, old.End.Name, x) >= reach.rollout(ExitSpeed(x))
	}
	o := ArrivalOptions{SpawnNm: old.SpawnNm, Route: opts, GroundAGL: req.GroundAGL, NoseOffset: req.NoseOffset, StandSide: req.StandSide,
		TouchdownKts: approachProfileOf(req).TouchdownKts, BrakeDecel: req.Rollout.BrakeDecel, shortRun: shortFieldRun(req)}
	clear := func(x airport.RunwayExit, endTurn bool) bool {
		xo := o
		xo.Exit, xo.endTurn = &x, endTurn
		p, err := PlanArrival(g, old.End.Name, req.Parking, xo)
		return err == nil && p.EndTurn == endTurn && !g.TurnsBack(p.Route) &&
			!g.PassesOccupied(exitStretch(g, p.Route, p.VacateIndex), 0, opts)
	}
	for _, x := range exits {
		if x.Node != old.Exit.Node && makes(x) && clear(x, false) {
			return c.takeExitClear(old, x, false)
		}
	}
	if state == ArrivalRollout {
		return nil
	}
	for i := len(exits) - 1; i >= 0; i-- {
		if x := exits[i]; x.Node != old.Exit.Node && clear(x, true) {
			return c.takeExitClear(old, x, true)
		}
	}
	return nil
}

// takeExitClear is the step that puts exit x in place of plan old's
// (planExitClear), back to it from the end when endTurn: false when it was
// planned afresh meanwhile, or is past where it could.
func (c *ArrivalController) takeExitClear(old *ArrivalPlan, x airport.RunwayExit, endTurn bool) func() bool {
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.plan != old || c.state > ArrivalRollout || endTurn && c.state >= ArrivalRollout {
			return false
		}
		var err error
		if endTurn {
			if err = c.planExit(x, true, true); err == nil {
				c.exitNamed = true
			}
		} else {
			err = c.vacateAt(x)
		}
		if err != nil {
			c.note("way off clear of a departure holding short: "+x.Taxiway, err)
			return false
		}
		c.seq.add(c.now(), "way off taken: by "+x.Taxiway+", clear of a departure holding short", 0, 0)
		return true
	}
}
