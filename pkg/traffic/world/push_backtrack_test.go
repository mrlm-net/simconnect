package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// legDep is a departure on its stand with its way out.
type legDep struct {
	passDep
	route *airport.Route
}

func (d *legDep) Route() *airport.Route { return d.route }

func legOff(p airport.LatLon, hdg, m float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, hdg, m)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// TestPushHeldForBacktrackOnApproach (#1641): a backtracking arrival still
// on approach will come the other way along the lane a departure is to push
// onto and taxi out along. The push and the start of an airliner take
// longer than pushStartS (XTWN: 169 s to the end of the push, 74 s more to
// taxi), so one cleared by that estimate stood in the lane as the arrival
// came in, each waiting for the other (TFFJ). Ground holds the push though
// the estimate says the departure would be off the lane in time.
func TestPushHeldForBacktrackOnApproach(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	w := New(Options{DataDir: t.TempDir()})
	cc := newControlCenter(nil, w.st.core)
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case f := <-cc.cmds:
				f()
			case <-stop:
				return
			}
		}
	}()
	_, end, ok := g.Layout.RunwayEnd("24")
	if !ok {
		t.Fatal("no runway 24")
	}
	const model = "FSLTL_A320_LYX"
	stand := g.Layout.Parking[0].Position
	lane := legOff(stand, 0, 120)
	route := &airport.Route{Length: 420, Points: []airport.LatLon{stand, lane, legOff(lane, 90, 300)}}
	way := []airport.LatLon{legOff(lane, 90, 400), legOff(lane, 90, 300), lane, legOff(lane, 270, 300)}
	plan := &traffic.ArrivalPlan{Backtrack: true, ClearIndex: 1, VacateIndex: 1, Route: &airport.Route{Points: way}}
	plan.End = end

	half := traffic.MotionProfileFor(model).SpanMeters / 2
	reach := 2*half + traffic.HeadOnMarginMeters
	fa, fb, ln, _, ok := traffic.OpposedStretch(way[1:], firstMeters(route.Points, pushLegM), reach)
	if !ok {
		t.Fatal("the ways do not run along the lane the other way")
	}
	taxiMps := traffic.TaxiSpeedKts * 1852 / 3600
	estimate := pushStartS + pushLegMarginS + (fb+ln-fa)/taxiMps
	before := estimate + 60
	out := before * traffic.ApproachSpeedKts * 1852 / 3600

	arr := &controlled{ID: 1, Tail: "TST1", ICAO: "LKPR", cc: cc, arr: &passArr{state: traffic.ArrivalApproaching, plan: plan}, graph: g, stand: -1}
	arr.view.Model, arr.view.Position = model, legOff(end.LandingThreshold(), end.Heading+180, out)
	dep := &controlled{ID: 2, Tail: "TST2", ICAO: "LKPR", cc: cc, graph: g, spoken: map[string]bool{"standby passing": true, "standby backtrack": true},
		dep: &legDep{passDep: passDep{area: airport.Occupied{Points: []airport.LatLon{stand, lane}, HalfSpan: half}}, route: route}}
	dep.view.Model, dep.view.Position = model, stand
	dep.request = "pushback"
	cc.mu.Lock()
	cc.items[1], cc.items[2] = arr, dep
	cc.mu.Unlock()

	if _, b, ok := arr.inboundWay(); !ok || b < estimate {
		t.Fatalf("arrival %.0f s from its way off, want past the estimate %.0f s", b, estimate)
	}
	if !dep.holdPushForPassing("pushback") && !dep.holdForBacktrack("pushback") {
		t.Errorf("pushed with the arrival %.0f s from its way off: on the lane as it comes in", before)
	}
}
