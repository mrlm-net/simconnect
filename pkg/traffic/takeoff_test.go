//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTakeoffMover: an A320 take-off from a standing start rotates at Vr,
// lifts off within a typical distance and climbs smoothly; past the
// acceleration altitude it speeds up towards the clean speed, climbing
// more slowly meanwhile.
func TestTakeoffMover(t *testing.T) {
	p := DefaultTakeoffProfile()
	m := NewTakeoffMover(lkpr, 244, 0, p)
	var liftoff TakeoffPose
	prevVS, maxVSStep := 0.0, 0.0
	for i := 0; i < 60*120; i++ {
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
	if final.HeightFt < 1500 || math.Abs(final.VerticalFpm-p.ClimbFpm*TakeoffAccelClimbFactor) > 1 || final.GroundSpeedKts <= p.ClimbKts || final.PitchDeg != p.ClimbPitch {
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
		// Widebodies taxi a little slower and softer (#324).
		if p.CruiseKts < a320.CruiseKts-1 || p.CruiseKts > a320.CruiseKts || p.RefAheadMeters != a320.RefAheadMeters {
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

// TestRequiredTakeoffRun: computed from the aircraft's take-off, longer for
// bigger aircraft and at higher, hotter airports.
func TestRequiredTakeoffRun(t *testing.T) {
	a320 := RequiredTakeoffRun(DefaultTakeoffProfile(), TakeoffConditions{})
	b77w := RequiredTakeoffRun(TakeoffProfileFor("B77W"), TakeoffConditions{})
	high := RequiredTakeoffRun(DefaultTakeoffProfile(), TakeoffConditions{ElevationFt: 5000, ISADeviationC: 15})
	if a320 < 1600 || a320 > 2600 {
		t.Errorf("A320 needs %.0f m", a320)
	}
	if b77w <= a320 || high <= a320*1.5 {
		t.Errorf("777 %.0f m, A320 at 5000 ft ISA+15 %.0f m, A320 %.0f m", b77w, high, a320)
	}
	t.Logf("A320 %.0f m, 777-300 %.0f m, A320 high and hot %.0f m", a320, b77w, high)
}

// TestEntryTooShort: an intersection departure is refused when the runway
// ahead of the entry is shorter than the aircraft needs.
func TestEntryTooShort(t *testing.T) {
	g := lkprGraph(t)
	entries, err := g.RunwayEntries("06")
	if err != nil {
		t.Fatal(err)
	}
	c22, _ := g.Layout.ParkingIndex("C22")
	need := RequiredTakeoffRun(DefaultTakeoffProfile(), TakeoffConditions{ElevationFt: 1247})
	var short, long string
	for _, e := range entries {
		if e.Taxiway == "" {
			continue
		}
		if e.Remaining < need && short == "" {
			short = e.Taxiway
		}
		if e.Remaining >= need && long == "" {
			long = e.Taxiway
		}
	}
	if short == "" || long == "" {
		t.Skipf("LKPR 06 entries not all long or short for an A320 (%.0f m)", need)
	}
	for _, c := range []struct {
		entry string
		ok    bool
	}{{short, false}, {long, true}} {
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "06", Entry: c.entry, Model: "A320"})
		if c.ok && err != nil && !errors.Is(err, airport.ErrNoRoute) {
			t.Errorf("06 at %s: %v", c.entry, err)
		}
		if !c.ok && !errors.Is(err, ErrEntryTooShort) {
			t.Errorf("06 at %s: %v, want ErrEntryTooShort", c.entry, err)
		}
	}
}

// The take-off roll builds up as the engines spool: little speed in the
// first second, a lot more once at take-off thrust; the A320 still lifts
// off within a normal distance.
func TestTakeoffSpoolUp(t *testing.T) {
	m := NewTakeoffMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 245, 0, DefaultTakeoffProfile())
	kts := func() float64 { return m.Pose().GroundSpeedKts }
	second := func() float64 {
		v := kts()
		for i := 0; i < 60; i++ {
			m.Step(1.0 / 60)
		}
		return kts() - v
	}
	first := second()
	for i := 0; i < 6; i++ {
		second()
	}
	eighth := second()
	if first > eighth/3 || eighth < 3.5 {
		t.Errorf("gained %.1f kt in the first second, %.1f in the eighth", first, eighth)
	}
	for i := 0; i < 60*120 && m.Pose().LiftoffDistance == 0; i++ {
		m.Step(1.0 / 60)
	}
	if d := m.Pose().LiftoffDistance; d < 1200 || d > 2000 {
		t.Errorf("lift-off after %.0f m", d)
	}
}

// TestTakeoffPitchProfile: lift-off at 5–7°, then the climb pitch, and
// after GearUp the pitch settles to about 10° (2026-10-03).
func TestTakeoffPitchProfile(t *testing.T) {
	p := DefaultTakeoffProfile()
	m := NewTakeoffMover(lkpr, 244, 0, p)
	var liftoff float64
	pose := m.Pose()
	for pose.Phase != TakeoffAirborne {
		pose = m.Step(0.05)
	}
	liftoff = pose.PitchDeg
	if liftoff < 5 || liftoff > 7 {
		t.Errorf("lift-off pitch %.1f°, want 5–7°", liftoff)
	}
	for pose.HeightFt < 600 {
		pose = m.Step(0.05)
	}
	if pose.PitchDeg != p.ClimbPitch {
		t.Errorf("first climb at %.1f°, want the climb pitch %.1f°", pose.PitchDeg, p.ClimbPitch)
	}
	m.GearUp()
	pose = m.Step(2)
	if pose.PitchDeg >= p.ClimbPitch || pose.PitchDeg < p.ClimbPitch-2.5 {
		t.Errorf("2 s after gear-up %.1f°: want easing down at about 1°/s", pose.PitchDeg)
	}
	pose = m.Step(10)
	if pose.PitchDeg != 10 {
		t.Errorf("settled at %.1f°, want 10°", pose.PitchDeg)
	}
}
