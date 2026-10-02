//go:build windows
// +build windows

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
func TestTouchAndGo(t *testing.T) {
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
		InjectApproach: true, Circuit: &ci, TouchAndGos: 1, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var states []ArrivalState
	tngSeen := false
	go func() {
		for ev := range ctl.Events() {
			mu.Lock()
			if len(states) == 0 || states[len(states)-1] != ev.State {
				states = append(states, ev.State)
			}
			tngSeen = tngSeen || ev.TouchAndGo
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
