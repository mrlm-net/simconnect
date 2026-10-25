package traffic

import (
	"testing"
	"time"
)

// An arrival missing from the arrivals for a look (a handover) keeps its
// place: back, it is where it was, not a newcomer at the back (offline
// LKPR: TVS993 out of the sequence and back as number 6, the order behind
// it turned over every pass). Gone for longer than GoneGrace, it is out.
func TestSequencerKeepsPlaceOfOneMissingForALook(t *testing.T) {
	var gone []string
	s := NewApproachSequencer("24", SequencerOptions{OnChange: func(c SequenceChange) {
		if c.Gone {
			gone = append(gone, c.Entry.Callsign)
		}
	}})
	now := time.Now()
	s.Update(now, []ApproachAircraft{arr("A1", "A320", 40), arr("B2", "A320", 41), arr("C3", "A320", 42)})
	s.Update(now.Add(5*time.Second), []ApproachAircraft{arr("A1", "A320", 39.7), arr("C3", "A320", 41.7)})
	// Back, its way now longer (the handover's route): behind C3 by its
	// prediction, ahead of it by its place.
	seq := s.Update(now.Add(10*time.Second), []ApproachAircraft{arr("A1", "A320", 39.4), arr("B2", "A320", 44), arr("C3", "A320", 41.4)})
	order := ""
	for _, e := range seq {
		order += e.Callsign + " "
	}
	if order != "A1 B2 C3 " {
		t.Errorf("order %s, want B2 back in its place (A1 B2 C3)", order)
	}
	if len(gone) != 0 {
		t.Errorf("reported out of the sequence: %v", gone)
	}
	// Gone for good: out once GoneGrace has passed.
	s.Update(now.Add(20*time.Second), []ApproachAircraft{arr("A1", "A320", 38), arr("C3", "A320", 40)})
	s.Update(now.Add(20*time.Second+3*time.Minute), []ApproachAircraft{arr("A1", "A320", 10), arr("C3", "A320", 12)})
	if len(gone) != 1 || gone[0] != "B2" {
		t.Errorf("gone %v, want B2 out after GoneGrace", gone)
	}
}
