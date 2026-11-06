package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Landed on its main gear, the nose wheel coming down on the rollout (live
// LKPR 24, QTR337 B789 at 142 kt, AFR473 A320 at 128 kt): at its static
// pitch with the on-ground flag the simulator shows the aircraft at its
// own static height on compressed struts at once (16.85 ft), then took
// ours again, still blending down from its height on the main gear (17.64
// ft half a second on). From the frame the nose wheel is down it is placed
// at its static height: no second step.
func TestRolloutNoseDownAtStaticHeight(t *testing.T) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 5, 20, 12, 10, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, cg, static, ground = 42, 16.85, 1.00, 1161.0
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	title := "FSLTL_B789_GE_QTR"
	inj.SetModel(obj, title)
	prof := MotionProfileFor(title)
	start, hdg := airport.LatLon{Lat: 50.1, Lon: 14.25}, 245.0
	for x := -50.0; x <= 1500; x += 0.5 {
		for y := -20.0; y <= 20; y += 0.5 {
			inj.ground.put(offsetHeading(offsetHeading(start, hdg, x), hdg+90, y), ground)
		}
	}
	pos, alt, pitch := start, ground+cg*(1+RestAboveStaticShare), static
	v, x := 145*ktsToMS, 0.0
	nosePitch := 3.0 // nose up, degrees
	var downAt float64 = -1
	for k := 0; k < 6*60; k++ {
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: ground, CGFt: cg, StaticPitch: static,
			PlaneFt: alt, PlanePitch: pitch, OnGround: 1, GS: v / ktsToMS, Lat: pos.Lat, Lon: pos.Lon}))
		w = w.Add(time.Second / 60)
		x += v / 60
		v -= 1.5 / 60
		pos = offsetHeading(start, hdg, x)
		nosePitch -= 1.0 / 60 // down over 3 s
		pose := ApproachPose{Position: pos, Heading: hdg, OnGround: true, Landing: true, PitchDeg: math.Max(0, nosePitch),
			GroundSpeedKts: v / ktsToMS, MainGearBehindM: prof.RefAheadMeters, RunwayFt: ground}
		n := len(placements(ec))
		if err := inj.PlaceAir(obj, pose); err != nil {
			t.Fatal(err)
		}
		p := placements(ec)
		if len(p) == n {
			t.Fatal("not placed")
		}
		alt, pitch = p[len(p)-1].Altitude, p[len(p)-1].Pitch
		if pose.PitchDeg > 0 {
			continue
		}
		if downAt < 0 {
			downAt = float64(k) / 60
		}
		if got := alt - ground; math.Abs(got-cg) > 0.01 {
			t.Fatalf("nose wheel down %.2f s: placed %.2f ft above the runway, want its static %.2f (the simulator shows it there)", float64(k)/60-downAt, got, cg)
		}
	}
	if downAt < 0 {
		t.Fatal("the nose wheel never came down")
	}
}
