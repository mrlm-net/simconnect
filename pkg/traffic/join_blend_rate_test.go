package traffic

import (
	"math"
	"testing"
)

// A normal arrival's writes are the same with the rate limit as without
// it: its offset fades over JoinBlendSeconds either way (#1681).
func TestNormalJoinWritesUnchanged(t *testing.T) {
	limited, n := normalJoinWrites(t)
	keep := joinBlendMaxFpm
	joinBlendMaxFpm = math.Inf(1)
	t.Cleanup(func() { joinBlendMaxFpm = keep })
	free, m := normalJoinWrites(t)
	if limited != free || n != m {
		t.Errorf("with the limit %d writes %s, without %d %s", n, limited, m, free)
	}
}

// The hand-over's height blend changes the height no faster than
// joinBlendMaxFpm with the approach's descent, however high MSFS AI
// brings it in: 7,000 ft over 30 s before (#1681).
func TestJoinBlendRate(t *testing.T) {
	const pathFpm, dt = 850.0, 1.0 / 60
	for _, dFt := range []float64{100, 600, 3000, 7000, -2000} {
		b := joinBlend{dFt: dFt, left: JoinBlendSeconds}
		b.ftFor = joinBlendFtSeconds(dFt, pathFpm)
		b.ftLeft = b.ftFor
		prev, worst := b.apply(ApproachPose{}, 0).HeightFt, 0.0
		for i := 0; b.ftLeft > 0 && i < 3600*60; i++ {
			h := b.apply(ApproachPose{}, dt).HeightFt
			worst = math.Max(worst, math.Abs(h-prev)/dt*60)
			prev = h
		}
		if worst+pathFpm > joinBlendMaxFpm+1 {
			t.Errorf("%.0f ft above the final: the blend at %.0f fpm, with the approach's %.0f over %.0f", dFt, worst, pathFpm, joinBlendMaxFpm)
		}
		if math.Abs(prev) > 1e-9 {
			t.Errorf("%.0f ft: %.1f ft left at the end", dFt, prev)
		}
		if b.ftFor < JoinBlendSeconds {
			t.Errorf("%.0f ft: over %.1f s, under JoinBlendSeconds", dFt, b.ftFor)
		}
	}
}
