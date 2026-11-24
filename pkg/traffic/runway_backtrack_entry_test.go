package traffic

import (
	"slices"
	"testing"
	"time"
)

// TestBacktrackEntersOutsideLandingWindow (#1680): a departure that
// backtracks to its threshold is on the landing path the whole way, so it
// is not told to line up with an arrival inside the landing clearance's
// window (ClearToLandNM, 6 NM); it waits at its holding point. Outside it, and a
// departure lining up at its threshold, as before.
func TestBacktrackEntersOutsideLandingWindow(t *testing.T) {
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name  string
		back  bool
		nm    float64
		enter bool
	}{
		{"backtrack, arrival inside the window", true, 5.5, false},
		{"backtrack, arrival at the window's edge", true, 5.9, false},
		{"backtrack, arrival outside the window", true, 7.5, true},
		{"at the threshold, arrival at 5.5 NM", false, 5.5, true},
	} {
		got := NewRunwayController(RunwayControllerOptions{}).Decide(now, []RunwayUser{
			{Callsign: "DEP1", Phase: RunwayHoldingShort, End: "10", Backtracks: c.back, LineUpIn: 40 * time.Second},
			{Callsign: "ARR1", Phase: RunwayFinal, Arrival: true, End: "10", DistanceNM: c.nm, GroundKts: 90, Established: true},
		})
		entered := slices.Contains(got.LineUp, "DEP1") || slices.Contains(got.Takeoff, "DEP1") || got.LineUpBehind["DEP1"] != "" || got.LineUpBehindDeparting["DEP1"] != ""
		if entered != c.enter {
			t.Errorf("%s: entered %v, want %v (waiting: %q)", c.name, entered, c.enter, got.Waiting["DEP1"])
		}
	}
}
