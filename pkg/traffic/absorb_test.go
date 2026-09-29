//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

func TestPlanAbsorption(t *testing.T) {
	// 40 NM of STAR at 250 kt: 9.6 min. A minute is absorbed by speed.
	a := PlanAbsorption(time.Minute, 40, 250, 210)
	if a.SpeedKts < 210 || a.ExtraNM != 0 || a.Left != 0 {
		t.Fatalf("1 min: %+v", a)
	}
	if got := 40/a.SpeedKts - 40.0/250; math.Abs(got*60-1) > 0.05 {
		t.Errorf("1 min absorbed as %.2f min", got*60)
	}
	// 4 min: 210 kt (≈1.8 min) and a stretch for the rest at 210 kt.
	a = PlanAbsorption(4*time.Minute, 40, 250, 210)
	byspeed := 40.0/210 - 40.0/250
	want := (4.0/60 - byspeed) * 210
	if a.SpeedKts != 210 || math.Abs(a.ExtraNM-want) > 0.01 || a.Left != 0 {
		t.Fatalf("4 min: %+v, want 210 kt and +%.1f NM", a, want)
	}
	// 15 min: stretch capped, the rest to hold.
	a = PlanAbsorption(15*time.Minute, 40, 250, 210)
	if a.ExtraNM != MaxStretchNM || a.Left <= 0 {
		t.Fatalf("15 min: %+v", a)
	}
	total := time.Duration((40/a.SpeedKts-40.0/250+a.ExtraNM/a.SpeedKts)*float64(time.Hour)) + a.Left
	if absDuration(total-15*time.Minute) > time.Second {
		t.Errorf("15 min split into %v", total)
	}
	if (PlanAbsorption(-time.Minute, 40, 250, 210) != Absorption{}) {
		t.Error("no delay: nothing to do")
	}
}

func TestStretchLeg(t *testing.T) {
	a := airport.LatLon{Lat: 50, Lon: 13}
	b := offsetHeading(a, 90, 20*1852)
	apex := StretchLeg(a, b, 6, 1) // 20 NM leg, +6 NM
	l := calc.HaversineNM(a.Lat, a.Lon, apex.Lat, apex.Lon) + calc.HaversineNM(apex.Lat, apex.Lon, b.Lat, b.Lon)
	if math.Abs(l-26) > 0.05 {
		t.Errorf("dog-leg %.2f NM, want 26", l)
	}
	if apex.Lat >= a.Lat { // right of an eastbound track is south
		t.Errorf("apex %v not to the right", apex)
	}
}

// TestAbsorbDelayOnSTAR: an arrival on GOLOP 4T asked to lose 4 min gets a
// slower STAR and a dog-leg; the final part stays as it was.
func TestAbsorbDelayOnSTAR(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
		InjectApproach: true, Procedure: route}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	if _, err := ctl.AbsorbDelay(time.Minute); err == nil {
		t.Fatal("absorbed before its position was known")
	}
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, 90, 250, false))
	before := append(ctl.proc.Waypoints[:0:0], ctl.proc.Waypoints...)
	routeBefore := pathNM(ctl.ProcedureRoute())
	sets := len(ec.waypoints)
	a, err := ctl.AbsorbDelay(4 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if a.SpeedKts != MinProcedureSpeedKts || a.ExtraNM <= 0 || len(ec.waypoints) == sets {
		t.Fatalf("absorption %+v, waypoints sent %v", a, len(ec.waypoints) > sets)
	}
	after := ctl.proc.Waypoints
	if n, m := len(before), len(after); after[m-1] != before[n-1] || after[m-2] != before[n-2] {
		t.Error("the align and join points changed")
	}
	for _, w := range after[:len(after)-2] {
		if w.KtsSpeed > MinProcedureSpeedKts+0.1 {
			t.Errorf("a STAR point at %.0f kt", w.KtsSpeed)
		}
	}
	if grown := pathNM(ctl.ProcedureRoute()) - routeBefore; math.Abs(grown-a.ExtraNM) > 1 {
		t.Errorf("route grew %.1f NM, stretch %.1f", grown, a.ExtraNM)
	}
}

func pathNM(pts []airport.LatLon) float64 {
	d := 0.0
	for i := 1; i < len(pts); i++ {
		d += calc.HaversineNM(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
	}
	return d
}

// Direct to the join point: the rest of the STAR is left out; the final is
// kept.
func TestDirectToJoin(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
		InjectApproach: true, Procedure: route}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, 90, 250, false))
	before := append(ctl.proc.Waypoints[:0:0], ctl.proc.Waypoints...)
	sets := len(ec.waypoints)
	if err := ctl.DirectToJoin(); err != nil {
		t.Fatal(err)
	}
	after := ctl.proc.Waypoints
	if len(after) != 2 || after[0] != before[len(before)-2] || after[1] != before[len(before)-1] || len(ec.waypoints) == sets {
		t.Fatalf("direct: %d points (sent %v)", len(after), len(ec.waypoints) > sets)
	}
	if r := ctl.ProcedureRoute(); len(r) != 2 {
		t.Errorf("still to fly: %d points", len(r))
	}
}
