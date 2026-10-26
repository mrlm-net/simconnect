package traffic

import (
	"testing"
	"time"
)

// TestLineUpLeftEstimatesTheLineUp: holding short of its departure runway,
// LineUpLeft is about the time its line-up then takes, and it counts down
// while it lines up (the tower times a take-off ahead of an arrival on it:
// live, OKJTX lining up from B for 66–83 s, counted at 30).
func TestLineUpLeftEstimatesTheLineUp(t *testing.T) {
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
	est := ctl.LineUpLeft()
	cleared := *now
	ctl.ClearToLineUp()
	if !run(TaxiLiningUp, 60*120) {
		t.Fatalf("state %v, want lining up", ctl.State())
	}
	first := ctl.LineUpLeft()
	if !run(TaxiLinedUp, 60*600) {
		t.Fatalf("state %v, want lined up", ctl.State())
	}
	took := now.Sub(cleared)
	t.Logf("estimated %v at the holding point, %v once moving; lined up in %v", est.Round(time.Second), first.Round(time.Second), took.Round(time.Second))
	if est <= 0 || est < took*2/3 || est > took*3/2 {
		t.Errorf("estimated %v at the holding point, lined up in %v", est, took)
	}
	if first <= 0 || first > est {
		t.Errorf("lining up: %v left, %v estimated before", first, est)
	}
	if left := ctl.LineUpLeft(); left != 0 {
		t.Errorf("lined up: %v left", left)
	}
}
