package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestInterceptNoHook: an extended downwind at LKPR 24 turns base, and the
// intercept leg starts on the base: no point of the route out beyond the
// base turn, and extended again it runs on along the downwind, not along
// a leg beside the final (the user, live: CSA188's base flew out past its
// turn and hooked back; the next extension ran on 1.5 NM beside the final).
func TestInterceptNoHook(t *testing.T) {
	g := lkprGraph(t)
	for _, star := range []string{"LOMKI", "VLM"} {
		route, err := lkprProcedures(t).Arrival("24", star)
		if err != nil {
			t.Fatal(err)
		}
		ec := &eventClient{}
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
		c22, _ := g.Layout.ParkingIndex("C22")
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "A320", Tail: "CSA188",
			InjectApproach: true, Procedure: route}); err != nil {
			t.Fatal(err)
		}
		go func() {
			for range ctl.Events() {
			}
		}()
		ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 12000, 90, 250, false))
		thr, hdg := ctl.plan.End.Threshold, ctl.plan.End.Heading
		far := offsetHeading(thr, hdg+180, 30*1852)
		ctl.AbsorbDelay(3 * time.Minute) // speed takes it
		for i := 1; i <= 2; i++ {
			if a, err := ctl.AbsorbDelay(3 * time.Minute); err != nil || !a.Downwind {
				t.Fatalf("%s %d: %+v %v, want the downwind extended", star, i, a, err)
			}
			base, out, beside := -1.0, 0.0, 0.0
			for j, w := range ctl.corners {
				al := calc.AlongTrackMeters(thr.Lat, thr.Lon, far.Lat, far.Lon, w.Latitude, w.Longitude) / 1852
				if al > out {
					out = al
				}
				if j < len(ctl.cornerNames) && ctl.cornerNames[j] == "BASE" {
					base = al
					cr := calc.CrossTrackMeters(thr.Lat, thr.Lon, far.Lat, far.Lon, w.Latitude, w.Longitude) / 1852
					beside = max(cr, -cr)
				}
			}
			if base < 0 || out > base+0.1 {
				t.Errorf("%s %d: base turn %.1f NM out, the route out to %.1f NM", star, i, base, out)
			}
			if beside < 4 {
				t.Errorf("%s %d: base turn %.1f NM beside the final, want the downwind's 5", star, i, beside)
			}
		}
	}
}
