package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A point picked on the map (#443): near a named fix ahead, direct to the
// fix; elsewhere, a vector to it, then direct to the next named fix once
// there; the route then goes on from the point after the nearest one.
func TestArrivalDirectTo(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "VLM")
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
		hdg := calc.BearingDegrees(route[0].Position.Lat, route[0].Position.Lon, route[1].Position.Lat, route[1].Position.Lon)
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, hdg, 250, false))
		return ctl
	}
	at := func(name string) airport.LatLon {
		for _, n := range route {
			if n.Ident == name {
				return n.Position
			}
		}
		t.Fatalf("no %s", name)
		return airport.LatLon{}
	}
	// Half a mile off PR523: direct PR523.
	near := airport.LatLon{Lat: at("PR523").Lat + 0.005, Lon: at("PR523").Lon}
	fix, v, err := start().DirectTo(near)
	if err != nil || fix != "PR523" || v != (Vector{}) {
		t.Errorf("near PR523: %q %+v %v", fix, v, err)
	}
	// Between PR722 and PR723, off the route: a vector, then back direct.
	a, b := at("PR722"), at("PR723")
	off := airport.LatLon{Lat: (a.Lat+b.Lat)/2 + 0.05, Lon: (a.Lon+b.Lon)/2 - 0.1}
	ctl := start()
	fix, v, err = ctl.DirectTo(off)
	if err != nil || fix != "" || v.HeadingDeg == 0 {
		t.Fatalf("a point: %q %+v %v", fix, v, err)
	}
	want := calc.BearingDegrees(route[0].Position.Lat, route[0].Position.Lon, off.Lat, off.Lon)
	if d := headingDiff(v.HeadingDeg, want); d > 1 || d < -1 {
		t.Errorf("heading %.0f, want %.0f to the point", v.HeadingDeg, want)
	}
	if len(ctl.vectors) != 1 || ctl.vectors[0].Fix == "" {
		t.Errorf("no resume direct after the point: %+v", ctl.vectors)
	}
	// Not due before the point is reached (live, OKRVJ: told to resume 0.5 s
	// after its heading to a point off the route).
	if v, due := ctl.VectorDue(); due {
		t.Errorf("resume due before the point: %+v", v)
	}
	if r := ctl.ProcedureRoute(); len(r) == 0 || calc.HaversineNM(r[0].Lat, r[0].Lon, off.Lat, off.Lon) > 3 {
		t.Errorf("route does not go to the point first")
	}
}

// Joining the final at a picked distance (#443): the route ends on the
// centreline that far out, intercepting from the aircraft's side.
func TestArrivalJoinFinal(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "VLM")
	if err != nil {
		t.Fatal(err)
	}
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
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, hdg, 250, false))
	end := ctl.plan.End
	out := end.Heading + 180
	pLat, pLon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, out, 14*1852)
	nm, v, err := ctl.JoinFinal(airport.LatLon{Lat: pLat + 0.01, Lon: pLon})
	if err != nil || nm < 13 || nm > 15 || v.HeadingDeg == 0 {
		t.Fatalf("join at 14 NM: %.1f %+v %v", nm, v, err)
	}
	r := ctl.ProcedureCorners()
	found := false
	for _, p := range r {
		if calc.HaversineNM(p.Lat, p.Lon, pLat, pLon) < 0.3 {
			found = true
		}
	}
	if !found {
		t.Errorf("no point on the centreline at 14 NM: %v", r)
	}
}

// TestInterceptOnHeading: an arrival told "heading … to intercept" flies
// that heading from where it is to the final's centreline, then the final
// (the user: told 060, it follows 060).
func TestInterceptOnHeading(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA253",
		InjectApproach: true, Procedure: route}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	_, end, _ := g.Layout.RunwayEnd("06")
	out := math.Mod(end.Heading+180, 360)
	// 20 NM out on the final's line, 3 NM to its side, on a base.
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, out, 20*1852)
	lat, lon = calc.DisplaceByHeading(lat, lon, math.Mod(out+90, 360), 3*1852)
	pos := airport.LatLon{Lat: lat, Lon: lon}
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, pos, 4000, math.Mod(end.Heading+300, 360), 180, false))
	hdg := math.Mod(end.Heading+30, 360) // a 30° intercept from its side
	if !ctl.InterceptOnHeading(hdg) {
		t.Fatal("not flown on the heading")
	}
	ctl.mu.Lock()
	first := ctl.corners[0]
	ctl.mu.Unlock()
	if d := math.Abs(headingDiff(calc.BearingDegrees(pos.Lat, pos.Lon, first.Latitude, first.Longitude), hdg)); d > 1 {
		t.Errorf("first leg %.0f° off the heading told", d)
	}
	fLat, fLon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, out, 30*1852)
	if x := math.Abs(calc.CrossTrackMeters(end.Threshold.Lat, end.Threshold.Lon, fLat, fLon, first.Latitude, first.Longitude)); x > 200 {
		t.Errorf("the intercept point %.0f m off the centreline", x)
	}
	// A heading away from the final: not flown, nothing changed.
	if ctl.InterceptOnHeading(math.Mod(end.Heading+180, 360)) {
		t.Error("a heading away from the final taken")
	}
}
