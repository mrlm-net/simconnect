package airport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// crossingAirports names the graphs crossingGraphs returns, in order.
var crossingAirports = []string{"LKPR", "EDDM", "LKPR-data", "EDDM-data"}

// crossingGraphs are the airports the crossing cache tests run on (#1621):
// LKPR and EDDM from testdata, and from SIMCONNECT_DATA when it is set
// and has them ("-data").
func crossingGraphs(t testing.TB) map[string]*Graph {
	t.Helper()
	out := map[string]*Graph{"LKPR": lkprGraph(t)}
	build := func(name string, l *Layout) {
		g, err := BuildGraph(l)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = g
	}
	b, err := os.ReadFile("testdata/EDDM-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var l Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	build("EDDM", &l)
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		return out
	}
	for _, icao := range []string{"LKPR", "EDDM"} {
		b, err := os.ReadFile(filepath.Join(dir, icao, "layout.json"))
		if err != nil {
			continue
		}
		var raw RawAirport
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatal(err)
		}
		l, err := BuildLayout(raw)
		if err != nil {
			t.Fatal(err)
		}
		build(icao+"-data", l)
	}
	return out
}

// standRunwayJobs lists every aircraft stand and runway end of g.
func standRunwayJobs(g *Graph) (stands []int, ends []string) {
	for _, p := range g.Layout.Parking {
		if p.Type != types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE {
			stands = append(stands, p.Index)
		}
	}
	for _, r := range g.Layout.Runways {
		ends = append(ends, r.Primary.Name, r.Secondary.Name)
	}
	return stands, ends
}

// routeDump routes stand to end and renders the outcome as text.
func routeDump(g *Graph, stand int, end string) string {
	r, err := g.RouteToRunway(stand, end, RouteOptions{})
	if err != nil {
		return fmt.Sprintf("%d %s err %v\n", stand, end, err)
	}
	b, _ := json.Marshal(r)
	return fmt.Sprintf("%d %s %s\n", stand, end, b)
}

// TestCrossingCacheMatchesEdges: the per-edge crossing counts filled once
// for the graph are what edgeCrossings works out for every edge (#1621).
func TestCrossingCacheMatchesEdges(t *testing.T) {
	for icao, g := range crossingGraphs(t) {
		n, crossing := 0, 0
		for a, es := range g.Adj {
			for ei, e := range es {
				want := g.edgeCrossings(NodeID(a), e.To)
				if got := g.edgeCrossing(NodeID(a), ei); got != want {
					t.Errorf("%s edge %d→%d: cached %d, want %d", icao, a, e.To, got, want)
				}
				if got := g.crossings(NodeID(a), e.To); got != want {
					t.Errorf("%s crossings(%d, %d) = %d, want %d", icao, a, e.To, got, want)
				}
				n++
				if want > 0 {
					crossing++
				}
			}
		}
		if crossing == 0 {
			t.Errorf("%s: no edge crosses a runway of %d", icao, n)
		}
	}
}

// TestCrossingsHandBuiltGraph: a graph built without BuildGraph, or with
// edges added after the first search, still counts crossings (#1621).
func TestCrossingsHandBuiltGraph(t *testing.T) {
	g := lkprGraph(t)
	h := &Graph{Layout: g.Layout, Nodes: g.Nodes, Adj: make([][]Edge, len(g.Adj)), local: g.local}
	for a, es := range g.Adj {
		h.Adj[a] = append([]Edge(nil), es...)
	}
	for a, es := range h.Adj {
		for ei, e := range es {
			if got, want := h.edgeCrossing(NodeID(a), ei), g.edgeCrossings(NodeID(a), e.To); got != want {
				t.Fatalf("edge %d→%d: %d, want %d", a, e.To, got, want)
			}
		}
	}
	h.Adj[0] = append(h.Adj[0], h.Adj[0]...)
	for ei, e := range h.Adj[0] {
		if got, want := h.edgeCrossing(0, ei), g.edgeCrossings(0, e.To); got != want {
			t.Fatalf("added edge 0→%d: %d, want %d", e.To, got, want)
		}
	}
}

// TestParallelRoutesSame: RouteToRunway from many goroutines on one graph
// gives the routes one goroutine does (#1621), every stand to every runway
// end at LKPR and EDDM.
func TestParallelRoutesSame(t *testing.T) {
	for icao, g := range crossingGraphs(t) {
		stands, ends := standRunwayJobs(g)
		want := make([]string, 0, len(stands)*len(ends))
		for _, s := range stands {
			for _, e := range ends {
				want = append(want, routeDump(g, s, e))
			}
		}
		fresh := crossingGraphs(t)[icao]
		got := make([]string, len(want))
		var wg sync.WaitGroup
		jobs := make(chan int)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					got[j] = routeDump(fresh, stands[j/len(ends)], ends[j%len(ends)])
				}
			}()
		}
		for j := range want {
			jobs <- j
		}
		close(jobs)
		wg.Wait()
		for j := range want {
			if got[j] != want[j] {
				t.Errorf("%s: parallel %q, serial %q", icao, got[j], want[j])
			}
		}
	}
}

// BenchmarkRouteToRunwayParallel routes every aircraft stand to every
// runway end with 1, 4 and 16 goroutines on one graph, as an airport
// check does (#1621); one op is the whole set.
func BenchmarkRouteToRunwayParallel(b *testing.B) {
	graphs := crossingGraphs(b)
	for _, icao := range crossingAirports {
		g, ok := graphs[icao]
		if !ok {
			continue
		}
		stands, ends := standRunwayJobs(g)
		for _, workers := range []int{1, 4, 16} {
			b.Run(fmt.Sprintf("%s/%d", icao, workers), func(b *testing.B) {
				for range b.N {
					var wg sync.WaitGroup
					jobs := make(chan int)
					for range workers {
						wg.Add(1)
						go func() {
							defer wg.Done()
							for j := range jobs {
								g.RouteToRunway(stands[j/len(ends)], ends[j%len(ends)], RouteOptions{})
							}
						}()
					}
					for j := range len(stands) * len(ends) {
						jobs <- j
					}
					close(jobs)
					wg.Wait()
				}
			})
		}
	}
}
