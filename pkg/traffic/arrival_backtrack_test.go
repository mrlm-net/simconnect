package traffic

import (
	"testing"
	"time"
)

// An arrival that will backtrack along the runway after landing (#1563)
// comes back toward the departures' end: none is lined up behind it.
func TestRunwayControllerNoLineUpBehindBacktrack(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	arr := final("DLH2", 3)
	arr.Backtracks = true
	c := r.Decide(time.Now(), []RunwayUser{dep("CSA1", "A320", RunwayHoldingShort), arr})
	if c.LineUpBehind["CSA1"] != "" {
		t.Fatalf("lined up behind a backtracking arrival: %+v", c)
	}
	// Rolling or backtracking, it has the runway.
	arr.Phase, arr.DistanceNM = RunwayRolling, 0
	c = r.Decide(time.Now(), []RunwayUser{dep("CSA1", "A320", RunwayHoldingShort), arr})
	if len(c.LineUp) != 0 || len(c.Takeoff) != 0 {
		t.Fatalf("cleared with a backtracking arrival on the runway: %+v", c)
	}
}

// The arrival behind one that keeps the runway for a backtrack is spaced
// for that time, not only for its wake's occupancy.
func TestSequencerBacktrackOccupancy(t *testing.T) {
	s := NewApproachSequencer("10", SequencerOptions{})
	lead := ApproachAircraft{Callsign: "AAL1", Wake: WakeFor("A320"), FinalKts: 140}
	follow := ApproachAircraft{Callsign: "AAL2", Wake: WakeFor("A320"), FinalKts: 140}
	plain, _, _ := s.gap(lead, follow, ApproachConditions{})
	lead.RunwayOccupancy = RunwayOccupancyIn(lead.Wake, true, RunwayDry) + BacktrackOccupancy(1400)
	back, _, why := s.gap(lead, follow, ApproachConditions{})
	if back < lead.RunwayOccupancy || back <= plain || why != "runway occupancy" {
		t.Errorf("behind a 1400 m backtrack: %s (%q), plain %s, occupancy %s", back, why, plain, lead.RunwayOccupancy)
	}
	if BacktrackOccupancy(0) != 0 {
		t.Error("no backtrack, no extra time")
	}
}
