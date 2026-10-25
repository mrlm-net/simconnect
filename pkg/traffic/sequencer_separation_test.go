package traffic

import (
	"testing"
	"time"
)

// A unit keeping 5 NM on final (MinSpacingNM) plans with it, but the
// separation minimum behind the leader stays the wake or radar one: the
// spacing go-around guards that (offline LKPR: THY1537 sent around 4.4 NM
// behind DLH1775 at 7.2 NM to go for the 5 NM; the minimum was 3).
func TestSequencerSeparationMinimumBelowUnitSpacing(t *testing.T) {
	s := NewApproachSequencer("24", SequencerOptions{MinSpacingNM: 5})
	seq := s.Update(time.Now(), []ApproachAircraft{arr("DLH1775", "A320", 10), arr("THY1537", "A320", 14)})
	if len(seq) != 2 {
		t.Fatalf("sequence %+v", seq)
	}
	if e := seq[1]; e.MinimumNM != 5 || e.SeparationNM != MinRadarSeparationNM {
		t.Errorf("THY1537: minimum %.1f NM, separation %.1f NM; want 5 (the unit's) and %.1f (radar)", e.MinimumNM, e.SeparationNM, MinRadarSeparationNM)
	}
}
