package world

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

const pairModel = "FSLTL_A320_LYX"

// pairDep is a departure waiting with its way out.
type pairDep struct {
	passDep
	route *airport.Route
}

func (d *pairDep) Route() *airport.Route { return d.route }

func offBy(p airport.LatLon, hdg, m float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, hdg, m)
	return airport.LatLon{Lat: lat, Lon: lon}
}

func pairBed(t *testing.T) (*controlCenter, *airport.Graph) {
	t.Helper()
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
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
	return cc, g
}

// inAlong is a backtracking arrival's plan with its way in westwards (or
// eastwards) through mid.
func inAlong(t *testing.T, g *airport.Graph, mid airport.LatLon, west bool) *traffic.ArrivalPlan {
	t.Helper()
	_, end, ok := g.Layout.RunwayEnd("24")
	if !ok {
		t.Fatal("no runway 24")
	}
	from, to := 90.0, 270.0
	if !west {
		from, to = to, from
	}
	p := &traffic.ArrivalPlan{Backtrack: true, ClearIndex: 1, VacateIndex: 1, Route: &airport.Route{Points: []airport.LatLon{
		offBy(mid, from, 400), offBy(mid, from, 300), mid, offBy(mid, to, 300)}}}
	p.End = end
	return p
}

// pairOf puts an arrival coming as plan says and a departure waiting on
// stand, its way out to lane and east along it, at cc.
func pairOf(cc *controlCenter, g *airport.Graph, state traffic.ArrivalState, plan *traffic.ArrivalPlan, out float64, stand, lane airport.LatLon) (arr, dep *controlled) {
	arr = &controlled{ID: 1, Tail: "TST1", ICAO: "LKPR", cc: cc, arr: &passArr{state: state, plan: plan}, graph: g, stand: -1}
	arr.view.Model, arr.view.Position = pairModel, offBy(plan.End.LandingThreshold(), 60, out)
	r := &airport.Route{Length: 420, Points: []airport.LatLon{stand, lane, offBy(lane, 90, 300)}}
	dep = &controlled{ID: 2, Tail: "TST2", ICAO: "LKPR", cc: cc, dep: &pairDep{route: r}, graph: g, spoken: map[string]bool{}}
	dep.view.Model, dep.view.Position = pairModel, stand
	cc.mu.Lock()
	cc.items[1], cc.items[2] = arr, dep
	cc.mu.Unlock()
	return arr, dep
}

// TestPairedArrival (#1641): a departure waits for a backtracking arrival
// whose way in meets its own, from the approach until that is parked; not
// one it can be gone before, nor, still on approach, one it stands in the
// way of.
func TestPairedArrival(t *testing.T) {
	cc, g := pairBed(t)
	stand := g.Layout.Parking[0].Position
	lane := offBy(stand, 0, 120)
	for _, tc := range []struct {
		name      string
		state     traffic.ArrivalState
		out       float64 // metres to the threshold
		mid       airport.LatLon
		west      bool
		backtrack bool
		player    bool
		held      bool
	}{
		{"approaching, opposed", traffic.ArrivalApproaching, 5000, lane, true, true, false, true},
		{"approaching, converging the same way", traffic.ArrivalApproaching, 5000, lane, false, true, false, true},
		{"a player's arrival is waited for too", traffic.ArrivalApproaching, 5000, lane, true, true, true, true},
		{"gone before it is down", traffic.ArrivalApproaching, 100000, lane, true, true, false, false},
		{"in its way where it stands: first", traffic.ArrivalApproaching, 5000, stand, true, true, false, false},
		{"in its way, committed", traffic.ArrivalLanding, 100, stand, true, true, false, true},
		{"rolling out", traffic.ArrivalRollout, 0, lane, true, true, false, true},
		{"onto its stand", traffic.ArrivalParking, 0, offBy(lane, 90, 300), true, true, false, true},
		{"parked", traffic.ArrivalParked, 0, lane, true, true, false, false},
		{"elsewhere", traffic.ArrivalApproaching, 5000, offBy(stand, 0, 1000), true, true, false, false},
		{"no backtrack", traffic.ArrivalApproaching, 5000, lane, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := inAlong(t, g, tc.mid, tc.west)
			plan.Backtrack = tc.backtrack
			arr, dep := pairOf(cc, g, tc.state, plan, tc.out, stand, lane)
			arr.gates.Store(tc.player)
			if o, _ := dep.pairedArrival(); (o == arr) != tc.held {
				t.Errorf("held %v, want %v", o == arr, tc.held)
			}
		})
	}
}

// TestPairedHoldEnds (#1641): the wait for one still on approach ends after
// pushLegMaxWait, so a stream of arrivals never holds it for good; for one
// landed it doesn't.
func TestPairedHoldEnds(t *testing.T) {
	cc, g := pairBed(t)
	stand := g.Layout.Parking[0].Position
	lane := offBy(stand, 0, 120)
	arr, dep := pairOf(cc, g, traffic.ArrivalApproaching, inAlong(t, g, lane, true), 5000, stand, lane)
	dep.request = "pushback"
	dep.spoken["standby backtrack"] = true
	if !dep.holdForBacktrack("pushback") {
		t.Fatal("not held")
	}
	dep.pairHeldFrom = cc.clock.Now().Add(-pushLegMaxWait - time.Minute)
	if dep.holdForBacktrack("pushback") {
		t.Error("still held for one on approach")
	}
	arr.arr.(*passArr).state = traffic.ArrivalRollout
	if !dep.holdForBacktrack("pushback") {
		t.Error("let onto the way of one landed")
	}
}

// TestDepartureStandClearOfWayIn (#1641): a departure's stand is out of a
// backtracking arrival's way in; none such, none is given; a player's is
// not chosen at all (#1096).
func TestDepartureStandClearOfWayIn(t *testing.T) {
	cc, g := pairBed(t)
	req := traffic.StandRequirements{Owner: "TST2", HalfSpan: halfSpanOf(pairModel)}
	if _, ok, _ := cc.clearDepartureStand(g, traffic.NewStandAllocator(nil, g), req, "", pairModel, "24"); ok {
		t.Fatal("chose with no arrival to be clear of")
	}
	// The way in through the stand it would be given.
	usual, err := assignStand(traffic.NewStandAllocator(nil, g), req, "")
	if err != nil {
		t.Fatal(err)
	}
	plan := inAlong(t, g, g.Layout.Parking[usual].Position, true)
	arr := &controlled{ID: 1, Tail: "TST1", ICAO: g.Layout.ICAO, cc: cc, arr: &passArr{state: traffic.ArrivalApproaching, plan: plan}, graph: g, stand: -1}
	arr.view.Model = pairModel
	cc.mu.Lock()
	cc.items[1] = arr
	cc.mu.Unlock()
	in, ok := arr.wayIn()
	if !ok {
		t.Fatal("no way in")
	}
	a := traffic.NewStandAllocator(nil, g)
	got, ok, err := cc.clearDepartureStand(g, a, req, "", pairModel, "24")
	if !ok || err != nil {
		t.Fatalf("no stand: %v %v", ok, err)
	}
	if got == usual || meetsPlace(in, cc.placeOnStand(g, got, pairModel)) {
		t.Errorf("stand %s is in the way in", g.Layout.Parking[got].Label())
	}
	if o, ok := a.Occupant(got); !ok || o.Owner != "TST2" {
		t.Error("not reserved for it")
	}
	full := traffic.NewStandAllocator(nil, g)
	for i := range g.Layout.Parking {
		if i != usual {
			_ = full.Occupy(i, "TST3", 1)
		}
	}
	if i, ok, err := cc.clearDepartureStand(g, full, req, "", pairModel, "24"); !ok || !errors.Is(err, traffic.ErrNoStand) {
		t.Errorf("stand %d (%v, %v) with only one in the way free", i, ok, err)
	}
	if i := cc.standForDeparture(g, SpawnRequest{Kind: "departure", Stand: -1, Gates: true, Model: pairModel, Runway: "24"}); i != -1 {
		t.Errorf("chose stand %d for a player", i)
	}
}

// TestAssignAvoids (#1641): a stand to avoid is never assigned.
func TestAssignAvoids(t *testing.T) {
	_, g := pairBed(t)
	req := traffic.StandRequirements{Owner: "TST2", HalfSpan: halfSpanOf(pairModel)}
	usual, err := traffic.NewStandAllocator(nil, g).Assign(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Avoid = []int{usual}
	if got, err := traffic.NewStandAllocator(nil, g).Assign(req); err != nil || got == usual {
		t.Errorf("stand %d (%v), one to avoid", got, err)
	}
}

// apronArrival has a way clear of the runway's places where narrow says;
// clear of the apron's only to another stand, where other says.
type apronArrival struct {
	arrivalCtl
	endTurn bool
	narrow  bool
	other   bool
	took    []int
}

func (a *apronArrival) Plan() *traffic.ArrivalPlan {
	return &traffic.ArrivalPlan{EndTurn: a.endTurn, Backtrack: true}
}

func (a *apronArrival) State() traffic.ArrivalState { return traffic.ArrivalApproaching }

func (a *apronArrival) WayOffClear(occ []airport.Occupied) bool { return a.narrow && len(occ) == 0 }

func (a *apronArrival) PlanWayOff(parking int, _ []airport.Occupied) func() bool {
	if parking < 0 || !a.other {
		return nil
	}
	return func() bool {
		a.took = append(a.took, parking)
		return true
	}
}

// TestWayInClearOfApron (#1641): the arrival is planned round who stands on
// the apron before it touches down; with no way round it isn't sent round
// for them, and a player's is never moved (#1096).
func TestWayInClearOfApron(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		endTurn, narrow, other, player bool
		moves, apron, round            bool
	}{
		{"another stand", true, true, true, false, true, false, false},
		{"none: the departure first", true, true, false, false, false, true, false},
		{"none, no turn at the end", false, false, false, false, false, true, false},
		{"none clear of the runway's either", true, false, false, false, false, false, true},
		{"a player's", true, true, true, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cc, g := pairBed(t)
			was, err := g.Layout.ParkingIndex("C22")
			if err != nil {
				t.Fatal(err)
			}
			a := traffic.NewStandAllocator(nil, g)
			if err := a.Occupy(was, "TST1", halfSpanOf(pairModel)); err != nil {
				t.Fatal(err)
			}
			arr := &apronArrival{endTurn: tc.endTurn, narrow: tc.narrow, other: tc.other}
			it := &controlled{ID: 1, Tail: "TST1", ICAO: "LKPR", cc: cc, arr: arr, graph: g, stands: a, stand: was}
			it.view.Model, it.view.Stand = pairModel, g.Layout.Parking[was].Label()
			it.gates.Store(tc.player)
			dep := &controlled{ID: 2, Tail: "TST2", ICAO: "LKPR", cc: cc, dep: &staleDeparture{state: traffic.TaxiAwaitingTaxi}, graph: g}
			dep.view.Model, dep.view.Position = pairModel, g.Layout.Parking[0].Position
			cc.mu.Lock()
			cc.items[1], cc.items[2] = it, dep
			cc.mu.Unlock()
			items := cc.snapshotItems()
			if tc.player {
				cc.keepWayOffClear(items)
				if it.wayOffPlanning.Load() {
					t.Fatal("searched for a player's")
				}
			}
			if ap := apronPlacesFor(it, items); len(ap.bodies) != 1 {
				t.Fatalf("%d bodies on the apron, want the departure's", len(ap.bodies))
			}
			cc.planWayOff(it, arr, wayOffTaken(it, items))
			if moved := it.stand != was; moved != tc.moves || slices.Contains(arr.took, was) {
				t.Errorf("moved %v (took %v), want %v", moved, arr.took, tc.moves)
			}
			if got := it.apronBlocked.Load(); got != tc.apron {
				t.Errorf("departure first %v, want %v", got, tc.apron)
			}
			if got := it.wayOffBlocked.Load(); got != tc.round {
				t.Errorf("waits for the runway %v, want %v", got, tc.round)
			}
		})
	}
}
