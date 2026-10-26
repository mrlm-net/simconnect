package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestHeavyExitAtItsOwnSpeed: a widebody's exit is planned at its own
// touchdown speed, not an airliner's: one it makes braking as planned, a
// later one than an A320's (the user, live: a 777 on LKPR 24 planned onto D
// like an A320, braking at its limit to make it).
func TestHeavyExitAtItsOwnSpeed(t *testing.T) {
	g := lkprGraph(t)
	b14, err := g.Layout.ParkingIndex("B14")
	if err != nil {
		t.Fatal(err)
	}
	heavy := ProfileFor("B77W")
	reach := exitReach{heavy.Approach.TouchdownKts, heavy.Rollout.BrakeDecel}
	for _, rwy := range []string{"24", "06"} {
		a, _, err := bestExit(g, rwy, b14, airport.RouteOptions{}, exitReach{})
		if err != nil {
			t.Fatal(err)
		}
		h, _, err := bestExit(g, rwy, b14, airport.RouteOptions{}, reach)
		if err != nil {
			t.Fatal(err)
		}
		// Its own touchdown speed, braking at RolloutDecel, to the exit's speed.
		v0, v1 := reach.touchdownKts*knot, ExitSpeed(h)*knot
		need := exitTouchdownMeters + (v0*v0-v1*v1)/(2*RolloutDecel)
		t.Logf("%s: A320 %s at %.0f m, B77W %s at %.0f m (needs %.0f m)", rwy, a.Taxiway, a.Along, h.Taxiway, h.Along, need)
		if h.Along < need {
			t.Errorf("%s: B77W exit %s at %.0f m, short of its rollout %.0f m", rwy, h.Taxiway, h.Along, need)
		}
		if h.Along < a.Along {
			t.Errorf("%s: B77W exit %s before the A320's %s", rwy, h.Taxiway, a.Taxiway)
		}
		// Narrowbodies, faster than the plan too, keep the airliner's exit.
		for _, m := range []string{"A320", "B738", "B739"} {
			p := ProfileFor(m)
			n, _, err := bestExit(g, rwy, b14, airport.RouteOptions{}, exitReach{p.Approach.TouchdownKts, p.Rollout.BrakeDecel})
			if err != nil {
				t.Fatal(err)
			}
			if n.Taxiway != a.Taxiway {
				t.Errorf("%s: %s exit %s, not the airliner's %s", rwy, m, n.Taxiway, a.Taxiway)
			}
		}
	}
}
