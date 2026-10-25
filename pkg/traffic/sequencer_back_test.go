package traffic

import (
	"testing"
	"time"
)

// One back after a look missing (a reroute) is not a newcomer to the merge
// rule: the one behind took its number for that look, but it was ahead
// and stays ahead (offline LKPR: TVS993, slowed for its slot, back after
// a look with TVS1137 in its number 5, was put behind it at their merge
// fix and its key reset; TVS1137 went around twice).
func TestSequencerBackAfterALookKeepsItsPlaceAtTheMerge(t *testing.T) {
	s := NewApproachSequencer("24", SequencerOptions{})
	now := time.Now()
	at := func(cs string, nm, fix float64) ApproachAircraft {
		a := arr(cs, "A320", nm)
		a.Fixes = []FixAhead{{Name: "MERGE", NM: fix}}
		return a
	}
	s.Update(now, []ApproachAircraft{at("A1", 60, 30), at("B2", 62, 32)})
	s.Update(now.Add(time.Second), []ApproachAircraft{at("A1", 60, 30), at("B2", 62, 32)})
	s.mu.Lock()
	keyA, keyB := s.keys["A1"], s.keys["B2"]
	s.mu.Unlock()
	s.Update(now.Add(2*time.Second), []ApproachAircraft{at("B2", 61.9, 31.9)})
	seq := s.Update(now.Add(3*time.Second), []ApproachAircraft{at("A1", 61, 31), at("B2", 61.8, 31.8)})
	order := ""
	for _, e := range seq {
		order += e.Callsign + " "
	}
	if order != "A1 B2 " {
		t.Errorf("order %s, want A1 back ahead of B2 (A1 B2)", order)
	}
	// Their keys (first come, first served) untouched: no swapping behind
	// each other at the merge fix by turns.
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.keys["A1"].Equal(keyA) || !s.keys["B2"].Equal(keyB) {
		t.Errorf("keys moved: A1 %v, B2 %v after the look missing", s.keys["A1"].Sub(keyA), s.keys["B2"].Sub(keyB))
	}
}
