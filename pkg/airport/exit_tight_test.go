package airport

import "testing"

// TestExitTight: an aircraft fitting the exit vacates by it; wings too wide or
// a hook back on the way is too tight to vacate.
func TestExitTight(t *testing.T) {
	g, err := BuildGraph(loadAirport(t, "LKPR"))
	if err != nil {
		t.Fatal(err)
	}
	_, end, ok := g.Layout.RunwayEnd("24")
	if !ok {
		t.Fatal("no runway 24")
	}
	exits, err := g.RunwayExits("24")
	if err != nil || len(exits) == 0 {
		t.Fatalf("%d exits: %v", len(exits), err)
	}
	x := exits[len(exits)-1]
	if why := g.ExitTight(x, end.Heading, RouteOptions{HalfSpan: 18}); why != "" {
		t.Errorf("an A320 at %s: %s", x.Taxiway, why)
	}
	wide := x
	wide.Path = append([]NodeID(nil), x.Path...)
	opts := RouteOptions{HalfSpan: 18, TaxiwayMaxSpan: map[string]float64{}}
	named := false
	for _, e := range g.Adj[x.Path[0]] {
		if e.To == x.Path[1] && e.Name != "" {
			opts.TaxiwayMaxSpan[e.Name] = 20
			named = true
		}
	}
	if !named {
		t.Skip("the exit's first edge has no name")
	}
	if why := g.ExitTight(wide, end.Heading, opts); why == "" {
		t.Error("wider than the taxiway allows, and not too tight")
	}
	// Out to the second node and back: a hook.
	hook := x
	hook.Path = []NodeID{x.Path[0], x.Path[1], x.Path[0]}
	if why := g.ExitTight(hook, end.Heading, RouteOptions{}); why == "" {
		t.Error("a path doubling back is not too tight")
	}
}
