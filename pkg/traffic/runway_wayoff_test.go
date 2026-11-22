package traffic

import (
	"slices"
	"testing"
	"time"
)

// TestWayOffBlockedDepartureFirst (#1640): an arrival with no way off clear
// of departures is not cleared to land and goes around while it still can;
// a player's (Other) is left to the player (#1096).
func TestWayOffBlockedDepartureFirst(t *testing.T) {
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name           string
		nm             float64
		blocked, other bool
		land, around   bool
	}{
		{"free", 2, false, false, true, false},
		{"blocked", 2, true, false, false, false},
		{"blocked on short final", 0.4, true, false, false, true},
		{"a player's", 0.4, true, true, false, false},
	} {
		got := NewRunwayController(RunwayControllerOptions{}).Decide(now, []RunwayUser{
			{Callsign: "DEP1", Phase: RunwayHoldingShort, End: "10"},
			{Callsign: "ARR1", Phase: RunwayFinal, Arrival: true, End: "10", DistanceNM: c.nm, GroundKts: 90, Established: true, Backtracks: true, WayOffBlocked: c.blocked, Other: c.other},
		})
		if land := slices.Contains(got.Land, "ARR1"); land != c.land {
			t.Errorf("%s: cleared to land %v, want %v", c.name, land, c.land)
		}
		if around := slices.Contains(got.GoAround, "ARR1"); around != c.around {
			t.Errorf("%s: sent around %v, want %v", c.name, around, c.around)
		}
	}
}
