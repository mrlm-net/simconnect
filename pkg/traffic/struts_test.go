package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// simStruts is how the simulator shows an aircraft on the ground, frame by
// frame (struts.go has the live evidence): at any pitch but its static one
// on its extended struts, struts above its static CG height, whatever was
// sent; at its static pitch with the flag the height sent while its struts
// reach the ground (up to struts above static), beyond that airborne on
// that frame and back at its static height the next; without the flag
// the height sent, put back at its static height every third frame when
// off it (CSA1156, ground_reground_test.go).
type simStruts struct {
	cg, static, struts float64
	reach              float64 // how far above static its struts hold it at its static pitch
	shown              float64 // above its ground
	air                bool
	frame              int
	rolling            bool    // a take-off roll or rollout, not a stand or taxi
	pitch              float64 // the pitch sent last frame
}

// simStaticPitchDeg: a pitch this close to its static one the simulator
// takes for it (live: the first frames of a rotation, under 0.01° drawn up,
// still showed it on compressed struts).
const simStaticPitchDeg = 0.02

func (s *simStruts) step(p types.SIMCONNECT_DATA_INITPOSITION, groundFt float64, onGround bool) float64 {
	s.frame++
	h := p.Altitude - groundFt
	// The flag and height take at once, the pitch a frame later (live
	// CSA1883: the frame its nose started up still showed its static pitch).
	pitch := s.pitch
	if s.frame == 1 {
		pitch = p.Pitch
	}
	s.pitch = p.Pitch
	switch {
	case !onGround:
		s.air = false
	case math.Abs(pitch-s.static) > simStaticPitchDeg || p.Bank != 0:
		h, s.air = s.cg+s.struts, false
	case p.OnGround == 0 && s.rolling:
		// Rolling at its static pitch without the flag: back at its static
		// height at once (live LKPR 24, CSA1883 BCS3 at 136 kt, the frame its
		// nose started up: 7.39 → 7.05 → 7.47 ft).
		h, s.air = s.cg, false
	case p.OnGround == 0:
		if math.Abs(h-s.cg) > 0.05 && s.frame%3 == 0 {
			h = s.cg
		}
		s.air = false
	default:
		if s.air {
			h = s.cg
		}
		// Its struts reach a little short of their full extension at its
		// static pitch (live AFR284 A321, 0.77 ft of struts: back at its
		// static height from somewhere between 0.62 and 0.77 above it).
		reach := s.reach
		if reach == 0 {
			reach = s.struts - 0.05
		}
		s.air = h-s.cg > reach
	}
	s.shown = h
	return h
}

// strutsStepFt: a change of the height shown on the ground from one frame
// to the next beyond this is a step (struts.go leaves its margin, up to
// 0.15 ft for an A321, 0.20 for a CJ3 whose struts reach only 0.23 of their
// 0.41 ft at its static pitch: the glitch watch counts 0.25).
const strutsStepFt = 0.21

// Two take-offs of one model in two liveries one after the other, the
// simulator modelled as live (simStruts). A seeded model (B738, 0.49 ft of
// struts, live 8.48 → 8.97 at every rotation; RYR874 0.44) rolls and
// rotates without a step on the ground from its first take-off; an
// unseeded one learns its struts at its first rotation and the other
// livery takes them.
func TestRotationLetsStrutsOut(t *testing.T) {
	for _, c := range []struct {
		name               string
		cg, static, struts float64
		reach              float64
		titles             [2]string
		firstSmooth        bool
	}{
		{"B738 seeded", 8.48, 0.71, 0.49, 0.44, [2]string{"FSLTL_FAIB_B738_Ryanair", "FSLTL_FAIB_B738_TVS-Skytravel_TSOC"}, true},
		{"B738 seeded, shorter struts", 8.48, 0.71, 0.44, 0.39, [2]string{"FSLTL_FAIB_B738_Ryanair", "FSLTL_FAIB_B738_TVS-Skytravel_TSOC"}, true},
		{"unseeded", 9.93, 0.5, 0.45, 0.40, [2]string{"Model X Livery A", "Model X Livery B"}, false},
		// Live AFR284, BAW709 A321 (0.77 ft of struts): held at 0.62 above
		// static, snapped back to it beyond.
		// Live OKBZI CJ3 (0.41 ft of struts): flickered between 0.23 above
		// static and static.
		{"CJ3 seeded", 4.85, 1.55, 0.41, 0.23, [2]string{"Asobo PassiveAircraft Citation CJ3", "Asobo PassiveAircraft Citation CJ3 2"}, true},
		{"A321 seeded", 12.46, 1.0, 0.77, 0.62, [2]string{"FSLTL_FAIB_A321_SWR-Swiss Air", "FSLTL_A321_BAW-British Airways"}, true},
	} {
		t.Run(c.name, func(t *testing.T) { strutsTakeoffs(t, c.cg, c.static, c.struts, c.reach, c.titles, c.firstSmooth) })
	}
}

func strutsTakeoffs(t *testing.T, cg, static, struts, reach float64, titles [2]string, firstSmooth bool) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 8, 20, 11, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const groundFt = 1165.0
	start := airport.LatLon{Lat: 50.1, Lon: 14.25}
	for run, obj := range []uint32{42, 43} {
		if err := inj.Takeover(obj); err != nil {
			t.Fatal(err)
		}
		inj.SetModel(obj, titles[run])
		sim := &simStruts{cg: cg, static: static, struts: struts, reach: reach, shown: cg, rolling: true}
		m := NewTakeoffMover(start, 245, 100, DefaultTakeoffProfile())
		pose := m.Pose()
		pitch, kts := static, pose.GroundSpeedKts
		prev, steps, worst := math.NaN(), 0, 0.0
		for k := 0; k < 60*30; k++ {
			inj.Handle(sampleMsg(DefaultInjectRequestBase+2*uint32(inj.objects[obj].slot)+1, obj, injectGround{GroundFt: groundFt, CGFt: cg, StaticPitch: static,
				PlaneFt: groundFt + sim.shown, PlanePitch: pitch, OnGround: 1, GS: kts, Lat: pose.Position.Lat, Lon: pose.Position.Lon}))
			pose = m.Step(1.0 / 60)
			if pose.Phase == TakeoffAirborne {
				break
			}
			n := len(placements(ec))
			if err := inj.PlaceAir(obj, pose.ApproachPose()); err != nil {
				t.Fatal(err)
			}
			ps := placements(ec)
			if len(ps) == n {
				t.Fatal("not placed")
			}
			s := ps[len(ps)-1]
			h := sim.step(s, groundFt, true)
			pitch, kts = s.Pitch, pose.GroundSpeedKts
			if !math.IsNaN(prev) && math.Abs(h-prev) > strutsStepFt {
				steps++
				worst = math.Max(worst, math.Abs(h-prev))
				if steps == 1 {
					t.Logf("run %d at %.0f kt, pitch %.2f°: %.2f → %.2f ft (flag %d, sent %.2f)", run, pose.GroundSpeedKts, pose.PitchDeg, prev, h, s.OnGround, s.Altitude-groundFt)
				}
			}
			prev = h
			w = w.Add(time.Second / 60)
		}
		if (run == 1 || firstSmooth) && steps > 0 {
			t.Errorf("take-off %d (%s): stepped %d times on the ground (worst %.2f ft)", run+1, titles[run], steps, worst)
		}
		if run == 0 && !firstSmooth && steps == 0 {
			t.Errorf("unseeded first take-off did not step: the model sim lost its struts")
		}
		if err := inj.Release(obj); err != nil {
			t.Fatal(err)
		}
	}
}

// A landing A320 (0.62 ft of struts, live 12.87 → 12.25 as the nose came
// down on every rollout): touched down nose up on its extended struts, the
// nose lowered to its static pitch, rolled out: never a step on the
// ground, its struts learnt in the last frames before the nose came down.
func TestDerotationLetsStrutsDown(t *testing.T) {
	for _, c := range []struct {
		name                      string
		cg, static, struts, reach float64
		title                     string
	}{
		{"A320", 12.25, 0.95, 0.62, 0.57, "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery"},
		// Live THY1627 A321 (0.77 ft of struts): let down from 0.67 above
		// static as the nose came down, back at static for a frame.
		{"A321", 12.46, 1.00, 0.77, 0.62, "FSLTL_FAIB_A321_THY-Turkish Airlines"},
	} {
		t.Run(c.name, func(t *testing.T) { derotation(t, c.cg, c.static, c.struts, c.reach, c.title) })
	}
}

func derotation(t *testing.T, cg, static, struts, reach float64, title string) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 8, 20, 4, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, groundFt = 42, 1165.0
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, title)
	sim := &simStruts{cg: cg, static: static, struts: struts, reach: reach, shown: cg + struts}
	start := airport.LatLon{Lat: 50.1, Lon: 14.25}
	x, kts, nose := 0.0, 135.0, 4.0
	pitch := static - nose
	prev, steps, worst := math.NaN(), 0, 0.0
	for k := 0; k < 60*12; k++ {
		pos := offsetHeading(start, 245, x)
		inj.Handle(sampleMsg(DefaultInjectRequestBase+2*uint32(inj.objects[obj].slot)+1, obj, injectGround{GroundFt: groundFt, CGFt: cg, StaticPitch: static,
			PlaneFt: groundFt + sim.shown, PlanePitch: pitch, OnGround: 1, GS: kts, Lat: pos.Lat, Lon: pos.Lon}))
		if k > 60 { // a second on its main gear, then the nose down at 1.5°/s
			nose = math.Max(0, nose-1.5/60)
		}
		n := len(placements(ec))
		if err := inj.PlaceAir(obj, ApproachPose{Position: pos, Heading: 245, PitchDeg: nose, GroundSpeedKts: kts,
			OnGround: true, Landing: true, MainGearBehindM: 2.5}); err != nil {
			t.Fatal(err)
		}
		ps := placements(ec)
		if len(ps) == n {
			t.Fatal("not placed")
		}
		s := ps[len(ps)-1]
		h := sim.step(s, groundFt, true)
		pitch = s.Pitch
		if !math.IsNaN(prev) && math.Abs(h-prev) > strutsStepFt {
			steps++
			worst = math.Max(worst, math.Abs(h-prev))
			if steps == 1 {
				t.Logf("at %.0f kt, nose %.2f°: %.2f → %.2f ft (flag %d, sent %.2f)", kts, nose, prev, h, s.OnGround, s.Altitude-groundFt)
			}
		}
		prev = h
		x += kts * ktsToMS / 60
		kts = math.Max(60, kts-3.0/60*1.94)
		w = w.Add(time.Second / 60)
	}
	if steps > 0 {
		t.Errorf("rollout: stepped %d times on the ground (worst %.2f ft)", steps, worst)
	}
	if math.Abs(prev-cg) > 0.05 {
		t.Errorf("rolled out at %.2f ft, want its static %.2f", prev, cg)
	}
}
