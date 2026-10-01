//go:build windows
// +build windows

package airport

import (
	"container/heap"
	"errors"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Ground vehicles (pushback tugs, #tug) drive on the airport's vehicle roads
// (VEHICLE and ROAD taxi paths) where they can, and on aprons and taxiways
// (PATH, TAXI, PARKING) only where they must; never on a runway or a
// closed path. They start and end at the vehicle parking spots
// (TAXI_PARKING_TYPE_VEHICLE): VehicleDepots.

// ErrNoVehicleRoute is returned when no vehicle route joins two points.
var ErrNoVehicleRoute = errors.New("airport: no vehicle route")

// Vehicle route costs: meters on a vehicle road count once, on an apron or
// taxiway vehicleOffRoad times.
const (
	vehicleOffRoad = 4.0
	// vehicleSnapM: a route starts and ends at a node this close at most.
	vehicleSnapM = 150.0
	// VehicleRoadReachM: a route joins a vehicle road this close to its
	// start or end straight across the apron (through an empty stand), not
	// by the taxiway the nearest node may be on.
	VehicleRoadReachM = 150.0
)

type vehicleEdge struct {
	to   int
	cost float64
}

type vehicleGraph struct {
	pos   []LatLon // taxi points, then parking spots
	adj   [][]vehicleEdge
	roads [][2]int // the vehicle road segments (VEHICLE, ROAD)
}

var vehicleGraphs sync.Map // *Layout -> *vehicleGraph

func (l *Layout) vehicleGraph() *vehicleGraph {
	if g, ok := vehicleGraphs.Load(l); ok {
		return g.(*vehicleGraph)
	}
	np := len(l.TaxiPoints)
	g := &vehicleGraph{pos: make([]LatLon, np+len(l.Parking)), adj: make([][]vehicleEdge, np+len(l.Parking))}
	for i, t := range l.TaxiPoints {
		g.pos[i] = t.Position
	}
	for k, p := range l.Parking {
		g.pos[np+k] = p.Position
	}
	for _, p := range l.TaxiPaths {
		w := 0.0
		switch p.Type {
		case types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_VEHICLE, types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_ROAD:
			w = 1
		case types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PATH, types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI,
			types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING:
			w = vehicleOffRoad
		default:
			continue // runway, closed, painted line
		}
		if p.Start < 0 || int(p.Start) >= np || p.End < 0 {
			continue
		}
		a, b := int(p.Start), int(p.End)
		if p.EndsAtParking() {
			if int(p.End) >= len(l.Parking) {
				continue
			}
			b = np + int(p.End)
		} else if b >= np || a == b {
			continue
		}
		c := w * distM(g.pos[a], g.pos[b])
		g.adj[a] = append(g.adj[a], vehicleEdge{b, c})
		g.adj[b] = append(g.adj[b], vehicleEdge{a, c})
		if w == 1 {
			g.roads = append(g.roads, [2]int{a, b})
		}
	}
	got, _ := vehicleGraphs.LoadOrStore(l, g)
	return got.(*vehicleGraph)
}

// nearest is the connected node nearest p, within vehicleSnapM.
func (g *vehicleGraph) nearest(p LatLon) (int, bool) {
	best, bestD := -1, vehicleSnapM
	for i, q := range g.pos {
		if len(g.adj[i]) == 0 {
			continue
		}
		if d := distM(p, q); d < bestD {
			best, bestD = i, d
		}
	}
	return best, best >= 0
}

// vehicleEnd is where a route leaves or joins the graph: the point on the
// nearest vehicle road within VehicleRoadReachM (at, between the nodes of
// seg), else the nearest node (at its position, seg both that node).
type vehicleEnd struct {
	at  LatLon
	seg [2]int
}

func (g *vehicleGraph) end(p LatLon) (vehicleEnd, bool) {
	best, bestD := vehicleEnd{}, VehicleRoadReachM
	found := false
	for _, r := range g.roads {
		q := closestOnSegment(p, g.pos[r[0]], g.pos[r[1]])
		if d := distM(p, q); d < bestD {
			best, bestD, found = vehicleEnd{at: q, seg: r}, d, true
		}
	}
	if found {
		return best, true
	}
	n, ok := g.nearest(p)
	if !ok {
		return vehicleEnd{}, false
	}
	return vehicleEnd{at: g.pos[n], seg: [2]int{n, n}}, true
}

// closestOnSegment is the point of the segment ab nearest p (local, flat).
func closestOnSegment(p, a, b LatLon) LatLon {
	kx := 111320 * math.Cos(a.Lat*math.Pi/180)
	bx, by := (b.Lon-a.Lon)*kx, (b.Lat-a.Lat)*111320
	px, py := (p.Lon-a.Lon)*kx, (p.Lat-a.Lat)*111320
	l2 := bx*bx + by*by
	if l2 == 0 {
		return a
	}
	t := math.Max(0, math.Min(1, (px*bx+py*by)/l2))
	return LatLon{Lat: a.Lat + t*(b.Lat-a.Lat), Lon: a.Lon + t*(b.Lon-a.Lon)}
}

// VehicleRoute is the way a ground vehicle drives from a to b, a and b
// included. Each end joins the vehicle road nearest it when one is within
// VehicleRoadReachM, straight across the apron: a tug at a stand drives
// through the stand to the service road behind it, not out along the
// taxiway among the aircraft. Otherwise it joins at the nearest node. In
// between it takes the cheapest way (roads 1×, aprons and taxiways
// vehicleOffRoad×).
func (l *Layout) VehicleRoute(a, b LatLon) ([]LatLon, error) {
	g := l.vehicleGraph()
	from, ok1 := g.end(a)
	to, ok2 := g.end(b)
	if !ok1 || !ok2 {
		return nil, ErrNoVehicleRoute
	}
	// Both on one road segment: along it.
	if from.seg == to.seg || (from.seg[0] == to.seg[1] && from.seg[1] == to.seg[0]) {
		return dedupeLatLon([]LatLon{a, from.at, to.at, b}), nil
	}
	dist := make([]float64, len(g.pos))
	prev := make([]int, len(g.pos))
	for i := range dist {
		dist[i], prev[i] = math.Inf(1), -1
	}
	q := &vehicleQueue{}
	for _, n := range from.seg {
		if d := distM(from.at, g.pos[n]); d < dist[n] {
			dist[n] = d
			heap.Push(q, vehicleItem{n, d})
		}
	}
	for q.Len() > 0 {
		it := heap.Pop(q).(vehicleItem)
		if it.d > dist[it.n] {
			continue
		}
		for _, e := range g.adj[it.n] {
			if d := it.d + e.cost; d < dist[e.to] {
				dist[e.to], prev[e.to] = d, it.n
				heap.Push(q, vehicleItem{e.to, d})
			}
		}
	}
	// The end of the segment b joins by, cheapest with the way on to b.
	last, lastD := -1, math.Inf(1)
	for _, n := range to.seg {
		if d := dist[n] + distM(g.pos[n], to.at); d < lastD {
			last, lastD = n, d
		}
	}
	if last < 0 || math.IsInf(lastD, 1) {
		return nil, ErrNoVehicleRoute
	}
	var rev []LatLon
	for n := last; n >= 0; n = prev[n] {
		rev = append(rev, g.pos[n])
	}
	out := []LatLon{a, from.at}
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return dedupeLatLon(append(out, to.at, b)), nil
}

// dedupeLatLon drops points within half a meter of the one before.
func dedupeLatLon(pts []LatLon) []LatLon {
	out := pts[:0:0]
	for _, p := range pts {
		if len(out) > 0 && distM(out[len(out)-1], p) < 0.5 {
			continue
		}
		out = append(out, p)
	}
	return out
}

// VehicleDepots are the vehicle parking spots: where ground vehicles
// appear and leave, by parking index.
func (l *Layout) VehicleDepots() []int {
	var out []int
	for i, p := range l.Parking {
		if p.Type == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE {
			out = append(out, i)
		}
	}
	return out
}

type vehicleItem struct {
	n int
	d float64
}

type vehicleQueue []vehicleItem

func (q vehicleQueue) Len() int           { return len(q) }
func (q vehicleQueue) Less(i, j int) bool { return q[i].d < q[j].d }
func (q vehicleQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *vehicleQueue) Push(x any)        { *q = append(*q, x.(vehicleItem)) }
func (q *vehicleQueue) Pop() any          { o := *q; it := o[len(o)-1]; *q = o[:len(o)-1]; return it }

// distM is the distance between two points in meters (local, flat).
func distM(a, b LatLon) float64 {
	kx := 111320 * math.Cos((a.Lat+b.Lat)/2*math.Pi/180)
	dx, dy := (b.Lon-a.Lon)*kx, (b.Lat-a.Lat)*111320
	return math.Sqrt(dx*dx + dy*dy)
}

// NearVehicleRoad reports whether a vehicle road is within
// VehicleRoadReachM of p: a route from p joins it there.
func (l *Layout) NearVehicleRoad(p LatLon) bool {
	g := l.vehicleGraph()
	for _, r := range g.roads {
		if distM(p, closestOnSegment(p, g.pos[r[0]], g.pos[r[1]])) < VehicleRoadReachM {
			return true
		}
	}
	return false
}
