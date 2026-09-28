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
	if liftoff.LiftoffDistance < 1200 || liftoff.LiftoffDistance > 1700 {
		t.Errorf("lift-off after %.0f m, want an A320-like 1200-1700 m", liftoff.LiftoffDistance)
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

// TestTakeoffNoTailstrike: a 777-300 (tail strike at 8.5°) lifts off with a
// small pull, holds that pitch until a positive climb and only then pitches
// up to its climb pitch, never faster than the tail clears the runway.
func TestTakeoffNoTailstrike(t *testing.T) {
	p := TakeoffProfileFor("FSLTL B77W Emirates")
	if p.TailstrikePitch != 8.5 {
		t.Fatalf("777-300 profile %+v", p)
	}
	ground := p.TailstrikePitch - TailstrikeMarginDeg
	m := NewTakeoffMover(lkpr, 244, 0, p)
	var liftPitch float64
	for i := 0; i < 60*120; i++ {
		pose := m.Step(1.0 / 60)
		switch {
		case pose.Phase != TakeoffAirborne:
			if pose.PitchDeg > ground+1e-9 {
				t.Fatalf("pitch %.2f° on the runway, tail strikes at %.1f°", pose.PitchDeg, p.TailstrikePitch)
			}
		case liftPitch == 0:
			liftPitch = pose.PitchDeg
		case pose.HeightFt < PositiveClimbFt && pose.PitchDeg != liftPitch:
			t.Fatalf("pitch %.2f° at %.0f ft, before a positive climb", pose.PitchDeg, pose.HeightFt)
		case pose.PitchDeg > ground+pose.HeightFt/TailClearFtPerDeg+1e-9:
			t.Fatalf("pitch %.2f° at %.1f ft", pose.PitchDeg, pose.HeightFt)
		}
		if pose.HeightFt > 1500 {
			break
		}
	}
	if liftPitch == 0 || liftPitch > ground+1e-9 {
		t.Errorf("lift-off pitch %.2f°", liftPitch)
	}
	if final := m.Pose(); final.PitchDeg != p.ClimbPitch {
		t.Errorf("climb pitch %.1f°, want %.1f°", final.PitchDeg, p.ClimbPitch)
	}
}

func TestTakeoffProfileFor(t *testing.T) {
	for _, c := range []struct {
		model string
		tail  float64
	}{
		{"FSLTL B77W Emirates", 8.5},
		{"Boeing 777-300ER", 8.5},
		{"FSLTL B772 British Airways", 10.5},
		{"FSLTL A21N BAW British Airways", 9.5},
		{"FSLTL A20N MBU Marabu Airlines", 11.5},
		{"AIB_B738_BAW-British Airways", 11},
		{"FSLTL A320 Air France SL", 11.5},
		{"Something unknown", 11.5},
	} {
		if got := TakeoffProfileFor(c.model).TailstrikePitch; got != c.tail {
			t.Errorf("%s: tail strike %.1f°, want %.1f°", c.model, got, c.tail)
		}
	}
}

func TestMotionProfileFor(t *testing.T) {
	a320 := DefaultMotionProfile()
	for _, c := range []struct {
		model     string
		wheelbase float64
	}{
		{"Asobo PassiveAircraft B777-300ER :: B777_300ER_KLM", 31.2},
		{"FSLTL A21N BAW British Airways", 16.9},
		{"AIB_B738_BAW-British Airways", 15.6},
		{"FSLTL A320 Air France SL", a320.WheelbaseMeters},
	} {
		p := MotionProfileFor(c.model)
		if p.WheelbaseMeters != c.wheelbase || p.TailMeters <= 0 || p.SpanMeters <= 0 {
			t.Errorf("%s: %+v", c.model, p)
		}
		if p.CruiseKts != a320.CruiseKts || p.RefAheadMeters != a320.RefAheadMeters {
			t.Errorf("%s: motion figures changed", c.model)
		}
	}
}

func TestMotionProfileForAsoboTitles(t *testing.T) {
	for model, wb := range map[string]float64{
		"Asobo PassiveAircraft B787-09":    25.9,
		"Asobo PassiveAircraft B737-Max8":  15.6,
		"Asobo PassiveAircraft B737-900ER": 17.2,
		"Asobo PassiveAircraft A330-200":   22.2,
		"Asobo PassiveAircraft A321 NEO":   16.9,
	} {
		if got := MotionProfileFor(model).WheelbaseMeters; got != wb {
			t.Errorf("%s: wheelbase %.1f, want %.1f", model, got, wb)
		}
	}
}
