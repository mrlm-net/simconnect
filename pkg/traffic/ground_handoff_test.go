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
// ft half a second on). The simulator modelled as live (simStruts: nose up
// on its extended struts whatever was sent; at its static pitch the height
// sent while its struts reach): from the frame the nose wheel is down no
// step, and its static height a few seconds on (struts.go lets the struts
// down from there).
func TestRolloutNoseDownAtStaticHeight(t *testing.T) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 5, 20, 12, 10, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, cg, static, ground, struts = 42, 16.85, 1.00, 1161.0, 0.79
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
	sim := &simStruts{cg: cg, static: static, struts: struts, shown: cg + struts}
	pos, pitch := start, static
	v, x := 145*ktsToMS, 0.0
	nosePitch := 3.0 // nose up, degrees
	var downAt float64 = -1
	prev := math.NaN()
	for k := 0; k < 8*60; k++ {
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: ground, CGFt: cg, StaticPitch: static,
			PlaneFt: ground + sim.shown, PlanePitch: pitch, OnGround: 1, GS: v / ktsToMS, Lat: pos.Lat, Lon: pos.Lon}))
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
		s := p[len(p)-1]
		h := sim.step(s, ground, true)
		pitch = s.Pitch
		if pose.PitchDeg > 0 {
			prev = h
			continue
		}
		if downAt < 0 {
			downAt = float64(k) / 60
		}
		if math.Abs(h-prev) > strutsStepFt {
			t.Fatalf("nose wheel down %.2f s: shown %.2f → %.2f ft above the runway", float64(k)/60-downAt, prev, h)
		}
		prev = h
	}
	if downAt < 0 {
		t.Fatal("the nose wheel never came down")
	}
	if math.Abs(prev-cg) > 0.01 {
		t.Errorf("rolled out at %.2f ft, want its static %.2f", prev, cg)
	}
}
