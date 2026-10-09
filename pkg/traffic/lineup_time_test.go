package traffic

import (
	"testing"
	"time"
)

// How long a departure cleared for take-off at the holding point takes to
// start its roll (the tower's ImmediateLineUpTime and LineUpTime rest on it).
func TestLineUpTimeFromHoldingPoint(t *testing.T) {
	for _, c := range []struct {
		kts       float64
		expedite  bool
	}{{LineUpSpeedKts, false}, {LineUpRollingKts, false}, {LineUpRollingKts, true}} {
		kts := c.kts
		old := LineUpRollingKts
		LineUpRollingKts = kts
		ctl, _, run, now := injectedDeparture(t, TaxiRequest{HoldForClearances: true})
		for _, gate := range []struct {
			state TaxiState
			clear func()
		}{{TaxiAwaitingPushback, ctl.ClearPushback}, {TaxiAwaitingTaxi, ctl.ClearToTaxi}} {
			if !run(gate.state, 60*900) {
				t.Fatalf("state %v", ctl.State())
			}
			gate.clear()
		}
		if !run(TaxiHoldingShort, 60*900) {
			t.Fatalf("state %v, want holding short", ctl.State())
		}
		cleared := *now
		ctl.Expedite(c.expedite)
		ctl.ClearForTakeoff()
		if !run(TaxiDeparting, 60*600) {
			t.Fatalf("state %v, want departing", ctl.State())
		}
		roll := now.Sub(cleared)
		LineUpRollingKts = old
		t.Logf("alignment at %.0f kt, expedite %v: cleared at the holding point → take-off roll in %v", kts, c.expedite, roll.Round(time.Second))
		if c.kts == LineUpRollingKts && roll > 60*time.Second { // the tower's LineUpTime
			t.Errorf("rolling take-off starts %v after the clearance", roll)
		}
	}
}
