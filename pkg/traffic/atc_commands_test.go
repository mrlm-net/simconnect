//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestHoldPosition: a taxiing departure told to hold position stops within
// its braking distance and stays; a taxi clearance lets it go on.
func TestHoldPosition(t *testing.T) {
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
	run(TaxiAwaitingPushback, 60*60)
	ctl.ClearPushback()
	ctl.ClearToTaxi()
	if !run(TaxiTaxiing, 60*600) {
		t.Fatal(ctl.State())
	}
	for i := 0; i < 60*40; i++ { // up to taxi speed
		run(TaxiHoldingShort, 1)
	}
	v := ctl.mover.Pose().GroundSpeedKts * ktsToMS
	at := ctl.mover.Pose().Distance
	if err := ctl.HoldPosition(); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(0)
	run(TaxiHoldingShort, 60*30)
	stopped := ctl.mover.Pose()
	if !stopped.Stopped {
		t.Fatalf("still moving at %.1f kt", stopped.GroundSpeedKts)
	}
	if d := stopped.Distance - at; d > v*v/(2*HoldPositionDecel)+5 {
		t.Errorf("stopped %.0f m on from %.1f m/s", d, v)
	}
	ctl.ClearToTaxi()
	run(TaxiHoldingShort, 60*20)
	if ctl.mover.Pose().Distance <= stopped.Distance+1 {
		t.Error("did not move on after the taxi clearance")
	}
}

// TestAbortTakeoff: rejected before V1 the aircraft stops on the runway,
// vacates and taxis back to the holding point; past V1 the take-off goes
// on.
func TestAbortTakeoff(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
	run(TaxiAwaitingPushback, 60*60)
	ctl.ClearPushback()
	ctl.ClearToTaxi()
	if !run(TaxiHoldingShort, 60*900) {
		t.Fatal(ctl.State())
	}
	ctl.ClearToLineUp()
	if !run(TaxiLinedUp, 60*300) {
		t.Fatal(ctl.State())
	}
	if err := ctl.ClearForTakeoff(); err != nil {
		t.Fatal(err)
	}
	if !run(TaxiDeparting, 60*30) {
		t.Fatal(ctl.State())
	}
	for i := 0; i < 60*12; i++ { // 12 s into the roll
		run(TaxiComplete, 1)
	}
	kts := ctl.takeoff.Pose().GroundSpeedKts
	if err := ctl.AbortTakeoff(); err != nil {
		t.Fatalf("at %.0f kt: %v", kts, err)
	}
	stopAt := ctl.takeoff.Pose()
	if !run(TaxiTaxiing, 60*120) {
		t.Fatalf("not vacating: %v", ctl.State())
	}
	if ctl.mover == nil || ctl.last.GroundSpeed > 1 {
		t.Fatal("not stopped before vacating")
	}
	v := kts * ktsToMS
	if d := calc.HaversineMeters(stopAt.Position.Lat, stopAt.Position.Lon, ctl.last.Position.Lat, ctl.last.Position.Lon); d > v*v/(2*RejectDecel)+20 {
		t.Errorf("stopped %.0f m after the abort at %.0f kt", d, kts)
	}
	if !run(TaxiHoldingShort, 60*1200) || ctl.last.HoldingShortOf != ctl.runway.Name() {
		t.Fatalf("state %v (%s), want holding short of the runway again", ctl.State(), ctl.last.HoldingShortOf)
	}

	// Past V1: too late.
	ctl.ClearToLineUp()
	run(TaxiLinedUp, 60*300)
	ctl.ClearForTakeoff()
	run(TaxiDeparting, 60*30)
	for i := 0; i < 60*60 && ctl.takeoff.Pose().GroundSpeedKts < ctl.takeoffProfile().RotateKts-2; i++ {
		run(TaxiComplete, 1)
	}
	if err := ctl.AbortTakeoff(); !errors.Is(err, ErrTooLate) {
		t.Errorf("past V1: %v, want ErrTooLate", err)
	}
}

// TestGoAround: an injected arrival sent around on short final is released
// to MSFS AI with a circuit back to the join point, and taken over there
// again for another approach, which it lands from.
func TestGoAround(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "A320", Tail: "CSA9",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	p := ctl.Plan()
	tick := func() {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
	}
	for i := 0; i < 60*600 && ctl.approach != nil && ctl.approach.Pose().HeightFt > 300; i++ {
		tick()
	}
	if err := ctl.GoAround(); err != nil {
		t.Fatal(err)
	}
	if inj.Driven(77) || !ctl.flyingProc || ctl.goArounds != 1 {
		t.Fatal("not released for the circuit")
	}
	// MSFS AI back on the final at the join point.
	out := math.Mod(p.End.Heading+180, 360)
	lat, lon := calc.DisplaceByHeading(p.End.Threshold.Lat, p.End.Threshold.Lon, out, ctl.proc.JoinMeters-100)
	ctl.Handle(arrivalPositionMsg(mon, 77, airportLatLon(lat, lon), 2500, p.End.Heading, 160, false))
	if ctl.flyingProc || ctl.approach == nil {
		t.Fatal("not taken over for the second approach")
	}
	for i := 0; i < 60*1500 && ctl.State() != ArrivalParked; i++ {
		tick()
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("stuck in %v", ctl.State())
	}
	// Too late once on the runway: covered by the approach pose check.
}

func airportLatLon(lat, lon float64) airport.LatLon { return airport.LatLon{Lat: lat, Lon: lon} }
