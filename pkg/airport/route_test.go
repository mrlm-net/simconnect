//go:build windows
// +build windows

package airport

import (
	"errors"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func lkprGraph(t testing.TB) *Graph {
	t.Helper()
	g, err := BuildGraph(loadLKPR(t))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestBuildGraphNodes(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	if len(g.Nodes) != len(l.TaxiPoints)+len(l.Parking) {
		t.Fatalf("nodes = %d, want %d", len(g.Nodes), len(l.TaxiPoints)+len(l.Parking))
	}
	holds := 0
	for _, n := range g.Nodes {
		if n.Kind != NodeHoldShort {
			continue
		}
		holds++
		h := n.HoldShort
		if h == nil {
			t.Errorf("hold-short point %d has no runway", n.Index)
			continue
		}
		if h.Name != "06/24" && h.Name != "12/30" {
			t.Errorf("hold-short point %d on runway %q", n.Index, h.Name)
		}
		if h.Offset < 30 || h.Offset > 200 {
			t.Errorf("hold-short point %d is %.0f m from the %s centreline", n.Index, h.Offset, h.Name)
		}
		if h.ILS != l.TaxiPoints[n.Index].IsILSHoldShort() {
			t.Errorf("hold-short point %d ILS = %v", n.Index, h.ILS)
		}
	}
	if holds != 21 {
		t.Errorf("hold-short nodes = %d, want 21", holds)
	}
}

func TestBuildGraphExcludesNonAircraftPaths(t *testing.T) {
	g := lkprGraph(t)
	for id, edges := range g.Adj {
		for _, e := range edges {
			if !routable(e.Type) {
				t.Fatalf("node %d has a %d edge", id, e.Type)
			}
		}
	}
}

func TestBuildGraphNoTaxiNetwork(t *testing.T) {
	l := loadLKPR(t)
	l.TaxiPaths = nil
	if _, err := BuildGraph(l); !errors.Is(err, ErrNoTaxiNetwork) {
		t.Errorf("BuildGraph(no paths) error = %v, want ErrNoTaxiNetwork", err)
	}
}

func TestRouteToRunwayC22(t *testing.T) {
	g := lkprGraph(t)
	c22, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		end       string
		crossings []string
	}{
		{"24", nil},
		{"06", []string{"12/30"}},
		{"30", nil},
		{"12", nil},
	} {
		r, err := g.RouteToRunway(c22, c.end, RouteOptions{})
		if err != nil {
			t.Errorf("C22 → %s: %v", c.end, err)
			continue
		}
		checkRoute(t, g, r)
		start, _ := g.ParkingNode(c22)
		if r.Nodes[0] != start {
			t.Errorf("C22 → %s starts at node %d, want parking node %d", c.end, r.Nodes[0], start)
		}
		last := g.Nodes[r.Nodes[len(r.Nodes)-1]]
		rwy, end, _ := g.Layout.RunwayEnd(c.end)
		if last.Kind != NodeHoldShort || last.HoldShort == nil || last.HoldShort.Runway != rwy.Index || last.HoldShort.ILS {
			t.Errorf("C22 → %s ends at node %d (%+v), want a runway hold-short of %s", c.end, last.ID, last.HoldShort, rwy.Name())
		}
		if r.RunwayEnd != end.Name || r.Runway != rwy.Name() {
			t.Errorf("C22 → %s: Runway %q, RunwayEnd %q", c.end, r.Runway, r.RunwayEnd)
		}
		if len(r.Taxiways) == 0 {
			t.Errorf("C22 → %s has no taxiway names", c.end)
		}
		if !equal(r.RunwayCrossings, c.crossings) {
			t.Errorf("C22 → %s crosses %v, want %v", c.end, r.RunwayCrossings, c.crossings)
		}
		t.Logf("C22 → %s: %.0f m via %v, crossings %v", c.end, r.Length, r.Taxiways, r.RunwayCrossings)
	}
}

// checkRoute verifies a route is a connected walk over graph edges.
func checkRoute(t *testing.T, g *Graph, r *Route) {
	t.Helper()
	if len(r.Points) != len(r.Nodes) || len(r.Edges) != len(r.Nodes)-1 {
		t.Fatalf("route shape: %d nodes, %d points, %d edges", len(r.Nodes), len(r.Points), len(r.Edges))
	}
	sum := 0.0
	for i, e := range r.Edges {
		if e.To != r.Nodes[i+1] || math.IsInf(e.Length, 1) {
			t.Fatalf("edge %d does not join nodes %d → %d", i, r.Nodes[i], r.Nodes[i+1])
		}
		if e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY {
			t.Errorf("edge %d taxis along a runway path", i)
		}
		if i > 0 && g.Nodes[r.Nodes[i]].Kind == NodeParking {
			t.Errorf("route passes through parking node %d", r.Nodes[i])
		}
		sum += e.Length
	}
	if math.Abs(sum-r.Length) > 1e-6 {
		t.Errorf("Length %.1f, sum of edges %.1f", r.Length, sum)
	}
}

func TestEveryAircraftStandReachesEveryRunwayEnd(t *testing.T) {
	g := lkprGraph(t)
	for _, p := range g.Layout.Parking {
		vehicle := p.Type == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE
		for _, end := range []string{"06", "24", "12", "30"} {
			r, err := g.RouteToRunway(p.Index, end, RouteOptions{})
			switch {
			case vehicle && !errors.Is(err, ErrNoRoute):
				t.Errorf("vehicle parking %d (%s) → %s: error %v, want ErrNoRoute", p.Index, p.Label(), end, err)
			case !vehicle && err != nil:
				t.Errorf("parking %d (%s) → %s: %v", p.Index, p.Label(), end, err)
			case !vehicle:
				checkRoute(t, g, r)
			}
		}
	}
}

func TestRouteToParkingIsReverseOfDeparture(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	out, err := g.RouteToRunway(c22, "24", RouteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	back, err := g.RouteToParking(out.Nodes[len(out.Nodes)-1], c22, RouteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	checkRoute(t, g, back)
	if math.Abs(back.Length-out.Length) > 1e-6 {
		t.Errorf("taxi-in %.1f m, taxi-out %.1f m on the same shortest path", back.Length, out.Length)
	}
}

func TestUseRunwayPathsNeverLonger(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	for _, end := range []string{"06", "24", "12", "30"} {
		hold, _ := g.RouteToRunway(c22, end, RouteOptions{})
		to := hold.Nodes[len(hold.Nodes)-1]
		start, _ := g.ParkingNode(c22)
		without, err1 := g.Route(start, to, RouteOptions{})
		with, err2 := g.Route(start, to, RouteOptions{UseRunwayPaths: true})
		if err1 != nil || err2 != nil {
			t.Fatalf("%s: %v / %v", end, err1, err2)
		}
		if with.Length > without.Length+1e-6 {
			t.Errorf("%s: with runway paths %.0f m > without %.0f m", end, with.Length, without.Length)
		}
	}
}

func TestRouteErrors(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	dup := &Layout{Parking: []Parking{{Index: 0, Number: 5}, {Index: 1, Number: 5}}}
	if _, err := dup.ParkingIndex("5"); !errors.Is(err, ErrAmbiguousParking) {
		t.Errorf("ParkingIndex(duplicate) error = %v, want ErrAmbiguousParking", err)
	}
	if _, err := g.Layout.ParkingIndex("X99"); !errors.Is(err, ErrUnknownParking) {
		t.Errorf("ParkingIndex(X99) error = %v, want ErrUnknownParking", err)
	}
	if _, err := g.RouteToRunway(c22, "18", RouteOptions{}); !errors.Is(err, ErrUnknownRunway) {
		t.Errorf("RouteToRunway(18) error = %v, want ErrUnknownRunway", err)
	}
	if _, err := g.RouteToRunway(9999, "24", RouteOptions{}); !errors.Is(err, ErrUnknownParking) {
		t.Errorf("RouteToRunway(9999) error = %v, want ErrUnknownParking", err)
	}
	if _, err := g.Route(0, NodeID(len(g.Nodes)), RouteOptions{}); !errors.Is(err, ErrNoRoute) {
		t.Errorf("Route(out of range) error = %v, want ErrNoRoute", err)
	}
}

func TestIntersectionToleranceZeroPicksNearestThreshold(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	for _, end := range []string{"06", "24", "12", "30"} {
		r, err := g.RouteToRunway(c22, end, RouteOptions{IntersectionTolerance: 1e-9})
		if err != nil {
			t.Fatal(err)
		}
		rwy, e, _ := g.Layout.RunwayEnd(end)
		along := func(h *HoldShort) float64 {
			if e.Name == rwy.Secondary.Name {
				return rwy.Length - h.FromPrimary
			}
			return h.FromPrimary
		}
		got := along(r.HoldShort)
		for _, id := range g.HoldShortNodes(rwy.Index) {
			h := g.Nodes[id].HoldShort
			if !h.ILS && along(h) < got-1e-6 {
				if _, err := g.Route(r.Nodes[0], id, RouteOptions{}); err == nil {
					t.Errorf("%s: picked hold %.0f m from threshold, reachable hold %d is %.0f m", end, got, id, along(h))
				}
			}
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func BenchmarkBuildGraphLKPR(b *testing.B) {
	l := loadLKPR(b)
	b.ResetTimer()
	for range b.N {
		if _, err := BuildGraph(l); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRouteToRunwayLKPR(b *testing.B) {
	g := lkprGraph(b)
	c22, _ := g.Layout.ParkingIndex("C22")
	b.ResetTimer()
	for range b.N {
		if _, err := g.RouteToRunway(c22, "24", RouteOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

// junctionTurns sums the heading changes above TurnFreeAngle at junctions
// along r, and reports the largest single turn.
func junctionTurns(g *Graph, r *Route) (sum, largest float64) {
	for i := 1; i+1 < len(r.Nodes); i++ {
		a, b, c := r.Points[i-1], r.Points[i], r.Points[i+1]
		ax, az := g.local.xz(a)
		bx, bz := g.local.xz(b)
		cx, cz := g.local.xz(c)
		h1, h2 := math.Atan2(bx-ax, bz-az), math.Atan2(cx-bx, cz-bz)
		angle := math.Abs(math.Mod(math.Abs(h2-h1)*180/math.Pi+180, 360) - 180)
		largest = math.Max(largest, angle)
		if len(g.Adj[r.Nodes[i]]) > 2 && angle > TurnFreeAngle {
			sum += angle
		}
	}
	return sum, largest
}

// TestRouteTurnCosts: with turn costs every LKPR departure turns no more
// than the plain shortest route, is at most a little longer and never turns
// back (#307).
func TestRouteTurnCosts(t *testing.T) {
	g := lkprGraph(t)
	plain := RouteOptions{TurnPenalty: -1, TaxiwayChangePenalty: -1, RunwayCrossingPenalty: -1}
	better, total := 0, 0
	for _, p := range g.Layout.Parking {
		if p.Index%7 != 0 { // a sample of stands keeps the test fast
			continue
		}
		for _, end := range []string{"06", "24", "12", "30"} {
			r, err := g.RouteToRunway(p.Index, end, RouteOptions{})
			if err != nil {
				continue
			}
			s, err := g.RouteToRunway(p.Index, end, plain)
			if err != nil {
				t.Fatalf("%s → %s: plain route failed: %v", p.Label(), end, err)
			}
			if r.Nodes[len(r.Nodes)-1] != s.Nodes[len(s.Nodes)-1] {
				continue // different hold-short chosen; not comparable
			}
			total++
			rt, big := junctionTurns(g, r)
			st, _ := junctionTurns(g, s)
			if rt > st+1e-6 {
				t.Errorf("%s → %s: turns %.0f° with turn costs, %.0f° without", p.Label(), end, rt, st)
			}
			if r.Length > s.Length*1.5+50 {
				t.Errorf("%s → %s: %.0f m with turn costs, shortest %.0f m", p.Label(), end, r.Length, s.Length)
			}
			if big >= UTurnAngle {
				t.Errorf("%s → %s turns back (%.0f°)", p.Label(), end, big)
			}
			if rt < st-1e-6 {
				better++
			}
		}
	}
	t.Logf("%d of %d routes turn less than the shortest route", better, total)
	if total == 0 {
		t.Fatal("no comparable routes")
	}
}

// TestDriveThroughStand: LKPR N52 has a lead-in from G behind it and one
// ahead of it towards H. Arrivals enter nose-in from G; departures leave
// forward towards H without a pushback.
func TestDriveThroughStand(t *testing.T) {
	g := lkprGraph(t)
	n52, err := g.Layout.ParkingIndex("N52")
	if err != nil {
		t.Fatal(err)
	}
	stand, _ := g.ParkingNode(n52)
	exits, _ := g.RunwayExits("24")
	for _, x := range exits {
		r, err := g.RouteFromRunway(x, n52, RouteOptions{})
		if err != nil {
			continue
		}
		lead := r.Nodes[len(r.Nodes)-2]
		if g.LeadInAhead(stand, lead) {
			t.Errorf("24 exit %s → N52 enters through the lead-in ahead (%d), want nose-in from G", x.Taxiway, lead)
		}
	}
	for _, end := range []string{"06", "24", "12", "30"} {
		r, err := g.RouteToRunway(n52, end, RouteOptions{})
		if err != nil {
			continue
		}
		if !g.LeadInAhead(stand, r.Nodes[1]) {
			t.Errorf("N52 → %s leaves through the lead-in behind (pushback), want forward", end)
		}
	}
}
