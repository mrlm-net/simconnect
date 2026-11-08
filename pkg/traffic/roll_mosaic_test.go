package traffic

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The take-off roll over odd cells (live LKPR 24, 2026-11-06: AFR704 A320 at
// 36, 60 and 92 kt, OKLVO at 17 and 77, WZZ626 A20N and CSA354 BCS3, with
// and without nose wheel steering). The simulator's ground under them was
// smooth; the height shown above it stepped by one frame's impulse and then
// half of it back, halving frame by frame (WZZ626 at 22 kt: 0.00 → +0.21 →
// -0.13 → -0.07 → 0.00; AFR704 at 92 kt: -0.22 → +0.11 → +0.05 → +0.03).
// Along a runway most cells were written by the last departure at the
// detail it was shown (0.1 ft above this one's: its cellBias takes that
// off); where its samples skipped a cell, an older aircraft's ground stayed
// there, 0.2 ft below. Placed on such a cell the aircraft dropped by it for
// a frame, and measured there its cellBias took half of the difference on
// to the cells after (cellBiasFollow).
//
// Modelled: the simulator shows this aircraft its own ground (a 0.4 %
// slope, with a 0.3 ft step in the runway mesh 150 m in, which the cells
// have too); every eleventh cell or so is an older one's. Sampled where it
// was placed a frame or two before. From a standing and a 12 kt rolling
// start to rotation, the height shown above its own ground never changes by
// more than 0.06 ft from one frame to the next, the mesh step included, and
// stays within 0.06 ft of its static CG height.
func TestTakeoffRollOverOddCells(t *testing.T) {
	for _, startKts := range []float64{0, 12} {
		for _, lag := range []int{1, 2} {
			t.Run(fmt.Sprintf("from %.0f kt, sampled %d frame(s) late", startKts, lag), func(t *testing.T) {
				rollOverOddCells(t, startKts, lag)
			})
		}
	}
}

func rollOverOddCells(t *testing.T, startKts float64, lag int) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 6, 19, 49, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, cg, static = 42, 12.07, 0.8
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, "FSLTL_FAIB_A20N_easyJet")
	start, hdg := airport.LatLon{Lat: 50.1149723, Lon: 14.2700990}, 245.0
	own := func(p airport.LatLon) float64 {
		x := alongHeading(start, hdg, p)
		g := 1161.72 + 0.004*x/0.3048
		if x >= 150 {
			g += 0.3
		}
		return g
	}
	older := func(c groundCell) bool { return (c.lat*7+c.lon*3)%11 == 0 }
	for x := -20.0; x <= 1600; x += 0.25 {
		for y := -1.0; y <= 1; y += 0.25 {
			p := offsetHeading(offsetHeading(start, hdg, x), hdg+90, y)
			c := cellOf(p)
			ft, by := own(c.at())+0.1, uint32(7) // the last departure's detail
			if older(c) {
				ft, by = ft-0.2, 8
			}
			inj.ground.putBy(c.at(), ft, by, w.Add(-90*time.Second))
		}
	}
	placed := []airport.LatLon{start, start, start}
	alt := own(start) + cg
	sample := func(kts float64) {
		p := placed[len(placed)-1-lag] // where the simulator shows it: placed lag frames before
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: own(p), CGFt: cg, StaticPitch: static,
			PlaneFt: alt, PlanePitch: static, OnGround: 1, GS: kts, Lat: p.Lat, Lon: p.Lon}))
	}
	for k := 0; k < 8*60; k++ { // lined up, 8 s
		sample(0)
		if k%30 == 0 {
			if err := inj.Place(obj, GroundPose{Position: start, Heading: hdg, Stopped: true}); err != nil {
				t.Fatal(err)
			}
			p := placements(ec)
			alt = p[len(p)-1].Altitude
		}
		placed = append(placed, start)
		w = w.Add(time.Second / 60)
	}
	m := NewTakeoffMover(start, hdg, startKts, DefaultTakeoffProfile())
	prev, kts := math.NaN(), startKts
	steps := 0
	first := ""
	for k := 0; k < 60*60; k++ {
		sample(kts)
		tp := m.Step(1.0 / 60)
		if tp.Phase != TakeoffRoll {
			break
		}
		kts = tp.GroundSpeedKts
		n := len(placements(ec))
		if err := inj.PlaceAir(obj, tp.ApproachPose()); err != nil {
			t.Fatal(err)
		}
		p := placements(ec)
		if len(p) == n {
			t.Fatal("not placed")
		}
		alt = p[len(p)-1].Altitude
		placed = append(placed, tp.Position)
		shown := alt - own(tp.Position)
		w = w.Add(time.Second / 60)
		if k < 4*60 { // the CG blending from its rest to its static height
			prev = shown
			continue
		}
		// Its struts let out before rotation (struts.go) on top of its
		// static height.
		want := cg + inj.strutsAhead(inj.objects[obj], tp.ApproachPose())
		bad := ""
		switch {
		case math.Abs(shown-prev) > 0.06:
			bad = "stepped"
		case math.Abs(shown-want) > 0.06:
			bad = "off its static height"
		}
		if bad != "" {
			steps++
			if first == "" {
				first = fmt.Sprintf("%s: %.2f → %.2f ft at %.0f kt, %.0f m", bad, prev, shown, kts, tp.Distance)
			}
		}
		prev = shown
	}
	if steps > 0 {
		t.Errorf("%d frames stepped or off over the odd cells, the first %s (static %.2f)", steps, first, cg)
	}
}
