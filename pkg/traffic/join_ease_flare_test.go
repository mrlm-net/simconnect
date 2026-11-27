package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestJoinEaseByFlare (#1682): an injected approach taken over fast, as
// on a direct-to-final join close in, eases to its scheduled speed by
// JoinEaseByFt above the runway, never changing faster than
// JoinEaseMaxKtsPerSecond: it does not reach the flare fast.
func TestJoinEaseByFlare(t *testing.T) {
	for _, tc := range []struct {
		model   string
		nm, kts float64
	}{
		{"FSLTL A320 Air France SL", 3, 220},
		{"FSLTL A320 Air France SL", 8, 250},
		{"FSLTL_DHC6_WIA", 3, 140},
	} {
		p := withDefaults(ProfileFor(tc.model).Approach, DefaultApproachProfile())
		m := NewApproachMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 244, tc.nm*1852, p)
		m.EaseFrom(tc.kts)
		const dt = 1.0 / 60
		prev, checked := m.Pose().GroundSpeedKts, false
		for i := 0; i < 60*900 && m.Pose().Phase == ApproachFinal; i++ {
			pose := m.Step(dt)
			if r := math.Abs(pose.GroundSpeedKts-prev) / dt; r > JoinEaseMaxKtsPerSecond+0.01 {
				t.Fatalf("%s %.0f NM at %.0f kt: %.2f kt/s at %.0f ft", tc.model, tc.nm, tc.kts, r, pose.HeightFt)
			}
			prev = pose.GroundSpeedKts
			if !checked && pose.HeightFt <= JoinEaseByFt {
				checked = true
				if want := m.speedAt(m.x) / ktsToMS; math.Abs(pose.GroundSpeedKts-want) > 1 {
					t.Errorf("%s %.0f NM at %.0f kt: %.0f kt at %.0f ft, scheduled %.0f", tc.model, tc.nm, tc.kts, pose.GroundSpeedKts, pose.HeightFt, want)
				}
			}
		}
		if !checked {
			t.Errorf("%s: never down to %.0f ft on the final", tc.model, JoinEaseByFt)
		}
	}
}
