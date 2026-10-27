package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestNoClearanceOnDownwind: on an extended downwind the approach
// clearance (with its heading to intercept) is not due before the turn in
// (live, OKZSU "fly heading 060, for spacing", then 31 s later "cleared
// visual approach runway 24", its turn in never said).
func TestNoClearanceOnDownwind(t *testing.T) {
	g := lkprGraph(t)
	for _, star := range []string{"VLM", "GOLOP"} {
		for _, d := range []time.Duration{2 * time.Minute, 4 * time.Minute, 6 * time.Minute, 9 * time.Minute} {
			route, err := lkprProcedures(t).Arrival("24", star)
			if err != nil {
				t.Fatal(err)
			}
			ec := &eventClient{}
			ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
			c22, _ := g.Layout.ParkingIndex("C22")
			if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "Asobo PassiveAircraft Phenom 100", Tail: "OKZSU",
				InjectApproach: true, Procedure: route}); err != nil {
				t.Fatal(err)
			}
			go func() {
				for range ctl.Events() {
				}
			}()
			ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
			ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, 0, 250, false))
			ctl.AbsorbDelay(d)
			a, err := ctl.AbsorbDelay(d)
			if err != nil || !a.Downwind || len(ctl.vectors) != 2 {
				continue
			}
			leave, base := ctl.vectors[0], ctl.vectors[1]
			flyTo(t, ctl, leave.At)
			ctl.mu.Lock()
			from := ctl.last.Position
			ctl.mu.Unlock()
			// Along the downwind to 2 NM before the turn in.
			n := int(calc.HaversineNM(from.Lat, from.Lon, base.At.Lat, base.At.Lon)) - 2
			for i := 1; i <= n; i++ {
				f := float64(i) / float64(n+2)
				p := airport.LatLon{Lat: from.Lat + (base.At.Lat-from.Lat)*f, Lon: from.Lon + (base.At.Lon-from.Lon)*f}
				moveTo(ctl, p, calc.BearingDegrees(from.Lat, from.Lon, base.At.Lat, base.At.Lon))
				if ctl.TurningFinal() {
					t.Errorf("%s %v: clearance due %.1f NM before the turn in, on the downwind", star, d, calc.HaversineNM(p.Lat, p.Lon, base.At.Lat, base.At.Lon))
					break
				}
			}
		}
	}
}
