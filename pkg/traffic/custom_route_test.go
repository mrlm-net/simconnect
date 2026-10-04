package traffic

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// taxiwayNode returns the first taxi point along taxiway name.
func taxiwayNode(t *testing.T, g *airport.Graph, name string) airport.NodeID {
	t.Helper()
	for id, es := range g.Adj {
		if g.Nodes[id].Kind == airport.NodeTaxiPoint && len(es) == 2 && es[0].Name == name && es[1].Name == name {
			return airport.NodeID(id)
		}
	}
	t.Fatalf("no node on %s", name)
	return -1
}

// TestDepartureCustomRoute: a departure's RouteOptions.Via and Taxiways
// (#340) survive the pushback planning, which replans the taxi-out from the
// junction the tail is pushed at.
func TestDepartureCustomRoute(t *testing.T) {
	g := lkprGraph(t)
	onB := taxiwayNode(t, g, "B")
	for _, stand := range []string{"C22", "C17", "B14"} {
		for _, opts := range []airport.RouteOptions{{Taxiways: []string{"B"}}, {Via: []airport.NodeID{onB}}} {
			pi, _ := g.Layout.ParkingIndex(stand)
			ec := &eventClient{}
			ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
			if err := ctl.Start(TaxiRequest{Graph: g, Parking: pi, Runway: "24", Model: "A320", Options: opts}); err != nil {
				t.Fatalf("%s %+v: %v", stand, opts, err)
			}
			r := ctl.Route()
			if !slices.Contains(r.Taxiways, "B") || (len(opts.Via) > 0 && !slices.Contains(r.Nodes, onB)) {
				t.Errorf("%s %+v: route %v", stand, opts, r.Taxiways)
			}
			t.Logf("%s %+v: %v (push branch %v, turn %v)", stand, opts, r.Taxiways, ctl.havePushBranch, ctl.pushTurn)
		}
	}
}

// TestArrivalCustomRoute: an arrival's taxi-in follows RouteOptions.Via and
// Taxiways, for the chosen exit and a forced one.
func TestArrivalCustomRoute(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	onG := taxiwayNode(t, g, "G")
	for _, opts := range []airport.RouteOptions{{Taxiways: []string{"G"}}, {Via: []airport.NodeID{onG}}} {
		p, err := PlanArrival(g, "24", c22, ArrivalOptions{SpawnNm: 5, Route: opts})
		if err != nil {
			t.Fatalf("%+v: %v", opts, err)
		}
		if !slices.Contains(p.Route.Taxiways, "G") || (len(opts.Via) > 0 && !slices.Contains(p.Route.Nodes, onG)) {
			t.Errorf("%+v: taxi-in %v", opts, p.Route.Taxiways)
		}
		t.Logf("%+v: exit %s, taxi-in %v", opts, p.Exit.Taxiway, p.Route.Taxiways)
	}
}
