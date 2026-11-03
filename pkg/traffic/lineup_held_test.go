package traffic

import (
	"testing"
	"time"
)

// A departure lined up and waiting is still placed, as one holding short
// is: left alone for a minute, the simulator moved it itself (live, LKPR
// 06: EZY1075's nose came up while it waited lined up behind MCR1396).
func TestLinedUpStillPlaced(t *testing.T) {
	g := airportGraph(t, "LKPR")
	i, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: "24", Model: "FSLTL A20N EZY EasyJet", Tail: "T1", RollingTakeoffChance: -1, HoldForClearances: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	st := g.Layout.Parking[i]
	frame := func() {
		now = now.Add(time.Second / 60)
		ctl.Handle(positionMsg(mon, 77, st.Position, 0, 0, true))
	}
	run := func(until TaxiState, n int) bool {
		for k := 0; k < n && ctl.State() != until && !ctl.State().Terminal(); k++ {
			frame()
		}
		return ctl.State() == until
	}
	run(TaxiAwaitingPushback, 60*900)
	ctl.ClearPushback()
	run(TaxiAwaitingTaxi, 60*900)
	ctl.ClearToTaxi()
	if !run(TaxiHoldingShort, 60*1800) {
		t.Fatalf("state %v, want holding short", ctl.State())
	}
	ctl.ClearToLineUp()
	if !run(TaxiLinedUp, 60*600) {
		t.Fatalf("state %v, want lined up", ctl.State())
	}
	before := len(ec.waypoints)
	for k := 0; k < 60*60; k++ {
		frame()
	}
	if ctl.State() != TaxiLinedUp {
		t.Fatalf("state %v, want still lined up", ctl.State())
	}
	if n := len(ec.waypoints) - before; n < 60 {
		t.Errorf("%d placements in a minute lined up, want it held in place (a still aircraft's rate)", n)
	}
}
