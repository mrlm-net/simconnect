//go:build windows
// +build windows

package airport

import (
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// ErrNoExit is returned when no runway exit suits the request.
var ErrNoExit = fmt.Errorf("airport: no runway exit")

// Exit geometry.
const (
	// exitSurfaceMargin is how far beyond the runway half-width a node still
	// counts as on the runway surface.
	exitSurfaceMargin = 5.0
	// exitMaxPath bounds the search from a runway node to the first node off
	// the runway surface.
	exitMaxPath = 250.0
	// HighSpeedExitAngle is the largest turn-off angle of a high-speed
	// (rapid) exit, in degrees.
	HighSpeedExitAngle = 45.0
	// MaxExitAngle is the largest turn-off angle a landing aircraft can take
	// (and, for entries, the largest turn onto the runway); larger angles point
	// back along the runway and are skipped.
	MaxExitAngle = 90.0
	// exitHoldShortSearch bounds the search for the hold-short behind an exit.
	exitHoldShortSearch = 300.0
)

// ExitSide is the side of the runway an exit leaves on, seen in the landing
// direction.
type ExitSide uint8

const (
	ExitLeft ExitSide = iota
	ExitRight
)

func (s ExitSide) String() string {
	if s == ExitLeft {
		return "left"
	}
	return "right"
}

// RunwayExit is a taxiway leaving a runway, for aircraft landing on a given
// runway end.
type RunwayExit struct {
	// RunwayNode is where the exit leaves the runway centreline.
	RunwayNode NodeID `json:"runwayNode"`
	// Node is the first taxiway node off the runway surface.
	Node NodeID `json:"node"`
	// Path runs from RunwayNode to Node.
	Path []NodeID `json:"path"`
	// Along is the distance from the landing threshold to RunwayNode, in meters.
	Along float64 `json:"along"`
	// Angle is the turn-off angle from the landing direction, 0–180°.
	Angle float64 `json:"angle"`
	// HighSpeed reports Angle ≤ HighSpeedExitAngle.
	HighSpeed bool     `json:"highSpeed"`
	Side      ExitSide `json:"side"`
	// Taxiway is the name of the exit taxiway, "" if unnamed.
	Taxiway string `json:"taxiway"`
	// HoldShort is the hold-short node behind the exit for this runway, -1 if
	// none within exitHoldShortSearch.
	HoldShort NodeID `json:"holdShort"`
}

// RunwayExits returns the exits usable by aircraft landing on runwayEnd,
// sorted by distance from the landing threshold. Exits turning back towards
// the threshold (Angle > MaxExitAngle) are left out.
func (g *Graph) RunwayExits(runwayEnd string) ([]RunwayExit, error) {
	rwy, end, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return nil, fmt.Errorf("%w: %q at %s", ErrUnknownRunway, runwayEnd, g.Layout.ICAO)
	}
	fromPrimary := end.Name == rwy.Primary.Name
	surface := rwy.Width/2 + exitSurfaceMargin
	onSurface := func(id NodeID) bool {
		along, off := g.runwayCoords(rwy, g.Nodes[id].Position)
		return off <= surface && along >= -surface && along <= rwy.Length+surface
	}

	// Runway nodes: endpoints of RUNWAY edges on this runway's surface.
	var runwayNodes []NodeID
	for id, edges := range g.Adj {
		for _, e := range edges {
			if e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY && onSurface(NodeID(id)) {
				runwayNodes = append(runwayNodes, NodeID(id))
				break
			}
		}
	}

	seen := map[[2]NodeID]bool{}
	var exits []RunwayExit
	for _, rn := range runwayNodes {
		for _, path := range g.pathsOffSurface(rn, onSurface) {
			exitNode := path[len(path)-1]
			key := [2]NodeID{rn, exitNode}
			if seen[key] {
				continue
			}
			seen[key] = true

			along, _ := g.runwayCoords(rwy, g.Nodes[rn].Position)
			if !fromPrimary {
				along = rwy.Length - along
			}
			ax, az := g.local.xz(g.Nodes[rn].Position)
			bx, bz := g.local.xz(g.Nodes[exitNode].Position)
			brg := math.Mod(math.Atan2(bx-ax, bz-az)*180/math.Pi+360, 360)
			rel := math.Mod(brg-end.Heading+540, 360) - 180 // -180..180, positive = right
			angle := math.Abs(rel)
			if angle > MaxExitAngle {
				continue
			}
			side := ExitRight
			if rel < 0 {
				side = ExitLeft
			}
			exits = append(exits, RunwayExit{
				RunwayNode: rn, Node: exitNode, Path: path, Along: math.Max(0, along),
				Angle: angle, HighSpeed: angle <= HighSpeedExitAngle, Side: side,
				Taxiway: g.exitName(path), HoldShort: g.holdShortBehind(exitNode, rwy.Index),
			})
		}
	}
	// Taxiway segments can run along the runway before turning off, so several
	// runway nodes may lead to the same exit node. Keep the shortest path: its
	// runway node is where the taxiway actually leaves the centreline.
	best := map[NodeID]int{}
	for i, e := range exits {
		if j, ok := best[e.Node]; !ok || g.pathLength(e.Path) < g.pathLength(exits[j].Path) {
			best[e.Node] = i
		}
	}
	uniq := make([]RunwayExit, 0, len(best))
	for _, i := range best {
		uniq = append(uniq, exits[i])
	}
	sort.Slice(uniq, func(i, j int) bool { return uniq[i].Along < uniq[j].Along })
	return uniq, nil
}

// ExitFor returns the first exit at least rollout meters past the landing
// threshold of runwayEnd, or the last exit when the rollout is longer than
// every exit.
func (g *Graph) ExitFor(runwayEnd string, rollout float64) (RunwayExit, error) {
	exits, err := g.RunwayExits(runwayEnd)
	if err != nil {
		return RunwayExit{}, err
	}
	if len(exits) == 0 {
		return RunwayExit{}, fmt.Errorf("%w for runway %s", ErrNoExit, runwayEnd)
	}
	for _, e := range exits {
		if e.Along >= rollout {
			return e, nil
		}
	}
	return exits[len(exits)-1], nil
}

// RouteFromRunway returns a taxi-in route from a runway exit to a parking
// spot: the exit path off the runway, then the shortest route to the stand.
func (g *Graph) RouteFromRunway(exit RunwayExit, parking int, opts RouteOptions) (*Route, error) {
	to, ok := g.ParkingNode(parking)
	if !ok {
		return nil, fmt.Errorf("%w: index %d", ErrUnknownParking, parking)
	}
	// Continue in the direction the exit leaves the runway.
	prev := NodeID(-1)
	if len(exit.Path) > 1 {
		prev = exit.Path[len(exit.Path)-2]
	}
	opts.OwnStands = append(slices.Clone(opts.OwnStands), parking)
	in, err := fitOrTight(opts, func(o RouteOptions) (*Route, error) { return g.routeVia(exit.Node, prev, to, o) })
	if err != nil {
		return nil, err
	}
	nodes := append(append([]NodeID(nil), exit.Path...), in.Nodes[1:]...)
	r := g.routeFromNodes(nodes)
	r.Tight = in.Tight
	// The runway being vacated is not a crossing.
	r.RunwayCrossings = g.runwayCrossings(r.Points[len(exit.Path)-1:])
	return r, nil
}

// pathLength is the length of a node path along its edges, in meters.
func (g *Graph) pathLength(path []NodeID) float64 {
	sum := 0.0
	for i := 1; i < len(path); i++ {
		sum += g.edge(path[i-1], path[i]).Length
	}
	return sum
}

// pathsOffSurface returns, for each taxiway leaving the runway at rn, the
// node path from rn through on-surface nodes to the first node off the
// surface. Runway edges are not followed.
func (g *Graph) pathsOffSurface(rn NodeID, onSurface func(NodeID) bool) [][]NodeID {
	type item struct {
		id   NodeID
		path []NodeID
		dist float64
	}
	var out [][]NodeID
	visited := map[NodeID]bool{rn: true}
	queue := []item{{id: rn, path: []NodeID{rn}}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range g.Adj[cur.id] {
			if e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY || e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING ||
				visited[e.To] || cur.dist+e.Length > exitMaxPath {
				continue
			}
			visited[e.To] = true
			path := append(append([]NodeID(nil), cur.path...), e.To)
			if !onSurface(e.To) {
				out = append(out, path)
				continue
			}
			queue = append(queue, item{id: e.To, path: path, dist: cur.dist + e.Length})
		}
	}
	return out
}

// holdShortBehind finds the nearest hold-short node of runway rwy within
// exitHoldShortSearch of from, over taxiway edges; -1 if none.
func (g *Graph) holdShortBehind(from NodeID, rwy int) NodeID {
	dist := map[NodeID]float64{from: 0}
	queue := []NodeID{from}
	best, bestD := NodeID(-1), math.Inf(1)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if h := g.Nodes[cur].HoldShort; h != nil && h.Runway == rwy && dist[cur] < bestD {
			best, bestD = cur, dist[cur]
		}
		for _, e := range g.Adj[cur] {
			// Stay off the runway: back on it the search reaches other exits.
			if e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY || e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING || g.RunwayAt(g.Nodes[e.To].Position, exitSurfaceMargin) == rwy {
				continue
			}
			d := dist[cur] + e.Length
			if old, ok := dist[e.To]; (ok && old <= d) || d > exitHoldShortSearch {
				continue
			}
			dist[e.To] = d
			queue = append(queue, e.To)
		}
	}
	return best
}

// exitName names an exit (or entry) path off a runway: the last named edge
// on it, or, when the path is an unnamed connector, the named taxiway it
// continues onto most straight (EDDM: an unnamed link joins A4 to 08L/26R).
func (g *Graph) exitName(path []NodeID) string {
	for i := len(path) - 1; i > 0; i-- {
		if n := g.edge(path[i-1], path[i]).Name; n != "" {
			return n
		}
	}
	last, prev := path[len(path)-1], path[len(path)-2]
	px, pz := g.local.xz(g.Nodes[prev].Position)
	lx, lz := g.local.xz(g.Nodes[last].Position)
	in := math.Atan2(lx-px, lz-pz)
	best, bestTurn := "", math.Inf(1)
	for _, e := range g.Adj[last] {
		if e.Name == "" || e.To == prev {
			continue
		}
		nx, nz := g.local.xz(g.Nodes[e.To].Position)
		turn := math.Abs(math.Mod(math.Abs(math.Atan2(nx-lx, nz-lz)-in)*180/math.Pi+180, 360) - 180)
		if turn < bestTurn {
			best, bestTurn = e.Name, turn
		}
	}
	return best
}
