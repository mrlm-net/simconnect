package airport

import (
	"errors"
	"testing"
)

func TestRunwayExitsLKPR(t *testing.T) {
	g := lkprGraph(t)
	for _, end := range []string{"24", "06", "12", "30"} {
		exits, err := g.RunwayExits(end)
		if err != nil {
			t.Fatal(err)
		}
		if len(exits) < 3 {
			t.Errorf("RWY %s: %d exits, want at least 3", end, len(exits))
		}
		rwy, _, _ := g.Layout.RunwayEnd(end)
		nodes := map[NodeID]bool{}
		highSpeed := 0
		for i, e := range exits {
			if i > 0 && e.Along < exits[i-1].Along {
				t.Errorf("RWY %s: exits not sorted by distance", end)
			}
			if nodes[e.Node] {
				t.Errorf("RWY %s: exit node %d listed twice", end, e.Node)
			}
			nodes[e.Node] = true
			if e.Along < 0 || e.Along > rwy.Length {
				t.Errorf("RWY %s: exit at %.0f m outside the runway", end, e.Along)
			}
			if e.Angle > MaxExitAngle {
				t.Errorf("RWY %s: exit angle %.0f°", end, e.Angle)
			}
			if e.Path[0] != e.RunwayNode || e.Path[len(e.Path)-1] != e.Node {
				t.Errorf("RWY %s: exit path does not run from runway node to exit node", end)
			}
			if _, off := g.runwayCoords(rwy, g.Nodes[e.Node].Position); off <= rwy.Width/2 {
				t.Errorf("RWY %s: exit node %d is still on the runway (%.0f m from centreline)", end, e.Node, off)
			}
			if e.HighSpeed {
				highSpeed++
			}
		}
		if highSpeed == 0 {
			t.Errorf("RWY %s: no high-speed exit", end)
		}
	}
}

func TestExitForAndRouteFromRunway(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	for _, c := range []struct {
		end     string
		rollout float64
		taxiway string
	}{
		{"24", 1500, "D"},
		{"06", 1500, "L"},
		{"30", 1500, "G"},
		{"12", 1500, "P"},
	} {
		exit, err := g.ExitFor(c.end, c.rollout)
		if err != nil {
			t.Fatal(err)
		}
		if exit.Along < c.rollout || exit.Taxiway != c.taxiway {
			t.Errorf("ExitFor(%s, %.0f) = %s at %.0f m, want %s past %.0f m", c.end, c.rollout, exit.Taxiway, exit.Along, c.taxiway, c.rollout)
		}
		r, err := g.RouteFromRunway(exit, c22, RouteOptions{})
		if err != nil {
			t.Fatalf("RWY %s → C22: %v", c.end, err)
		}
		checkRoute(t, g, r)
		if r.Nodes[0] != exit.RunwayNode || r.Nodes[len(r.Nodes)-1] != NodeID(len(g.Layout.TaxiPoints)+c22) {
			t.Errorf("RWY %s → C22 does not run from the runway to the stand", c.end)
		}
		// Rolling off the runway is not a crossing; crossings count from the
		// exit node (off the runway). Crossing the vacated runway again later
		// is real: at LKPR every exit from 24 leaves to the side away from C22.
		exitAt := len(exit.Path) - 1
		if want := g.runwayCrossings(r.Points[exitAt:]); !equal(r.RunwayCrossings, want) {
			t.Errorf("RWY %s → C22 crossings %v, want %v", c.end, r.RunwayCrossings, want)
		}
		t.Logf("RWY %s: exit %s at %.0f m (%.0f°) → C22 %.0f m via %v, crossings %v", c.end, exit.Taxiway, exit.Along, exit.Angle, r.Length, r.Taxiways, r.RunwayCrossings)
	}
}

func TestExitForLongRolloutReturnsLastExit(t *testing.T) {
	g := lkprGraph(t)
	exits, _ := g.RunwayExits("24")
	e, err := g.ExitFor("24", 1e6)
	if err != nil || e.Node != exits[len(exits)-1].Node {
		t.Errorf("ExitFor(24, 1e6) = %+v, %v; want the last exit", e, err)
	}
	if _, err := g.RunwayExits("18"); !errors.Is(err, ErrUnknownRunway) {
		t.Errorf("RunwayExits(18) error = %v", err)
	}
}
