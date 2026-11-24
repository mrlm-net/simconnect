package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestEndTurnoffTaken (#1674): on the builder sample's field (no rapid
// exits F, G), C at mid-length out of every type's reach and B and D
// leading back about 115° from the runway's ends to the parallel, arrivals
// both ways to either stand leave by the end taxiway ahead at taxi speed:
// no turn round on the runway, no backtrack. Before, they rolled to the
// end, turned round and backtracked 1.2 km to C.
func TestEndTurnoffTaken(t *testing.T) {
	ahead := map[string]string{"27": "B", "09": "D"}
	for _, model := range []string{"A321", "A20N", "B738", "B77W"} {
		ap := ProfileFor(model)
		ro := withDefaults(ap.Rollout, DefaultRolloutProfile())
		o := ArrivalOptions{TouchdownKts: ap.Approach.TouchdownKts, BrakeDecel: ro.BrakeDecel}
		reach := exitReach{o.TouchdownKts, o.BrakeDecel}
		for rwy, want := range ahead {
			for parking := range 2 {
				// Sharper than MaxEntryAngle: no turnoff, the roll to the end
				// and back to C, as #1631 plans it.
				p, err := PlanArrival(midExitGraph(t, 100), rwy, parking, o)
				if err != nil || !p.EndTurn || p.Exit.Taxiway != "C" {
					t.Fatalf("%s %s stand %d, no turnoff: %v, planned %s, end turn %v: want the roll to the end and back to C: nothing checked", model, rwy, parking, err, p.Exit.Taxiway, p.EndTurn)
				}

				g := midExitGraph(t, 35)
				p, err = PlanArrival(g, rwy, parking, o)
				if err != nil {
					t.Fatalf("%s %s stand %d: %v", model, rwy, parking, err)
				}
				if p.EndTurn || p.Backtrack || p.Exit.Taxiway != want {
					t.Errorf("%s %s stand %d: planned %s, end turn %v, backtrack %v (%.0f m); want %s at the end, no backtrack",
						model, rwy, parking, p.Exit.Taxiway, p.EndTurn, p.Backtrack, p.BacktrackMeters(), want)
					continue
				}
				if p.Exit.Angle <= airport.MaxExitAngle || !reach.makes(g, rwy, p.Exit) {
					t.Errorf("%s %s stand %d: %s at %.0f°, made %v", model, rwy, parking, want, p.Exit.Angle, reach.makes(g, rwy, p.Exit))
				}
				// Reached at taxi speed: the landing chain's last rollout
				// point, on the turnoff's runway node.
				rn := p.Route.Points[0]
				for _, w := range p.Waypoints {
					if w.Latitude == rn.Lat && w.Longitude == rn.Lon && w.KtsSpeed > SharpTurnSpeedKts+0.5 {
						t.Errorf("%s %s stand %d: %s turned into at %.0f kt, want %.0f", model, rwy, parking, want, w.KtsSpeed, SharpTurnSpeedKts)
					}
				}
			}
		}
	}
}

// TestEndTurnoffPlayerExit (#1674, #1096): an arrival planned onto the end
// taxiway keeps the exit the player names instead: C, which it cannot
// make, by the roll to the end and back to it.
func TestEndTurnoffPlayerExit(t *testing.T) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	g := midExitGraph(t, 35)
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "27", Parking: 0, Model: "A321", Tail: "TEX1",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	if p := ctl.Plan(); p.EndTurn || p.Exit.Taxiway != "B" {
		t.Fatalf("planned %s, end turn %v: want B at the end: nothing checked", p.Exit.Taxiway, p.EndTurn)
	}
	exits, err := g.RunwayExits("27")
	if err != nil {
		t.Fatal(err)
	}
	var c *airport.RunwayExit
	for i := range exits {
		if exits[i].Taxiway == "C" {
			c = &exits[i]
		}
	}
	if c == nil {
		t.Fatal("C is no exit off 27")
	}
	if err := ctl.VacateAt(*c); err != nil {
		t.Fatalf("vacate at C: %v", err)
	}
	if p := ctl.Plan(); p.Exit.Node != c.Node || !p.EndTurn {
		t.Errorf("vacate at C: planned %s, end turn %v; want C by the roll to the end and back", p.Exit.Taxiway, p.EndTurn)
	}
}

// midRollPastC starts an A321 onto 27 of the builder sample with B leading
// back at the end, planned instead to C at mid-length (as when a slower
// touchdown made it), and returns it with its nose gear 100 m before C.
func midRollPastC(t *testing.T) (*ArrivalController, airport.RunwayExit, airport.LatLon) {
	t.Helper()
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	g := midExitGraph(t, 35)
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "27", Parking: 0, Model: "A321", Tail: "TEX1",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	exits, err := g.RunwayExits("27")
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range exits {
		if x.Taxiway != "C" {
			continue
		}
		if err := ctl.planExit(x, false, false); err != nil {
			t.Fatalf("plan to C: %v", err)
		}
		hdg := ctl.Plan().End.Heading
		return ctl, x, offsetHeading(g.Nodes[x.RunwayNode].Position, hdg, -100)
	}
	t.Fatal("C is no exit off 27")
	return nil, airport.RunwayExit{}, airport.LatLon{}
}

// TestEndTurnoffMidRoll (#1674): an arrival rolling at 70 kt 100 m before
// C, too fast for it, goes on to B leading back at the runway's end and
// takes it at taxi speed: no turn round at the end, no backtrack. Before,
// it rolled to the end, turned round and backtracked to C.
func TestEndTurnoffMidRoll(t *testing.T) {
	const kts = 70.0
	ctl, _, nose := midRollPastC(t)
	if ctl.exitTurnReach(nose) >= kts*ktsToMS {
		t.Fatalf("C made at %.0f kt from 100 m: nothing checked", kts)
	}
	turn := ctl.endTurn(kts, nose)
	p := ctl.Plan()
	if len(turn) > 0 || p.EndTurn || p.Backtrack || p.Exit.Taxiway != "B" {
		t.Fatalf("missed C at %.0f kt: planned %s, end turn %v (%d points), backtrack %v; want B at the end, no turn round",
			kts, p.Exit.Taxiway, p.EndTurn, len(turn), p.Backtrack)
	}
	if p.Exit.Angle <= airport.MaxExitAngle {
		t.Errorf("B at %.0f°: no turnoff leading back", p.Exit.Angle)
	}
	if ctl.exitTurnReach(nose) < kts*ktsToMS {
		t.Errorf("B not slowed to at taxi speed from %.0f kt", kts)
	}
}
