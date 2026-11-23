package traffic

import (
	"sync"
	"testing"
	"time"
)

// landingRoll flies an injected arrival of model to LKPR runway end onto
// stand C22 and reports where it touched down, the seconds from touchdown
// to clear of the runway, and the exit taken.
func landingRoll(t *testing.T, model, end string) (touchdown, secs, along float64, exit string) {
	t.Helper()
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	st, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: end, Parking: st, Model: model, Tail: "OKABC",
		InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var td float64 // the event goroutine's, not the named result (read after the unlock)
	go func() {
		for e := range ctl.Events() {
			if e.Touchdown != 0 {
				mu.Lock()
				td = e.Touchdown
				mu.Unlock()
			}
		}
	}()
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	var down, clear time.Time
	for i := 0; i < 60*1200 && ctl.State() < ArrivalAwaitingTaxi; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		if down.IsZero() && ctl.State() == ArrivalRollout {
			down = now
		}
		if clear.IsZero() && ctl.State() == ArrivalVacating {
			clear = now
		}
	}
	if down.IsZero() || clear.IsZero() {
		t.Fatalf("%s: state %v", model, ctl.State())
	}
	mu.Lock()
	touchdown = td
	mu.Unlock()
	return touchdown, clear.Sub(down).Seconds(), p.Exit.Along, p.Exit.Taxiway
}

// lightTouchdownMeters: where a light single's injected landing usually
// touches down (exitLightTouchdownMeters less its spread).
const lightTouchdownMeters = exitLightTouchdownMeters - TouchdownSpreadMeters

// TestLightAircraftLandingRoll: a light single touches down near the
// threshold and turns off at the first exit it can make, clear of the
// runway well before an airliner would be; an airliner keeps its exit.
// LKPR 24's first exit is C, 1305 m in (the C172 rolled 83 s to D before).
// maxSecs is for a light single touching down at its usual point
// (lightTouchdownMeters) or later; touching down earlier, it has that much
// more runway to roll before the same exit, at the mean of its touchdown
// and exit speeds (the rollout's one even deceleration), and is allowed
// that time too (#1609: the P28A on 30 touches down 150–270 m in and
// clears 39 s to 32 s later, about 0.06 s for every metre earlier).
//
// The P28A's 38.5 s (37 before #1651): R is a right-angle exit, about 31 m
// in radius as it is driven, now turned off at ICAO's operational speed and
// code A's turn rate instead of about 0.3 g, 1.3 s and 0.9 s more at the
// same touchdown. ICAO Doc 9157 Part 2 (5th ed., Amendment 1, 18/6/21;
// https://www.bazl.admin.ch/dam/en/sd-web/h7SYVJZ5kRTq/9157_p2_cons_en.pdf):
//   - 1.2.22: "it is assumed that the radii of curvature are equal to
//     one-half of the separation distance indicated in Table 1-1": code
//     letter A, 11.5 m.
//   - 1.2.23: "if it is assumed that the lateral load factor is limited to
//     0.133 g: V = (127.133 × 0.133 R)½ = 4.1120(R½), where R is in
//     metres" (km/h): 13.9 km/h on 11.5 m, 19.4°/s (plannedTurnDegPerSec).
//   - Table 1-8, "Aircraft speed versus the radius of a rapid exit taxiway"
//     (R m: Vdes kt / Vop kt): 40: 14/13, 60: 17/16, 120: 24/22, 160:
//     28/24; "Based on the design exit speed Vdes complying with a lateral
//     acceleration of 0.133 g, the operational turn-off speed Vop is
//     determined empirically": Vop²/R is 1.1 m/s² (ExitLateralAccel), 11.4
//     kt on 31 m.
func TestLightAircraftLandingRoll(t *testing.T) {
	for _, c := range []struct {
		model, end, exit string
		maxSecs          float64
	}{
		{"C172", "24", "C", 60}, {"C172", "06", "L", 70}, {"P28A", "30", "R", 38.5}, {"A320", "24", "D", 60},
	} {
		td, secs, along, exit := landingRoll(t, c.model, c.end)
		t.Logf("%s on %s: touchdown %.0f m, clear %.1f s later at exit %s (%.0f m along)", c.model, c.end, td, secs, exit, along)
		if exit != c.exit {
			t.Errorf("%s on %s: exit %s, want %s", c.model, c.end, exit, c.exit)
		}
		if td <= 0 {
			t.Errorf("%s on %s: no touchdown seen", c.model, c.end)
		}
		limit := c.maxSecs
		if p := ProfileFor(c.model); c.model != "A320" && td < lightTouchdownMeters {
			mean := (p.Approach.TouchdownKts + p.Rollout.ExitKts) / 2 * ktsToMS
			limit += (lightTouchdownMeters - td) / mean
		}
		if secs > limit {
			t.Errorf("%s on %s: clear %.1f s after touchdown at %.0f m, want ≤ %.1f", c.model, c.end, secs, td, limit)
		}
		if c.model != "A320" && td > exitLightTouchdownMeters {
			t.Errorf("%s: touchdown %.0f m past the threshold, planned by %.0f", c.model, td, exitLightTouchdownMeters)
		}
	}
}
