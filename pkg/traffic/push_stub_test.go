package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A push never leaves the aircraft facing up a dead-end stub whose taxi-out
// turns round at its end and comes back past the push (#1227: LFPO 11
// facing up a 17 m stub, KJFK F7 the same 178° off its way out): the
// taxi-out passes no node twice, and starts within 30° of the nose.
func TestPushNotUpAStub(t *testing.T) {
	for _, c := range []struct{ icao, stand, rwy string }{
		{"KJFK", "F7", "04L"}, {"KJFK", "B1", "04L"}, {"KJFK", "260", "22R"}, {"EGLL", "1026", "09L"},
	} {
		g := airportGraph(t, c.icao)
		i, err := g.Layout.ParkingIndex(c.stand)
		if err != nil {
			t.Fatal(err)
		}
		req := TaxiRequest{Graph: g, Parking: i, Model: "A320", Runway: c.rwy}
		req.resolveAircraft()
		req.Options = withSpan(req.Options, req.Profile)
		req.Options.OwnStands = []int{i}
		route, err := g.RouteToRunwayEntry(i, c.rwy, "", req.Options)
		if err != nil {
			t.Fatal(err)
		}
		ctl := &TaxiController{req: req, route: route, origRoute: route, pushJunction: 1}
		ctl.planPushback()
		p, err := ctl.pushPath()
		if err != nil {
			t.Fatal(err)
		}
		seen := map[airport.NodeID]bool{}
		for k, walked := ctl.pushJunction, 0.0; k < len(ctl.route.Nodes) && walked < 150; k++ {
			if k > ctl.pushJunction {
				walked += localDist(ctl.route.Points[k-1], ctl.route.Points[k])
			}
			if n := ctl.route.Nodes[k]; seen[n] {
				t.Errorf("%s %s: the taxi-out passes node %d twice, %.0f m after the push", c.icao, c.stand, n, walked)
			}
			seen[ctl.route.Nodes[k]] = true
		}
		// The way out 40 m on from the route point nearest where the push ends.
		end := p.PointAt(p.Length())
		hdg := localBearing(p.PointAt(math.Max(0, p.Length()-3)), end) + 180
		near, best := ctl.pushJunction, math.Inf(1)
		for k := ctl.pushJunction; k < len(ctl.route.Points) && k < ctl.pushJunction+4; k++ {
			if d := localDist(end, ctl.route.Points[k]); d < best {
				near, best = k, d
			}
		}
		to := ctl.route.Points[len(ctl.route.Points)-1]
		for k, w := near+1, 0.0; k < len(ctl.route.Points); k++ {
			if w += localDist(ctl.route.Points[k-1], ctl.route.Points[k]); w >= 40 {
				to = ctl.route.Points[k]
				break
			}
		}
		if off := math.Abs(headingDiff(hdg, localBearing(end, to))); off > 30 {
			t.Errorf("%s %s: the push ends %.0f° off the way out", c.icao, c.stand, off)
		}
	}
}
