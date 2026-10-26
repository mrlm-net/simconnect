package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestAbsorbWithinExtendsTheDownwindBeforeTheMeeting: spacing behind
// another arrival met where the base turn joins the final, the STAR's
// downwind ahead: the downwind is extended, "fly heading, for spacing"
// along it, then base and the intercept (live, LKPR VLM for 24: UAE364
// sent off to the side at "heading 115" with the downwind 065 ahead of it
// and SWR1658 met at PR532). Met before the downwind ends, a vector off the
// STAR as before (OKCMB).
func TestAbsorbWithinExtendsTheDownwindBeforeTheMeeting(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("24", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	idx := func(id string) int {
		for i, p := range route {
			if p.Ident == id {
				return i
			}
		}
		t.Fatalf("no %s on the route", id)
		return -1
	}
	a0, b0 := route[idx("PR572")].Position, route[idx("PR573")].Position
	at := airport.LatLon{Lat: (a0.Lat + b0.Lat) / 2, Lon: (a0.Lon + b0.Lon) / 2}
	along := func(to string) float64 { // from at along the route to fix to
		nm, prev := 0.0, at
		for _, p := range route[idx("PR573"):] {
			nm += calc.HaversineNM(prev.Lat, prev.Lon, p.Position.Lat, p.Position.Lon)
			prev = p.Position
			if p.Ident == to {
				break
			}
		}
		return nm
	}
	start := func() *ArrivalController {
		ec := &eventClient{}
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
		c22, _ := g.Layout.ParkingIndex("C22")
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "UAE364",
			InjectApproach: true, Procedure: route}); err != nil {
			t.Fatal(err)
		}
		go func() {
			for range ctl.Events() {
			}
		}()
		ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, at, 7000, calc.BearingDegrees(a0.Lat, a0.Lon, b0.Lat, b0.Lon), 210, false))
		return ctl
	}

	ctl := start()
	a, err := ctl.AbsorbWithin(3*time.Minute, along("PR532"))
	if err != nil {
		t.Fatal(err)
	}
	if !a.Downwind || ctl.dogLeg != nil {
		t.Fatalf("%+v (dog-leg %v), want the downwind extended", a, ctl.dogLeg != nil)
	}
	if len(ctl.vectors) < 2 || ctl.vectors[0].For != "spacing" || ctl.vectors[1].For != "base" {
		t.Fatalf("vectors %+v, want for spacing then for base", ctl.vectors)
	}
	down := calc.BearingDegrees(route[idx("PR574")].Position.Lat, route[idx("PR574")].Position.Lon, route[idx("RATEV")].Position.Lat, route[idx("RATEV")].Position.Lon)
	if d := math.Abs(headingDiff(ctl.vectors[0].HeadingDeg, down)); d > 10 {
		t.Errorf("for spacing heading %.0f, want along the downwind %.0f", ctl.vectors[0].HeadingDeg, down)
	}

	ctl = start()
	if a, err = ctl.AbsorbWithin(3*time.Minute, along("PR574")); err != nil {
		t.Fatal(err)
	}
	if a.Downwind {
		t.Errorf("met before the downwind: %+v, want a vector off the STAR", a)
	}
}
