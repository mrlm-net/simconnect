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
// reports whether the route changed.
func (c *TaxiController) AvoidOccupied(occ []airport.Occupied) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil || c.mover == nil || len(occ) == 0 || c.state != TaxiTaxiing || c.hasLimit || c.hasPendingLimit || c.hereVia != nil || c.track == nil {
		return false
	}
	g := c.req.Graph
	opts := c.req.Options
	opts.Occupied = occ
	seg := c.track.segmentAt(c.track.pos)
	if !g.PassesOccupied(c.route, seg, opts) {
		return false
	}
	rest, err := g.RouteFromNodes(c.route.Nodes[max(seg, 0):])
	if err != nil {
		return false
	}
	route, junction, fromHere, track := c.route, c.pushJunction, c.fromHere, c.track
	restore := func() { c.route, c.pushJunction, c.fromHere, c.track = route, junction, fromHere, track }
	c.req.Options.Occupied = occ
	err = c.routeFromHere()
	c.req.Options.Occupied = nil
	if err != nil || c.route.Occupied || c.route.Tight || g.TurnsBack(c.route) || !slices.Equal(c.route.RunwayCrossings, rest.RunwayCrossings) {
		restore()
		return false
	}
	if err := c.startTaxiOut(); err != nil {
		restore()
		return false
	}
	c.note("taxi-out re-planned round an occupied taxiway", nil)
	c.emit(nil, true)
	return true
}
