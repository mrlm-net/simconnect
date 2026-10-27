package traffic

import (
	"testing"
	"time"
)

// TestLineUpBehindRollPast: told to line up at B behind one starting its
// roll from the threshold, a departure lines up only once that one has
// rolled past B; the room before the next arrival counts that wait (live,
// OKSZG at B waited 31 s for CSA813, lined up 75 s, CSA236 sent around
// with it on the runway). Far enough out, it lines up behind it still.
func TestLineUpBehindRollPast(t *testing.T) {
	for _, c := range []struct {
		nm     float64
		lineUp bool
	}{{8, false}, {12, true}} {
		r := NewRunwayController(RunwayControllerOptions{})
		csa := dep("CSA813", "A320", RunwayRolling)
		csa.AlongM, csa.GroundKts = 50, 20
		ok := dep("OKSZG", "PC12", RunwayHoldingShort)
		ok.AlongM, ok.Route = 700, "DOBE4A" // another SID: a 1 min interval
		now := time.Now()
		r.Decide(now, []RunwayUser{csa}) // its roll started 10 s ago: 50 s of the interval left
		got := r.Decide(now.Add(10*time.Second), []RunwayUser{csa, ok, dep("AFR634", "A320", RunwayHoldingShort), final("CSA236", c.nm)})
		if lined := got.LineUpBehindDeparting["OKSZG"] != ""; lined != c.lineUp {
			t.Errorf("arrival %.1f NM out: line up behind %v, want %v (waits %q)", c.nm, got.LineUpBehindDeparting, c.lineUp, got.Waiting["OKSZG"])
		}
	}
}

func TestRollPastIn(t *testing.T) {
	if d := RollPastIn(1300, 20); d < 25*time.Second || d > 40*time.Second {
		t.Errorf("1300 m from 20 kt: %v", d)
	}
	if RollPastIn(-5, 100) != 0 {
		t.Error("passed: want 0")
	}
}
