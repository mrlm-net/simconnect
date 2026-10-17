package traffic

import (
	"slices"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Occupies is the place an injected departure takes on the taxiways for
// others to keep clear of (airport.Occupied): cleared to push, the push path
// still ahead of it; pushed back and waiting for its taxi clearance, where
// it stands. Otherwise false: taxiing, it moves on (the ground picture has
// others follow or give way).
func (c *TaxiController) Occupies() (airport.Occupied, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	half := c.req.Options.HalfSpan
	switch {
	case c.state == TaxiAwaitingPushback && c.pushCleared && c.pushPlanned != nil:
		return airport.Occupied{Points: thin(c.pushPlanned.pts, occupiedStepMeters), HalfSpan: half}, true
	case c.state == TaxiPushback && c.mover != nil:
		path, pose := c.mover.Path(), c.mover.Pose()
		pts := []airport.LatLon{pose.Position}
		for i, d := range path.cum {
			if d > pose.Distance {
				pts = append(pts, path.pts[i])
			}
		}
		return airport.Occupied{Points: thin(pts, occupiedStepMeters), HalfSpan: half}, true
	case c.state == TaxiAwaitingTaxi && c.mover != nil:
		return airport.Occupied{Points: []airport.LatLon{c.mover.Pose().Position}, HalfSpan: half}, true
	}
	return airport.Occupied{}, false
}

// occupiedStepMeters thins an occupied path: points about this far apart.
const occupiedStepMeters = 5.0

// AvoidOccupied re-plans an injected departure's taxi-out round occ, as
// ArrivalController.AvoidOccupied does an arrival's taxi-in: taxiing, from
// the taxiway under its nose, with no clearance limit and no custom route,
// and only to a route that keeps clear and crosses the same runways. It
// reports whether the route changed. PlanAvoidOccupied does it in two
// steps, the search off the caller's goroutine.
func (c *TaxiController) AvoidOccupied(occ []airport.Occupied) bool {
	if apply := c.PlanAvoidOccupied(occ); apply != nil {
		return apply()
	}
	return false
}

// PlanAvoidOccupied is AvoidOccupied's route search, without the
// controller's lock held during it (a frame waits for that lock): nil
// when the route stays, else the step that puts the new route in place,
// for the connection's goroutine (false: the aircraft moved on meanwhile,
// nothing changed). Live, the search held every aircraft back 80–90 ms
// each time it ran (LKPR, CSA1701 every second behind a push).
func (c *TaxiController) PlanAvoidOccupied(occ []airport.Occupied) func() bool {
	c.mu.Lock()
	if c.inj == nil || c.mover == nil || len(occ) == 0 || c.state != TaxiTaxiing || c.hasLimit || c.hasPendingLimit || c.hereVia != nil || c.track == nil {
		c.mu.Unlock()
		return nil
	}
	g, opts, route, prof, pose := c.req.Graph, c.req.Options, c.route, c.profile(), c.mover.Pose()
	runway, entry := c.req.Runway, c.req.Entry
	seg := c.track.segmentAt(c.track.pos)
	c.mu.Unlock()
	opts.Occupied = occ
	if !g.PassesOccupied(route, seg, opts) {
		return nil
	}
	rest, err := g.RouteFromNodes(route.Nodes[max(seg, 0):])
	if err != nil {
		return nil
	}
	opts.Via, opts.Taxiways = nil, nil
	full, err := routeHere(g, prof, pose, runway, entry, opts)
	if err != nil || full.Occupied || full.Tight || g.TurnsBack(full) || !slices.Equal(full.RunwayCrossings, rest.RunwayCrossings) ||
		slices.Equal(full.Nodes[1:], rest.Nodes[min(1, len(rest.Nodes)):]) {
		return nil // none that keeps clear, or the same way
	}
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.state != TaxiTaxiing || c.route != route || c.hasLimit || c.hasPendingLimit || c.hereVia != nil {
			return false
		}
		junction, fromHere, track := c.pushJunction, c.fromHere, c.track
		c.route, c.pushJunction, c.fromHere, c.track = full, 0, true, newRouteTracker(full)
		if err := c.startTaxiOut(); err != nil {
			c.route, c.pushJunction, c.fromHere, c.track = route, junction, fromHere, track
			return false
		}
		c.note("taxi-out re-planned round an occupied taxiway", nil)
		c.emit(nil, true)
		return true
	}
}
