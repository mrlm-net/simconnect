package traffic

import (
	"testing"
	"time"
)

// A departure gap opened while an arrival is near its final, too late for
// it to lose with speed and its path, goes to the next arrival instead,
// which has the room (offline LKPR: THY1537 given 6.8 NM turning base, short
// on the final, sent around).
func TestDepartureGapNotForOneThatCannotLoseIt(t *testing.T) {
	s := NewApproachSequencer("24", SequencerOptions{})
	now := time.Now()
	arrivals := []ApproachAircraft{arr("RYR669", "B738", 8), arr("THY1537", "B739", 13), arr("DLH1775", "A320", 60)}
	s.Update(now, arrivals)
	s.SetDepartureSlots(1) // a departure waits now
	seq := s.Update(now.Add(time.Second), arrivals)
	why := map[string]string{}
	for _, e := range seq {
		why[e.Callsign] = e.SpacingWhy
	}
	if why["THY1537"] == "departure gap" {
		t.Errorf("THY1537, 3 NM from its final, given the departure gap")
	}
	if why["DLH1775"] != "departure gap" {
		t.Errorf("DLH1775 (%q): want the departure gap", why["DLH1775"])
	}
}
