//go:build windows

package traffic

import (
	"math"
	"time"

	"errors"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"strings"
	"testing"
)

// A runway change re-plans a departure (#456): on the stand before the
// push, and taxiing, it ends up holding short of the new runway; lined up
// it is too late.
func TestChangeRunway(t *testing.T) {
	g := lkprGraph(t)
	rwy06, _, _ := g.Layout.RunwayEnd("06")
	holdingFor06 := func(ctl *TaxiController) bool {
		r := ctl.Route()
		return ctl.State() == TaxiHoldingShort && r != nil && r.RunwayEnd == "06"
	}

	// On the stand.
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{})
	if err := ctl.ChangeRunway("06", "", nil); err != nil {
		t.Fatal(err)
	}
	if !run(TaxiHoldingShort, 60*900) || !holdingFor06(ctl) {
		t.Fatalf("on the stand: %v, route to %q", ctl.State(), ctl.Route().RunwayEnd)
	}
	if ctl.runway.Index != rwy06.Index {
		t.Errorf("runway %d, want 06/24 (%d)", ctl.runway.Index, rwy06.Index)
	}

	// Taxiing: from where it is, without stopping.
	ctl, _, run, _ = injectedDeparture(t, TaxiRequest{})
	if !run(TaxiTaxiing, 60*900) {
		t.Fatalf("never taxied: %v", ctl.State())
	}
	run(TaxiHoldingShort, 60*20) // under way
	speed := ctl.mover.Pose().GroundSpeedKts
	if err := ctl.ChangeRunway("06", "", nil); err != nil {
		t.Fatal(err)
	}
	if got := ctl.mover.Pose().GroundSpeedKts; speed > 3 && got < speed-1 {
		t.Errorf("re-planned at %.1f kt, going on at %.1f kt", speed, got)
	}
	if !run(TaxiHoldingShort, 60*900) || !holdingFor06(ctl) {
		t.Fatalf("taxiing: %v, route to %q", ctl.State(), ctl.Route().RunwayEnd)
	}

	// Lined up: too late.
	ctl, _, run, _ = injectedDeparture(t, TaxiRequest{HoldForRunway: true})
	if !run(TaxiHoldingShort, 60*900) {
		t.Fatal(ctl.State())
	}
	ctl.ClearToLineUp()
	if !run(TaxiLinedUp, 60*300) {
		t.Fatal(ctl.State())
	}
	if err := ctl.ChangeRunway("06", "", nil); !errors.Is(err, ErrTooLate) {
		t.Errorf("lined up: %v, want ErrTooLate", err)
	}
}

// An arrival on its STAR for 06 is re-planned for 24 (#456): it flies the
// new procedure, is taken over on the 24 final and parks; on the injected
// final a change is too late.
func TestArrivalChangeRunway(t *testing.T) {
	g := lkprGraph(t)
	procs := lkprProcedures(t)
	to06, err := procs.Arrival("06", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	to24, err := procs.Arrival("24", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
		InjectApproach: true, Procedure: to06, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(arrivalPositionMsg(mon, 77, to06[1].Position, 6000, 200, 250, false))
	sent := len(ec.waypoints)
	if err := ctl.ChangeRunway("24", to24, nil); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	if p.End.Name != "24" || len(ec.waypoints) == sent {
		t.Fatalf("plan for %s, waypoints sent again %v", p.End.Name, len(ec.waypoints) > sent)
	}
	// Established on the 24 final: taken over, lands and parks.
	out := math.Mod(p.End.Heading+180, 360)
	lat, lon := calc.DisplaceByHeading(p.End.Threshold.Lat, p.End.Threshold.Lon, out, ProcedureJoinNm*1852-200)
	ctl.Handle(arrivalPositionMsg(mon, 77, airport.LatLon{Lat: lat, Lon: lon}, 2400, p.End.Heading, 160, false))
	if ctl.approach == nil {
		t.Fatal("no takeover on the 24 final")
	}
	if err := ctl.ChangeRunway("06", to06, nil); !errors.Is(err, ErrTooLate) {
		t.Errorf("on the final: %v, want ErrTooLate", err)
	}
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	for i := 0; i < 60*1500 && ctl.State() != ArrivalParked; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("stuck in %v", ctl.State())
	}
}

// A departure changes its runway entry on the stand and while taxiing; an
// unknown entry is refused.
func TestChangeEntry(t *testing.T) {
	g := lkprGraph(t)
	entries, err := g.RunwayEntries("24")
	if err != nil {
		t.Fatal(err)
	}
	var named string
	for _, e := range entries[1:] {
		if e.Taxiway != "" {
			named = e.Taxiway
			break
		}
	}
	if named == "" {
		t.Skip("no named intersection on 24")
	}
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{RollingTakeoffChance: -1})
	go func() {
		for range ctl.Events() {
		}
	}()
	if err := ctl.ChangeEntry(named); err != nil {
		t.Fatalf("on the stand: %v", err)
	}
	if r := ctl.Route(); r == nil || !strings.EqualFold(r.Entry, named) {
		t.Fatalf("route entry %q, want %s", r.Entry, named)
	}
	if !run(TaxiTaxiing, 60*900) {
		t.Fatalf("state %v", ctl.State())
	}
	if err := ctl.ChangeEntry(""); err != nil {
		t.Fatalf("taxiing, back to full length: %v", err)
	}
	if err := ctl.ChangeEntry("NOPE"); err == nil {
		t.Error("unknown entry accepted")
	}
	if !run(TaxiDeparting, 60*1500) {
		t.Fatalf("state %v after the change", ctl.State())
	}
}

// TestChangeEntryWaitingForTaxi: pushed back and waiting for its taxi
// clearance, a departure given an intersection (a crew's request) has its
// route planned at once, so the taxi clearance names it (#621).
func TestChangeEntryWaitingForTaxi(t *testing.T) {
	g := lkprGraph(t)
	entries, err := g.RunwayEntries("24")
	if err != nil {
		t.Fatal(err)
	}
	var named string
	for _, e := range entries[1:] {
		if e.Taxiway != "" {
			named = e.Taxiway
			break
		}
	}
	if named == "" {
		t.Skip("no named intersection on 24")
	}
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{HoldForClearances: true, RollingTakeoffChance: -1})
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiAwaitingPushback, 60*300) {
		t.Fatalf("state %v", ctl.State())
	}
	ctl.ClearPushback()
	if !run(TaxiAwaitingTaxi, 60*900) {
		t.Fatalf("never waited for the taxi: %v", ctl.State())
	}
	if err := ctl.ChangeEntry(named); err != nil {
		t.Fatal(err)
	}
	if r := ctl.Route(); r == nil || !strings.EqualFold(r.Entry, named) {
		t.Fatalf("route entry %q right after the change, want %s", r.Entry, named)
	}
}
