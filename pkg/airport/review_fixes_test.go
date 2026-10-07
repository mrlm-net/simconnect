package airport

import (
	"math"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestRunwayExitsDeterministic: the same exits in the same order every
// time, ties included (#44: EDDF 07C at 1000 m gave L16 or M28).
func TestRunwayExitsDeterministic(t *testing.T) {
	g, err := BuildGraph(loadAirport(t, "EDDF"))
	if err != nil {
		t.Fatal(err)
	}
	for _, end := range runwayEnds(g.Layout) {
		first, err := g.RunwayExits(end)
		if err != nil {
			t.Fatal(err)
		}
		firstFor, _ := g.ExitFor(end, 1000)
		for range 15 {
			again, _ := g.RunwayExits(end)
			if !reflect.DeepEqual(again, first) {
				t.Fatalf("%s: exits differ between calls", end)
			}
			if x, _ := g.ExitFor(end, 1000); x.Node != firstFor.Node {
				t.Fatalf("%s at 1000 m: exit %s then %s", end, firstFor.Taxiway, x.Taxiway)
			}
		}
		if !slices.IsSortedFunc(first, func(a, b RunwayExit) int {
			switch {
			case a.Along < b.Along:
				return -1
			case a.Along > b.Along:
				return 1
			}
			return 0
		}) {
			t.Errorf("%s: exits not by distance", end)
		}
	}
}

// TestRunwayEntryTies: among entries with the same runway ahead, the route
// goes to the nearest, every time (#43: LKPR "30 at D").
func TestRunwayEntryTies(t *testing.T) {
	g := lkprGraph(t)
	entries, err := g.RunwayEntries("30")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range g.Layout.Parking {
		if p.Type == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE {
			continue
		}
		from, _ := g.ParkingNode(p.Index)
		s := g.shortestPaths(from, -1, RouteOptions{OwnStands: []int{p.Index}})
		var first *Route
		for range 5 {
			r, err := g.RouteToRunwayEntry(p.Index, "30", "D", RouteOptions{})
			if err != nil {
				break
			}
			if first == nil {
				first = r
			} else if !slices.Equal(r.Nodes, first.Nodes) {
				t.Fatalf("%s: 30 at D differs between calls", p.Label())
			}
		}
		if first == nil {
			continue
		}
		// No entry D with as much runway ahead is nearer than the one taken.
		end := first.Nodes[len(first.Nodes)-1]
		var remaining float64
		for _, e := range entries {
			if e.HoldShort == end || e.HoldShort < 0 && e.Node == end {
				remaining = e.Remaining
			}
		}
		for _, e := range entries {
			tgt := e.HoldShort
			if tgt < 0 {
				tgt = e.Node
			}
			if e.Taxiway == "D" && e.Remaining == remaining && s.dist[tgt] < s.dist[end]-1e-6 {
				t.Errorf("%s: 30 at D to node %d, node %d is nearer with the same runway", p.Label(), end, tgt)
			}
		}
	}
}

// TestRouteKeepsSearchedParallelEdge: of two parallel edges the route keeps
// the one its search took, not the shorter (#48).
func TestRouteKeepsSearchedParallelEdge(t *testing.T) {
	g := &Graph{Layout: &Layout{ICAO: "TEST"}, local: newLocalFrame(50, 14)}
	for i, p := range []LatLon{{50, 14}, {50, 14.001}} {
		g.Nodes = append(g.Nodes, Node{ID: NodeID(i), Position: p})
	}
	g.Adj, g.stands = make([][]Edge, len(g.Nodes)), make([]bool, len(g.Nodes))
	d := calc.HaversineMeters(50, 14, 50, 14.001)
	link := func(a, b NodeID, name string, length float64) {
		g.Adj[a] = append(g.Adj[a], Edge{To: b, Length: length, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI, Name: name})
		g.Adj[b] = append(g.Adj[b], Edge{To: a, Length: length, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI, Name: name})
	}
	link(0, 1, "A", d)
	link(0, 1, "B", d+2)
	r, err := g.Route(0, 1, RouteOptions{Taxiways: []string{"B"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Taxiways, []string{"B"}) || len(r.Edges) != 1 || r.Edges[0].Name != "B" || math.Abs(r.Length-(d+2)) > 1e-9 {
		t.Errorf("taxiways %v edges %+v length %.1f", r.Taxiways, r.Edges, r.Length)
	}
	// Without a taxiway asked for: the shorter one, as before.
	if r, err := g.Route(0, 1, RouteOptions{}); err != nil || r.Edges[0].Name != "A" {
		t.Errorf("free route: %+v %v", r, err)
	}
}

// TestRunwayCrossingsStartOnRunway: the runway a route starts on is not a
// crossing; leaving it and coming back is (#49).
func TestRunwayCrossingsStartOnRunway(t *testing.T) {
	rwy := Runway{Index: 0, Length: calc.HaversineMeters(50, 14, 50, 14.03), Width: 45,
		Primary:   RunwayEnd{Name: "09", Threshold: LatLon{Lat: 50, Lon: 14}},
		Secondary: RunwayEnd{Name: "27", Threshold: LatLon{Lat: 50, Lon: 14.03}}}
	g := &Graph{Layout: &Layout{ICAO: "TEST", Runways: []Runway{rwy}}, local: newLocalFrame(50, 14)}
	on, off, on2 := LatLon{Lat: 50, Lon: 14.01}, LatLon{Lat: 50.001, Lon: 14.01}, LatLon{Lat: 50, Lon: 14.012}
	if got := g.runwayCrossings([]LatLon{on, off}); len(got) != 0 {
		t.Errorf("vacating: %v", got)
	}
	if got := g.runwayCrossings([]LatLon{on, off, on2}); !slices.Equal(got, []string{"09/27"}) {
		t.Errorf("back onto it: %v", got)
	}
	if got := g.runwayCrossings([]LatLon{off, {Lat: 49.999, Lon: 14.01}}); !slices.Equal(got, []string{"09/27"}) {
		t.Errorf("across: %v", got)
	}
}

// TestLimitsForDeepCopy: changing what LimitsFor returns leaves the table
// alone (E19).
func TestLimitsForDeepCopy(t *testing.T) {
	old := knownLimitsNow.Load()
	defer knownLimitsNow.Store(old)
	table := map[string]Limits{"ZZZZ": {InitialClimbs: map[string]float64{"ABC1A": 5000}, Tower: &TowerSite{CabM: 40}}}
	knownLimitsNow.Store(table)
	lim := LimitsFor(&Layout{ICAO: "ZZZZ"}, nil)
	lim.InitialClimbs["ABC1A"] = 1
	lim.Tower.CabM = 1
	again := LimitsFor(&Layout{ICAO: "ZZZZ"}, nil)
	if again.InitialClimbs["ABC1A"] != 5000 || again.Tower.CabM != 40 {
		t.Errorf("table changed: %v, cab %v", again.InitialClimbs, again.Tower.CabM)
	}
}

// TestVehicleGraphEvicted: a layout's vehicle graph goes with the layout
// (E18).
func TestVehicleGraphEvicted(t *testing.T) {
	count := func() int {
		n := 0
		vehicleGraphs.Range(func(any, any) bool { n++; return true })
		return n
	}
	before := count()
	l := loadLKPR(t)
	g := l.vehicleGraph()
	if l.vehicleGraph() != g || count() != before+1 {
		t.Fatal("not cached")
	}
	l = nil
	for range 20 {
		runtime.GC()
		if count() == before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("%d graphs cached, want %d", count(), before)
}

// TestLoaderLateReply: an expired airport keeps its slot until its late
// replies are in, which never land in another airport (#45).
func TestLoaderLateReply(t *testing.T) {
	c := &fakeClient{}
	l := NewLoader(c, LoaderWithTimeout(time.Second))
	for i := range loaderSlots {
		if err := l.Request("A" + string(rune('A'+i))); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Add(2 * time.Second)
	if res := l.Expire(now); len(res) != loaderSlots {
		t.Fatalf("expired %d", len(res))
	}
	if len(l.Pending()) != 0 {
		t.Errorf("pending %v", l.Pending())
	}
	if err := l.Request("LKPR"); err == nil {
		t.Fatal("a held slot was reused")
	}
	// The first slot's late replies come in: free again.
	for p := range loaderDefinitions {
		if r, ok := l.Handle(endMsg(DefaultLoaderRequestBase + uint32(p))); ok {
			t.Fatalf("late reply reported %+v", r)
		}
	}
	if err := l.Request("LKPR"); err != nil {
		t.Fatal(err)
	}
	if got := c.requests[len(c.requests)-1].req; got >= DefaultLoaderRequestBase+uint32(len(loaderDefinitions)) {
		t.Errorf("LKPR on request %d, want the freed first slot", got)
	}
	// The others come back after another timeout without replies.
	l.Expire(now.Add(500 * time.Millisecond))
	if err := l.Request("LKTB"); err == nil {
		t.Fatal("held slots freed early")
	}
	l.Expire(now.Add(2 * time.Second))
	if err := l.Request("LKTB"); err != nil {
		t.Fatal(err)
	}
}

// TestProcedureLoaderExpire: loads never finished end (also by Request
// when every slot is taken), their late replies are absorbed, and Reset
// starts over (#46).
func TestProcedureLoaderExpire(t *testing.T) {
	c := &fakeClient{}
	l := NewProcedureLoader(c)
	l.SetTimeout(time.Millisecond)
	for i := range procedureSlots {
		if err := l.Request("A" + string(rune('A'+i))); err != nil {
			t.Fatal(err)
		}
	}
	if len(l.Pending()) != procedureSlots {
		t.Fatalf("pending %v", l.Pending())
	}
	time.Sleep(5 * time.Millisecond)
	// Every slot stuck: the next request ends them, but their IDs stay
	// held for another timeout.
	if err := l.Request("LKPR"); err == nil {
		t.Fatal("a held slot was reused at once")
	}
	if got := l.Expire(time.Now()); len(got) != procedureSlots || got[0] != "AA" {
		t.Errorf("Expire = %v", got)
	}
	time.Sleep(5 * time.Millisecond)
	if err := l.Request("LKPR"); err != nil {
		t.Fatalf("stuck loads still block: %v", err)
	}
	// A late reply to an ended load is absorbed and not taken for LKPR's.
	l.SetTimeout(time.Hour)
	l.Reset(nil)
	if err := l.Request("AAAA"); err != nil {
		t.Fatal(err)
	}
	if got := l.Expire(time.Now().Add(2 * time.Hour)); len(got) != 1 || got[0] != "AAAA" {
		t.Fatalf("Expire = %v", got)
	}
	base := DefaultProcedureRequestBase
	for i := uint32(0); i < procedureParts; i++ {
		if p, ok := l.Handle(endMsg(base + i)); ok {
			t.Fatalf("late reply reported %+v", p)
		}
	}
	if err := l.Request("LKPR"); err != nil {
		t.Fatal(err)
	}
	if c.requests[len(c.requests)-1].req != base+procedureParts-1 {
		t.Errorf("LKPR not on the freed first slot")
	}
	done := false
	for i := uint32(0); i < procedureParts; i++ {
		if p, ok := l.Handle(endMsg(base + i)); ok {
			done = p.ICAO == "LKPR"
		}
	}
	if !done {
		t.Error("LKPR not finished")
	}
	// Reset forgets everything and registers the definitions again.
	defs := len(c.defs[DefaultProcedureDefinitionBase])
	l.Reset(nil)
	if len(l.Pending()) != 0 {
		t.Error("Reset kept loads")
	}
	if err := l.Request("LKPR"); err != nil || len(c.defs[DefaultProcedureDefinitionBase]) != defs+len(procedureDefinitions()[0]) {
		t.Errorf("after Reset: %v, %d fields", err, len(c.defs[DefaultProcedureDefinitionBase]))
	}
}

// TestProcedureMagVarFromDepartures: MAGVAR is read from the departures
// request's AIRPORT record only (E17).
func TestProcedureMagVarFromDepartures(t *testing.T) {
	l := NewProcedureLoader(&fakeClient{})
	if err := l.Request("LKPR"); err != nil {
		t.Fatal(err)
	}
	base := DefaultProcedureRequestBase
	f32 := func(v float32) []byte {
		b := make([]byte, 4)
		u := math.Float32bits(v)
		b[0], b[1], b[2], b[3] = byte(u), byte(u>>8), byte(u>>16), byte(u>>24)
		return b
	}
	l.Handle(procRecord(base, types.SIMCONNECT_FACILITY_DATA_AIRPORT, 1, 0, f32(356)))
	l.Handle(procRecord(base+2, types.SIMCONNECT_FACILITY_DATA_AIRPORT, 2, 0, f32(123))) // approaches: no MAGVAR field
	var got Procedures
	for i := uint32(0); i < procedureParts; i++ {
		if p, ok := l.Handle(endMsg(base + i)); ok {
			got = p
		}
	}
	if got.MagVar != 356 {
		t.Errorf("MagVar %v, want 356", got.MagVar)
	}
}
