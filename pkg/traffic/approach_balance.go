package traffic

import (
	"math"
	"math/rand/v2"
)

// Balancing on final: a real final is never flown on rails, there is
// always some small balancing in roll, pitch and heading, more in gusts and
// crosswind (the user). The movements are slow sums of sines (periods
// 2.6–5.9 s) with random phases per aircraft, never frame-to-frame jitter,
// faded in below balanceFadeFromFt, reduced through the flare and gone at
// touchdown: the landing handoff and the ground placement do not change.

// The balancing amplitude in roll, degrees: balanceCalmDeg in calm air, plus
// balancePerWindKt per knot of wind, balancePerGustKt per knot the gusts
// exceed it and balancePerCrossKt per knot of crosswind, at most
// balanceMaxRollDeg (times the type's ApproachProfile.BalanceScale, never
// above balanceCapRollDeg). Pitch moves balancePitchShare of the roll,
// heading balanceHeadingShare of it, at most balanceMaxHeadingDeg.
const (
	balanceCalmDeg       = 0.3
	balancePerWindKt     = 0.04
	balancePerGustKt     = 0.06
	balancePerCrossKt    = 0.03
	balanceMaxRollDeg    = 2.0
	balanceCapRollDeg    = 3.0
	balancePitchShare    = 1.0 / 3
	balanceHeadingShare  = 0.25
	balanceMaxHeadingDeg = 0.5
)

// ApproachProfile.BalanceScale by type: a light aircraft moves most, a
// widebody least; the A320 family and other jets 1.
const (
	LightBalanceScale     = 1.6
	TurbopropBalanceScale = 1.3
	HeavyBalanceScale     = 0.6
)

// The balancing fades in from balanceFadeFromFt down to balanceFullFt
// (wheel height above the runway) and out below balanceLowFt: about a fifth
// is left at a 30 ft flare, nothing at touchdown.
const (
	balanceFadeFromFt = 1500.0
	balanceFullFt     = 1200.0
	balanceLowFt      = 100.0
	// balanceRampS: an approach taken over already low eases in over its
	// first seconds, no step from the spawn pose.
	balanceRampS = 5.0
)

// balanceWave: each axis is a sum of sines, weights summing to 1 so the
// amplitude bounds it.
type balanceWave struct{ weight, periodS float64 }

var (
	balanceRollWaves    = [...]balanceWave{{0.55, 4.3}, {0.30, 2.6}, {0.15, 5.9}}
	balancePitchWaves   = [...]balanceWave{{0.6, 3.4}, {0.4, 5.3}}
	balanceHeadingWaves = [...]balanceWave{{0.6, 5.7}, {0.4, 3.9}}
)

// balance is an ApproachMover's balancing (SetBalance).
type balance struct {
	on               bool
	windKts, gustKts float64
	phase            [len(balanceRollWaves) + len(balancePitchWaves) + len(balanceHeadingWaves)]float64
}

// SetBalance turns on the small balancing movements on final for wind
// windKts gusting gustKts (0: no gusts); the crosswind is SetCrosswind's.
// rng draws the aircraft's own phases (nil: all zero).
func (m *ApproachMover) SetBalance(windKts, gustKts float64, rng *rand.Rand) {
	m.bal.on, m.bal.windKts, m.bal.gustKts = true, math.Max(0, windKts), math.Max(0, gustKts)
	for i := range m.bal.phase {
		m.bal.phase[i] = 0
		if rng != nil {
			m.bal.phase[i] = 2 * math.Pi * rng.Float64()
		}
	}
}

// balanceRand draws an aircraft's balancing phases from its object ID.
func balanceRand(objectID uint32) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(objectID), 0xba1a))
}

// BalanceRollDeg is the full roll amplitude of the balancing (degrees, ±)
// for this wind; pitch and heading follow from it.
func (m *ApproachMover) BalanceRollDeg() float64 {
	if !m.bal.on {
		return 0
	}
	gust := math.Max(0, m.bal.gustKts-m.bal.windKts)
	a := math.Min(balanceMaxRollDeg, balanceCalmDeg+balancePerWindKt*m.bal.windKts+balancePerGustKt*gust+balancePerCrossKt*math.Abs(m.cross))
	scale := m.p.BalanceScale
	if scale <= 0 {
		scale = 1
	}
	return math.Min(balanceCapRollDeg, a*scale)
}

// balanceFade is how much of the balancing is on at height h (feet above
// the runway) in phase: 0 above balanceFadeFromFt and on the ground.
func balanceFade(h float64, phase ApproachPhase) float64 {
	if phase >= ApproachDerotate || h <= 0 {
		return 0
	}
	in := smoothstep((balanceFadeFromFt - h) / (balanceFadeFromFt - balanceFullFt))
	out := smoothstep(h / balanceLowFt)
	return in * out
}

func smoothstep(x float64) float64 {
	x = math.Max(0, math.Min(1, x))
	return x * x * (3 - 2*x)
}

// balanceNow is the balancing now: roll (right wing down), pitch (nose up)
// and heading offsets, degrees.
func (m *ApproachMover) balanceNow() (roll, pitch, hdg float64) {
	if !m.bal.on {
		return 0, 0, 0
	}
	k := balanceFade(m.h, m.phase) * smoothstep(m.t/balanceRampS)
	if k == 0 {
		return 0, 0, 0
	}
	a := m.BalanceRollDeg() * k
	i := 0
	wave := func(ws []balanceWave) float64 {
		s := 0.0
		for _, w := range ws {
			s += w.weight * math.Sin(2*math.Pi*m.t/w.periodS+m.bal.phase[i])
			i++
		}
		return s
	}
	roll = a * wave(balanceRollWaves[:])
	pitch = a * balancePitchShare * wave(balancePitchWaves[:])
	hdg = math.Min(balanceMaxHeadingDeg, a*balanceHeadingShare) * wave(balanceHeadingWaves[:])
	return roll, pitch, hdg
}
