package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// simRegrounder is how the simulator shows an aircraft placed on the
// ground at exactly its static pitch, wings level, frame by frame: at the
// height sent on the frame a placement lands, and back at its own static
// CG height over its own ground on others, whenever the two differ.
//
// Live LKPR 2026-11-07 (gw-combo2, the nose wheel put on its ground by
// moving the height):
//   - DLH368's A20N (static 12.07 ft), placed every frame with the flag:
//     sent up to 0.32 ft above its static height it was shown there (799
//     frames, SIM ON GROUND 1); sent 0.33 ft or more, it read SIM ON GROUND
//     0 on that frame and was back at 12.04 to 12.06 the next, every other
//     frame from 91 kt on the rollout down to 15 kt taxiing (514 frames of
//     553 at 0.34 to 0.38 ft). The glitch watch reports a height jump only
//     between two frames on the ground, so it caught just the frames where
//     the raise dipped under 0.33 ft: 12 of hundreds.
//   - TVS1166's and TVS547's B738s (static 8.48), sent 0.35 to 0.43 ft
//     above it with the flag, SIM ON GROUND 1 throughout: shown there for 2
//     s and more, then at 8.48 for one or two frames (12:00:13.885,
//     12:00:24.747, 11:42:53.649), the altitude itself stepping 0.37 to
//     0.43 ft, not the ground under it.
//
// Live LKPR 2026-11-04 (gw-base): CSA1156's A320 on its take-off roll
// without the flag, at its static pitch 0.95°, sent 0.26 ft over its
// ground was shown at 12.50 and at its static 12.25 every second or third
// frame (16 to 30 kt). And earlier live, a B738 standing at its rest 8.06
// ft with the flag showed its static 8.48 for a frame every 5.6 s (RYR314,
// Handle).
//
// So whatever the flag, a height off the simulator's own shows as a step
// of that size in some frames: the nose wheel cannot be put on a grade by
// the height (at any pitch but its static one the simulator holds it on
// its extended struts, #1463).
type simRegrounder struct {
	cg float64 // its static CG height, ft
	// tolFt: with the flag, sent more than this above its static height it
	// is airborne on that frame and back at its static height the next.
	tolFt float64
	// every: with the flag, within tolFt, back at its static height for one
	// frame every this many frames (0 never).
	every int
	// offEvery: without the flag, back at its static height every this
	// many frames.
	offEvery int

	shown float64 // above its ground
	flag  bool
	air   bool
	frame int
}

// step is one simulator frame: sent the placement landing in it (nil
// none), groundFt the simulator's ground under the aircraft. It returns
// the height shown above that ground.
func (s *simRegrounder) step(sent *types.SIMCONNECT_DATA_INITPOSITION, groundFt float64) float64 {
	s.frame++
	h := s.shown
	if sent != nil {
		h = sent.Altitude - groundFt
		s.flag = sent.OnGround != 0
	}
	switch {
	case !s.flag:
		if s.offEvery > 0 && s.frame%s.offEvery == 0 {
			h = s.cg
		}
	case s.air, s.every > 0 && s.frame%s.every == 0:
		h = s.cg
	}
	s.air = s.flag && h-s.cg > s.tolFt
	s.shown = h
	return h
}

// regroundStepFt: a change of the height shown above the ground from one
// frame to the next beyond this is a step. Above the 0.10 ft the ground
// map's metre cells give on a 2.7 % grade (1.1 m × 2.7 %), below the 0.33
// to 0.43 ft the simulator stepped live.
const regroundStepFt = 0.15

// Taxiing up and down LKPR's 2.7 % taxiway (the DLH A20N's, 2026-11-07)
// and around a 0.9 % apron, placed every frame at 14 kt, the simulator
// modelled as it showed the live aircraft (simRegrounder), its shown height
// fed back to the injector: the height shown never steps in a frame.
// Moving the height to put the nose wheel on its ground (v0.167.10 +
// nosedive, mainsOffMaxFt 0.4) fails: the A20N alternates every other
// frame between the raise and its static height, the B738 drops to its
// static height for a frame every 2 s.
func TestGroundHeightSurvivesSimRegrounding(t *testing.T) {
	for _, c := range []struct {
		name, title    string
		cg, static     float64
		tolFt          float64
		every, offEvry int
	}{
		{"A20N", "FSLTL A20N DLH Lufthansa", 12.07, 0.80, 0.33, 0, 3},
		{"B738", "FSLTL_FAIB_B738_TVS-Skytravel_TSOC", 8.48, 0.71, 0.6, 120, 3},
	} {
		for _, grade := range []float64{0.027, -0.027, 0.009} {
			c, grade := c, grade
			t.Run(c.name+"/"+gradeName(grade), func(t *testing.T) {
				sim := &simRegrounder{cg: c.cg, tolFt: c.tolFt, every: c.every, offEvery: c.offEvry, shown: c.cg}
				regroundRun(t, sim, c.title, c.cg, c.static, grade)
			})
		}
	}
}

func gradeName(g float64) string {
	switch {
	case g > 0.01:
		return "uphill"
	case g < 0:
		return "downhill"
	}
	return "apron"
}

func regroundRun(t *testing.T, sim *simRegrounder, title string, cg, static, grade float64) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 7, 11, 56, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj = 42
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, title)
	start, hdg := airport.LatLon{Lat: 50.105280, Lon: 14.262244}, 215.0
	// Flat for 20 m, then the grade coming in over the next 20 m (no corner
	// sharper than a real taxiway's).
	const flatM, easeM = 20.0, 20.0
	ground := func(p airport.LatLon) float64 {
		a := math.Max(0, alongHeading(start, hdg, p)-flatM)
		if a < easeM {
			return 1168.13 + grade*a*a/(2*easeM)/0.3048
		}
		return 1168.13 + grade*(a-easeM/2)/0.3048
	}
	// The taxiway as earlier traffic left it in the ground map, each cell
	// the ground at its centre.
	for x := -30.0; x <= 140; x += 0.5 {
		for y := -4.0; y <= 4; y += 0.5 {
			c := cellOf(offsetHeading(offsetHeading(start, hdg, x), hdg+90, y)).at()
			inj.ground.put(c, ground(c))
		}
	}
	const kts = 14.0
	pos := start
	prev, steps, worst := math.NaN(), 0, 0.0
	logged := false
	for x := 0.0; x <= 100; x += kts * ktsToMS / 60 {
		// The simulator's sample of the frame before: where it shows it.
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: ground(pos), CGFt: cg, StaticPitch: static,
			PlaneFt: ground(pos) + sim.shown, PlanePitch: static, OnGround: 1, GS: kts, Lat: pos.Lat, Lon: pos.Lon}))
		pos = offsetHeading(start, hdg, x)
		n := len(placements(ec))
		if err := inj.Place(obj, GroundPose{Position: pos, Heading: hdg, GroundSpeedKts: kts}); err != nil {
			t.Fatal(err)
		}
		p := placements(ec)
		if len(p) == n {
			t.Fatal("not placed")
		}
		s := p[len(p)-1]
		if s.Pitch != static || s.Bank != 0 {
			t.Fatalf("at %.0f m: pitch %.2f bank %.2f, want its static %.2f wings level", x, s.Pitch, s.Bank, static)
		}
		h := sim.step(&s, ground(pos))
		if !math.IsNaN(prev) {
			if d := math.Abs(h - prev); d > regroundStepFt {
				steps++
				worst = math.Max(worst, d)
				if !logged {
					logged = true
					t.Logf("first step at %.1f m: %.2f → %.2f ft above the ground (static %.2f, sent %.2f, flag %d)", x, prev, h, cg, s.Altitude-ground(pos), s.OnGround)
				}
			}
		}
		prev = h
		w = w.Add(time.Second / 60)
	}
	if steps > 0 {
		t.Errorf("%.1f %% grade: the height shown stepped %d times (worst %.2f ft) as the simulator put it back at its static height", grade*100, steps, worst)
	}
}
