package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestVFRNearMiss: VFR traffic not separated in class D is still kept
// apart when it would nearly meet (live, OKFVQ and OKJRU: predicted 0.6
// NM, 223 ft; TCAS RA at 0.8 NM, 436 ft); passing well clear, traffic
// information only.
func TestVFRNearMiss(t *testing.T) {
	for _, c := range []struct {
		nm, ft float64
		want   bool
	}{{0.6, 223, true}, {0.6, 800, false}, {2.9, 200, false}} {
		if got := vfrNearMiss(traffic.Conflict{ClosestNM: c.nm, VerticalFt: c.ft}); got != c.want {
			t.Errorf("closest %.1f NM, %.0f ft: near miss %v, want %v", c.nm, c.ft, got, c.want)
		}
	}
}
