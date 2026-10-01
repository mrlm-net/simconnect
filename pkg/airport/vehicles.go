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
)

type vehicleEdge struct {
	to   int
	cost float64
}

type vehicleGraph struct {
	pos []LatLon // taxi points, then parking spots
	adj [][]vehicleEdge
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

// VehicleRoute is the way a ground vehicle drives from a to b: the points
// of the cheapest route between the nodes nearest them, a and b included.
func (l *Layout) VehicleRoute(a, b LatLon) ([]LatLon, error) {
	g := l.vehicleGraph()
	from, ok1 := g.nearest(a)
	to, ok2 := g.nearest(b)
	if !ok1 || !ok2 {
		return nil, ErrNoVehicleRoute
	}
	dist := make([]float64, len(g.pos))
	prev := make([]int, len(g.pos))
	for i := range dist {
		dist[i], prev[i] = math.Inf(1), -1
	}
	dist[from] = 0
	q := &vehicleQueue{{from, 0}}
	for q.Len() > 0 {
		it := heap.Pop(q).(vehicleItem)
		if it.d > dist[it.n] {
			continue
		}
		if it.n == to {
			break
		}
		for _, e := range g.adj[it.n] {
			if d := it.d + e.cost; d < dist[e.to] {
				dist[e.to], prev[e.to] = d, it.n
				heap.Push(q, vehicleItem{e.to, d})
			}
		}
	}
	if math.IsInf(dist[to], 1) {
		return nil, ErrNoVehicleRoute
	}
	var rev []LatLon
	for n := to; n >= 0; n = prev[n] {
		rev = append(rev, g.pos[n])
	}
	out := []LatLon{a}
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return append(out, b), nil
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
