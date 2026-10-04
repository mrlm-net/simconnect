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
	// In tens of knots, down to the ten below: a little more than the minute,
	// less than the next ten would take.
	if math.Mod(a.SpeedKts, 10) != 0 {
		t.Errorf("%.0f kt: not in tens", a.SpeedKts)
	}
	got, next := 40/a.SpeedKts-40.0/250, 40/(a.SpeedKts+10)-40.0/250
	if got*60 < 1 || next*60 >= 1 {
		t.Errorf("1 min absorbed as %.2f min at %.0f kt", got*60, a.SpeedKts)
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
	// Asked again at the minimum speed already: no 0 kt waypoint, no
	// infinite or negative time left.
	a2, err := ctl.AbsorbDelay(4 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if a2.Left < 0 || math.IsInf(a2.ExtraNM, 0) || math.IsNaN(a2.ExtraNM) {
		t.Errorf("second absorption %+v", a2)
	}
	for _, w := range ctl.proc.Waypoints {
		if w.KtsSpeed <= 0 {
			t.Fatalf("a waypoint at %.0f kt after the second absorption", w.KtsSpeed)
		}
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

// Near the end of its STAR, no leg long enough to stretch: a minute's
// delay is lost by vectors from where it is, not in a hold (live, LOT775
// held at PR532 for a minute).
func TestAbsorbDelayVectorsNearTheEnd(t *testing.T) {
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
	wps := ctl.proc.Waypoints
	// Halfway along the last leg of 2 NM or more before the final: the
	// legs after it are all shorter than MinStretchLegNM.
	a0, b0 := wps[len(wps)-6], wps[len(wps)-5]
	at := airport.LatLon{Lat: (a0.Latitude + b0.Latitude) / 2, Lon: (a0.Longitude + b0.Longitude) / 2}
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, at, 5000, 90, 210, false))
	routeBefore := pathNM(ctl.ProcedureRoute())
	a, err := ctl.AbsorbDelay(90 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if a.Left > 0 || a.ExtraNM <= 0 {
		t.Fatalf("absorption %+v: want vectors, nothing left for a hold", a)
	}
	if grown := pathNM(ctl.ProcedureRoute()) - routeBefore; grown < a.ExtraNM-1 {
		t.Errorf("route grew %.1f NM, stretch %.1f", grown, a.ExtraNM)
	}
}

// Stop descent: the STAR ahead no lower than the level for the distance
// asked, then as planned; the align and join points untouched.
func TestStopDescent(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "AUA529",
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
	n := len(before)
	sets := len(ec.waypoints)
	// The level: above every point of the first 15 NM, so each is raised.
	level, p0, d0 := 0.0, route[0].Position, 0.0
	for _, w := range before[:n-2] {
		d0 += calc.HaversineNM(p0.Lat, p0.Lon, w.Latitude, w.Longitude)
		p0 = airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
		if d0 <= 15 {
			level = math.Max(level, w.Altitude+1000)
		}
	}
	if err := ctl.StopDescent(level, 15); err != nil {
		t.Fatal(err)
	}
	if len(ec.waypoints) == sets {
		t.Fatal("no waypoints sent")
	}
	wps := ctl.proc.Waypoints
	prev, gone, raised := route[0].Position, 0.0, 0
	for i, w := range wps[:len(wps)-2] {
		gone += calc.HaversineNM(prev.Lat, prev.Lon, w.Latitude, w.Longitude)
		prev = airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
		if gone <= 15 && w.Altitude < level {
			t.Errorf("point %d, %.1f NM on, at %.0f ft: below the level", i, gone, w.Altitude)
		}
		if w.Altitude >= level {
			raised++
		}
	}
	if raised == 0 {
		t.Error("nothing at the level")
	}
	if a, b := wps[len(wps)-1], before[n-1]; a.Altitude != b.Altitude || a.Latitude != b.Latitude {
		t.Error("the join point changed")
	}
}

// Near the end of the STAR, about a turn's worth of delay is a 360 where
// it is, not out and back on a short leg (live, OKYDV); a little is still
// a small dog-leg.
func TestAbsorbDelayOrbitNearTheEnd(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	start := func() *ArrivalController {
		ec := &eventClient{}
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
		c22, _ := g.Layout.ParkingIndex("C22")
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "OKYDV",
			InjectApproach: true, Procedure: route}); err != nil {
			t.Fatal(err)
		}
		go func() {
			for range ctl.Events() {
			}
		}()
		ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
		wps := ctl.proc.Waypoints
		a0, b0 := wps[len(wps)-6], wps[len(wps)-5]
		at := airport.LatLon{Lat: (a0.Latitude + b0.Latitude) / 2, Lon: (a0.Longitude + b0.Longitude) / 2}
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, at, 5000, 90, 210, false))
		return ctl
	}
	a, err := start().AbsorbDelay(3 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if a.Orbit == "" || a.ExtraNM <= 0 {
		t.Errorf("3 minutes near the end: %+v, want a 360", a)
	}
	a, err = start().AbsorbDelay(20 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if a.Orbit != "" {
		t.Errorf("20 s near the end: %+v, want no 360", a)
	}
}

// A shortcut: direct to a named fix further on, as much as the room ahead
// allows, where the descent still works; none too high or without room.
func TestShortcut(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	start := func(aglFt float64) *ArrivalController {
		ec := &eventClient{}
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
		c22, _ := g.Layout.ParkingIndex("C22")
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "OKYDV",
			InjectApproach: true, Procedure: route}); err != nil {
			t.Fatal(err)
		}
		go func() {
			for range ctl.Events() {
			}
		}()
		ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
		hdg := calc.BearingDegrees(route[0].Position.Lat, route[0].Position.Lon, route[1].Position.Lat, route[1].Position.Lon)
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, aglFt, hdg, 250, false))
		return ctl
	}
	ctl := start(6000)
	before := pathNM(ctl.ProcedureRoute())
	fix, saved, err := ctl.Shortcut(30)
	if err != nil || fix == "" || saved <= 0 || saved > 30 {
		t.Fatalf("shortcut %q %.1f NM, %v", fix, saved, err)
	}
	if after := pathNM(ctl.ProcedureRoute()); before-after < saved-1.5 {
		t.Errorf("route %.1f → %.1f NM for %.1f saved", before, after, saved)
	}
	if fix, _, _ := start(40000).Shortcut(30); fix != "" {
		t.Errorf("40000 ft up: direct %s, want none (too high)", fix)
	}
	if fix, _, _ := start(6000).Shortcut(0.5); fix != "" {
		t.Errorf("room for 0.5 NM: direct %s", fix)
	}
}

// A shortcut never crosses the field: a leg across LKPR's runway is over
// the airport, one 10 NM off is not (live: CSA1909 direct PR532).
func TestShortcutNotOverAirport(t *testing.T) {
	g := lkprGraph(t)
	c := &ArrivalController{}
	for _, r := range g.Layout.Runways {
		if r.Name() == "06/24" {
			c.plan = &ArrivalPlan{Runway: r}
		}
	}
	if c.plan == nil {
		t.Fatal("no 06/24")
	}
	mid := c.plan.Runway.Primary.Threshold
	across := [2]airport.LatLon{offsetHeading(mid, 150, 15*1852), offsetHeading(mid, 330, 15*1852)}
	if !c.overAirport(across[0], across[1]) {
		t.Error("a leg across the runway is not over the airport")
	}
	away := [2]airport.LatLon{offsetHeading(mid, 150, 10*1852), offsetHeading(offsetHeading(mid, 150, 10*1852), 60, 20*1852)}
	if c.overAirport(away[0], away[1]) {
		t.Error("a leg 10 NM away is over the airport")
	}
}

// An en route arrival handed over at its STAR entry is adopted as it flies:
// no new aircraft, straight onto the procedure (#643: the respawn at the
// entry was a jump of 15 km and 11,000 ft).
func TestArrivalAdopts(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	before := len(ec.waypoints)
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA877",
		InjectApproach: true, Procedure: route, ObjectID: 4242}); err != nil {
		t.Fatal(err)
	}
	if ctl.ObjectID() != 4242 || ctl.State() != ArrivalApproaching {
		t.Fatalf("object %d, state %v: want 4242 flying its procedure", ctl.ObjectID(), ctl.State())
	}
	if len(ec.waypoints) == before {
		t.Error("no waypoints sent to the adopted aircraft")
	}
	// Not without a procedure.
	ctl2 := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	if err := ctl2.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "X", InjectApproach: true, ObjectID: 1}); err == nil {
		t.Error("adopted without a procedure")
	}
}

// TestDogLegApexSide: a leg running alongside the final, the dog-leg goes
// to the side away from it, where there is room (live, FINZX, #706).
func TestDogLegApexSide(t *testing.T) {
	thr := airport.LatLon{Lat: 50, Lon: 14}
	// Runway heading 090: the final comes from the west. A leg 4 NM north
	// of the centreline, running east, 10 to 2 NM out.
	at := func(westNM, northNM float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, 270, westNM*1852)
		lat, lon = calc.DisplaceByHeading(lat, lon, 0, northNM*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	apex := dogLegApex(at(10, 4), at(2, 4), 6, thr, 90)
	if n := calc.AlongTrackMeters(thr.Lat, thr.Lon, thr.Lat+1, thr.Lon, apex.Lat, apex.Lon) / 1852; n < 4 {
		t.Errorf("apex %.1f NM north of the centreline, want north of the leg (away from the final)", n)
	}
	if finalDistanceNM(at(5, 0), thr, 90) > 0.3 || math.Abs(finalDistanceNM(at(5, 3), thr, 90)-3) > 0.3 {
		t.Error("finalDistanceNM off")
	}
}
