package airport

import "math"

// Occupied is a place on the ground another aircraft takes: where it stands,
// or the path it pushes back along, with half its wing span. A route search
// with RouteOptions.Occupied keeps off the taxiway edges where the two could
// not pass: an edge that comes within both half spans and the wingtip margin
// of it. Generic, any airport: at LKPR a B738 pushed onto JB leaves J beside
// it too narrow to pass, while JO, further out, is wide enough for a CRJ
// (and JO's own span limit keeps the wide-bodies off it).
type Occupied struct {
	Points   []LatLon `json:"points"`
	HalfSpan float64  `json:"halfSpan"`
}

// clearOf reports whether the aircraft (HalfSpan) taxiing along e from node
// passes every Occupied place with WingtipMargin to spare.
func (o RouteOptions) clearOf(g *Graph, from NodeID, e Edge) bool {
	if len(o.Occupied) == 0 {
		return true
	}
	a, b := g.Nodes[from].Position, g.Nodes[e.To].Position
	margin := o.WingtipMargin
	switch {
	case margin < 0:
		margin = 0
	case margin == 0:
		margin = DefaultWingtipMargin
	}
	for _, oc := range o.Occupied {
		if occupiedNear(a, b, oc.Points, o.HalfSpan+oc.HalfSpan+margin) {
			return false
		}
	}
	return true
}

// PassesOccupied reports whether r, from edge index from on, comes too near
// any of opts.Occupied for the aircraft (opts.HalfSpan) to pass: the route a
// search with Occupied would avoid.
func (g *Graph) PassesOccupied(r *Route, from int, opts RouteOptions) bool {
	if r == nil {
		return false
	}
	for i := max(from, 0); i < len(r.Edges); i++ {
		if !opts.clearOf(g, r.Nodes[i], r.Edges[i]) {
			return true
		}
	}
	return false
}

// occupiedNear reports whether segment a–b comes within meters of the
// polyline pts (a single point: a standing aircraft).
func occupiedNear(a, b LatLon, pts []LatLon, meters float64) bool {
	cos := math.Cos(a.Lat * math.Pi / 180)
	xy := func(p LatLon) (float64, float64) {
		return (p.Lon - a.Lon) * cos * 111320, (p.Lat - a.Lat) * 110540
	}
	bx, by := xy(b)
	for i, p := range pts {
		px, py := xy(p)
		if i == 0 || len(pts) == 1 {
			if segPointDist(0, 0, bx, by, px, py) < meters {
				return true
			}
			continue
		}
		qx, qy := xy(pts[i-1])
		if segSegDist(0, 0, bx, by, qx, qy, px, py) < meters {
			return true
		}
	}
	return false
}

// segPointDist is the distance from (px, py) to segment (ax, ay)–(bx, by).
func segPointDist(ax, ay, bx, by, px, py float64) float64 {
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l))
	}
	return math.Hypot(ax+t*dx-px, ay+t*dy-py)
}

// segSegDist is the distance between two segments: zero when they cross,
// else the nearest of their ends to the other segment.
func segSegDist(ax, ay, bx, by, cx, cy, dx, dy float64) float64 {
	cross := func(ox, oy, px, py, qx, qy float64) float64 { return (px-ox)*(qy-oy) - (py-oy)*(qx-ox) }
	d1, d2 := cross(ax, ay, bx, by, cx, cy), cross(ax, ay, bx, by, dx, dy)
	d3, d4 := cross(cx, cy, dx, dy, ax, ay), cross(cx, cy, dx, dy, bx, by)
	if ((d1 > 0) != (d2 > 0)) && ((d3 > 0) != (d4 > 0)) {
		return 0
	}
	return math.Min(math.Min(segPointDist(ax, ay, bx, by, cx, cy), segPointDist(ax, ay, bx, by, dx, dy)),
		math.Min(segPointDist(cx, cy, dx, dy, ax, ay), segPointDist(cx, cy, dx, dy, bx, by)))
}

// RouteToParkingFrom is RouteToParking for an aircraft at node from that
// arrived there from prev (-1 if its heading is free): the route does not
// turn back into prev. A taxiing arrival re-planned on its way (round an
// aircraft pushed back into its path) goes on this way.
func (g *Graph) RouteToParkingFrom(from, prev NodeID, parking int, opts RouteOptions) (*Route, error) {
	to, ok := g.ParkingNode(parking)
	if !ok {
		return nil, ErrUnknownParking
	}
	if !g.valid(from) || (prev >= 0 && !g.valid(prev)) {
		return nil, ErrNoRoute
	}
	opts.OwnStands = append(append([]int(nil), opts.OwnStands...), parking)
	return g.fitOrTight(opts, func(o RouteOptions) (*Route, error) { return g.routeVia(from, prev, to, o) })
}
