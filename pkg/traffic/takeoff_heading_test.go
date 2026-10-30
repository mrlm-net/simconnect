package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTakeoffStraightensOut: a roll starting 5° off the runway heading starts
// on the aircraft's own heading, turns no faster than a rolling aircraft does
// and is on the runway heading by takeoffAlignMeters; across north too.
func TestTakeoffStraightensOut(t *testing.T) {
	for _, tc := range []struct{ runway, from float64 }{{260, 265}, {260, 255}, {2, 357}} {
		m := NewTakeoffMover(airport.LatLon{Lat: 47, Lon: 11}, tc.runway, 1, ProfileFor("A320").Takeoff)
		m.FromHeading(tc.from)
		was := m.Pose().Heading
		if math.Abs(headingDiff(was, tc.from)) > 0.01 {
			t.Errorf("%v from %v: starts facing %.2f", tc.runway, tc.from, was)
		}
		const dt = 1.0 / 60
		far := 0.0
		for i := 0; i < 60*20; i++ {
			p := m.Step(dt)
			if d := math.Abs(headingDiff(p.Heading, was)); d/dt > 8 {
				t.Fatalf("%v from %v: %.1f°/s at %.0f m", tc.runway, tc.from, d/dt, p.Distance)
			}
			was, far = p.Heading, p.Distance
			if p.Distance >= takeoffAlignMeters && math.Abs(headingDiff(p.Heading, tc.runway)) > 0.01 {
				t.Fatalf("%v from %v: faces %.2f at %.0f m", tc.runway, tc.from, p.Heading, p.Distance)
			}
		}
		if far < takeoffAlignMeters {
			t.Errorf("%v from %v: rolled only %.0f m", tc.runway, tc.from, far)
		}
	}
}
