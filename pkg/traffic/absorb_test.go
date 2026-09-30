//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
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

// A longer downwind: on along the downwind x NM past its last point, then
// onto the centreline x NM beyond the align point — about 2x more track;
// a straight-in arrival has no downwind to extend.
func TestExtendDownwind(t *testing.T) {
	thr := airport.LatLon{Lat: 50.1, Lon: 14.23}
	rwy := 65.0
	out := rwy + 180 // away from the runway
	at := func(alongNM, sideNM float64) types.SIMCONNECT_DATA_WAYPOINT {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, out, alongNM*1852)
		lat, lon = calc.DisplaceByHeading(lat, lon, rwy-90, sideNM*1852) // left of the final
		return types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: 5000, KtsSpeed: 210}
	}
	align, join := at(11, 0), at(8, 0)
	align.Altitude = 4700
	pos := airport.LatLon{Lat: at(-6, 4).Latitude, Lon: at(-6, 4).Longitude}
	chain := []types.SIMCONNECT_DATA_WAYPOINT{at(-3, 4), at(0, 4), at(6, 4)} // the downwind, then base to align
	ext, ok := extendDownwind(pos, chain, align, join, 3)
	if !ok || len(ext) != len(chain)+2 {
		t.Fatalf("extended: %v, %d points", ok, len(ext))
	}
	d2, e := ext[len(ext)-2], ext[len(ext)-1]
	near := func(w, want types.SIMCONNECT_DATA_WAYPOINT) float64 {
		return calc.HaversineNM(w.Latitude, w.Longitude, want.Latitude, want.Longitude)
	}
	if near(d2, at(9, 4)) > 0.05 || near(e, at(14, 0)) > 0.05 {
		t.Errorf("downwind to %.2f NM off 9 NM out, final from %.2f NM off 14 NM out", near(d2, at(9, 4)), near(e, at(14, 0)))
	}
	if want := 4700 + 3*ProcedureDescentFtPerNm; math.Abs(e.Altitude-want) > 1 {
		t.Errorf("onto the final at %.0f ft, want %.0f", e.Altitude, want)
	}
	if grown := pathNMOf(pos, ext, align) - pathNMOf(pos, chain, align); math.Abs(grown-6) > 0.5 {
		t.Errorf("track grew %.1f NM, want about 6", grown)
	}
	// Straight in: nothing beside the centreline.
	straight := []types.SIMCONNECT_DATA_WAYPOINT{at(30, 0), at(20, 0)}
	if _, ok := extendDownwind(airport.LatLon{Lat: at(35, 0).Latitude, Lon: at(35, 0).Longitude}, straight, align, join, 3); ok {
		t.Error("extended a straight-in arrival")
	}
}

// LKPR VLM6T to 06 turns base some 16 NM out, well beyond the align point:
// a delay beyond what speed takes extends its downwind beyond that base
// turn (live, the first version turned base inside it, shortening the
// route, and everything went to the hold), again as more is asked, up to
// MaxStretchNM; no dog-leg.
func TestAbsorbDelayExtendsDownwind(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "VLM")
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
	thr := ctl.plan.End.Threshold
	farLat, farLon := calc.DisplaceByHeading(thr.Lat, thr.Lon, ctl.plan.End.Heading+180, 30*1852)
	farthest := func() float64 { // the base turn: the farthest out along the extended centreline
		d := 0.0
		for _, w := range ctl.proc.Waypoints {
			d = math.Max(d, calc.AlongTrackMeters(thr.Lat, thr.Lon, farLat, farLon, w.Latitude, w.Longitude)/1852)
		}
		return d
	}
	if _, err := ctl.AbsorbDelay(3 * time.Minute); err != nil { // speed takes it
		t.Fatal(err)
	}
	base := farthest()
	total := 0.0
	for i := 0; i < 3; i++ {
		a, err := ctl.AbsorbDelay(3 * time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if a.ExtraNM < 5 {
			t.Fatalf("call %d: %+v, want the downwind extended", i+2, a)
		}
		total += a.ExtraNM
	}
	if farthest() < base+10 || ctl.tromboneNM > MaxStretchNM/2+0.01 || total > MaxStretchNM+1 {
		t.Errorf("base turn %.1f → %.1f NM out, extended %.1f NM, track added %.1f NM", base, farthest(), ctl.tromboneNM, total)
	}
	if a, _ := ctl.AbsorbDelay(3 * time.Minute); a.ExtraNM != 0 || a.Left < 2*time.Minute {
		t.Errorf("beyond the limit: %+v, want the hold", a)
	}
}
