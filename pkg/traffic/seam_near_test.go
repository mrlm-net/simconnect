package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TVS414 (live 2026-11-08) crossed a seam a metre off the line the ones
// before had taken: no cells on its own line, so nothing ahead to ramp it.
// The cells beside its line must ramp it the same.
func TestSeamOffTheSeenLine(t *testing.T) {
	for _, c := range []struct{ kts, step, hdg float64 }{{15, 0.27, 128}, {15, -0.27, 128}, {4, 0.44, 341}, {30, 0.39, 35}} {
		ec := &eventClient{}
		inj := NewInjector(ec)
		w := time.Date(2026, 11, 8, 17, 14, 0, 0, time.UTC)
		inj.now = func() time.Time { return w }
		const obj, cg, static, seam = 42, 12.25, 0.95, 30.0
		if err := inj.Takeover(obj); err != nil {
			t.Fatal(err)
		}
		inj.SetModel(obj, "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery")
		start := airport.LatLon{Lat: 50.11007, Lon: 14.26575}
		hdg := c.hdg
		ground := func(p airport.LatLon) float64 {
			if alongHeading(start, hdg, p) >= seam {
				return 1164.00 + c.step
			}
			return 1164.00
		}
		line := map[groundCell]bool{}
		for x := -20.0; x <= 80; x += 0.2 {
			line[cellOf(offsetHeading(start, hdg, x))] = true
		}
		for x := -20.0; x <= 80; x += 0.5 {
			for y := -4.0; y <= 4; y += 0.5 {
				cl := cellOf(offsetHeading(offsetHeading(start, hdg, x), hdg+90, y))
				if line[cl] {
					continue
				}
				p := cl.at()
				inj.ground.put(cl.at(), ground(p))
			}
		}
		pos := start
		prev, worst := math.NaN(), 0.0
		for x := 0.0; x <= 60; x += c.kts * ktsToMS / 60 {
			inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: ground(pos), CGFt: cg, StaticPitch: static,
				PlaneFt: ground(pos) + cg, PlanePitch: static, OnGround: 1, GS: c.kts, Lat: pos.Lat, Lon: pos.Lon}))
			pos = offsetHeading(start, hdg, x)
			n := len(placements(ec))
			if err := inj.Place(obj, GroundPose{Position: pos, Heading: hdg, GroundSpeedKts: c.kts}); err != nil {
				t.Fatal(err)
			}
			p := placements(ec)
			if len(p) == n {
				t.Fatal("not placed")
			}
			alt := p[len(p)-1].Altitude
			if !math.IsNaN(prev) {
				worst = math.Max(worst, math.Abs(alt-prev))
			}
			prev = alt
			w = w.Add(time.Second / 60)
		}
		want := math.Max(seamStepFt, groundFollowFtPerSec(c.kts)/60+0.002)
		t.Logf("%.0f kt %+.2f: worst %.3f ft a frame (want ≤ %.3f)", c.kts, c.step, worst, want)
		if worst > want {
			t.Errorf("%.0f kt %+.2f: %.3f ft in a frame off the seen line", c.kts, c.step, worst)
		}
	}
}
