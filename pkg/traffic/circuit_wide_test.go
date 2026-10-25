package traffic

import (
	"testing"
	"time"
)

// TestCircuitWideExtend: a C172 extended as far as CircuitMaxExtendNM goes
// takes up to CircuitWideExtendNM more once the sequence allows it
// (SetCircuitExtendMax): the rest of a long delay on its downwind, not left
// over for the ones behind.
func TestCircuitWideExtend(t *testing.T) {
	g := lkprGraph(t)
	p := ProfileFor("C172")
	c, _ := NewCircuit(g.Layout, "24", CircuitConfig{}, p)
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: st, Model: "Asobo PassiveAircraft C172", Tail: "OKWID",
		InjectApproach: true, Circuit: &c}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 80))
	entry, _ := c.JoinDownwind()
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 80, entry.Position, entry.AltFt, 0, CircuitKts(p), false))
	a, err := ctl.AbsorbDelay(9 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if a.Left <= 0 {
		t.Fatalf("absorption %+v: 9 minutes taken within %.0f NM", a, CircuitMaxExtendNM)
	}
	ctl.SetCircuitExtendMax(CircuitWideExtendNM)
	b, err := ctl.AbsorbDelay(a.Left)
	if err != nil {
		t.Fatal(err)
	}
	if b.ExtraNM < 2*(CircuitWideExtendNM-CircuitMaxExtendNM)-0.1 || b.ExtraNM > 2*(CircuitWideExtendNM-CircuitMaxExtendNM)+0.1 || b.Left >= a.Left {
		t.Errorf("wider: %+v after %+v, want %.0f NM more each way and less left", b, a, CircuitWideExtendNM-CircuitMaxExtendNM)
	}
	ctl.SetCircuitExtendMax(20)
	if d, _ := ctl.AbsorbDelay(b.Left); d.ExtraNM > 0.1 {
		t.Errorf("past CircuitWideExtendNM: %+v", d)
	}
}
