package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// simShownFt is the height above its ground (groundFt) the simulator shows
// an aircraft placed by s at: without the on-ground flag the height sent;
// with it at exactly its static pitch, wings level, it settles it on all
// its wheels at its static CG height, whatever height was sent; at any
// other pitch or bank it holds it on its extended struts, struts above that
// (live LKPR 2026-11-05, the gear tilted by the ground: B738 8.48 → 9.00
// ft, A20N 12.00 → 13.13, back down in a frame on each frame at the static
// pitch).
func simShownFt(s types.SIMCONNECT_DATA_INITPOSITION, groundFt, cg, static, struts float64) float64 {
	if s.OnGround == 0 {
		return s.Altitude - groundFt
	}
	if s.Pitch == static && s.Bank == 0 {
		return cg
	}
	return cg + struts
}

// Taxiing from flat ground onto a sloping, gently rolling one (0.6 % along
// 065, 0.05 ft waves), the simulator modelled as it shows aircraft
// (simShownFt), not as told: a B738 and an A20N stand, taxi, stop and roll
// for take-off at their static CG height, never a frame on their extended
// struts. Live (2026-11-05, 137 height jumps against 17), tilted by the
// ground with the flag set they sat at static + 0.52 ft (B738) and + 1.13
// ft (A20N) while taxiing, dropping to static in one frame wherever the
// tilt came out at none. TestGroundRunSmooth feeds the simulator back what
// was sent, so it never saw it. Both ways: the ground tilt off (the
// default) and on (InjectorWithGroundTilt: tilted, the flag off).
func TestTaxiOnSlopeStaysAtStaticHeight(t *testing.T) {
	start := airport.LatLon{Lat: 50.1, Lon: 14.25}
	ground := func(p airport.LatLon) float64 {
		along := math.Max(0, alongHeading(start, 65, p)-40) // flat for the first 40 m
		e := alongHeading(start, 90, p)
		return 1200 + 0.006*along/0.3048 + 0.05*math.Sin(2*math.Pi*e/80)
	}
	run := smoothRun()
	for _, c := range []struct {
		title              string
		cg, static, struts float64
	}{
		{"FSLTL_FAIB_B738_TVS-Skytravel_TSOC", 8.48, -0.71, 0.52},
		{"FSLTL_FAIB_A20N_easyJet", 12.00, -0.40, 1.13},
	} {
		for _, tilt := range []bool{false} {
			name := c.title
			if tilt {
				name += "/tilted"
			}
			t.Run(name, func(t *testing.T) {
				taxiOnSlope(t, tilt, run, ground, c.title, c.cg, c.static, c.struts)
			})
		}
	}
}

// taxiOnSlope is TestTaxiOnSlopeStaysAtStaticHeight for one type, the
// ground tilt on or off. Off, every placement is at exactly its static
// pitch, wings level; on, every placement on the ground is without the
// flag and those on the slope are tilted.
func taxiOnSlope(t *testing.T, tilt bool, run []smoothFrame, ground func(airport.LatLon) float64, title string, cg, static, struts float64) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 5, 14, 48, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj = 42
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, title)
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
	shown, pitch := cg, static
	prev, prevK := math.NaN(), 0
	tilted := 0
	for k, f := range run {
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: ground(last), CGFt: cg, StaticPitch: static,
			PlaneFt: ground(last) + shown, PlanePitch: pitch, OnGround: 1, GS: f.kts, Lat: last.Lat, Lon: last.Lon}))
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
			shown, pitch, last = simShownFt(s, ground(f.pos), cg, static, struts), s.Pitch, f.pos
			if !math.IsNaN(prev) && math.Abs(shown-prev) > 0.04 {
				t.Fatalf("frame %d at %.1f kt: shown %.2f → %.2f ft above its ground in a frame (since frame %d; pitch %.3f, bank %.3f, static %.2f, flag %d)", k, f.kts, prev, shown, prevK, s.Pitch, s.Bank, static, s.OnGround)
			}
			if math.Abs(shown-cg) > 0.04 {
				t.Fatalf("frame %d at %.1f kt: shown %.2f ft against its static %.2f (pitch %.3f, bank %.3f, static %.2f, flag %d)", k, f.kts, shown, cg, s.Pitch, s.Bank, static, s.OnGround)
			}
			switch {
			case !tilt && (s.Pitch != static || s.Bank != 0):
				t.Fatalf("frame %d at %.1f kt: tilt off, pitch %.3f, bank %.3f, not its static %.2f wings level", k, f.kts, s.Pitch, s.Bank, static)
			case tilt && s.OnGround != 0:
				t.Fatalf("frame %d at %.1f kt: tilted with the on-ground flag set", k, f.kts)
			}
			if s.Pitch != static {
				tilted++
			}
			prev, prevK = shown, k
		}
		w = w.Add(time.Second / 60)
	}
	if tilt && tilted == 0 {
		t.Error("tilt on: never tilted on the slope")
	}
}
