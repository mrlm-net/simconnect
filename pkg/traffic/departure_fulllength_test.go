package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestDepartureRouteFullLength (#1571): a full-length departure (Entry
// "") the route sends to an intersection with less runway ahead than the
// aircraft needs goes to the full-length entry instead; one with enough
// keeps its route.
func TestDepartureRouteFullLength(t *testing.T) {
	g := lkprGraph(t)
	rerouted, kept := 0, 0
	for _, rwy := range g.Layout.Runways {
		for _, end := range []string{rwy.Primary.Name, rwy.Secondary.Name} {
			if !g.ThresholdEntry(end) {
				continue
			}
			for st := range g.Layout.Parking {
				for _, model := range []string{"B77W", "A320"} {
					req := TaxiRequest{Graph: g, Parking: st, Runway: end, Model: model}
					req.resolveAircraft()
					req.Options.OwnStands = []int{st}
					base, err := g.RouteToRunwayEntry(st, end, "", req.Options)
					if err != nil || len(base.Nodes) == 0 {
						continue
					}
					got, err := departureRoute(req)
					if err != nil {
						t.Fatalf("%s stand %d %s: %v", model, st, end, err)
					}
					if got.Entry != "" {
						t.Errorf("%s stand %d %s: entry %q, want full length", model, st, end, got.Entry)
					}
					last := func(r *airport.Route) airport.NodeID { return r.Nodes[len(r.Nodes)-1] }
					if !runShort(req, base) {
						if last(got) != last(base) {
							t.Errorf("%s stand %d %s: long enough at %d, moved to %d", model, st, end, last(base), last(got))
						}
						kept++
						continue
					}
					had, _, _ := runwayAhead(req, last(base))
					left, _, _ := runwayAhead(req, last(got))
					if last(got) != last(base) {
						if left <= had {
							t.Errorf("%s stand %d %s: moved to %d with %.0f m, had %.0f m", model, st, end, last(got), left, had)
						}
						rerouted++
					}
				}
			}
		}
	}
	if kept == 0 {
		t.Fatal("no route kept")
	}
	t.Logf("rerouted %d, kept %d", rerouted, kept)
}
