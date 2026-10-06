package traffic

import (
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
	if r := ctl.ProcedureRoute(); len(r) == 0 || calc.HaversineNM(r[0].Lat, r[0].Lon, off.Lat, off.Lon) > 3 {
		t.Errorf("route does not go to the point first")
	}
}
