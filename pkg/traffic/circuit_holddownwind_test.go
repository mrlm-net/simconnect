package traffic

import (
	"math"
	"testing"
	"time"
)

// TestHoldDownwindUntilCalled: a VFR arrival told "I'll call your base"
// whose leader is not past yet flies on downwind, a mile at a time, its
// base still to be called, up to HoldDownwindMaxNM (the user, live: OKCJV
// called by position turned in ahead of the Citation it followed).
func TestHoldDownwindUntilCalled(t *testing.T) {
	g := lkprGraph(t)
	p := ProfileFor("DA42")
	c, err := NewCircuit(g.Layout, "24", CircuitConfig{}, p)
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: st, Model: "Asobo PassiveAircraft DA42", Tail: "OKCJV",
		InjectApproach: true, Circuit: &c}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 80))
	dw, _ := c.Point(LegDownwind)
	kts := CircuitKts(p)
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 80, dw.Position, dw.AltFt, c.heading+180, kts, false))
	if _, err := ctl.AbsorbDelay(time.Minute); err != nil {
		t.Fatal(err)
	}
	base, _ := c.Point(LegBase)
	near := func() { // short of the base turn as it stands, by less than the call's lead
		at := offsetHeading(base.Position, c.heading+180, ctl.tromboneNM*1852-BaseCallLeadM+20)
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 80, at, base.AltFt, c.heading+180, kts, false))
	}
	near()
	if !ctl.BaseDue() {
		t.Fatal("no base call due at the extended base turn")
	}
	f0 := ctl.CircuitFinalNM()
	if !ctl.HoldDownwind() {
		t.Fatal("not held on downwind")
	}
	if f := ctl.CircuitFinalNM(); math.Abs(f-f0-HoldDownwindStepNM) > 0.05 {
		t.Errorf("base turn %.2f NM out, was %.2f: want %.1f NM further", f, f0, HoldDownwindStepNM)
	}
	if ctl.BaseDue() {
		t.Error("base due again where it was: it flies on")
	}
	near()
	if !ctl.BaseDue() {
		t.Fatal("no base call at the base turn further out")
	}
	n := 0
	for ctl.HoldDownwind() {
		near()
		ctl.BaseDue()
		if n++; n > 20 {
			t.Fatal("held for ever")
		}
	}
	if ctl.tromboneNM > HoldDownwindMaxNM+0.01 {
		t.Errorf("held %.1f NM out, beyond %.0f", ctl.tromboneNM, HoldDownwindMaxNM)
	}
	t.Logf("held to %.1f NM extension, base turn %.1f NM out", ctl.tromboneNM, ctl.CircuitFinalNM())
}
