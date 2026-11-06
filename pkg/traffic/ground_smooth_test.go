package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// standStepMaxFt: a standing aircraft's height changes at most this much
// from one placement to the next (a blend step), whatever the gap.
const standStepMaxFt = CGBlendFtPerSecond*standStepMaxSec + 1e-6

// Holding short at 0 kt (live LKPR LOT1111, DH8D, 2026-11-04): the
// simulator's ground under it swapped between 1159.89 and 1160.35 ft (its
// scenery's detail), the aircraft itself frozen where we put it. The
// height above that ground was learnt as its rest (12.84 against its static
// 13.30), the ground eased in at the standing rate in one placement, and
// after a pause the first placement snapped: -0.45 ft in a frame. Standing,
// its height never changes by more than a blend step a placement, and a
// swap of the ground is not its rest.
func TestStandingGroundSwapNoStep(t *testing.T) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 4, 20, 6, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, cg, static = 42, 13.30, 1.30
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, "FSLTL_DH8D_LOT")
	at := airport.LatLon{Lat: 50.1148441, Lon: 14.2740712}
	ground := func(s float64) float64 { // seconds in
		switch {
		case s >= 2 && s < 150, s >= 200 && s < 250.3: // swapped back just after a pause
			return 1160.35
		}
		return 1159.89
	}
	plane := 1159.89 + cg // frozen: what we last sent
	// Requested with SIMCONNECT_DATA_REQUEST_FLAG_CHANGED: a sample comes
	// only when something in it changed (the ground swapped, or we placed it
	// elsewhere), so a single one may follow a swap.
	var last injectGround
	sample := func(s float64) {
		g := injectGround{GroundFt: ground(s), CGFt: cg, StaticPitch: static,
			PlaneFt: plane, PlanePitch: static, OnGround: 1, Lat: at.Lat, Lon: at.Lon}
		if g != last {
			inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, g))
			last = g
		}
	}
	var alts, grounds []float64
	start := w
	for f := 0; f < 300*60; f++ {
		s := w.Sub(start).Seconds()
		if s >= 230 && s < 250 { // the simulator paused: no frames, nothing placed
			w = w.Add(time.Second / 60)
			continue
		}
		sample(s)
		if f%30 == 0 { // standing: every 30th frame (Detail.StillInterval)
			n := len(placements(ec))
			if err := inj.Place(obj, GroundPose{Position: at, Heading: 338.58, Stopped: true}); err != nil {
				t.Fatal(err)
			}
			if p := placements(ec); len(p) > n {
				plane = p[len(p)-1].Altitude
				alts, grounds = append(alts, plane), append(grounds, ground(s))
			}
		}
		w = w.Add(time.Second / 60)
	}
	for k := 1; k < len(alts); k++ {
		if d := alts[k] - alts[k-1]; math.Abs(d) > standStepMaxFt {
			t.Fatalf("standing: placement %d moved %+.3f ft in one go (%.2f → %.2f), want at most %.3f", k, d, alts[k-1], alts[k], standStepMaxFt)
		}
	}
	if got := alts[len(alts)-1] - grounds[len(grounds)-1]; math.Abs(got-cg) > 0.02 {
		t.Errorf("standing: %.2f ft above the ground at the end, want its static %.2f (a swap of the ground is not its rest)", got, cg)
	}
}

// The take-off roll on cells another aircraft wrote at another detail of
// the scenery (live LKPR 24, CSA1156 A320 at 16 to 30 kt, 2026-11-04): the
// simulator's own ground lay up to 0.26 ft below the cells, and the
// aircraft flickered between its height and ours frame after frame (12.25
// ↔ 12.51 ft above the ground). Rolling over such cells it is placed on the
// simulator's ground as its samples show it, the map's shape kept.
func TestTakeoffRollOnStaleCells(t *testing.T) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 4, 20, 7, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, cg, static = 42, 12.25, 0.95
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery")
	start, hdg := airport.LatLon{Lat: 50.1149723, Lon: 14.2700990}, 244.0
	real := func(p airport.LatLon) float64 { return 1161.72 - 0.006*alongHeading(start, hdg, p)/0.3048 } // falling 0.6 %
	// The detail another aircraft saw: up to 0.26 ft higher.
	stale := func(p airport.LatLon) float64 {
		x := alongHeading(start, hdg, p)
		return real(p) + 0.26*math.Exp(-math.Pow((x-40)/30, 2))
	}
	for x := -60.0; x <= 400; x += 0.5 {
		for y := -6.0; y <= 6; y += 0.5 {
			p := offsetHeading(offsetHeading(start, hdg, x), hdg+90, y)
			inj.ground.put(p, stale(p))
		}
	}
	pos := start
	sample := func() {
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: real(pos), CGFt: cg, StaticPitch: static,
			PlaneFt: real(pos) + cg, PlanePitch: static, OnGround: 1, Lat: pos.Lat, Lon: pos.Lon}))
	}
	for k := 0; k < 120; k++ { // lined up, 2 s
		sample()
		if err := inj.Place(obj, GroundPose{Position: pos, Heading: hdg, Stopped: true}); err != nil {
			t.Fatal(err)
		}
		w = w.Add(time.Second / 60)
	}
	v, x := 0.0, 0.0
	prev := math.NaN()
	for k := 0; k < 8*60; k++ { // 8 s of the roll
		v += 1.0 / 60 // about 2 kt a second, as live
		x += v / 60
		sample() // where the simulator shows it: the last placement
		pos = offsetHeading(start, hdg, x)
		n := len(placements(ec))
		if err := inj.PlaceAir(obj, ApproachPose{Position: pos, Heading: hdg, OnGround: true, Unpinned: true, GroundSpeedKts: v / ktsToMS}); err != nil {
			t.Fatal(err)
		}
		p := placements(ec)
		if len(p) == n {
			t.Fatal("not placed")
		}
		alt := p[len(p)-1].Altitude
		if off := alt - (real(pos) + cg); k > 10 && math.Abs(off) > 0.03 {
			t.Fatalf("roll at %.0f kt, %.0f m: placed %+.3f ft off the simulator's own height (its ground + static CG), want within 0.03", v/ktsToMS, x, off)
		}
		if !math.IsNaN(prev) && math.Abs(alt-prev) > 0.03 {
			t.Fatalf("roll at %.0f kt: height stepped %+.3f ft in a frame", v/ktsToMS, alt-prev)
		}
		prev = alt
		w = w.Add(time.Second / 60)
	}
}

// smoothFrame is one frame of a scripted ground run: the reference
// point, heading and ground speed, whether it is placed this frame and
// how (PlaceAir on the take-off roll).
type smoothFrame struct {
	pos         airport.LatLon
	hdg, kts    float64
	placed, air bool
}

// smoothRun is a departure from a stand at 60 Hz: standing 3 s (placed
// every 30th frame, Detail.StillInterval), taxiing out to 15 kt with a 60°
// turn, stopping 5 s, starting again and the take-off roll to 80 kt.
func smoothRun() []smoothFrame {
	var fs []smoothFrame
	pos := airport.LatLon{Lat: 50.1, Lon: 14.25}
	hdg, kts := 65.0, 0.0
	add := func(sec float64, accel, turn float64) {
		for k := 0; k < int(sec*60); k++ {
			if kts = kts + accel/60; kts < 1e-6 {
				kts = 0
			}
			hdg += turn / 60
			pos = offsetHeading(pos, hdg, kts*ktsToMS/60)
			standing := kts <= restMaxKts
			fs = append(fs, smoothFrame{pos: pos, hdg: hdg, kts: kts, placed: !standing || len(fs)%30 == 0})
		}
	}
	add(3, 0, 0)       // on the stand
	add(8, 15.0/8, 0)  // starting
	add(15, 0, 4)      // a 60° turn at 15 kt
	add(6, -15.0/6, 0) // stopping
	add(5, 0, 0)       // stopped
	add(5, 10.0/5, 0)  // starting again
	n := len(fs)
	add(15, 70.0/15, 0) // the take-off roll
	for k := n; k < len(fs); k++ {
		fs[k].air, fs[k].placed = true, true
	}
	return fs
}

// From the stand to the take-off roll on gently sloping, gently rolling
// ground (0.5 % one way, 0.3 % the other, 0.05 ft waves), five types: from
// one placement to the next the height changes by no more than the ground
// under it does (a cell's quantisation and a frame's blend step aside), at
// exactly its static pitch, wings level (TestTaxiOnSlopeStaysAtStaticHeight
// has why). Live, a standing DH8D
// stepped 0.46 ft in a frame (LOT1111) and an A320 flickered 0.26 ft at the
// start of its roll (CSA1156).
func TestGroundRunSmooth(t *testing.T) {
	ground := func(p airport.LatLon) float64 {
		o := airport.LatLon{Lat: 50.1, Lon: 14.25}
		e, n := alongHeading(o, 90, p), alongHeading(o, 0, p)
		return 1200 + (0.005*e+0.003*n)/0.3048 + 0.05*math.Sin(2*math.Pi*e/80)
	}
	run := smoothRun()
	for _, title := range []string{
		"FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery",
		"FSLTL_FAIB_B738_TVS-Skytravel_TSOC",
		"FSLTL_DH8D_LOT",
		"FSLTL_SBAI_BCS3_CSA-Lines",
		"Asobo C172",
	} {
		t.Run(title, func(t *testing.T) {
			ec := &eventClient{}
			inj := NewInjector(ec)
			w := time.Date(2026, 11, 5, 20, 0, 0, 0, time.UTC)
			inj.now = func() time.Time { return w }
			const obj, cg, static = 42, 10.0, 0.9
			if err := inj.Takeover(obj); err != nil {
				t.Fatal(err)
			}
			inj.SetModel(obj, title)
			// The cells along the way, as aircraft before it saw them.
			for k := 0; k < len(run); k += 6 {
				for x := -25.0; x <= 25; x += 0.5 {
					for y := -25.0; y <= 25; y += 0.5 {
						p := offsetHeading(offsetHeading(run[k].pos, run[k].hdg, x), run[k].hdg+90, y)
						if _, ok := inj.ground.at(p); !ok {
							inj.ground.put(p, ground(p))
						}
					}
				}
			}
			last := run[0].pos
			alt, pitch := ground(last)+cg, static
			type placed struct {
				alt, pitch, bank, g float64
				at                  time.Time
				k                   int
			}
			var prev *placed
			for k, f := range run {
				inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: ground(last), CGFt: cg, StaticPitch: static,
					PlaneFt: alt, PlanePitch: pitch, OnGround: 1, GS: f.kts, Lat: last.Lat, Lon: last.Lon}))
				if f.placed {
					n := len(placements(ec))
					var err error
					if f.air {
						err = inj.PlaceAir(obj, ApproachPose{Position: f.pos, Heading: f.hdg, OnGround: true, Unpinned: true, GroundSpeedKts: f.kts})
					} else {
						err = inj.Place(obj, GroundPose{Position: f.pos, Heading: f.hdg, GroundSpeedKts: f.kts, Stopped: f.kts <= restMaxKts})
					}
					if err != nil {
						t.Fatal(err)
					}
					p := placements(ec)
					if len(p) == n {
						t.Fatalf("frame %d: not placed", k)
					}
					s := p[len(p)-1]
					cur := &placed{alt: s.Altitude, pitch: s.Pitch, bank: s.Bank, g: ground(f.pos), at: w, k: k}
					if prev != nil {
						if d := (cur.alt - prev.alt) - (cur.g - prev.g); math.Abs(d) > 0.04 {
							t.Fatalf("frame %d at %.1f kt: height %+.3f ft beyond the ground's own change (%+.3f) since frame %d", k, f.kts, d, cur.g-prev.g, prev.k)
						}
					}
					if cur.pitch != static || cur.bank != 0 {
						t.Fatalf("frame %d at %.1f kt: pitch %.3f, bank %.3f, not its static %.2f wings level", k, f.kts, cur.pitch, cur.bank, static)
					}
					prev = cur
					alt, pitch, last = s.Altitude, s.Pitch, f.pos
				}
				w = w.Add(time.Second / 60)
			}
		})
	}
}
