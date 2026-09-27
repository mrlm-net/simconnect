//go:build windows
// +build windows

package airport

import (
	"container/heap"
	"fmt"
	"math"
	"sort"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// RouteOptions controls which edges a route may use.
type RouteOptions struct {
	// UseRunwayPaths allows taxiing along RUNWAY paths (back-tracking or
	// using a runway as a taxiway). Crossing a runway on a taxiway is always
	// allowed and reported in Route.RunwayCrossings.
	UseRunwayPaths bool
	// IntersectionTolerance selects hold-short points for RouteToRunway: any
	// hold-short within this many meters (along the runway) of the one nearest
	// the threshold is a candidate, and the shortest route among them wins.
	// Zero means DefaultIntersectionTolerance.
	IntersectionTolerance float64
}

// DefaultIntersectionTolerance is the RouteOptions.IntersectionTolerance used
// when none is set.
const DefaultIntersectionTolerance = 300.0

// Route is a taxi route through the graph.
type Route struct {
	Nodes  []NodeID `json:"nodes"`
	Points []LatLon `json:"points"`
	Edges  []Edge   `json:"edges"`  // Edges[i] leads from Nodes[i] to Nodes[i+1]
	Length float64  `json:"length"` // meters
	// Taxiways is the sequence of taxiway names along the route with
	// consecutive repeats and unnamed segments removed, e.g. ["C", "L", "D"].
	Taxiways []string `json:"taxiways"`
	// RunwayCrossings names each runway the route crosses, in order.
	RunwayCrossings []string `json:"runwayCrossings"`
	// Runway and RunwayEnd are set by RouteToRunway.
	Runway    string `json:"runway,omitempty"`
	RunwayEnd string `json:"runwayEnd,omitempty"`
	// HoldShort is the final node's hold-short data when the route ends at one.
	HoldShort *HoldShort `json:"holdShort,omitempty"`
}

// ParkingIndex resolves a parking label such as "C22" to a parking index.
// It returns ErrUnknownParking when no spot matches and ErrAmbiguousParking
// when several do.
func (l *Layout) ParkingIndex(label string) (int, error) {
	m := l.ParkingByLabel(label)
	switch len(m) {
	case 0:
		return 0, fmt.Errorf("%w: %q", ErrUnknownParking, label)
	case 1:
		return m[0].Index, nil
	}
	idx := make([]int, len(m))
	for i, p := range m {
		idx[i] = p.Index
	}
	return 0, fmt.Errorf("%w: %q (indexes %v)", ErrAmbiguousParking, label, idx)
}

// Route returns the shortest route from one node to another.
func (g *Graph) Route(from, to NodeID, opts RouteOptions) (*Route, error) {
	if !g.valid(from) || !g.valid(to) {
		return nil, fmt.Errorf("%w: node out of range", ErrNoRoute)
	}
	dist, prev := g.shortestPaths(from, opts)
	if math.IsInf(dist[to], 1) {
		return nil, fmt.Errorf("%w: node %d to node %d", ErrNoRoute, from, to)
	}
	return g.buildRoute(from, to, prev), nil
}

// RouteToRunway returns a departure taxi route from a parking spot to a
// hold-short point of the given runway end (e.g. "24"). Runway holding points
// are preferred over ILS critical area holds; among holds within
// IntersectionTolerance of the one nearest the threshold, the shortest route
// wins, so aircraft depart from (or near) the full runway length.
func (g *Graph) RouteToRunway(parking int, runwayEnd string, opts RouteOptions) (*Route, error) {
	from, ok := g.ParkingNode(parking)
	if !ok {
		return nil, fmt.Errorf("%w: index %d", ErrUnknownParking, parking)
	}
	rwy, end, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return nil, fmt.Errorf("%w: %q at %s", ErrUnknownRunway, runwayEnd, g.Layout.ICAO)
	}
	holds := g.HoldShortNodes(rwy.Index)
	if len(holds) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoHoldShort, rwy.Name())
	}
	dist, prev := g.shortestPaths(from, opts)

	type cand struct {
		id         NodeID
		fromThresh float64
		route      float64
		ils        bool
	}
	var cands []cand
	for _, id := range holds {
		if math.IsInf(dist[id], 1) {
			continue
		}
		h := g.Nodes[id].HoldShort
		along := h.FromPrimary
		if end.Name == rwy.Secondary.Name {
			along = rwy.Length - h.FromPrimary
		}
		cands = append(cands, cand{id: id, fromThresh: along, route: dist[id], ils: h.ILS})
	}
	if len(cands) == 0 {
		return nil, fmt.Errorf("%w: no reachable hold-short for runway %s", ErrNoRoute, end.Name)
	}
	// Prefer runway holding points; fall back to ILS holds only if none is reachable.
	var runwayHolds []cand
	for _, c := range cands {
		if !c.ils {
			runwayHolds = append(runwayHolds, c)
		}
	}
	if len(runwayHolds) > 0 {
		cands = runwayHolds
	}
	tol := opts.IntersectionTolerance
	if tol <= 0 {
		tol = DefaultIntersectionTolerance
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].fromThresh < cands[j].fromThresh })
	best := cands[0]
	for _, c := range cands[1:] {
		if c.fromThresh-cands[0].fromThresh > tol {
			break
		}
		if c.route < best.route {
			best = c
		}
	}

	r := g.buildRoute(from, best.id, prev)
	r.Runway, r.RunwayEnd = rwy.Name(), end.Name
	return r, nil
}

// RouteToParking returns a taxi-in route from any node to a parking spot.
func (g *Graph) RouteToParking(from NodeID, parking int, opts RouteOptions) (*Route, error) {
	to, ok := g.ParkingNode(parking)
	if !ok {
		return nil, fmt.Errorf("%w: index %d", ErrUnknownParking, parking)
	}
	return g.Route(from, to, opts)
}

// usable reports whether a route may traverse e.
func usable(e Edge, opts RouteOptions) bool {
	return (e.Type != types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY && !e.AlongRunway) || opts.UseRunwayPaths
}

// shortestPaths runs Dijkstra from src. Parking nodes other than src are
// dead ends: a route may end at a stand but never pass through one.
func (g *Graph) shortestPaths(src NodeID, opts RouteOptions) (dist []float64, prev []NodeID) {
	n := len(g.Nodes)
	dist = make([]float64, n)
	prev = make([]NodeID, n)
	for i := range dist {
		dist[i] = math.Inf(1)
		prev[i] = -1
	}
	dist[src] = 0
	pq := &nodeQueue{{id: src}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(queued)
		if cur.dist > dist[cur.id] {
			continue
		}
		if cur.id != src && g.Nodes[cur.id].Kind == NodeParking {
			continue
		}
		for _, e := range g.Adj[cur.id] {
			if !usable(e, opts) {
				continue
			}
			if d := cur.dist + e.Length; d < dist[e.To] {
				dist[e.To] = d
				prev[e.To] = cur.id
				heap.Push(pq, queued{id: e.To, dist: d})
			}
		}
	}
	return dist, prev
}

// buildRoute walks prev back from to and assembles the Route.
func (g *Graph) buildRoute(from, to NodeID, prev []NodeID) *Route {
	var nodes []NodeID
	for n := to; n != -1; n = prev[n] {
		nodes = append(nodes, n)
		if n == from {
			break
		}
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	return g.routeFromNodes(nodes)
}

// routeFromNodes assembles a Route along consecutive, adjacent nodes.
func (g *Graph) routeFromNodes(nodes []NodeID) *Route {
	r := &Route{Nodes: nodes, Taxiways: []string{}, RunwayCrossings: []string{}}
	for i, id := range nodes {
		r.Points = append(r.Points, g.Nodes[id].Position)
		if i == 0 {
			continue
		}
		e := g.edge(nodes[i-1], id)
		r.Edges = append(r.Edges, e)
		r.Length += e.Length
		if e.Name != "" && (len(r.Taxiways) == 0 || r.Taxiways[len(r.Taxiways)-1] != e.Name) {
			r.Taxiways = append(r.Taxiways, e.Name)
		}
	}
	r.RunwayCrossings = g.runwayCrossings(r.Points)
	r.HoldShort = g.Nodes[nodes[len(nodes)-1]].HoldShort
	return r
}

// edge returns the shortest edge from a to b.
func (g *Graph) edge(a, b NodeID) Edge {
	best := Edge{To: b, Length: math.Inf(1)}
	for _, e := range g.Adj[a] {
		if e.To == b && e.Length < best.Length {
			best = e
		}
	}
	return best
}

// runwayCrossings lists the runways whose surface the polyline enters, in
// order, sampling every few meters.
func (g *Graph) runwayCrossings(pts []LatLon) []string {
	out := []string{}
	inside := -1
	for i := 1; i < len(pts); i++ {
		d := g.distance(pts[i-1], pts[i])
		steps := int(d/5) + 1
		for s := 1; s <= steps; s++ {
			f := float64(s) / float64(steps)
			p := LatLon{Lat: pts[i-1].Lat + (pts[i].Lat-pts[i-1].Lat)*f, Lon: pts[i-1].Lon + (pts[i].Lon-pts[i-1].Lon)*f}
			on := -1
			for _, r := range g.Layout.Runways {
				along, off := g.runwayCoords(r, p)
				if along >= 0 && along <= r.Length && off <= r.Width/2 {
					on = r.Index
					break
				}
			}
			if on != -1 && on != inside {
				out = append(out, g.Layout.Runways[on].Name())
			}
			inside = on
		}
	}
	return out
}

func (g *Graph) valid(id NodeID) bool { return id >= 0 && int(id) < len(g.Nodes) }

type queued struct {
	id   NodeID
	dist float64
}

type nodeQueue []queued

func (q nodeQueue) Len() int           { return len(q) }
func (q nodeQueue) Less(i, j int) bool { return q[i].dist < q[j].dist }
func (q nodeQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *nodeQueue) Push(x any)        { *q = append(*q, x.(queued)) }
func (q *nodeQueue) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}
