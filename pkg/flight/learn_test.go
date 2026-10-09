package flight

import (
	"math"
	"testing"
)

// syntheticFlight: rotation at 140 kt to 15° (2.5°/s), lift-off at 148,
// gear up at 150 ft, flaps 2→1 at 180 kt and 1→0 at 210 above 1500 ft;
// on the approach (the speed by height) flaps 1 at 205, 2 at 185, gear down at 2000 ft, 3,
// full at 150, final at 140 kt and 2.5°, flare from 30 ft to 5°, touchdown
// at −150 ft/min. A sample every 0.2 s; the ground at 300 ft, CG 9 ft.
func syntheticFlight() *Track {
	tr := &Track{Model: "A320"}
	t := 0.0
	add := func(s Sample) {
		s.T, s.GroundFt, s.CGFt, s.EngineCount = t, 300, 9, 2
		s.AltFt += 309 // AGL given
		tr.Samples = append(tr.Samples, s)
		t += 0.2
	}
	// Take-off roll and rotation.
	ias, pitch, h, flaps := 0.0, 0.5, 0.0, 2
	gear := true
	for ias < 148 || pitch < 8 {
		if ias >= 140 {
			pitch = math.Min(15, pitch+0.5)
		}
		ias += 0.6
		add(Sample{IAS: ias, Pitch: pitch, OnGround: true, GearHandle: true, FlapsIndex: flaps})
	}
	// Climb-out.
	for h < 3000 {
		h += 2000 * 0.2 / 60
		pitch = math.Min(15, pitch+0.5)
		if h > 150 {
			gear = false
		}
		if h > 1500 {
			ias = math.Min(220, ias+0.4)
		}
		if flaps == 2 && h > 1500 && ias >= 180 {
			flaps = 1
		}
		if flaps == 1 && ias >= 210 {
			flaps = 0
		}
		add(Sample{IAS: ias, Pitch: pitch, AltFt: h, VS: 2000, GearHandle: gear, FlapsIndex: flaps})
	}
	// The approach from 9000 ft.
	h, ias, pitch = 9000, 250, 2.5
	for h > 0 {
		h -= 1000 * 0.2 / 60
		ias = math.Max(140, 140+(h-1000)/70)
		switch {
		case flaps == 0 && ias <= 205:
			flaps = 1
		case flaps == 1 && ias <= 185:
			flaps = 2
		case flaps == 2 && ias <= 170 && gear:
			flaps = 3
		case flaps == 3 && ias <= 150:
			flaps = 4
		}
		if h <= 2000 {
			gear = true
		}
		vs := -1000.0
		if h < 30 {
			pitch = math.Min(5, pitch+0.3)
			vs = -150
		}
		add(Sample{IAS: ias, Pitch: pitch, AltFt: math.Max(h, 0), VS: vs, GearHandle: gear, FlapsIndex: flaps})
	}
	add(Sample{IAS: 135, Pitch: 5, OnGround: true, GearHandle: true, FlapsIndex: 4})
	return tr
}

// TestLearn: a type's profile from its flights (medians), the take-off,
// the climb-out, the approach configuration and the landing.
func TestLearn(t *testing.T) {
	tr := syntheticFlight()
	other := syntheticFlight()
	other.Model = "B738"
	l := Learn("A320", tr, tr, other)
	if l.Flights != 2 {
		t.Fatalf("%d flights", l.Flights)
	}
	near := func(name string, got, want, tol float64) {
		t.Helper()
		if math.Abs(got-want) > tol {
			t.Errorf("%s %.1f, want %.1f", name, got, want)
		}
	}
	near("rotate", l.RotateKts, 141, 2)
	near("lift-off", l.LiftoffKts, 148, 2)
	near("climb pitch", l.ClimbPitch, 15, 0.5)
	near("gear up", l.GearUpAGLFt, 150, 10)
	near("acceleration", l.AccelAGLFt, 2000, 60) // 180 kt reached about 2000 ft
	near("flaps 2→1", l.FlapsUpKts[2], 180, 2)
	near("flaps 1→0", l.FlapsUpKts[1], 210, 2)
	near("flaps 1 down", l.FlapsDownKts[1], 205, 2)
	near("flaps full down", l.FlapsDownKts[4], 150, 2)
	near("gear down", l.GearDownAGLFt, 2000, 20)
	near("approach", l.ApproachKts, 140, 1)
	near("flare", l.FlareAGLFt, 30, 10)
	near("touchdown", l.TouchdownFpm, -150, 1)
}
