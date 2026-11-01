package traffic

import (
	"math"
	"slices"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// AvoidOccupied re-plans an injected arrival's taxi-in round occ, places
// other aircraft take (one pushed back into its way), when the route ahead
// passes one too near for the two to pass (airport.Occupied) and another
// route keeps clear: from a route node far enough ahead to turn there, on
// to the stand as RouteToParkingFrom finds it. It reports whether the
// route changed. Only waiting for the taxi clearance or taxiing, with no
// clearance limit and no runway crossing left (they are planned along the
// old path).
func (c *ArrivalController) AvoidOccupied(occ []airport.Occupied) bool {
	if apply := c.PlanAvoidOccupied(occ); apply != nil {
		return apply()
	}
	return false
}

// PlanAvoidOccupied is AvoidOccupied's route search without the
// controller's lock held during it: nil when the route stays, else the
// step that puts the new route in place, for the connection's goroutine
// (false: the aircraft moved on meanwhile, nothing changed).
func (c *ArrivalController) PlanAvoidOccupied(occ []airport.Occupied) func() bool {
	c.mu.Lock()
	if c.mover == nil || c.plan == nil || len(occ) == 0 || (c.state != ArrivalAwaitingTaxi && c.state != ArrivalTaxiing) ||
		c.hasLimit || c.hasPendingLimit {
		c.mu.Unlock()
		return nil
	}
	g, prof, old, plan := c.req.Graph, c.profile(), c.plan.Route, c.plan
	pose := c.mover.Pose()
	v := pose.GroundSpeedKts * ktsToMS
	ahead := math.Max(prof.WheelbaseMeters-prof.RefAheadMeters, v*v/(2*prof.Decel)+avoidTurnMeters)
	// At or on a crossing: not now. One further on is planned afresh with
	// the new route (live, LKPR: TVS681's taxi-in crossed 12/30, so it
	// could not be sent round CSA398 coming the other way along F).
	for _, z := range c.crossZones {
		if pose.Distance+ahead >= z.from && pose.Distance < z.to {
			c.mu.Unlock()
			return nil
		}
	}
	k := c.track.segmentAt(c.track.pos+ahead) + 1
	opts, parking := c.req.Options, c.req.Parking
	c.mu.Unlock()
	if k < 1 || k <= plan.VacateIndex || k >= len(old.Nodes)-1 {
		return nil
	}
	opts.Occupied = occ
	if !g.PassesOccupied(old, k, opts) {
		return nil
	}
	r, err := g.RouteToParkingFrom(old.Nodes[k], old.Nodes[k-1], parking, opts)
	if err != nil || r.Occupied || r.Tight || g.TurnsBack(r) || slices.Equal(r.Nodes, old.Nodes[k:]) {
		return nil
	}
	// Not much longer, across no runway the old way did not cross.
	rest, err := g.RouteFromNodes(old.Nodes[k:])
	if err != nil || !shortDetour(r.Length, rest.Length) {
		return nil
	}
	for _, x := range r.RunwayCrossings {
		if !slices.Contains(rest.RunwayCrossings, x) {
			return nil
		}
	}
	full, err := g.RouteFromNodes(append(append([]airport.NodeID(nil), old.Nodes[:k]...), r.Nodes...))
	if err != nil {
		return nil
	}
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.plan != plan || c.mover == nil || (c.state != ArrivalAwaitingTaxi && c.state != ArrivalTaxiing) || c.hasLimit || c.hasPendingLimit {
			return false
		}
		prof := c.profile()
		pose := c.mover.Pose()
		if c.track.segmentAt(c.track.pos+prof.WheelbaseMeters-prof.RefAheadMeters)+1 >= k {
			return false // past where it would turn off meanwhile
		}
		nose := NoseGear(pose.Position, pose.Heading, prof)
		from := c.track.segmentAt(c.track.pos+prof.WheelbaseMeters-prof.RefAheadMeters) + 1
		oldPlan, oldTrack := c.plan, c.track
		np := *c.plan
		np.Route = full
		c.plan = &np
		c.track = newRouteTracker(full)
		c.track.pos = oldTrack.pos // the same route up to k
		path, zones, err := c.taxiPathFrom(nose, from)
		if err != nil {
			c.plan, c.track = oldPlan, oldTrack
			return false
		}
		c.mover = NewGroundMoverFrom(path, prof, pose.Heading, pose.GroundSpeedKts)
		if c.state == ArrivalAwaitingTaxi {
			c.mover.HoldAt(0) // standing until cleared (startTaxi)
		}
		c.crossZones, c.nextCross = zones, 0
		c.note("taxi-in re-planned round an occupied taxiway", nil)
		c.emit(nil, true)
		return true
	}
}

// avoidTurnMeters: a taxi-in re-planned on the way turns off at a route
// node at least this far beyond where the aircraft can stop.
const avoidTurnMeters = 10.0

// A re-planned taxi route may be at most avoidDetourShare longer than the
// way it replaces and avoidDetourMeters more: longer, the aircraft keeps
// its way and holds (live, LKPR: round CSA398 on F, TVS681 would have
// gone D, L, G, J, H, 2.7 km for 1.4, across 06/24).
const (
	avoidDetourShare  = 0.5
	avoidDetourMeters = 500.0
)

// shortDetour reports whether a new way of length meters is short enough
// to replace one of old.
func shortDetour(length, old float64) bool {
	return length <= old*(1+avoidDetourShare) && length <= old+avoidDetourMeters
}

// taxiPathFrom is the taxi-in ground path from the nose gear along the
// plan's route from point from on, onto the stand: takeover's path clear of
// the runway, and the runway crossings along it.
func (c *ArrivalController) taxiPathFrom(nose airport.LatLon, from int) (*GroundPath, []crossZone, error) {
	prof := c.profile()
	route := c.plan.Route.Points
	stopNose := NoseGear(c.plan.Stop, c.standHeading, prof)
	last, axisM := standJoin(route, from, nose, stopNose, c.standHeading)
	axis := offsetHeading(stopNose, c.standHeading, -axisM)
	faceOut := axisM == 0 || turnsOnStand(c.req.Graph, c.req.Parking, route, from)
	if faceOut {
		last = max(last, len(route)-2)
	}
	pts := []airport.LatLon{nose}
	apron := apronSpans{g: c.req.Graph}
	var holds []holdOnPath
	for i := from; i <= last; i++ {
		d := pathLen(pts) + localDist(pts[len(pts)-1], route[i])
		apron.add(c.plan.Route.Nodes[i], d)
		if hs := c.req.Graph.Nodes[c.plan.Route.Nodes[i]].HoldShort; hs != nil && hs.Runway != c.plan.Runway.Index {
			holds = append(holds, holdOnPath{runway: hs.Runway, index: i, dist: d})
		}
		pts = append(pts, route[i])
	}
	if faceOut {
		pts = append(pts, c.turnAround(stopNose)...)
	} else {
		pts = append(pts, axis, stopNose)
	}
	path, err := newGroundPath(pts, prof, firmZone{})
	if err != nil {
		return nil, nil, err
	}
	path.LimitRange(0, path.Length(), prof.CruiseKts, prof.Decel)
	apron.limit(path, c.req.Airport, prof.Decel)
	path.LimitEnd(StandSlowMeters, StandTaxiSpeedKts)
	return path, crossingZones(c.req.Graph, route, holds), nil
}
