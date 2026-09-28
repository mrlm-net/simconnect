//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
)

// TestTakeoffMover: an A320 take-off from a standing start rotates at Vr,
// lifts off within a typical distance and climbs smoothly.
func TestTakeoffMover(t *testing.T) {
	p := DefaultTakeoffProfile()
	m := NewTakeoffMover(lkpr, 244, 0, p)
	var liftoff TakeoffPose
	prevVS, maxVSStep := 0.0, 0.0
	for i := 0; i < 60*90; i++ {
		pose := m.Step(1.0 / 60)
		maxVSStep = math.Max(maxVSStep, math.Abs(pose.VerticalFpm-prevVS))
		prevVS = pose.VerticalFpm
		if pose.Phase == TakeoffAirborne && liftoff.Phase != TakeoffAirborne {
			liftoff = pose
		}
		if pose.HeightFt > 1500 {
			break
		}
	}
	if liftoff.Phase != TakeoffAirborne {
		t.Fatal("never lifted off")
	}
	if liftoff.LiftoffDistance < 1000 || liftoff.LiftoffDistance > 2200 {
		t.Errorf("lift-off after %.0f m, want an A320-like 1000-2200 m", liftoff.LiftoffDistance)
	}
	if liftoff.GroundSpeedKts < p.RotateKts || liftoff.GroundSpeedKts > p.RotateKts+15 {
		t.Errorf("lift-off at %.0f kt", liftoff.GroundSpeedKts)
	}
	final := m.Pose()
	if final.HeightFt < 1500 || math.Abs(final.VerticalFpm-p.ClimbFpm) > 1 || final.PitchDeg != p.ClimbPitch {
		t.Errorf("climb-out %+v", final)
	}
	if maxVSStep > 60 { // fpm per frame: no jolt into the climb
		t.Errorf("vertical speed jumps %.0f fpm between frames", maxVSStep)
	}
	if hd := headingDiff(final.Heading, 244); math.Abs(hd) > 1e-9 {
		t.Errorf("heading %.1f", final.Heading)
	}
	t.Logf("lift-off %.0f m at %.0f kt; 1500 ft after %.0f m at %.0f kt", liftoff.LiftoffDistance, liftoff.GroundSpeedKts, final.Distance, final.GroundSpeedKts)
}
