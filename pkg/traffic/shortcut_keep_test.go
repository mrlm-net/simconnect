package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A shortcut never goes past a merge point kept for an arrival ahead
// (#788): to it at the furthest.
func TestShortcutKeepsMergePoint(t *testing.T) {
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
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 6000, hdg, 250, false))
		return ctl
	}
	index := func(fix string) int {
		for i, n := range route {
			if n.Ident == fix {
				return i
			}
		}
		return -1
	}
	free, _, _ := start().Shortcut(30, nil)
	at := index(free)
	merge := ""
	for i := at - 1; i > 1; i-- {
		if route[i].Ident != "" {
			merge = route[i].Ident
			break
		}
	}
	if merge == "" {
		t.Fatalf("free shortcut to %q (at %d): no named fix before it to keep", free, at)
	}
	fix, _, _ := start().Shortcut(30, []string{merge})
	if fix != "" && index(fix) > index(merge) {
		t.Errorf("kept %s: direct %s past it (free: %s)", merge, fix, free)
	}
}
