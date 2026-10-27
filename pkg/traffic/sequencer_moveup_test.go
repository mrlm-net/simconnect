package traffic

import (
	"testing"
	"time"
)

// TestSequencerMoveUpNearFinal: a slow VFR arrival back on a short
// downwind is not moved up past a jet near its final, whose delay it would
// make more than the jet can still lose (live, OKJRU past DLH792 after a
// TCAS RA: DLH792 given 4m20s at 10 NM, sent around): on its downwind it
// is not fixed inside FreezeNM, it can extend it. Far out, the jet
// can lose it: the move stands.
func TestSequencerMoveUpNearFinal(t *testing.T) {
	for _, c := range []struct {
		jetNM float64
		ahead string
	}{{14, "DLH792"}, {45, "OKJRU"}} {
		s := NewApproachSequencer("24", SequencerOptions{})
		now := time.Now()
		vfr := arr("OKJRU", "C172", c.jetNM+20)
		vfr.GroundKts, vfr.FinalKts = 100, 70
		s.Update(now, []ApproachAircraft{arr("DLH792", "A320", c.jetNM), vfr})
		// Back on a short downwind: predicted well before the jet.
		vfr.DistanceToGoNM, vfr.Extendable = 4, true
		seq := s.Update(now.Add(time.Second), []ApproachAircraft{arr("DLH792", "A320", c.jetNM-0.1), vfr})
		if seq[0].Callsign != c.ahead {
			t.Errorf("jet %.0f NM to go: %s first (delay %v / %v), want %s", c.jetNM, seq[0].Callsign, seq[0].Delay, seq[1].Delay, c.ahead)
		}
	}
}
