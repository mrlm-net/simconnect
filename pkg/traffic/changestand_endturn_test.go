package traffic

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// endTurnArrival starts an injected arrival planned to roll to the end of
// 27 of midExitGraph and back to C (#1631).
func endTurnArrival(t *testing.T, model string) (*ArrivalController, *eventClient, *Injector) {
	t.Helper()
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	if err := ctl.Start(ArrivalRequest{Graph: midExitGraph(t, 100), Runway: "27", Parking: 0, Model: model, Tail: "TEX1",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	if p := ctl.Plan(); !p.EndTurn || p.Exit.Taxiway != "C" {
		t.Fatalf("planned %s, end turn %v: want the roll to the end and back to C", p.Exit.Taxiway, p.EndTurn)
	}
	return ctl, ec, inj
}

// TestChangeStandKeepsEndTurn (#1658): an arrival planned to the end and
// back stays so when it is given another stand or place on it: the same
// exit back, the same runway time, the rollout planned to the turn round.
func TestChangeStandKeepsEndTurn(t *testing.T) {
	changes := map[string]func(*ArrivalController) error{
		"another stand": func(c *ArrivalController) error { return c.ChangeStand(1) },
		"another place": func(c *ArrivalController) error {
			_, err := c.SetStandPlace(c.req.NoseOffset+3, c.req.StandSide)
			return err
		},
	}
	for name, change := range changes {
		for _, state := range []ArrivalState{ArrivalApproaching, ArrivalLanding} {
			ctl, _, _ := endTurnArrival(t, "A321")
			was := *ctl.Plan()
			ctl.mu.Lock()
			ctl.state = state
			ctl.mu.Unlock()
			if err := change(ctl); err != nil {
				t.Fatalf("%s, %v: %v", name, state, err)
			}
			p := ctl.Plan()
			if !p.EndTurn || !p.Backtrack || p.Exit.Node != was.Exit.Node || math.Abs(p.BacktrackMeters()-was.BacktrackMeters()) > 1 || p.ClearAt() != was.ClearAt() {
				t.Errorf("%s, %v: end turn %v, backtrack %.0f m, off at %d; want true, %.0f m, %d",
					name, state, p.EndTurn, p.BacktrackMeters(), p.ClearAt(), was.BacktrackMeters(), was.ClearAt())
			}
			if got, want := furthestAlong(p), furthestAlong(&was); math.Abs(got-want) > RolloutSpacingMeters {
				t.Errorf("%s, %v: rollout planned to %.0f m, want %.0f (the turn round)", name, state, got, want)
			}
		}
	}
}

// furthestAlong is how far past End's threshold the landing chain goes.
func furthestAlong(p *ArrivalPlan) float64 {
	m := 0.0
	for _, w := range p.Waypoints {
		m = math.Max(m, alongHeading(p.End.Threshold, p.End.Heading, airportLatLon(w.Latitude, w.Longitude)))
	}
	return m
}

// TestVacateAtKeepsEndTurn (#1658): named the exit it is planned back to,
// an arrival keeps the roll to the end and back; turned round there, it is
// not sent to an exit behind it.
func TestVacateAtKeepsEndTurn(t *testing.T) {
	ctl, _, _ := endTurnArrival(t, "A321")
	was := *ctl.Plan()
	exits, err := ctl.req.Graph.RunwayExits("27")
	if err != nil {
		t.Fatal(err)
	}
	named := -1
	for i, x := range exits {
		if x.Node == was.Exit.Node {
			named = i
		}
	}
	if named < 0 {
		t.Fatal("C is no exit off 27")
	}
	if err := ctl.VacateAt(exits[named]); err != nil {
		t.Fatalf("vacate at C: %v", err)
	}
	if p := ctl.Plan(); !p.EndTurn || p.BacktrackMeters() != was.BacktrackMeters() {
		t.Errorf("vacate at C: end turn %v, backtrack %.0f m; want true, %.0f", p.EndTurn, p.BacktrackMeters(), was.BacktrackMeters())
	}

	ctl, _, _ = endTurnArrival(t, "A321")
	end := ctl.Plan().End
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	ctl.state = ArrivalRollout
	ctl.last.Position = offsetHeading(end.Threshold, end.Heading, 1800)
	ctl.last.Heading, ctl.last.GroundSpeed = math.Mod(end.Heading+180, 360), 15
	behind := exits[named]
	behind.Along, behind.Node = 2400, -1
	if err := ctl.canMakeExit(behind); !errors.Is(err, ErrExitTooClose) {
		t.Errorf("turned round, sent to an exit behind it: %v", err)
	}
}

// TestChangeStandEndTurnFlown (#1658): given another stand on the approach
// and while landing, it brakes for the turn round near the far end, turns
// off no faster than a right-angle exit allows, and parks with no jump.
func TestChangeStandEndTurnFlown(t *testing.T) {
	for _, model := range []string{"A321", "B738"} {
		for _, when := range []ArrivalState{ArrivalApproaching, ArrivalLanding} {
			endTurnFlown(t, model, when)
		}
	}
}

func endTurnFlown(t *testing.T, model string, when ArrivalState) {
	ctl, ec, inj := endTurnArrival(t, model)
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	go func() {
		for range ctl.Events() {
		}
	}()
	end, length := ctl.Plan().End, ctl.Plan().Runway.Length
	mon, changed := DefaultArrivalRequestBase+arrReqMonitor, false
	for i := 0; i < 60*3600 && !ctl.State().Terminal(); i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, end.Threshold, 0, 0, 0, false))
		if changed || ctl.State() != when {
			continue
		}
		if err := ctl.ChangeStand(1); err != nil {
			t.Fatalf("%s, %v: %v", model, when, err)
		}
		changed = true
		if !ctl.Plan().EndTurn {
			t.Errorf("%s, %v: the end turn dropped", model, when)
		}
	}
	if !changed || ctl.State() != ArrivalParked {
		t.Fatalf("%s, %v: changed %v, ended %v", model, when, changed, ctl.State())
	}
	all := placements(ec)
	turned, fastest := -1.0, 0.0
	for i := 1; i < len(all); i++ {
		a, b := all[i-1], all[i]
		d := calc.HaversineMeters(a.Latitude, a.Longitude, b.Latitude, b.Longitude)
		if d > 1.5 {
			t.Errorf("%s, %v: jumped %.2f m at placement %d", model, when, d, i)
			break
		}
		if a.OnGround == 0 || b.OnGround == 0 {
			continue
		}
		dev := math.Abs(headingDiff(end.Heading, b.Heading))
		if dev >= 60 && turned < 0 {
			turned = alongHeading(end.Threshold, end.Heading, airportLatLon(b.Latitude, b.Longitude))
		}
		if dev >= 60 && dev <= 120 {
			fastest = math.Max(fastest, d*60/ktsToMS)
		}
	}
	if turned < length-200 {
		t.Errorf("%s, %v: turned %.0f m down the runway, want the far end's turn round (%.0f)", model, when, turned, length)
	}
	if fastest > 16 {
		t.Errorf("%s, %v: %.1f kt across the runway, want about 15 at most", model, when, fastest)
	}
}
