package traffic

import (
	"testing"
	"time"
)

// TestVFRDepartureHandover: a C172 on a VFR departure (TaxiRequest.VFR) is
// handed to MSFS AI soon after the take-off, low (VFRHandoverFt, below
// 1000 ft above the runway), with the circuit's waypoints at circuit
// speed: live, OKVFD flew injected for 4.5 minutes to 3400 ft.
func TestVFRDepartureHandover(t *testing.T) {
	g := lkprGraph(t)
	c, err := NewCircuit(g.Layout, "24", CircuitConfig{}, ProfileFor("C172"))
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: st, Runway: "24", Model: "Asobo PassiveAircraft C172", Tail: "OKVFD",
		VFR: true, Departure: c.Departure(c.heading), RollingTakeoffChance: -1}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	stand := g.Layout.Parking[st]
	var rolled time.Time
	var height float64
	for i := 0; i < 60*60*30 && ctl.State() != TaxiComplete && !ctl.State().Terminal(); i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(positionMsg(mon, 77, stand.Position, 0, 0, true))
		if ctl.State() == TaxiDeparting && rolled.IsZero() {
			rolled = now
		}
		height = ctl.last.HeightFt
	}
	if ctl.State() != TaxiComplete {
		t.Fatalf("state %v", ctl.State())
	}
	took := now.Sub(rolled)
	t.Logf("handed over %v after the take-off roll began, at %.0f ft", took.Round(time.Second), height)
	if height > 1000 || took > 2*time.Minute {
		t.Errorf("handed over at %.0f ft, %v after the roll began; want below 1000 ft within 2 min", height, took.Round(time.Second))
	}
	if len(ctl.climb) == 0 || ctl.climb[0].KtsSpeed > 100 {
		t.Errorf("climb waypoints %d, first at %.0f kt", len(ctl.climb), ctl.climb[0].KtsSpeed)
	}
}
