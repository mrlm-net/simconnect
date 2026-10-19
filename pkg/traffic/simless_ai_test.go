package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// With no simulator an arrival flies its published STAR and approach in
// the core, down to the join point, where the injected approach takes it
// as it does after MSFS AI (Vector).
func TestSimlessArrivalFliesItsProcedure(t *testing.T) {
	g := lkprGraph(t)
	procs := lkprProcedures(t)
	star, err := procs.Arrival("06", "LOMKI")
	if err != nil {
		t.Fatal(err)
	}
	ai := NewSimlessAI(&eventClient{})
	ctl := NewArrivalController(NewFleet(ai), ArrivalWithInjector(NewInjector(ai)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "A320", Tail: "CSA1",
		InjectApproach: true, Procedure: star, RollThroughChance: -1}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	if !ai.Flying(77) {
		t.Fatal("not flown here after its release to the AI")
	}
	mon := DefaultArrivalRequestBase + arrReqMonitor
	field := g.Layout.Altitude / 0.3048
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	start := ctl.proc.Waypoints[0]
	var first airport.LatLon
	moved, joined := 0.0, false
	for i := 0; i < 2*3600 && !joined; i++ { // half-second steps, an hour at most
		at = at.Add(500 * time.Millisecond)
		ai.Step(at)
		ai.mu.Lock()
		o := ai.objects[77]
		pos, alt, hdg, kts := o.pos, o.altFt, o.hdg, o.kts
		ai.mu.Unlock()
		if i == 0 {
			first = pos
		}
		moved = math.Max(moved, calc.HaversineNM(first.Lat, first.Lon, pos.Lat, pos.Lon))
		ctl.Handle(arrivalPositionMsg(mon, 77, pos, alt-field, hdg, kts, false))
		ctl.mu.Lock()
		joined = !ctl.flyingProc
		ctl.mu.Unlock()
	}
	if moved < 5 {
		t.Fatalf("moved %.1f NM from the STAR's start, want flown along it", moved)
	}
	if !joined {
		t.Fatal("never reached the join point: the injected approach did not take over")
	}
	if ai.Flying(77) {
		t.Error("still flown here after the injected approach took over")
	}
	ai.mu.Lock()
	alt := ai.objects[77].altFt
	ai.mu.Unlock()
	if alt >= start.Altitude {
		t.Errorf("at the join at %.0f ft, the STAR began at %.0f: no descent", alt, start.Altitude)
	}
}
