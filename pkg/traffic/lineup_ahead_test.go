package traffic

import (
	"testing"
	"time"
)

// TestLineUpAheadIntersection: behind one rolling from an intersection
// down the runway from its own way in, a departure at the threshold is
// told to line up with no condition: the one rolling never passes in front
// of it (Doc 4444 12.2.7; live, TVS737 at the threshold of 24 told "behind
// the departing Cirrus SR22" rolling from B). Behind one rolling from the
// same way in, still conditional.
func TestLineUpAheadIntersection(t *testing.T) {
	now := time.Now()
	for _, c := range []struct {
		name  string
		in    float64 // the one rolling: its way in (m down the runway)
		ahead bool
	}{{"from B", 600, true}, {"from the threshold", 0, false}} {
		r := NewRunwayController(RunwayControllerOptions{})
		first := dep("OKDWM", "SR22", RunwayHoldingShort)
		first.AlongM = c.in
		r.Decide(now, []RunwayUser{first})
		first.Phase, first.AlongM = RunwayRolling, c.in+50
		got := r.Decide(now.Add(time.Minute), []RunwayUser{first, dep("TVS737", "B738", RunwayHoldingShort), dep("WZZ3", "A321", RunwayHoldingShort)})
		if got.LineUpBehindDeparting["TVS737"] != "OKDWM" {
			t.Fatalf("%s: line up behind %v, want behind OKDWM", c.name, got.LineUpBehindDeparting)
		}
		if got.LineUpAhead["TVS737"] != c.ahead {
			t.Errorf("%s: ahead %v, want %v", c.name, got.LineUpAhead["TVS737"], c.ahead)
		}
	}
}
