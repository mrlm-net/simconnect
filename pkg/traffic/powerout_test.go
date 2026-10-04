package traffic

import (
	"testing"
	"time"
)

// A small aircraft on a GA stand leaves under its own power: no pushback,
// first forward off the stand, then round and away along the taxiway; with
// the neighbouring stands taken where the loop would cross them, pushed.
func TestPowerOut(t *testing.T) {
	g := lkprGraph(t)
	s16, _ := g.Layout.ParkingIndex("S16")
	stand := g.Layout.Parking[s16]
	run := func(busy func(int) bool) (*TaxiController, []TaxiState, bool) {
		ec := &eventClient{}
		inj := NewInjector(ec)
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
		req := TaxiRequest{Graph: g, Parking: s16, Runway: "24", Model: "FSLTL_GA_B350_ZZZZ", Tail: "OEPMQ", PowerOut: true, StandOccupied: busy, RollingTakeoffChance: -1}
		if err := ctl.Start(req); err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		ctl.now = func() time.Time { return now }
		ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
		var states []TaxiState
		forward := false
		start := NoseGear(StandPoint(stand, 0), stand.Heading, ctl.profile())
		for i := 0; i < 60*600 && ctl.State() != TaxiTaxiing; i++ {
			now = now.Add(time.Second / 60)
			ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, stand.Position, 0, 0, true))
			if s := ctl.State(); len(states) == 0 || states[len(states)-1] != s {
				states = append(states, s)
			}
		}
		for i := 0; i < 60*20 && ctl.State() == TaxiTaxiing; i++ {
			now = now.Add(time.Second / 60)
			ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, stand.Position, 0, 0, true))
			if p := ctl.mover.Pose(); localDist(p.Position, start) > 3 {
				forward = alongHeading(start, stand.Heading, NoseGear(p.Position, p.Heading, ctl.profile())) > 0
				break
			}
		}
		return ctl, states, forward
	}
	ctl, states, forward := run(nil)
	t.Logf("states %v, forward %v", states, forward)
	if !ctl.FacesOut() {
		t.Fatal("not planned out under its own power")
	}
	for _, s := range states {
		if s == TaxiPushback {
			t.Errorf("pushed back: %v", states)
		}
	}
	if !forward {
		t.Error("did not move forward off the stand first")
	}
	// S23: its loop crosses a neighbouring stand; taken, it is pushed.

	s16 = func() int { i, _ := g.Layout.ParkingIndex("S23"); return i }()
	stand = g.Layout.Parking[s16]
	ctl, _, _ = run(func(int) bool { return true })
	if ctl.FacesOut() {
		t.Error("S23 with its neighbours taken: out under its own power across them")
	}
	ctl, _, _ = run(nil)
	if !ctl.FacesOut() {
		t.Error("S23 with its neighbours free: pushed")
	}

}
