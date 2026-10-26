package traffic

import "testing"

// A departure gap behind a slower leader is set up wider by the closing
// the speeds make, as the normal spacing is: it must still be there at the
// threshold (live, CSA814 6.3 NM behind a Citation at 110 kt down to
// 4.6 NM at 7 NM to go, sent around).
func TestDepartureGapCompression(t *testing.T) {
	s := NewApproachSequencer("24", SequencerOptions{})
	a320 := ApproachAircraft{Callsign: "CSA814", Wake: Wake{WakeMedium, RecatD}, FinalKts: 140}
	same := ApproachAircraft{Callsign: "CSA1", Wake: Wake{WakeMedium, RecatD}, FinalKts: 140}
	citation := ApproachAircraft{Callsign: "OKXIT", Wake: Wake{WakeLight, RecatF}, FinalKts: 110}
	_, even := s.departureGap(same, a320, ApproachConditions{}, 1)
	_, slow := s.departureGap(citation, a320, ApproachConditions{}, 1)
	cmp := s.compressionNM(citation, a320)
	if want := DefaultDepartureGapNM + cmp; cmp < 0.5 || slow < want-0.05 || slow <= even {
		t.Errorf("departure gap behind the Citation %.1f NM (closing %.1f), behind an A320 %.1f: want %.1f and more than behind the A320", slow, cmp, even, want)
	}
}

// TestDepartureGapUnderTenMiles: a gap for one departure between two
// A320s is under 8.5 NM on final, not 10: the tower lets the departure go
// with the arrival still LandingClearanceNM (2 NM) out once airborne; the
// rest is its line-up behind the leader (LineUpBehindReady)
// (offline LKPR: 10 NM gaps before nearly every arrival held the runway to
// 14 arrivals an hour, every arrival 15–25 min late at 10 an hour).
func TestDepartureGapUnderTenMiles(t *testing.T) {
	s := NewApproachSequencer("24", SequencerOptions{})
	a := ApproachAircraft{Callsign: "CSA1", Wake: Wake{WakeMedium, RecatD}, FinalKts: 140}
	b := ApproachAircraft{Callsign: "CSA2", Wake: Wake{WakeMedium, RecatD}, FinalKts: 140}
	if _, nm := s.departureGap(a, b, ApproachConditions{}, 1); nm > 8.5 {
		t.Errorf("departure gap between two A320s %.1f NM", nm)
	}
}
