package traffic

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// examples/glitch-watch's per-frame thresholds: on the ground pitch and
// bank 0.4°, heading 2°, height 0.25 ft; in the air heading 5°.
const (
	glitchGroundPitchDeg = 0.4
	glitchGroundBankDeg  = 0.4
	glitchGroundHdgDeg   = 2.0
	glitchAirHdgDeg      = 5.0
)

type balanceRun struct {
	maxRoll, maxPitch, maxHdg      float64 // largest balancing offsets
	stepRoll, stepPitch, stepHdg   float64 // largest per-frame change of the balancing
	poseRoll, posePitch, poseHdg   float64 // largest per-frame change of the whole pose (air)
	gRoll, gPitch, gHdg, gHeight   float64 // the same on the ground
	atTouchdown                    [3]float64
	maxAbove1500                   float64
	groundSame, touched            bool
	maxRollBelow1000, maxRollFlare float64
	amp                            float64 // BalanceRollDeg
}

// flyBalanced flies the same approach twice, with and without balancing,
// at frame seconds per step, and measures the difference.
func flyBalanced(t *testing.T, p ApproachProfile, wind, gust, cross, frame float64, seed uint64) balanceRun {
	t.Helper()
	thr := airport.LatLon{Lat: 50.1, Lon: 14.26}
	const start = 5.5 * 1852 // about 1800 ft
	a, b := NewApproachMover(thr, 244, start, p), NewApproachMover(thr, 244, start, p)
	a.SetCrosswind(cross)
	b.SetCrosswind(cross)
	a.SetBalance(wind, gust, rand.New(rand.NewPCG(seed, 1)))
	var r balanceRun
	r.amp = a.BalanceRollDeg()
	r.groundSame = true
	pa, pb := a.Pose(), b.Pose()
	prevD := [3]float64{}
	for i := 0; i < int(400/frame) && pa.Phase != ApproachDone; i++ {
		na, nb := a.Step(frame), b.Step(frame)
		d := [3]float64{na.BankDeg - nb.BankDeg, na.PitchDeg - nb.PitchDeg, headingDiff(nb.Heading, na.Heading)}
		r.maxRoll, r.maxPitch, r.maxHdg = math.Max(r.maxRoll, math.Abs(d[0])), math.Max(r.maxPitch, math.Abs(d[1])), math.Max(r.maxHdg, math.Abs(d[2]))
		r.stepRoll = math.Max(r.stepRoll, math.Abs(d[0]-prevD[0]))
		r.stepPitch = math.Max(r.stepPitch, math.Abs(d[1]-prevD[1]))
		r.stepHdg = math.Max(r.stepHdg, math.Abs(d[2]-prevD[2]))
		prevD = d
		if na.HeightFt > 1500 {
			r.maxAbove1500 = math.Max(r.maxAbove1500, math.Abs(d[0])+math.Abs(d[1])+math.Abs(d[2]))
		}
		if na.HeightFt < 1000 && na.HeightFt > 100 {
			r.maxRollBelow1000 = math.Max(r.maxRollBelow1000, math.Abs(d[0]))
		}
		if na.Phase == ApproachFlare {
			r.maxRollFlare = math.Max(r.maxRollFlare, math.Abs(d[0]))
		}
		dh := math.Abs(headingDiff(pa.Heading, na.Heading))
		if na.OnGround && pa.OnGround {
			r.gRoll, r.gPitch = math.Max(r.gRoll, math.Abs(na.BankDeg-pa.BankDeg)), math.Max(r.gPitch, math.Abs(na.PitchDeg-pa.PitchDeg))
			r.gHdg, r.gHeight = math.Max(r.gHdg, dh), math.Max(r.gHeight, math.Abs(na.HeightFt-pa.HeightFt))
		} else {
			r.poseRoll, r.posePitch, r.poseHdg = math.Max(r.poseRoll, math.Abs(na.BankDeg-pa.BankDeg)), math.Max(r.posePitch, math.Abs(na.PitchDeg-pa.PitchDeg)), math.Max(r.poseHdg, dh)
		}
		if na.OnGround != nb.OnGround {
			t.Fatalf("touchdown moved: balanced on ground %v, plain %v", na.OnGround, nb.OnGround)
		}
		if na.OnGround {
			if !r.touched {
				r.touched, r.atTouchdown = true, d
			}
			if na != nb {
				r.groundSame = false
			}
		}
		pa, pb = na, nb
	}
	_ = pb
	return r
}

// Final approach balances: a little in calm air, more in a gusty
// crosswind, smoothly, nothing above 1500 ft, reduced in the flare, gone
// at touchdown, and the landing on the runway exactly as without it.
func TestApproachBalance(t *testing.T) {
	p := DefaultApproachProfile()
	for _, frame := range []float64{1.0 / 60, 0.05} {
		calm := flyBalanced(t, p, 0, 0, 0, frame, 7)
		gusty := flyBalanced(t, p, 22, 34, 15, frame, 7)
		t.Logf("frame %.3f s calm: roll ±%.2f° pitch ±%.2f° hdg ±%.2f°, step %.3f/%.3f/%.3f°", frame, calm.maxRoll, calm.maxPitch, calm.maxHdg, calm.stepRoll, calm.stepPitch, calm.stepHdg)
		t.Logf("frame %.3f s gusty: roll ±%.2f° pitch ±%.2f° hdg ±%.2f°, step %.3f/%.3f/%.3f°, flare roll ±%.2f°", frame, gusty.maxRoll, gusty.maxPitch, gusty.maxHdg, gusty.stepRoll, gusty.stepPitch, gusty.stepHdg, gusty.maxRollFlare)
		if calm.maxRollBelow1000 < 0.2 || calm.maxRoll > balanceCalmDeg+1e-9 {
			t.Errorf("calm roll ±%.2f° (±%.2f° below 1000 ft), want about ±%.1f°", calm.maxRoll, calm.maxRollBelow1000, balanceCalmDeg)
		}
		if gusty.maxRollBelow1000 < 1.5 || gusty.maxRoll > balanceMaxRollDeg+1e-9 {
			t.Errorf("gusty roll ±%.2f° (±%.2f° below 1000 ft), want 1.5–%.0f°", gusty.maxRoll, gusty.maxRollBelow1000, balanceMaxRollDeg)
		}
		if gusty.maxRoll <= 2*calm.maxRoll {
			t.Errorf("roll does not grow with the wind: calm ±%.2f°, gusty ±%.2f°", calm.maxRoll, gusty.maxRoll)
		}
		for _, r := range []balanceRun{calm, gusty} {
			if r.maxRoll > r.amp+1e-9 || r.maxPitch > r.amp/3+1e-9 || r.maxHdg > balanceMaxHeadingDeg+1e-9 {
				t.Errorf("roll ±%.2f° pitch ±%.2f° heading ±%.2f° for amplitude ±%.2f°", r.maxRoll, r.maxPitch, r.maxHdg, r.amp)
			}
			if r.maxAbove1500 != 0 {
				t.Errorf("balancing %.3f° above 1500 ft", r.maxAbove1500)
			}
			if r.maxRollFlare > 0.5*r.maxRoll {
				t.Errorf("flare roll ±%.2f°, want reduced from ±%.2f°", r.maxRollFlare, r.maxRoll)
			}
			// Smooth: per frame a small fraction of the ground thresholds,
			// even at 20 Hz.
			if lim := frame * 6; r.stepRoll > lim || r.stepPitch > lim/2 || r.stepHdg > lim/2 {
				t.Errorf("balancing per frame roll %.3f° pitch %.3f° heading %.3f°, want below %.3f°", r.stepRoll, r.stepPitch, r.stepHdg, lim)
			}
			if r.poseHdg > glitchAirHdgDeg || r.gRoll > glitchGroundBankDeg || r.gPitch > glitchGroundPitchDeg || r.gHdg > glitchGroundHdgDeg || r.gHeight > 0.25 {
				t.Errorf("glitch: air heading %.2f°, ground bank %.2f° pitch %.2f° heading %.2f° height %.2f ft per frame", r.poseHdg, r.gRoll, r.gPitch, r.gHdg, r.gHeight)
			}
			if !r.touched || r.atTouchdown != [3]float64{} {
				t.Errorf("balancing at touchdown %v (touched %v), want none", r.atTouchdown, r.touched)
			}
			if !r.groundSame {
				t.Error("the ground poses differ from an approach without balancing")
			}
		}
	}
}

// A light single balances more than an A320, a widebody less; the
// approach without SetBalance stays on rails (the old poses).
func TestApproachBalanceByType(t *testing.T) {
	light, heavy := ProfileFor("C172").Approach, ProfileFor("B77W").Approach
	if light.BalanceScale <= 1 || heavy.BalanceScale >= 1 || DefaultApproachProfile().BalanceScale != 1 {
		t.Fatalf("balance scale light %.1f heavy %.1f default %.1f", light.BalanceScale, heavy.BalanceScale, DefaultApproachProfile().BalanceScale)
	}
	l := flyBalanced(t, light, 10, 0, 5, 0.05, 3)
	a := flyBalanced(t, DefaultApproachProfile(), 10, 0, 5, 0.05, 3)
	h := flyBalanced(t, heavy, 10, 0, 5, 0.05, 3)
	t.Logf("wind 10 kt, crosswind 5: light ±%.2f°, A320 ±%.2f°, heavy ±%.2f°", l.maxRoll, a.maxRoll, h.maxRoll)
	if !(l.maxRoll > a.maxRoll && a.maxRoll > h.maxRoll) {
		t.Errorf("roll light ±%.2f° A320 ±%.2f° heavy ±%.2f°, want light > A320 > heavy", l.maxRoll, a.maxRoll, h.maxRoll)
	}
	m := NewApproachMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 244, 3000, DefaultApproachProfile())
	for i := 0; i < 200; i++ {
		if p := m.Step(0.05); p.BankDeg != 0 || p.Heading != 244 || p.PitchDeg != DefaultApproachProfile().ApproachPitchDeg {
			t.Fatalf("unbalanced approach moved: bank %.3f heading %.3f pitch %.3f", p.BankDeg, p.Heading, p.PitchDeg)
		}
	}
}

// Two aircraft in the same wind do not balance in step.
func TestApproachBalanceOwnPhases(t *testing.T) {
	thr := airport.LatLon{Lat: 50.1, Lon: 14.26}
	a, b := NewApproachMover(thr, 244, 3000, DefaultApproachProfile()), NewApproachMover(thr, 244, 3000, DefaultApproachProfile())
	a.SetBalance(15, 25, rand.New(rand.NewPCG(1, 2)))
	b.SetBalance(15, 25, rand.New(rand.NewPCG(3, 4)))
	diff := 0.0
	for i := 0; i < 400; i++ {
		diff = math.Max(diff, math.Abs(a.Step(0.05).BankDeg-b.Step(0.05).BankDeg))
	}
	if diff < 0.3 {
		t.Errorf("two aircraft balance alike (largest difference %.2f°)", diff)
	}
}
