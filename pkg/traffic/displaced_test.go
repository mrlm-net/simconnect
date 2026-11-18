package traffic

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// displacedGraph is LKPR's taxi graph with runway end's landing threshold
// displaced by m meters (#1560).
func displacedGraph(t *testing.T, end string, m float32) *airport.Graph {
	t.Helper()
	b, err := os.ReadFile("../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range l.Runways {
		switch end {
		case r.Primary.Name:
			raw.Runways[i].PrimaryDisplacedM = m
		case r.Secondary.Name:
			raw.Runways[i].SecondaryDisplacedM = m
		}
	}
	if l, err = airport.BuildLayout(raw); err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// displacedTouchdown flies an injected A320 arrival to end of g onto C22
// and returns where its main wheels touched down and its planned exit.
func displacedTouchdown(t *testing.T, g *airport.Graph, end string) (airport.LatLon, airport.RunwayExit) {
	t.Helper()
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: end, Parking: st, Model: "A320", Tail: "OKDSP",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	for i := 0; i < 60*1200 && ctl.State() < ArrivalAwaitingTaxi; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		if ap := ctl.approach; ap != nil && ctl.State() == ArrivalRollout {
			return ap.Pose().Position, p.Exit
		}
	}
	t.Fatalf("%s: no touchdown, state %v", end, ctl.State())
	return airport.LatLon{}, airport.RunwayExit{}
}

// TestDisplacedLandingTouchdown: an arrival on an end with a displaced
// threshold flies its final to the displaced threshold and touches down
// past it, as far past it as it touched down past the pavement end without
// one; its exit is planned for the shorter landing distance.
func TestDisplacedLandingTouchdown(t *testing.T) {
	const end, disp = "24", 400.0
	plain := lkprGraph(t)
	g := displacedGraph(t, end, disp)
	_, e, _ := g.Layout.RunwayEnd(end)
	if e.DisplacedM != disp {
		t.Fatalf("RWY %s displaced %v", end, e.DisplacedM)
	}
	SetRandomSeed(1)
	at0, x0 := displacedTouchdown(t, plain, end)
	SetRandomSeed(1)
	at, x := displacedTouchdown(t, g, end)
	a0 := alongHeading(e.Threshold, e.Heading, at0)
	a := alongHeading(e.Threshold, e.Heading, at)
	t.Logf("touchdown %.0f m past the pavement end (%.0f without the displacement); exit %s at %.0f (%s at %.0f)", a, a0, x.Taxiway, x.Along, x0.Taxiway, x0.Along)
	if a <= disp {
		t.Errorf("touched down %.0f m from the pavement end, before the displaced threshold at %.0f", a, disp)
	}
	if d := a - a0 - disp; d < -1 || d > 1 {
		t.Errorf("touchdown moved %.0f m with a %.0f m displacement", a-a0, disp)
	}
	if x.Along-disp < RequiredRollout(ExitSpeed(x)) && x.Node != x0.Node {
		t.Errorf("exit %s %.0f m past the landing threshold, needs %.0f", x.Taxiway, x.Along-disp, RequiredRollout(ExitSpeed(x)))
	}
	if x.Along < x0.Along {
		t.Errorf("exit %s at %.0f before the undisplaced one %s at %.0f", x.Taxiway, x.Along, x0.Taxiway, x0.Along)
	}
}

// TestDisplacedTakeoffFromPavement: a departure from an end with a displaced
// landing threshold lines up at the pavement end, exactly where it does
// without one (TORA from the pavement).
func TestDisplacedTakeoffFromPavement(t *testing.T) {
	const end, disp = "24", 400.0
	lineUp := func(g *airport.Graph) GroundPose {
		i, err := g.Layout.ParkingIndex("C22")
		if err != nil {
			t.Fatal(err)
		}
		ec := &eventClient{}
		inj := NewInjector(ec)
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: end, Model: "FSLTL_B738_KLM", Tail: "T1", RollingTakeoffChance: -1, HoldForClearances: true}); err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		ctl.now = func() time.Time { return now }
		ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
		mon := DefaultTaxiRequestBase + reqOffMonitor
		st := g.Layout.Parking[i]
		var last GroundPose
		run := func(until TaxiState, n int) bool {
			for k := 0; k < n && ctl.State() != until && !ctl.State().Terminal(); k++ {
				now = now.Add(time.Second / 60)
				ctl.Handle(positionMsg(mon, 77, st.Position, 0, 0, true))
				if ctl.mover != nil {
					last = ctl.mover.Pose()
				}
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
		return last
	}
	SetRandomSeed(1)
	p0 := lineUp(lkprGraph(t))
	g := displacedGraph(t, end, disp)
	SetRandomSeed(1)
	p := lineUp(g)
	_, e, _ := g.Layout.RunwayEnd(end)
	a := alongHeading(e.Threshold, e.Heading, p.Position)
	t.Logf("lined up %.0f m past the pavement end", a)
	if p.Position != p0.Position {
		t.Errorf("lined up at %v, %v without the displacement", p.Position, p0.Position)
	}
	if a >= disp {
		t.Errorf("lined up %.0f m in, at or past the displaced threshold (%.0f)", a, disp)
	}
}
