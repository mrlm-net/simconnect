package traffic

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// An arrival flown by MSFS AI off its route, away from the runway (live,
// TVS1972 flew heading 065 off LOMK8S's end for 38 minutes until cancelled
// as stuck), reports MissedJoin once it is MissedJoinNM farther from the
// runway than its route goes; not while it is on its way.
func TestMissedJoin(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("24", "LOMKI")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "TVS1972",
		InjectApproach: true, Procedure: route}); err != nil {
		t.Fatal(err)
	}
	missed := make(chan string, 64)
	go func() {
		for ev := range ctl.Events() {
			if ev.MissedJoin != "" {
				missed <- ev.MissedJoin
			}
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	wps := ctl.ProcedurePlan()
	if len(wps) < 2 {
		t.Fatal("no procedure")
	}
	fly := func(p airport.LatLon, hdg float64) {
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, p, 5000, hdg, 210, false))
	}
	// On its way, at the STAR's start: no report.
	fly(wps[0].Position, 90)
	select {
	case m := <-missed:
		t.Fatalf("on its route: %q", m)
	case <-time.After(200 * time.Millisecond):
	}
	// At ERASU (the downwind's end), then on heading 065 away, 1 NM a step.
	var p airport.LatLon
	for _, n := range route {
		if n.Ident == "ERASU" {
			p = n.Position
		}
	}
	fly(p, 65)
	for range 25 {
		lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, 65, 1852)
		p = airport.LatLon{Lat: lat, Lon: lon}
		fly(p, 65)
	}
	select {
	case m := <-missed:
		if !strings.Contains(m, "off its route") || !strings.Contains(m, "heading 065") {
			t.Errorf("reported %q", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("flying away past its procedure's end: no MissedJoin")
	}
}
