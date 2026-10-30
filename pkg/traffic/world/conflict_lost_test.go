package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestSeparationLost: not separated where they are, only a near miss is
// reported (#1314).
func TestSeparationLost(t *testing.T) {
	for _, tc := range []struct {
		needed bool
		nm, ft float64
		want   bool
	}{
		{true, 3, 1000, true},
		{false, 3, 1001, false},
		{false, 1.0, 576, false},
		{false, 0.8, 436, true},
	} {
		if got := separationLost(tc.needed, traffic.Conflict{ClosestNM: tc.nm, VerticalFt: tc.ft}); got != tc.want {
			t.Errorf("needed %v, %.1f NM, %.0f ft: %v", tc.needed, tc.nm, tc.ft, got)
		}
	}
}
