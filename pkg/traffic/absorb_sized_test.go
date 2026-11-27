package traffic

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// timeToJoin is how long an arrival takes from where it is along its
// waypoints to the join point, each leg at its point's speed.
func timeToJoin(c *ArrivalController) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	wps := c.proc.Waypoints
	h, prev := 0.0, c.last.Position
	for _, w := range wps[c.procWaypoint(wps):] {
		h += calc.HaversineNM(prev.Lat, prev.Lon, w.Latitude, w.Longitude) / math.Max(w.KtsSpeed, 60)
		prev = airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
	}
	return time.Duration(h * float64(time.Hour))
}

// TestSpacingDelaySizedVector (TestSpottingObserver on v0.197.0): an
// arrival on LKPR's LOMKI 8S for 24 past PR518, slowed already, asked for
// 38-41 s more for spacing behind one it meets 15 NM on. It loses that and not
// minutes: the track added is the delay at its speed and a mile at most
// (sizedSlackNM, the shortest vector), the time lost within
// sizedSlack of the delay and never less, nothing left to a hold, and the
// vector flown before they meet. Its downwind extended, a whole base and
// intercept added 9.6 NM, 3m16s (CSA744 "+9.8 NM" for 41 s), and the
// landings behind it slid out of the window.
func TestSpacingDelaySizedVector(t *testing.T) {
	const (
		sizedSlackNM = 1.0
		sizedSlack   = 20 * time.Second
		withinNM     = 15.0
	)
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("24", "LOMKI")
	if err != nil {
		t.Fatal(err)
	}
	c22, _ := g.Layout.ParkingIndex("C22")
	next := 0
	for i := range route[1:] {
		if route[i].Ident == "PR518" {
			next = i + 1
		}
	}
	if next == 0 {
		t.Fatal("no PR518 on LOMKI 8S")
	}
	from, to := route[next-1].Position, route[next].Position
	at := airport.LatLon{Lat: (from.Lat + to.Lat) / 2, Lon: (from.Lon + to.Lon) / 2}
	for _, d := range []time.Duration{38 * time.Second, 41 * time.Second} {
		t.Run(fmt.Sprint(d), func(t *testing.T) {
			ec := &eventClient{}
			ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
			if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL_SBAI_BCS3_CSA-Lines", Tail: "CSA744",
				InjectApproach: true, Procedure: route}); err != nil {
				t.Fatal(err)
			}
			go func() {
				for range ctl.Events() {
				}
			}()
			ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
			ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, at, 7000, calc.BearingDegrees(from.Lat, from.Lon, to.Lat, to.Lon), 200, false))
			if first, err := ctl.AbsorbWithin(time.Minute, withinNM); err != nil || first.SpeedKts == 0 || first.ExtraNM > 0 {
				t.Fatalf("setup: a minute: %s, %v; want it slowed only", first, err)
			}
			was := timeToJoin(ctl)
			a, err := ctl.AbsorbWithin(d, withinNM)
			if err != nil {
				t.Fatal(err)
			}
			lost := timeToJoin(ctl) - was
			kts := math.Max(a.SpeedKts, ProcedureApproachSpeedKts)
			t.Logf("%v for spacing: %s, %v lost (downwind %v, dog-leg %v)", d, a, lost.Round(time.Second), a.Downwind, ctl.dogLeg != nil)
			if a.Left > 0 {
				t.Errorf("%v: %v left to a hold, want it lost", d, a.Left)
			}
			if lost < d-time.Second || lost > d+sizedSlack {
				t.Errorf("%v for spacing: %v lost, want %v to %v", d, lost.Round(time.Second), d, d+sizedSlack)
			}
			if max := d.Hours()*kts + sizedSlackNM; a.ExtraNM > max {
				t.Errorf("%v for spacing: %.1f NM of track added, want at most %.1f (the delay at %.0f kt and %.0f NM)", d, a.ExtraNM, max, kts, sizedSlackNM)
			}
			if dl := ctl.dogLeg; dl != nil {
				nm, prev, found := 0.0, at, false
				for _, p := range route[next:] {
					nm += calc.HaversineNM(prev.Lat, prev.Lon, p.Position.Lat, p.Position.Lon)
					prev = p.Position
					if p.Position == dl.to {
						found = true
						break
					}
				}
				if found && nm > withinNM+0.5 {
					t.Errorf("%v: the dog-leg ends %.1f NM on, past the meeting %.0f NM on", d, nm, withinNM)
				}
			}
		})
	}
}
