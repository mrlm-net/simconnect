//go:build windows
// +build windows

package airport

import (
	"errors"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// nodeOn returns the first taxi point that lies along taxiway name (both
// its edges carry the name).
func nodeOn(t *testing.T, g *Graph, name string) NodeID {
	t.Helper()
	for id, es := range g.Adj {
		if g.Nodes[id].Kind == NodeTaxiPoint && len(es) == 2 && es[0].Name == name && es[1].Name == name {
			return NodeID(id)
		}
	}
	t.Fatalf("no node on %s", name)
	return -1
}

// checkVia verifies r passes the via points in order and goes on from each
// without turning back.
func checkVia(t *testing.T, g *Graph, r *Route, via []NodeID) {
	t.Helper()
	at := 0
	for _, v := range via {
		i := slices.Index(r.Nodes[at:], v)
		if i < 0 {
			t.Fatalf("route %v does not pass via node %d after index %d", r.Taxiways, v, at)
		}
		at += i
		if at > 0 && at < len(r.Nodes)-1 {
			if a := g.turnAngle(r.Nodes[at-1], v, r.Nodes[at+1]); a >= UTurnAngle {
				t.Errorf("route turns back (%.0f°) at via node %d", a, v)
			}
		}
	}
}

// TestRouteVia: LKPR C22 → 30 goes by H1, K, L; via a node on A (or on A
// then B) every routing function takes that way, in order.
func TestRouteVia(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	plain, err := g.RouteToRunway(c22, "30", RouteOptions{})
	if err != nil || slices.Contains(plain.Taxiways, "A") {
		t.Fatalf("C22 → 30 via %v, %v", plain.Taxiways, err)
	}
	onA, onB := nodeOn(t, g, "A"), nodeOn(t, g, "B")
	for _, via := range [][]NodeID{{onA}, {onA, onB}, {onB, onA}} {
		opts := RouteOptions{Via: via}
		r, err := g.RouteToRunway(c22, "30", opts)
		if err != nil {
			t.Fatalf("via %v: %v", via, err)
		}
		checkRoute(t, g, r)
		checkVia(t, g, r, via)
		if !slices.Contains(r.Taxiways, "A") || r.Cost <= plain.Cost {
			t.Errorf("via %v: %v cost %.0f, plain %.0f", via, r.Taxiways, r.Cost, plain.Cost)
		}
		t.Logf("C22 → 30 via nodes %v: %v %.0f m", via, r.Taxiways, r.Length)

		hold := plain.Nodes[len(plain.Nodes)-1]
		in, err := g.RouteToParking(hold, c22, opts)
		if err != nil {
			t.Fatalf("taxi-in via %v: %v", via, err)
		}
		checkVia(t, g, in, via)
		from, err := g.RouteToRunwayFrom(r.Nodes[1], r.Nodes[0], "30", "", opts)
		if err != nil {
			t.Fatalf("RouteToRunwayFrom via %v: %v", via, err)
		}
		checkVia(t, g, from, via)
		entry, err := g.RouteToRunwayEntry(c22, "24", "B", opts)
		if err != nil || entry.Entry != "B" {
			t.Fatalf("24 at B via %v: %v", via, err)
		}
		checkVia(t, g, entry, via)
	}
}

// TestRouteViaNoUTurn: a via point at a dead end cannot be passed — the
// route would have to turn back there — while two plain legs would.
func TestRouteViaNoUTurn(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	src, _ := g.ParkingNode(c22)
	dead := NodeID(-1)
	for id, es := range g.Adj {
		if g.Nodes[id].Kind == NodeTaxiPoint && len(es) == 1 && es[0].Name != "" {
			if _, err := g.Route(src, NodeID(id), RouteOptions{}); err == nil {
				dead = NodeID(id)
				break
			}
		}
	}
	if dead < 0 {
		t.Fatal("no reachable dead end")
	}
	plain, err := g.RouteToRunway(c22, "24", RouteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hold := plain.Nodes[len(plain.Nodes)-1]
	if _, err := g.Route(dead, hold, RouteOptions{}); err != nil {
		t.Fatalf("dead end %d → hold: %v", dead, err)
	}
	var re *RouteError
	_, err = g.RouteToRunway(c22, "24", RouteOptions{Via: []NodeID{dead}})
	if !errors.Is(err, ErrNoRoute) || !errors.As(err, &re) || re.Via != 1 || re.Node != -1 {
		t.Errorf("via dead end %d: %v, want the leg from it to fail", dead, err)
	}
	_, err = g.RouteToRunway(c22, "24", RouteOptions{Via: []NodeID{dead, hold}})
	if !errors.Is(err, ErrViaUnreachable) || !errors.As(err, &re) || re.Via != 1 || re.Node != hold {
		t.Errorf("via dead end %d then %d: %v, want via 1 unreachable", dead, hold, err)
	}
	t.Log(err)
}

// TestRouteViaTooNarrow: a 777 cannot be sent along LKPR's code C taxilane
// JO, by a via point or by name; the error says so rather than returning a
// Tight route.
func TestRouteViaTooNarrow(t *testing.T) {
	g := lkprGraph(t)
	b14, _ := g.Layout.ParkingIndex("B14")
	onJO := nodeOn(t, g, "JO")
	if r, err := g.RouteToRunway(b14, "24", RouteOptions{Via: []NodeID{onJO}, HalfSpan: 17.9}); err != nil || !slices.Contains(r.Taxiways, "JO") {
		t.Fatalf("A320 via JO: %v", err)
	}
	for _, opts := range []RouteOptions{
		{Via: []NodeID{onJO}, HalfSpan: 32.4},
		{Taxiways: []string{"JO"}, HalfSpan: 32.4},
	} {
		r, err := g.RouteToRunway(b14, "24", opts)
		var re *RouteError
		if r != nil || !errors.Is(err, ErrTooNarrow) || !errors.As(err, &re) || (re.Taxiway != "JO" && re.Taxiway != "JB") {
			t.Errorf("777 %+v: %v, want ErrTooNarrow on JO", opts, err)
			continue
		}
		if len(opts.Via) > 0 && (re.Via != 0 || re.Node != onJO) {
			t.Errorf("777 via JO: error at via %d node %d", re.Via, re.Node)
		}
		t.Log(err)
	}
}

// TestRouteTaxiways: "via F, L" and friends are followed in order.
func TestRouteTaxiways(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	for _, c := range []struct {
		end string
		tw  []string
	}{
		{"30", []string{"F", "L"}},
		{"30", []string{"f", "l"}},
		{"30", []string{"B"}},
		{"12", []string{"H", "L", "D"}},
		{"24", []string{"J", "H", "A"}},
	} {
		r, err := g.RouteToRunway(c22, c.end, RouteOptions{Taxiways: c.tw})
		if err != nil {
			t.Errorf("%s via %v: %v", c.end, c.tw, err)
			continue
		}
		checkRoute(t, g, r)
		if followedTaxiways(r.Taxiways, c.tw) != len(c.tw) {
			t.Errorf("%s via %v: route %v", c.end, c.tw, r.Taxiways)
		}
		t.Logf("C22 → %s via %v: %v %.0f m", c.end, c.tw, r.Taxiways, r.Length)
	}
	// Following the taxiway the plain route takes changes nothing.
	plain, _ := g.RouteToRunway(c22, "24", RouteOptions{})
	same, err := g.RouteToRunway(c22, "24", RouteOptions{Taxiways: plain.Taxiways})
	if err != nil || !slices.Equal(same.Nodes, plain.Nodes) {
		t.Errorf("via %v: %v, %v", plain.Taxiways, same, err)
	}
}

func TestRouteCustomErrors(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	var re *RouteError
	_, err := g.RouteToRunway(c22, "24", RouteOptions{Via: []NodeID{NodeID(len(g.Nodes))}})
	if !errors.Is(err, ErrViaUnreachable) || !errors.As(err, &re) || re.Via != 0 {
		t.Errorf("via out of range: %v", err)
	}
	_, err = g.RouteToRunway(c22, "24", RouteOptions{Taxiways: []string{"A", "XX"}})
	if !errors.Is(err, ErrUnknownTaxiway) || !errors.As(err, &re) || re.Taxiway != "XX" {
		t.Errorf("unknown taxiway: %v", err)
	}
	// A vehicle stand reaches no runway at all: the plain error, not one
	// blaming the taxiways.
	for _, p := range g.Layout.Parking {
		if p.Type == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE {
			_, err := g.RouteToRunway(p.Index, "24", RouteOptions{Taxiways: []string{"A"}})
			if !errors.Is(err, ErrNoRoute) || errors.Is(err, ErrTaxiwaysNotFollowed) {
				t.Errorf("vehicle stand %s: %v", p.Label(), err)
			}
			break
		}
	}
}

// TestTaxiwaysNotFollowed: a taxiway that exists but does not connect.
func TestTaxiwaysNotFollowed(t *testing.T) {
	g := &Graph{Layout: &Layout{ICAO: "TEST"}, local: newLocalFrame(50, 14)}
	for i, p := range []LatLon{{50, 14}, {50, 14.001}, {50, 14.002}, {50.01, 14}, {50.01, 14.001}} {
		g.Nodes = append(g.Nodes, Node{ID: NodeID(i), Position: p})
	}
	g.Adj = make([][]Edge, len(g.Nodes))
	link := func(a, b NodeID, name string) {
		pa, pb := g.Nodes[a].Position, g.Nodes[b].Position
		d := calc.HaversineMeters(pa.Lat, pa.Lon, pb.Lat, pb.Lon)
		g.Adj[a] = append(g.Adj[a], Edge{To: b, Length: d, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI, Name: name})
		g.Adj[b] = append(g.Adj[b], Edge{To: a, Length: d, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI, Name: name})
	}
	link(0, 1, "A")
	link(1, 2, "A")
	link(3, 4, "C")
	if _, err := g.Route(0, 2, RouteOptions{Taxiways: []string{"A"}}); err != nil {
		t.Fatal(err)
	}
	var re *RouteError
	_, err := g.Route(0, 2, RouteOptions{Taxiways: []string{"A", "C"}})
	if !errors.Is(err, ErrTaxiwaysNotFollowed) || !errors.As(err, &re) || re.Taxiway != "C" {
		t.Errorf("via A, C: %v, want ErrTaxiwaysNotFollowed at C", err)
	}
	t.Log(err)
}

func TestRemainingOptions(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	r, err := g.RouteToRunway(c22, "24", RouteOptions{})
	if err != nil || len(r.Taxiways) < 3 {
		t.Fatal(r, err)
	}
	mid := r.Nodes[len(r.Nodes)/2]
	opts := RouteOptions{Via: []NodeID{r.Nodes[2], mid}, Taxiways: append(slices.Clone(r.Taxiways[:1]), "B")}
	got := g.RemainingOptions(opts, r.Nodes[:4])
	if !slices.Equal(got.Via, []NodeID{mid}) || !slices.Equal(got.Taxiways, []string{"B"}) {
		t.Errorf("after 4 nodes of %v: via %v taxiways %v", r.Taxiways, got.Via, got.Taxiways)
	}
}
