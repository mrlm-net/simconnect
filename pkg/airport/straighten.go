package airport

import "math"

// Straight on along a taxiway (live LOWW: "via L, EX9, M, EX6, L", over to
// the parallel M and back where L itself goes on). A route that leaves a
// taxiway and joins it again later stays on it instead, when the way along
// it is shorter than the detour, the aircraft fits it and keeps clear of
// what RouteOptions.Occupied holds, and it crosses no runway the detour
// did not. The detour usually comes from the apron penalty of a taxiway
// with stands beside it (LROP: through traffic keeps to N, not M); a
// controller still sends nobody across to the parallel and back for it.

// straightenMax: at most this many detours taken out of one route.
const straightenMax = 8

// straighten takes out a route's detours off a taxiway it comes back to
// (nodes, and edges[i] from nodes[i] to nodes[i+1]).
func (g *Graph) straighten(nodes []NodeID, edges []Edge, opts RouteOptions) ([]NodeID, []Edge) {
	for range straightenMax {
		n, e, ok := g.straightenOnce(nodes, edges, opts)
		if !ok {
			break
		}
		nodes, edges = n, e
	}
	return nodes, edges
}

// straightenOnce takes out the first detour it can; false when none.
func (g *Graph) straightenOnce(nodes []NodeID, edges []Edge, opts RouteOptions) ([]NodeID, []Edge, bool) {
	for i := 0; i+1 < len(edges); i++ {
		name := edges[i].Name
		if name == "" || edges[i+1].Name == name {
			continue
		}
		// Left name at nodes[i+1]: where does the route come back to it?
		for j := i + 2; j < len(edges); j++ {
			if edges[j].Name != name {
				continue
			}
			a, b := nodes[i+1], nodes[j]
			detour, crossings := 0.0, 0
			for k := i + 1; k < j; k++ {
				detour += edges[k].Length
				crossings += g.crossings(nodes[k], nodes[k+1])
			}
			path, way, ok := g.alongTaxiway(name, nodes[i], a, b, nodes[j+1], crossings, opts)
			if !ok || way >= detour-1 {
				break
			}
			out := append(append([]NodeID{}, nodes[:i+1]...), path...)
			out = append(out, nodes[j+1:]...)
			outEdges := append([]Edge{}, edges[:i+1]...)
			for k := 1; k < len(path); k++ {
				outEdges = append(outEdges, g.edge(path[k-1], path[k]))
			}
			outEdges = append(outEdges, edges[j:]...)
			return out, outEdges, true
		}
	}
	return nil, nil, false
}

// alongTaxiway is the shortest way from a to b on edges of taxiway name
// only, arriving at a from prev and going on from b to next without
// turning back at either end or on the way, on edges the aircraft fits and
// that keep clear, with at most crossings runway crossings: the nodes a…b
// and the length.
func (g *Graph) alongTaxiway(name string, prev, a, b, next NodeID, crossings int, opts RouteOptions) ([]NodeID, float64, bool) {
	dist := map[NodeID]float64{a: 0}
	from := map[NodeID]NodeID{a: prev}
	cross := map[NodeID]int{a: 0}
	done := map[NodeID]bool{}
	for {
		cur, best := NodeID(-1), math.Inf(1)
		for id, d := range dist {
			if !done[id] && d < best {
				cur, best = id, d
			}
		}
		if cur < 0 {
			return nil, 0, false
		}
		done[cur] = true
		if cur == b {
			if g.valid(next) && g.turnAngle(from[cur], cur, next) >= UTurnAngle {
				return nil, 0, false
			}
			path := []NodeID{b}
			for id := b; id != a; {
				id = from[id]
				path = append(path, id)
			}
			for l, r := 0, len(path)-1; l < r; l, r = l+1, r-1 {
				path[l], path[r] = path[r], path[l]
			}
			return path, best, true
		}
		for _, e := range g.Adj[cur] {
			if e.Name != name || done[e.To] || g.Nodes[e.To].Kind == NodeParking {
				continue
			}
			if !usable(e, opts) || !opts.fits(e) || !opts.clearOf(g, cur, e) {
				continue
			}
			if p := from[cur]; g.valid(p) && g.turnAngle(p, cur, e.To) >= UTurnAngle {
				continue
			}
			c := cross[cur] + g.crossings(cur, e.To)
			if c > crossings {
				continue
			}
			if d := best + e.Length; d < dist[e.To] || !has(dist, e.To) {
				dist[e.To], from[e.To], cross[e.To] = d, cur, c
			}
		}
	}
}

func has(m map[NodeID]float64, id NodeID) bool {
	_, ok := m[id]
	return ok
}
