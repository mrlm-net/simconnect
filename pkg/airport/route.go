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
	// TurnPenalty is the extra cost, in meters of route length, of a 90°
	// turn at a taxiway junction (proportional above TurnFreeAngle), and
	// TaxiwayChangePenalty that of turning onto another taxiway: routes with
	// fewer turns win even when a little longer. Zero means
	// DefaultTurnPenalty / DefaultTaxiwayChangePenalty; negative disables.
	TurnPenalty          float64
	TaxiwayChangePenalty float64
	// RunwayCrossingPenalty is the extra cost of each runway crossing. Zero
	// means DefaultRunwayCrossingPenalty; negative disables.
	RunwayCrossingPenalty float64
	// ApronPenalty is the extra cost, as a fraction of the length, of edges
	// at taxi points where a stand connects, so routes keep to taxiways
	// without stands. Zero means DefaultApronPenalty; negative disables.
	ApronPenalty float64
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
	// Entry is the entry taxiway set by RouteToRunwayEntry ("B" in "24 at B").
	Entry string `json:"entry,omitempty"`
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
	return g.routeVia(from, -1, to, opts)
}

// routeVia is Route for an aircraft that arrived at from via prev (-1 if
// its heading is free).
func (g *Graph) routeVia(from, prev, to NodeID, opts RouteOptions) (*Route, error) {
	s := g.shortestPaths(from, prev, opts)
	if math.IsInf(s.dist[to], 1) {
		return nil, fmt.Errorf("%w: node %d to node %d", ErrNoRoute, from, to)
	}
	return g.routeFromNodes(s.path(to)), nil
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
	s := g.shortestPaths(from, -1, opts)
	dist := s.dist

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

	r := g.routeFromNodes(s.path(best.id))
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

// usable reports whether a route may traverse e: RUNWAY paths only with
// RouteOptions.UseRunwayPaths. Taxiway edges along a runway surface are
// usable but expensive (AlongRunwayFactor), so a route can cross a runway
// where the taxiway runs a short way along it (LROP) but never backtracks
// along a runway while a taxiway will do.
func usable(e Edge, opts RouteOptions) bool {
	return e.Type != types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY || opts.UseRunwayPaths
}

// Turn costs (#307): pilots and ATC prefer routes with fewer and gentler
// turns, even when they are a little longer. The route search adds these to
// the length (Route.Length stays the real length).
const (
	// DefaultTurnPenalty is the extra cost, in meters, of a 90° turn at a
	// taxiway junction, proportional to the angle above TurnFreeAngle.
	DefaultTurnPenalty = 60.0
	// DefaultTaxiwayChangePenalty is the extra cost of turning onto a
	// differently named taxiway (going straight on where the name changes is
	// free).
	DefaultTaxiwayChangePenalty = 40.0
	// TurnFreeAngle is the largest heading change at a junction that costs
	// nothing (taxiways bend).
	TurnFreeAngle = 15.0
	// UTurnAngle is the smallest heading change counted as turning back;
	// it costs UTurnPenalty, so a route only turns back when it must.
	UTurnAngle   = 150.0
	UTurnPenalty = 2000.0
	// DefaultRunwayCrossingPenalty is the extra cost of each runway
	// crossing: a crossing needs a clearance and blocks the runway.
	DefaultRunwayCrossingPenalty = 1000.0
	// AlongRunwayFactor multiplies the cost of taxiway edges that run along
	// a runway surface (unless RouteOptions.UseRunwayPaths).
	AlongRunwayFactor = 20.0
	// DefaultApronPenalty is the extra cost, as a fraction of the length, of
	// edges at taxi points where a stand connects (apron taxilanes).
	DefaultApronPenalty = 0.5
)

func (o RouteOptions) turnPenalty() float64 {
	switch {
	case o.TurnPenalty < 0:
		return 0
	case o.TurnPenalty == 0:
		return DefaultTurnPenalty
	}
	return o.TurnPenalty
}

func (o RouteOptions) apronPenalty() float64 {
	switch {
	case o.ApronPenalty < 0:
		return 0
	case o.ApronPenalty == 0:
		return DefaultApronPenalty
	}
	return o.ApronPenalty
}

func (o RouteOptions) crossingPenalty() float64 {
	switch {
	case o.RunwayCrossingPenalty < 0:
		return 0
	case o.RunwayCrossingPenalty == 0:
		return DefaultRunwayCrossingPenalty
	}
	return o.RunwayCrossingPenalty
}

func (o RouteOptions) changePenalty() float64 {
	switch {
	case o.TaxiwayChangePenalty < 0:
		return 0
	case o.TaxiwayChangePenalty == 0:
		return DefaultTaxiwayChangePenalty
	}
	return o.TaxiwayChangePenalty
}

// turnCost is the extra cost of going on from node onto out, having arrived
// from prev on taxiway inName.
func (g *Graph) turnCost(prev, node NodeID, inName string, out Edge, opts RouteOptions) float64 {
	a, b, c := g.Nodes[prev].Position, g.Nodes[node].Position, g.Nodes[out.To].Position
	ax, az := g.local.xz(a)
	bx, bz := g.local.xz(b)
	cx, cz := g.local.xz(c)
	h1, h2 := math.Atan2(bx-ax, bz-az), math.Atan2(cx-bx, cz-bz)
	angle := math.Abs(math.Mod(math.Abs(h2-h1)*180/math.Pi+180, 360) - 180)
	cost := 0.0
	if angle >= UTurnAngle {
		cost += UTurnPenalty
	}
	junction := len(g.Adj[node]) > 2
	if junction && angle > TurnFreeAngle {
		cost += opts.turnPenalty() * (angle - TurnFreeAngle) / 90
	}
	if junction && angle > TurnFreeAngle && inName != "" && out.Name != "" && inName != out.Name {
		cost += opts.changePenalty()
	}
	return cost
}

// search is a turn-aware Dijkstra over (node, arrived-from) states.
type search struct {
	dist []float64 // cheapest cost per node
	best []int     // state with that cost, -1 if unreached
	node []NodeID  // per state
	from []int     // per state: previous state, -1 at the source
	name []string  // per state: taxiway arrived on (unnamed connectors inherit the previous name)
	cost []float64 // per state
}

// path returns the node sequence from the source to to.
func (s *search) path(to NodeID) []NodeID {
	var nodes []NodeID
	for st := s.best[to]; st != -1; st = s.from[st] {
		nodes = append(nodes, s.node[st])
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	return nodes
}

// shortestPaths runs the search from src; srcPrev, if valid, is the node the
// aircraft arrived at src from (its heading), otherwise any first direction
// is free. Parking nodes other than src are dead ends: a route may end at a
// stand but never pass through one.
func (g *Graph) shortestPaths(src, srcPrev NodeID, opts RouteOptions) *search {
	n := len(g.Nodes)
	s := &search{dist: make([]float64, n), best: make([]int, n)}
	for i := range s.dist {
		s.dist[i] = math.Inf(1)
		s.best[i] = -1
	}
	index := map[[2]NodeID]int{}
	crossed := map[[2]NodeID]int{} // runway crossings per edge
	state := func(node, prev NodeID) int {
		k := [2]NodeID{node, prev}
		if id, ok := index[k]; ok {
			return id
		}
		id := len(s.node)
		index[k] = id
		s.node, s.from, s.name, s.cost = append(s.node, node), append(s.from, -1), append(s.name, ""), append(s.cost, math.Inf(1))
		return id
	}
	start := state(src, -1)
	s.cost[start], s.dist[src], s.best[src] = 0, 0, start
	if g.valid(srcPrev) {
		s.name[start] = g.edge(srcPrev, src).Name
	}
	pq := &nodeQueue{{id: NodeID(start)}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(queued)
		st := int(cur.id)
		if cur.dist > s.cost[st] {
			continue
		}
		node := s.node[st]
		if node != src && g.Nodes[node].Kind == NodeParking {
			continue
		}
		prev := srcPrev
		if s.from[st] != -1 {
			prev = s.node[s.from[st]]
		}
		for _, e := range g.Adj[node] {
			if !usable(e, opts) {
				continue
			}
			d := cur.dist + e.Length
			if e.AlongRunway && !opts.UseRunwayPaths {
				d += (AlongRunwayFactor - 1) * e.Length
			}
			// Apron taxilanes (a stand connects at either end) cost extra, so
			// through traffic keeps to taxiways without stands (LROP: N, not M).
			if pen := opts.apronPenalty(); pen > 0 && g.stands != nil && (g.stands[node] || g.stands[e.To]) {
				d += pen * e.Length
			}
			if pen := opts.crossingPenalty(); pen > 0 {
				k := [2]NodeID{node, e.To}
				c, ok := crossed[k]
				if !ok {
					c = g.crossings(node, e.To)
					crossed[k] = c
				}
				d += pen * float64(c)
			}
			if g.valid(prev) {
				d += g.turnCost(prev, node, s.name[st], e, opts)
			}
			next := state(e.To, node)
			if d < s.cost[next] {
				nm := e.Name
				if nm == "" {
					nm = s.name[st]
				}
				s.cost[next], s.from[next], s.name[next] = d, st, nm
				if d < s.dist[e.To] {
					s.dist[e.To], s.best[e.To] = d, next
				}
				heap.Push(pq, queued{id: NodeID(next), dist: d})
			}
		}
	}
	return s
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

// onRunway returns the index of the runway whose surface contains p, or -1.
func (g *Graph) onRunway(p LatLon) int { return g.RunwayAt(p, 0) }

// RunwayAt returns the index of the runway whose surface, widened by margin
// meters on every side, contains p; -1 if none.
func (g *Graph) RunwayAt(p LatLon, margin float64) int {
	for _, r := range g.Layout.Runways {
		along, off := g.runwayCoords(r, p)
		if along >= -margin && along <= r.Length+margin && off <= r.Width/2+margin {
			return r.Index
		}
	}
	return -1
}

// crossings counts the runways the edge a → b enters, other than one a is
// already on (vacating or lining up is not a crossing).
func (g *Graph) crossings(a, b NodeID) int {
	on := g.onRunway(g.Nodes[a].Position)
	n := 0
	for _, name := range g.runwayCrossings([]LatLon{g.Nodes[a].Position, g.Nodes[b].Position}) {
		if on == -1 || name != g.Layout.Runways[on].Name() {
			n++
		}
	}
	return n
}
