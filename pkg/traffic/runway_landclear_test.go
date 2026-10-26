package traffic

import (
	"slices"
	"testing"
	"time"
)

// TestTakeoffLeavesTheArrivalItsLandingClearance: a take-off ahead of an
// arrival goes only when the runway is free with the arrival still
// LandingClearanceNM out (live LKPR: OKJTX cleared with CSA1312 close
// behind, its landing clearance late), one still lining up counted too.
func TestTakeoffLeavesTheArrivalItsLandingClearance(t *testing.T) {
	rot := RunwayOccupancyIn(WakeFor("A320"), false, RunwayDry)
	nm := func(d time.Duration) float64 { return d.Hours() * 140 }

	r := NewRunwayController(RunwayControllerOptions{})
	tight := nm(rot + 35*time.Second)
	c := r.Decide(time.Now(), []RunwayUser{dep("CSA1", "A320", RunwayLinedUp), final("KLM1", tight)})
	if slices.Contains(c.Takeoff, "CSA1") {
		t.Fatalf("take-off with the arrival %.1f NM out leaves it under 3 NM at the clear", tight)
	}
	room := nm(takeoffSaidIn + rot + SeparationTime(3, 140) + 5*time.Second)
	c = r.Decide(time.Now(), []RunwayUser{dep("CSA1", "A320", RunwayLinedUp), final("KLM1", room)})
	if !slices.Contains(c.Takeoff, "CSA1") {
		t.Fatalf("not cleared with the arrival %.1f NM out: %s", room, c.Waiting["CSA1"])
	}

	r = NewRunwayController(RunwayControllerOptions{})
	ok := dep("OKJTX", "A320", RunwayLinedUp)
	ok.LiningUp = true
	c = r.Decide(time.Now(), []RunwayUser{ok, final("CSA1312", 5)})
	if slices.Contains(c.Takeoff, "OKJTX") {
		t.Fatal("one still lining up cleared for take-off with the arrival 5 NM out")
	}
}
