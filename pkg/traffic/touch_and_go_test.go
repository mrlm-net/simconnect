package traffic

import (
	"slices"
	"sync"
	"testing"
	"time"
)

// TestTouchAndGo: a C172 in the LKPR 24 circuit with one touch-and-go
// lands, takes off again from its roll (never vacating), MSFS AI flies the
// circuit from the crosswind round to the final, the injected approach
// takes over there again, and the second landing is a full stop to its
// stand.
func TestTouchAndGo(t *testing.T) { testTouchAndGo(t, false) }

// TestStopAndGo: the same with a stop-and-go: it brakes to a stop on the
// runway, stands StopAndGoWait, and takes off from there (#567).
func TestStopAndGo(t *testing.T) { testTouchAndGo(t, true) }

func testTouchAndGo(t *testing.T, stopGo bool) {
	g := lkprGraph(t)
	p := ProfileFor("C172")
	ci, err := NewCircuit(g.Layout, "24", CircuitConfig{}, p)
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: st, Model: "Asobo PassiveAircraft C172", Tail: "OKTNG",
		InjectApproach: true, Circuit: &ci, TouchAndGos: 1, StopAndGo: stopGo, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var states []ArrivalState
	tngSeen, stopped := false, false
	go func() {
		for ev := range ctl.Events() {
			mu.Lock()
			if len(states) == 0 || states[len(states)-1] != ev.State {
				states = append(states, ev.State)
			}
			tngSeen = tngSeen || ev.TouchAndGo
			stopped = stopped || ev.TouchAndGo && ev.GroundSpeed < 0.5
			mu.Unlock()
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 79))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 79, 1200, 12))
	fin, _ := ci.Point(LegFinal)
	end := ctl.Plan().End
	// MSFS AI's part: it reports itself on the final, where the injected
	// approach takes over (each time it flies the circuit).
	joined := 0
	for i := 0; i < 60*60*20 && ctl.State() != ArrivalParked && !ctl.State().Terminal(); i++ {
		now = now.Add(time.Second / 60)
		if ctl.flyingProc {
			at := offsetHeading(fin.Position, end.Heading, 50)
			ctl.Handle(arrivalPositionMsg(mon, 79, at, fin.AltFt-g.Layout.Altitude/0.3048, end.Heading, fin.Kts, false))
			if !ctl.flyingProc {
				joined++
			}
			continue
		}
		ctl.Handle(arrivalPositionMsg(mon, 79, end.Threshold, 0, 0, 0, false)) // a frame tick
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("state %v, joined the final %d times", ctl.State(), joined)
	}
	mu.Lock()
	defer mu.Unlock()
	if stopGo && !stopped {
		t.Error("a stop-and-go that never stopped")
	}
	if joined != 2 || !tngSeen || ctl.TouchAndGosLeft() != 0 {
		t.Errorf("joined the final %d times (want 2), touch-and-go seen %v, left %d", joined, tngSeen, ctl.TouchAndGosLeft())
	}
	// Approaching, landing, rollout (the touch-and-go), approaching again,
	// landing, rollout, vacating, …, parked: no vacating in between.
	first := slices.Index(states, ArrivalVacating)
	if n := len(slices.DeleteFunc(slices.Clone(states[:max(first, 0)]), func(s ArrivalState) bool { return s != ArrivalRollout })); first < 0 || n < 2 {
		t.Errorf("states %v: want two rollouts before vacating", states)
	}
	t.Logf("states %v", states)
}

// TestCircuitGoAround: a C172 going around from its circuit's final climbs
// into its own circuit (upwind first, at circuit height), not the
// airliners' 3000 ft circuit 3.5 NM out (#569).
func TestCircuitGoAround(t *testing.T) {
	g := lkprGraph(t)
	ci, _ := NewCircuit(g.Layout, "24", CircuitConfig{}, ProfileFor("C172"))
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: st, Model: "Asobo PassiveAircraft C172", Tail: "OKGA",
		InjectApproach: true, Circuit: &ci, RollThroughChance: -1}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 81))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 81, 1200, 12))
	fin, _ := ci.Point(LegFinal)
	end := ctl.Plan().End
	for i := 0; i < 60*30 && ctl.flyingProc; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 81, offsetHeading(fin.Position, end.Heading, 50), fin.AltFt-g.Layout.Altitude/0.3048, end.Heading, fin.Kts, false))
	}
	for i := 0; i < 60*5; i++ { // a few seconds down the final
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 81, end.Threshold, 0, 0, 0, false))
	}
	if err := ctl.GoAround(); err != nil {
		t.Fatal(err)
	}
	if !ctl.flyingProc || len(ctl.corners) == 0 || ctl.cornerNames[0] != string(LegUpwind) {
		t.Fatalf("not in its circuit: flying %v, corners %v", ctl.flyingProc, ctl.cornerNames)
	}
	for _, w := range ctl.corners {
		if w.Altitude > ci.HeightFt+1 {
			t.Errorf("a circuit point at %.0f ft, circuit height %.0f", w.Altitude, ci.HeightFt)
		}
	}
}
