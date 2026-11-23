package traffic

import (
	"fmt"
	"testing"
	"time"
)

// TestShortDelayAtMinimumSpeedIsLost (#1691): an arrival at its minimum
// speed asked for 10-45 s more loses them: speed has nothing left to take,
// so its downwind is extended, by MinDownwindVector of flight at least (no
// heading for spacing with the base turn seconds after it), and nothing is
// left to a hold that never comes.
func TestShortDelayAtMinimumSpeedIsLost(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("24", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	c22, _ := g.Layout.ParkingIndex("C22")
	for _, d := range []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second, 45 * time.Second} {
		t.Run(fmt.Sprint(d), func(t *testing.T) {
			ec := &eventClient{}
			ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
			if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
				InjectApproach: true, Procedure: route}); err != nil {
				t.Fatal(err)
			}
			go func() {
				for range ctl.Events() {
				}
			}()
			ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
			ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, 0, 250, false))
			first, err := ctl.AbsorbDelay(3 * time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if first.SpeedKts != MinProcedureSpeedKts {
				t.Fatalf("setup: 3 min at %.0f kt, want the minimum %.0f", first.SpeedKts, MinProcedureSpeedKts)
			}
			a, err := ctl.AbsorbDelay(d)
			if err != nil {
				t.Fatal(err)
			}
			if a.Left > 0 {
				t.Errorf("%v more at the minimum speed: %s; want it lost, none left", d, a)
			}
			lost := time.Duration(a.ExtraNM / MinProcedureSpeedKts * float64(time.Hour))
			if !a.Downwind || lost < d {
				t.Errorf("%v more: %s, loses %v; want the downwind extended by that at least", d, a, lost.Round(time.Second))
			}
			if on := time.Duration(ctl.tromboneNM / MinProcedureSpeedKts * float64(time.Hour)); on < MinDownwindVector-time.Second {
				t.Errorf("%v more: the downwind extended for %v of flight, under MinDownwindVector", d, on.Round(time.Second))
			}
		})
	}
}
