package traffic

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// LKPR's taxi seams (live 2026-11-08): at 50.1110,14.2657 heading 035 the
// simulator's ground 0.39 ft higher from one frame to the next at 15 to 17
// kt, the aircraft lifted 0.38 ft in 4 frames; at 50.11487,14.27405
// heading 341 0.44 ft, TVS1774 lining up at 4 kt followed it late, below
// the simulator's static height over the higher side, and the simulator
// put it there for 3 frames, then ours 0.40 lower. Taxiing over such a
// step, the height sent moves at most seamStepFt a frame and stays within
// seamOffShare of the step of its static height over the ground.
func TestTaxiOverGroundSeam(t *testing.T) {
	for _, c := range []struct{ kts, step, hdg float64 }{{16, 0.39, 35}, {4, 0.44, 341}, {4, -0.44, 341}, {30, 0.39, 35}, {60, 0.39, 35}} {
		t.Run(fmt.Sprintf("%.0fkt %+.2f", c.kts, c.step), func(t *testing.T) { taxiOverSeam(t, c.kts, c.step, c.hdg) })
	}
}

func taxiOverSeam(t *testing.T, kts, stepFt, hdg float64) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 8, 17, 14, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, cg, static, seam = 42, 12.25, 0.95, 30.0
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery")
	start := airport.LatLon{Lat: 50.110800, Lon: 14.265500}
	ground := func(p airport.LatLon) float64 {
		if alongHeading(start, hdg, p) >= seam {
			return 1164.00 + stepFt
		}
		return 1164.00
	}
	for x := -20.0; x <= 80; x += 0.5 {
		for y := -4.0; y <= 4; y += 0.5 {
			c := cellOf(offsetHeading(offsetHeading(start, hdg, x), hdg+90, y)).at()
			inj.ground.put(c, ground(c))
		}
	}
	pos := start
	prevAlt, worst, worstOff := math.NaN(), 0.0, 0.0
	for x := 0.0; x <= 60; x += kts * ktsToMS / 60 {
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: ground(pos), CGFt: cg, StaticPitch: static,
			PlaneFt: ground(pos) + cg, PlanePitch: static, OnGround: 1, GS: kts, Lat: pos.Lat, Lon: pos.Lon}))
		pos = offsetHeading(start, hdg, x)
		n := len(placements(ec))
		if err := inj.Place(obj, GroundPose{Position: pos, Heading: hdg, GroundSpeedKts: kts}); err != nil {
			t.Fatal(err)
		}
		p := placements(ec)
		if len(p) == n {
			t.Fatal("not placed")
		}
		alt := p[len(p)-1].Altitude
		if !math.IsNaN(prevAlt) {
			worst = math.Max(worst, math.Abs(alt-prevAlt))
		}
		off := alt - cg - ground(pos)
		worstOff = math.Max(worstOff, math.Abs(off))
		if math.Abs(off) > math.Abs(stepFt)*seamOffShare+seamCellFt {
			t.Fatalf("at %.1f m: %+.2f ft off its static height over the ground", x, off)
		}
		prevAlt = alt
		w = w.Add(time.Second / 60)
	}
	if want := math.Max(seamStepFt, groundFollowFtPerSec(kts)/60+0.002); worst > want {
		t.Errorf("over the seam the height moved %.3f ft in a frame, want at most %.3f", worst, want)
	}
	t.Logf("worst %.3f ft a frame, %.2f ft off", worst, worstOff)
}

// seamStepFt: at most this much a frame taxiing over a seam (2 ft/s at 60
// Hz: 0.033; rolling faster, its follow rate). seamOffShare: at most this
// share of the step off its static height over the ground (the simulator
// put TVS1774 back at 0.44 below), plus seamCellFt for the cells (about a
// metre) the map knows the ground by.
const (
	seamStepFt   = 0.05
	seamOffShare = 0.5
	seamCellFt   = 0.05
)
