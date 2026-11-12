package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Taxi-through stands (#1508): a ramp stand the scenery joins to the
// taxiways only behind it, entered forward from there, whose nose faces a
// taxiway across open apron (LKTB 2A: taxiway A 31 m ahead of the nose).
// The departure taxis straight ahead onto that taxiway, no pushback.
// taxiThroughMaxMeters: the taxiway ahead lies at most this far from the
// nose gear (0: no stand is taxi-through).
var taxiThroughMaxMeters = 45.0

const (
	// taxiThroughMinCrossDeg: the taxiway crosses the line ahead of the
	// nose at least this steeply (not one running alongside the stand).
	taxiThroughMinCrossDeg = 45.0
	// taxiThroughTurnCost: meters of taxi a degree of the turn onto the
	// taxiway costs, choosing the way along it.
	taxiThroughTurnCost = 0.5
)

// faceOutOrThrough decides, from the route planned from the stand, whether
// the departure taxis out without a pushback: a self-manoeuvring stand
// (standFacesOut), or a taxi-through one (planTaxiThrough, which re-plans
// the route from the taxiway ahead).
func (c *TaxiController) faceOutOrThrough() bool {
	c.taxiThrough = false
	if c.standFacesOut() {
		return true
	}
	c.taxiThrough = c.planTaxiThrough()
	return c.taxiThrough
}

// planTaxiThrough plans the taxi straight ahead off a taxi-through stand:
// the stand's own junction behind the nose, a taxi edge crossing the line
// ahead of it within taxiThroughMaxMeters, and no other stand's circle
// within half a span of the way there. Gates are never taxi-through (a
// building ahead). The route becomes the taxiway's node behind the
// crossing, then the taxi-out from the node ahead, the cheaper way along
// it; powerOut the crossing. False, the route unchanged, otherwise.
func (c *TaxiController) planTaxiThrough() bool {
	if len(c.route.Points) < 2 || c.inj == nil || taxiThroughMaxMeters <= 0 {
		return false
	}
	g, prof := c.req.Graph, c.profile()
	stand := g.Layout.Parking[c.req.Parking]
	if stand.IsGate() {
		return false
	}
	h := stand.Heading
	nose := NoseGear(StandPointSide(stand, c.req.NoseOffset, c.req.StandSide), h, prof)
	if alongHeading(nose, h, c.route.Points[1]) >= 0 {
		return false // its junction ahead: standFacesOut's case
	}
	type crossing struct {
		at       airport.LatLon
		ahead    float64
		from, to airport.NodeID
	}
	var near *crossing
	var all []crossing
	for a := range g.Adj {
		pa := g.Nodes[a].Position
		if g.Nodes[a].Kind == airport.NodeParking || localDist(pa, nose) > taxiThroughMaxMeters+300 {
			continue
		}
		for _, e := range g.Adj[a] {
			if !pushEdge(g, e) {
				continue
			}
			pb := g.Nodes[e.To].Position
			va, vb := alongHeading(nose, h+90, pa), alongHeading(nose, h+90, pb)
			if va == vb || va*vb > 0 {
				continue
			}
			d := math.Abs(headingDiff(h, localBearing(pa, pb)))
			if math.Min(d, 180-d) < taxiThroughMinCrossDeg {
				continue
			}
			t := va / (va - vb)
			x := airport.LatLon{Lat: pa.Lat + t*(pb.Lat-pa.Lat), Lon: pa.Lon + t*(pb.Lon-pa.Lon)}
			u := alongHeading(nose, h, x)
			if u <= 0 || u > taxiThroughMaxMeters {
				continue
			}
			cr := crossing{at: x, ahead: u, from: airport.NodeID(a), to: e.To}
			all = append(all, cr)
			if near == nil || u < near.ahead-0.5 {
				near = &all[len(all)-1]
			}
		}
	}
	if near == nil {
		return false
	}
	half := c.halfSpan()
	for _, p := range g.Layout.Parking {
		if p.Index != stand.Index && pointSegDist(p.Position, nose, near.at) < p.Radius+half {
			return false
		}
	}
	// The way along the nearest taxiway: both directions of its edges at
	// that crossing (from→to and back), the cheaper taxi-out.
	toRunway := c.metersToRunway()
	routes := map[[2]airport.NodeID]*airport.Route{}
	var best *airport.Route
	var bestFrom airport.NodeID
	bestCost := math.Inf(1)
	for _, cr := range all {
		if math.Abs(cr.ahead-near.ahead) > 0.5 {
			continue
		}
		for _, way := range [][2]airport.NodeID{{cr.from, cr.to}, {cr.to, cr.from}} {
			from, to := way[0], way[1]
			left, ok := toRunway[to]
			if !ok {
				continue
			}
			out := c.poseRoute(pushPose{from: from, to: to}, routes)
			if out == nil {
				continue
			}
			turn := math.Abs(headingDiff(h, localBearing(cr.at, g.Nodes[to].Position)))
			if cost := localDist(cr.at, g.Nodes[to].Position) + left + taxiThroughTurnCost*turn; cost < bestCost {
				best, bestFrom, bestCost = out, from, cost
			}
		}
	}
	if best == nil {
		return false
	}
	full, err := g.RouteFromNodes(append([]airport.NodeID{bestFrom}, best.Nodes...))
	if err != nil {
		return false
	}
	full.Runway, full.RunwayEnd, full.Entry, full.HoldShort, full.Tight = best.Runway, best.RunwayEnd, best.Entry, best.HoldShort, best.Tight
	c.route, c.pushJunction, c.powerOut = full, 1, []airport.LatLon{near.at}
	return true
}
