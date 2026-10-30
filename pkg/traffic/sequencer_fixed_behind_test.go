package traffic

import (
	"testing"
	"time"
)

// TestSequencerFixedKeepsBehind: a VFR circuit arrival fixed at its own
// time (its downwind extended as far as it goes) stays behind the jet
// that was ahead of it when the jet is too near its final to lose the
// time; the ones behind take the rest, not the one ahead (live, LKPR:
// OKYQK fixed on its downwind took number 2 from CSA1293 12 NM out, given
// 6 minutes; short on the final, sent around). Far out, the jet can lose
// it: the circuit lands first.
func TestSequencerFixedKeepsBehind(t *testing.T) {
	for _, c := range []struct {
		jetNM float64
		ahead string
	}{{13, "CSA1293"}, {45, "OKYQK"}} {
		s := NewApproachSequencer("24", SequencerOptions{})
		now := time.Now()
		vfr := arr("OKYQK", "DA62", c.jetNM+20)
		vfr.GroundKts, vfr.FinalKts = 120, 90
		s.Update(now, []ApproachAircraft{arr("CSA1293", "A320", c.jetNM), vfr})
		// On its downwind, fixed at its own time: predicted before the jet.
		vfr.DistanceToGoNM, vfr.Fixed = 5.5, true
		seq := s.Update(now.Add(time.Second), []ApproachAircraft{arr("CSA1293", "A320", c.jetNM-0.1), vfr})
		if seq[0].Callsign != c.ahead {
			t.Errorf("jet %.0f NM to go: %s first (delay %v / %v), want %s", c.jetNM, seq[0].Callsign, seq[0].Delay, seq[1].Delay, c.ahead)
		}
	}
}
