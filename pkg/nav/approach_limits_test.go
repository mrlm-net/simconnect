package nav

import "testing"

// TestApproachForLimits: below VisualApproachVisibilityM or under a ceiling
// below VisualApproachCeilingFt the ILS, else visual (#1526: 1500 ft and
// 5 km advertised visual approaches no crew asked for).
func TestApproachForLimits(t *testing.T) {
	for _, c := range []struct {
		vis, ceil float64
		want      string
	}{
		{9999, 0, ApproachVisual}, {9999, 3000, ApproachVisual}, {8000, 0, ApproachVisual},
		{9999, 2000, ApproachILS}, {6000, 0, ApproachILS}, {9999, 600, ApproachILS}, {0, 0, ApproachVisual},
	} {
		if got := ApproachFor(Weather{VisibilityM: c.vis, CeilingFt: c.ceil}); got != c.want {
			t.Errorf("%.0f m, ceiling %.0f ft: %s, want %s", c.vis, c.ceil, got, c.want)
		}
	}
}
