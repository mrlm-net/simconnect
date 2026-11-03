package traffic

import (
	"testing"
	"time"
)

// TestSequencerGoAroundDropsFollowers: a VFR arrival told to follow an
// arrival that then goes around is no longer held behind it (live, LKPR:
// OKRDE kept behind OKXNU round its go-around, 21 minutes of delay, and the
// order swung: DFVKB told 2, 3, 1, 3).
func TestSequencerGoAroundDropsFollowers(t *testing.T) {
	now := time.Date(2026, 11, 3, 13, 22, 0, 0, time.UTC)
	s := NewApproachSequencer("06", SequencerOptions{})
	lead := ApproachAircraft{Callsign: "OKXNU", Wake: WakeFor("A320"), DistanceToGoNM: 4, GroundKts: 140, FinalKts: 130}
	vfr := ApproachAircraft{Callsign: "OKRDE", Wake: WakeFor("DA62"), DistanceToGoNM: 9, GroundKts: 120, FinalKts: 90}
	jet := ApproachAircraft{Callsign: "DFVKB", Wake: WakeFor("A320"), DistanceToGoNM: 20, GroundKts: 220, FinalKts: 140}
	s.Update(now, []ApproachAircraft{lead, vfr, jet})
	s.Behind("OKRDE", "OKXNU")
	// Gone around: 34 NM of its circuit to fly again.
	s.Rejoin("OKXNU")
	lead.DistanceToGoNM, lead.GroundKts = 34, 160
	vfr.DistanceToGoNM = 7
	jet.DistanceToGoNM = 17
	var seq []SequenceEntry
	for i := range 10 {
		seq = s.Update(now.Add(time.Duration(i+1)*5*time.Second), []ApproachAircraft{lead, vfr, jet})
	}
	for _, e := range seq {
		if e.Callsign == "OKRDE" {
			if e.Number != 1 || e.Delay > 2*time.Minute {
				t.Errorf("OKRDE after OKXNU's go-around: number %d, delay %s, want number 1 without a long delay (%+v)", e.Number, e.Delay, seq)
			}
		}
	}
	if seq[len(seq)-1].Callsign != "OKXNU" {
		t.Errorf("gone around with 34 NM to go: %s last, want OKXNU", seq[len(seq)-1].Callsign)
	}
}
