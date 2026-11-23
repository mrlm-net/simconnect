package traffic

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestArrivalChangeRunwayKeepsStandSide (#1667): an arrival whose stand stop
// is off the parking line (StandSide) still stops there after a runway
// change, as planned at its start.
func TestArrivalChangeRunwayKeepsStandSide(t *testing.T) {
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
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	const side = 3.0
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
		InjectApproach: true, Procedure: to06, StandSide: side, RollThroughChance: -1}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	before := ctl.Plan().Stop
	if d := localDist(before, StandPointSide(g.Layout.Parking[c22], ctl.req.NoseOffset, 0)); d < side-0.5 {
		t.Fatalf("planned %.1f m off the parking line at the start, want %.0f", d, side)
	}
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, to06[1].Position, 6000, 200, 250, false))
	if err := ctl.ChangeRunway("24", to24, nil); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	if p.End.Name != "24" {
		t.Fatalf("plan for %s", p.End.Name)
	}
	if d := localDist(p.Stop, before); d > 0.01 {
		t.Errorf("stand stop moved %.2f m by the runway change", d)
	}
	if !slices.ContainsFunc(p.TaxiWaypoints, func(w types.SIMCONNECT_DATA_WAYPOINT) bool {
		return localDist(before, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}) < 0.5
	}) {
		t.Error("the taxi-in does not stop at the stand stop")
	}
}
