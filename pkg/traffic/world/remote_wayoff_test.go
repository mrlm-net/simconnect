package world

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// This file uses nothing #1655 added, so that it builds and fails without
// the fix.

const splitModel = "FSLTL A320 Air France SL"

// splitClient is a simulator that takes every call an injected arrival's
// start makes.
type splitClient struct{ rescueClient }

func (c *splitClient) AICreateNonATCAircraft(string, string, types.SIMCONNECT_DATA_INITPOSITION, uint32) error {
	return nil
}

func (c *splitClient) AICreateNonATCAircraftEX1(string, string, string, types.SIMCONNECT_DATA_INITPOSITION, uint32) error {
	return nil
}

func (c *splitClient) ClearDataDefinition(uint32) error { return nil }

// midExitField is traffic's midExitGraph (#1631): a 2500 m 09/27, landing
// 27 nothing in reach but the roll to the end and back, to C at mid-length
// or to D at the far end, which leans the way it comes back so that it is
// one.
func midExitField(t *testing.T) *airport.Graph {
	t.Helper()
	var pts []airport.RawTaxiPoint
	var paths []airport.RawTaxiPath
	pt := func(x, z float32, typ int32) int32 {
		pts = append(pts, airport.RawTaxiPoint{Type: typ, BiasX: x, BiasZ: z})
		return int32(len(pts) - 1)
	}
	link := func(a, b, typ int32, name uint32, width float32, rwy int32) {
		paths = append(paths, airport.RawTaxiPath{Type: typ, Width: width, RunwayNumber: rwy, Start: a, End: b, NameIndex: name})
	}
	xs := []float32{-1250, -625, 0, 625, 1250}
	exits := map[float32]uint32{-1250: 2, 0: 3, 1250: 4}
	rwy, a := map[float32]int32{}, map[float32]int32{}
	for i, x := range xs {
		rwy[x] = pt(x, 0, 1)
		if i > 0 {
			link(rwy[xs[i-1]], rwy[x], 2, 0, 45, 9)
		}
		name, ok := exits[x]
		if !ok {
			continue
		}
		var hx float32
		if x != 0 {
			hx = x - float32(math.Copysign(10, float64(x)))
		}
		if x == 1250 {
			hx = x + 10
		}
		a[x] = pt(hx, 180, 1)
		h := pt(hx, 75, 2)
		link(rwy[x], h, 1, name, 23, 0)
		link(h, a[x], 1, name, 23, 0)
	}
	apron := pt(300, 180, 1)
	link(a[-1250], a[0], 1, 1, 23, 0)
	link(a[0], apron, 1, 1, 23, 0)
	link(apron, a[1250], 1, 1, 23, 0)
	lead := pt(300, 260, 1)
	link(apron, lead, 1, 5, 20, 0)
	link(lead, 0, 3, 0, 20, 0)
	l, err := airport.BuildLayout(airport.RawAirport{ICAO: "XMID", Name: "Mid exit", Latitude: 50.1214, Longitude: 13.8124, Altitude: 300,
		Runways:    []airport.RawRunway{{Latitude: 50.1214, Longitude: 13.8124, Altitude: 300, Heading: 90, Length: 2500, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking:    []airport.RawParking{{Name: 14, Number: 1, Type: 9, Heading: 0, Radius: 24, BiasX: 300, BiasZ: 310}},
		TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "B", "C", "D", "S"}})
	if err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// splitBed is a World with an arrival on approach to 27 and a departure
// holding short where the arrival would come off; split: the arrival's
// controller at the other end of a link.
type splitBed struct {
	cc    *controlCenter
	it    *controlled
	arr   arrivalCtl
	srv   *wireServer
	c     *wireClient
	act   link
	items []*controlled
	first []airport.NodeID // the route planned at the start
}

func newSplitBed(t *testing.T, split bool) *splitBed {
	t.Helper()
	g := midExitField(t)
	w := New(Options{DataDir: t.TempDir()})
	cc := newControlCenter(nil, w.st.core)
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case f := <-cc.cmds:
				f()
			case <-stop:
				return
			}
		}
	}()
	b := &splitBed{cc: cc}
	client := &splitClient{}
	sim := &localSim{client: client, fleet: traffic.NewFleet(client), inj: traffic.NewInjector(client),
		detail: cc.detail, world: cc.world, clock: cc.clock}
	var port simPort = sim
	var act *actuatorSim
	if split {
		dir, a := pipe()
		t.Cleanup(func() { dir.Close() })
		b.act, b.srv = a, newWireServer()
		act = &actuatorSim{localSim: sim, srv: b.srv, send: a.Send,
			graph: func(string) (*airport.Graph, error) { return g, nil }}
		b.srv.add("sim", act)
		go serve(a, b.srv)
		b.c = newWireClient(dir, nil)
		port = &remoteSim{c: b.c, t: "sim"}
	}
	const reqBase = 9000
	arr, evs, err := port.StartArrival(8000, reqBase, traffic.ArrivalRequest{Graph: g, Runway: "27", Parking: 0, Model: splitModel, Tail: "TST1", InjectApproach: true})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range evs {
		}
	}()
	b.arr = arr
	// The simulator's answer to the creation: on approach.
	if msg := assignedObject(reqBase, 77); split {
		act.handle(msg)
	} else {
		arr.Handle(msg)
	}
	if s := arr.State(); s != traffic.ArrivalApproaching {
		t.Fatalf("arrival %v, not on approach", s)
	}
	plan := arr.Plan()
	if plan == nil || !plan.EndTurn {
		t.Fatalf("no roll to the end planned: %+v", plan)
	}
	b.first = slices.Clone(plan.Route.Nodes)
	var hold airport.LatLon
	for i, n := range plan.Route.Nodes {
		if g.Nodes[n].HoldShort != nil {
			hold = plan.Route.Points[i]
			break
		}
	}
	if hold == (airport.LatLon{}) {
		t.Fatal("no holding point on the way off")
	}
	stands := traffic.NewStandAllocator(nil, g)
	if err := stands.Occupy(0, "TST1", halfSpanOf(splitModel)); err != nil {
		t.Fatal(err)
	}
	b.it = &controlled{ID: 1, Tail: "TST1", ICAO: "XMID", cc: cc, arr: arr, graph: g, stands: stands, stand: 0, removed: make(chan struct{}), spoken: map[string]bool{}}
	b.it.view.Model, b.it.view.Stand = splitModel, g.Layout.Parking[0].Label()
	dep := &controlled{ID: 2, Tail: "TST2", ICAO: "XMID", cc: cc, dep: &bedDeparture{staleDeparture{state: traffic.TaxiHoldingShort}}, graph: g, removed: make(chan struct{})}
	dep.view.Model, dep.view.Position = splitModel, hold
	cc.mu.Lock()
	cc.items[1], cc.items[2] = b.it, dep
	cc.mu.Unlock()
	b.items = cc.snapshotItems()
	return b
}

// bedDeparture is the bed's departure holding short: it finds no way round
// what it is to keep clear of, and stays.
type bedDeparture struct {
	staleDeparture
}

func (d *bedDeparture) AvoidOccupied([]airport.Occupied) bool { return false }

// look is one keep-clear second, its search waited for.
func (b *splitBed) look(t *testing.T) {
	t.Helper()
	b.cc.keepWayOffClear(b.items)
	for until := time.Now().Add(10 * time.Second); b.it.wayOffPlanning.Load(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(until) {
			t.Fatal("the search never ended")
		}
	}
}

// snapshot sends the director the actuator's state, as each second.
func (b *splitBed) snapshot() {
	if b.srv != nil {
		(&wireFeedOut{send: b.act.Send}).put("ctlstate", b.srv.snapshot())
	}
}

// TestWayOffPlannedOverLink (#1655): with the director split from the
// actuator an arrival's way off is planned round a departure holding short
// as it is in process, and the director sees the new plan at once, not its
// snapshot of the old.
func TestWayOffPlannedOverLink(t *testing.T) {
	route := func(split bool) []airport.NodeID {
		b := newSplitBed(t, split)
		b.snapshot()
		b.look(t)
		// Over the link the look itself may still be out: its route waited for.
		got := b.arr.Plan().Route.Nodes
		for until := time.Now().Add(10 * time.Second); slices.Equal(got, b.first) && time.Now().Before(until); got = b.arr.Plan().Route.Nodes {
			time.Sleep(10 * time.Millisecond)
		}
		if b.it.wayOffBlocked.Load() {
			t.Errorf("split %v: waits for the runway", split)
		}
		if slices.Equal(got, b.first) {
			t.Fatalf("split %v: way off not planned", split)
		}
		return got
	}
	if local, linked := route(false), route(true); !slices.Equal(local, linked) {
		t.Errorf("over the link %v, in process %v", linked, local)
	}
}

// TestWayOffPlayerNotPlannedOverLink (#1655, #1096): a player's arrival is
// never planned, over the link neither.
func TestWayOffPlayerNotPlannedOverLink(t *testing.T) {
	b := newSplitBed(t, true)
	b.it.gates.Store(true)
	b.look(t)
	if got := b.arr.Plan().Route.Nodes; !slices.Equal(got, b.first) {
		t.Error("a player's arrival re-planned")
	}
	if b.it.wayOffBlocked.Load() || b.it.stand != 0 {
		t.Error("a player's arrival blocked or moved")
	}
}

// oldActuator is an actuator's sim port from before #1655: an arrival's
// start has no result.
type oldActuator struct{}

func (oldActuator) StartArrival(arrivalStart) error { return nil }

// oldArrival is its controller, rolling to the end.
type oldArrival struct{ arrivalCtl }

func (oldArrival) State() traffic.ArrivalState { return traffic.ArrivalApproaching }

func (oldArrival) Plan() *traffic.ArrivalPlan { return &traffic.ArrivalPlan{EndTurn: true} }

// TestWayOffOlderActuator (#1655): an actuator from before keeps its
// behaviour: never asked, never blocked.
func TestWayOffOlderActuator(t *testing.T) {
	b := newSplitBed(t, true)
	dir, act := pipe()
	defer dir.Close()
	srv := newWireServer()
	srv.add("sim", oldActuator{})
	go serve(act, srv)
	c := newWireClient(dir, nil)
	var lost []error
	c.onError = func(err error) { lost = append(lost, err) }
	arr, _, err := (&remoteSim{c: c, t: "sim"}).StartArrival(1, 2, traffic.ArrivalRequest{})
	if err != nil {
		t.Fatalf("an older actuator's start: %v", err)
	}
	srv.add(arr.(*remoteArr).t, oldArrival{})
	b.it.arr = arr
	b.look(t)
	if len(lost) != 0 {
		t.Errorf("asked an older actuator: %v", lost)
	}
	if b.it.wayOffBlocked.Load() || b.it.apronBlocked.Load() {
		t.Error("blocked by an older actuator")
	}
}
