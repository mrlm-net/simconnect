package nav

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Direct is the airway name of a direct (off-airway) step.
const Direct = "DCT"

// DefaultAirwayChangePenaltyNM is added to a route's cost at every change
// of airway, so a route keeps to one airway rather than zig-zag between
// parallel ones for a shorter path.
const DefaultAirwayChangePenaltyNM = 10.0

// ErrNoRoute is returned when two fixes are not connected by airways.
var ErrNoRoute = errors.New("nav: no airway route")

// AirwaySegment is one leg of an airway, in the order the data lists it
// (From is the previous fix, To the next).
type AirwaySegment struct {
	From       FixKey  `json:"from"`
	To         FixKey  `json:"to"`
	DistanceNM float64 `json:"distanceNM"`
	// MinAltM is the segment's minimum altitude in meters: the higher of
	// the NEXT_ALTITUDE / PREV_ALTITUDE its two ends give (0 when neither
	// does; values are round feet, e.g. 1524 = 5000 ft).
	MinAltM float64 `json:"minAltM,omitempty"`
}

// Airway is a named airway with its segments. An identifier can stand for
// several disjoint pieces (an airway broken around a TMA): segments are
// not necessarily a single chain.
type Airway struct {
	Name     string          `json:"name"`
	Type     AirwayType      `json:"type"`
	Segments []AirwaySegment `json:"segments"`
}

// Edge is a traversable airway segment out of a fix.
type Edge struct {
	To         FixKey
	Airway     string
	DistanceNM float64
}

// AirwayGraph is the enroute network: fixes are the nodes, airway segments
// the edges. The facility data carries no one-way restriction, so every
// segment can be flown both ways.
type AirwayGraph struct {
	Center   airport.LatLon `json:"center"`
	RadiusNM float64        `json:"radiusNM"`
	// Fixes are the loaded fixes, then the airway ends beyond the crawl
	// (known only by key and position).
	Fixes   []Fix    `json:"fixes"`
	Airways []Airway `json:"airways"`

	index map[FixKey]int
	adj   map[FixKey][]Edge
}

// BuildAirwayGraph builds the graph from loaded fixes and their ROUTE
// links. Each segment appears once however many of its ends were loaded.
func BuildAirwayGraph(fixes []Fix, links map[FixKey][]RouteLink) *AirwayGraph {
	g := &AirwayGraph{Fixes: append([]Fix(nil), fixes...), Airways: []Airway{}}
	g.index = map[FixKey]int{}
	for i, f := range g.Fixes {
		g.index[f.Key()] = i
	}
	byName := map[string]*Airway{}
	type segKey struct {
		airway   string
		from, to FixKey
	}
	seen := map[segKey]int{} // -> index in its airway's segments
	pos := func(k FixKey, p airport.LatLon) airport.LatLon {
		if i, ok := g.index[k]; ok {
			return g.Fixes[i].Position
		}
		g.index[k] = len(g.Fixes)
		g.Fixes = append(g.Fixes, Fix{Ident: k.Ident, Region: k.Region, Kind: k.Kind, Position: p})
		return p
	}
	addSeg := func(rl RouteLink, from, to FixKey, fromPos, toPos airport.LatLon, alt float64) {
		a := byName[rl.Airway]
		if a == nil {
			a = &Airway{Name: rl.Airway, Type: rl.Type}
			byName[rl.Airway] = a
		}
		sk := segKey{rl.Airway, from, to}
		if i, ok := seen[sk]; ok {
			// Seen from its other end: keep the higher minimum altitude.
			a.Segments[i].MinAltM = max(a.Segments[i].MinAltM, alt)
			return
		}
		seen[sk] = len(a.Segments)
		a.Segments = append(a.Segments, AirwaySegment{From: from, To: to, MinAltM: alt,
			DistanceNM: calc.HaversineNM(fromPos.Lat, fromPos.Lon, toPos.Lat, toPos.Lon)})
	}
	for _, f := range fixes {
		k := f.Key()
		for _, rl := range links[k] {
			if rl.Airway == "" {
				continue
			}
			if rl.Prev != nil {
				p := pos(rl.Prev.Key, rl.Prev.Position)
				addSeg(rl, rl.Prev.Key, k, p, f.Position, rl.Prev.MinAltM)
			}
			if rl.Next != nil {
				n := pos(rl.Next.Key, rl.Next.Position)
				addSeg(rl, k, rl.Next.Key, f.Position, n, rl.Next.MinAltM)
			}
		}
	}
	for _, a := range byName {
		g.Airways = append(g.Airways, *a)
	}
	sort.Slice(g.Airways, func(i, j int) bool { return g.Airways[i].Name < g.Airways[j].Name })
	g.link()
	return g
}

// link (re)builds the fix index and adjacency.
func (g *AirwayGraph) link() {
	g.index = make(map[FixKey]int, len(g.Fixes))
	for i, f := range g.Fixes {
		g.index[f.Key()] = i
	}
	g.adj = map[FixKey][]Edge{}
	for _, a := range g.Airways {
		for _, s := range a.Segments {
			g.adj[s.From] = append(g.adj[s.From], Edge{To: s.To, Airway: a.Name, DistanceNM: s.DistanceNM})
			g.adj[s.To] = append(g.adj[s.To], Edge{To: s.From, Airway: a.Name, DistanceNM: s.DistanceNM})
		}
	}
}

// Fix returns the fix with key k.
func (g *AirwayGraph) Fix(k FixKey) (Fix, bool) {
	i, ok := g.index[k]
	if !ok {
		return Fix{}, false
	}
	return g.Fixes[i], true
}

// Find returns the fixes with identifier ident (any region and kind).
func (g *AirwayGraph) Find(ident string) []Fix {
	ident = strings.ToUpper(strings.TrimSpace(ident))
	var out []Fix
	for _, f := range g.Fixes {
		if f.Ident == ident {
			out = append(out, f)
		}
	}
	return out
}

// Edges returns the airway segments out of fix k (both directions).
func (g *AirwayGraph) Edges(k FixKey) []Edge { return g.adj[k] }

// SegmentCount returns the number of airway segments.
func (g *AirwayGraph) SegmentCount() int {
	n := 0
	for _, a := range g.Airways {
		n += len(a.Segments)
	}
	return n
}

// Nearest returns the fix on an airway nearest to p and its distance.
func (g *AirwayGraph) Nearest(p airport.LatLon) (Fix, float64, bool) {
	best, bestD := -1, 0.0
	for i, f := range g.Fixes {
		if len(g.adj[f.Key()]) == 0 {
			continue
		}
		d := calc.HaversineNM(p.Lat, p.Lon, f.Position.Lat, f.Position.Lon)
		if best < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return Fix{}, 0, false
	}
	return g.Fixes[best], bestD, true
}

// RouteStep is one step of a route: the airway flown to reach Fix (empty
// for the first step, Direct for a direct leg).
type RouteStep struct {
	Airway     string         `json:"airway"`
	Fix        FixKey         `json:"fix"`
	Position   airport.LatLon `json:"position"`
	DistanceNM float64        `json:"distanceNM"` // of the leg to Fix
}

// Route returns the shortest airway route from one fix to another by
// great-circle distance, with DefaultAirwayChangePenaltyNM per change of
// airway. It returns ErrNoRoute when they are not connected.
func (g *AirwayGraph) Route(from, to FixKey) ([]RouteStep, error) {
	return g.RouteWithPenalty(from, to, DefaultAirwayChangePenaltyNM)
}

// RouteOrDirect returns the airway route, or a direct leg when the fixes
// are not connected by airways or (maxStretch > 0) the airway route is
// more than maxStretch times the direct distance — airways around a TMA
// can make a long detour between nearby fixes.
func (g *AirwayGraph) RouteOrDirect(from, to FixKey, maxStretch float64) ([]RouteStep, error) {
	steps, err := g.Route(from, to)
	if errors.Is(err, ErrNoRoute) {
		return g.DirectTo(from, to)
	}
	if err != nil {
		return nil, err
	}
	if maxStretch > 0 && len(steps) > 1 {
		direct := dist(steps[0].Position, steps[len(steps)-1].Position)
		if RouteDistanceNM(steps) > maxStretch*direct {
			return g.DirectTo(from, to)
		}
	}
	return steps, nil
}

// DirectTo returns the direct leg between two fixes.
func (g *AirwayGraph) DirectTo(from, to FixKey) ([]RouteStep, error) {
	a, ok := g.Fix(from)
	if !ok {
		return nil, fmt.Errorf("nav: unknown fix %s", from)
	}
	b, ok := g.Fix(to)
	if !ok {
		return nil, fmt.Errorf("nav: unknown fix %s", to)
	}
	return []RouteStep{{Fix: from, Position: a.Position},
		{Airway: Direct, Fix: to, Position: b.Position, DistanceNM: dist(a.Position, b.Position)}}, nil
}

func dist(a, b airport.LatLon) float64 { return calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon) }

// routeState is a search node: a fix reached along an airway.
type routeState struct {
	fix    FixKey
	airway string
}

type routeItem struct {
	st      routeState
	cost, f float64
	index   int
}

type routeQueue []*routeItem

func (q routeQueue) Len() int           { return len(q) }
func (q routeQueue) Less(i, j int) bool { return q[i].f < q[j].f }
func (q routeQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i]; q[i].index, q[j].index = i, j }
func (q *routeQueue) Push(x any)        { it := x.(*routeItem); it.index = len(*q); *q = append(*q, it) }
func (q *routeQueue) Pop() any          { old := *q; it := old[len(old)-1]; *q = old[:len(old)-1]; return it }

// RouteWithPenalty is Route with a chosen airway-change penalty (A* over
// fix-and-airway states, great-circle distance to the goal as heuristic).
func (g *AirwayGraph) RouteWithPenalty(from, to FixKey, penaltyNM float64) ([]RouteStep, error) {
	a, ok := g.Fix(from)
	if !ok {
		return nil, fmt.Errorf("nav: unknown fix %s", from)
	}
	b, ok := g.Fix(to)
	if !ok {
		return nil, fmt.Errorf("nav: unknown fix %s", to)
	}
	if from == to {
		return []RouteStep{{Fix: from, Position: a.Position}}, nil
	}
	h := func(k FixKey) float64 {
		f, _ := g.Fix(k)
		return dist(f.Position, b.Position)
	}
	start := routeState{fix: from}
	best := map[routeState]float64{start: 0}
	prev := map[routeState]routeState{}
	q := &routeQueue{{st: start, f: h(from)}}
	for q.Len() > 0 {
		it := heap.Pop(q).(*routeItem)
		if it.cost > best[it.st] {
			continue
		}
		if it.st.fix == to {
			return g.unwind(it.st, start, prev), nil
		}
		for _, e := range g.adj[it.st.fix] {
			c := it.cost + e.DistanceNM
			if it.st.airway != "" && it.st.airway != e.Airway {
				c += penaltyNM
			}
			ns := routeState{fix: e.To, airway: e.Airway}
			if old, ok := best[ns]; ok && old <= c {
				continue
			}
			best[ns], prev[ns] = c, it.st
			heap.Push(q, &routeItem{st: ns, cost: c, f: c + h(e.To)})
		}
	}
	return nil, fmt.Errorf("%w from %s to %s", ErrNoRoute, from, to)
}

func (g *AirwayGraph) unwind(end, start routeState, prev map[routeState]routeState) []RouteStep {
	var sts []routeState
	for s := end; ; s = prev[s] {
		sts = append(sts, s)
		if s == start {
			break
		}
	}
	steps := make([]RouteStep, 0, len(sts))
	for i := len(sts) - 1; i >= 0; i-- {
		f, _ := g.Fix(sts[i].fix)
		st := RouteStep{Airway: sts[i].airway, Fix: sts[i].fix, Position: f.Position}
		if n := len(steps); n > 0 {
			st.DistanceNM = dist(steps[n-1].Position, f.Position)
		}
		steps = append(steps, st)
	}
	return steps
}

// RouteDistanceNM is the total length of a route.
func RouteDistanceNM(steps []RouteStep) float64 {
	d := 0.0
	for _, s := range steps {
		d += s.DistanceNM
	}
	return d
}

// FormatRoute writes a route the way a flight plan does, naming only the
// fixes where the airway changes: "VOZ T709 BODAL".
func FormatRoute(steps []RouteStep) string {
	if len(steps) == 0 {
		return ""
	}
	parts := []string{steps[0].Fix.Ident}
	for i := 1; i < len(steps); i++ {
		if i+1 < len(steps) && steps[i+1].Airway == steps[i].Airway {
			continue
		}
		parts = append(parts, steps[i].Airway, steps[i].Fix.Ident)
	}
	return strings.Join(parts, " ")
}

// WriteJSON writes the graph as JSON.
func (g *AirwayGraph) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	return enc.Encode(g)
}

// SaveJSON writes the graph as JSON to a file.
func (g *AirwayGraph) SaveJSON(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := g.WriteJSON(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ReadAirwayGraph reads a graph written by WriteJSON.
func ReadAirwayGraph(r io.Reader) (*AirwayGraph, error) {
	g := &AirwayGraph{}
	if err := json.NewDecoder(r).Decode(g); err != nil {
		return nil, err
	}
	g.link()
	return g, nil
}

// LoadAirwayGraph reads a graph saved by SaveJSON.
func LoadAirwayGraph(path string) (*AirwayGraph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadAirwayGraph(f)
}
