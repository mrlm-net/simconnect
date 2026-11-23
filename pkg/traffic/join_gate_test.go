package traffic

import (
	"math"
	"testing"
)

// MSFS AI 7,000 ft above the final at the join: the hand-over's height
// fades at an airliner's rate, no step written, and the approach is not
// stable at the gate, its height still fading there: more than can fade
// before touchdown, its crew goes round (#1681: it was snapped 1,672 ft
// onto the runway). One 300 ft above, as MSFS AI brings them in, is
// stable. The user's own with held gates lands from it (joinHeld).
func TestJoinUnstableAtGate(t *testing.T) {
	normal := joinFlight(t, 300)
	normal.ctl.mu.Lock()
	unstable := normal.ctl.last.Unstable
	normal.ctl.mu.Unlock()
	if unstable {
		t.Error("300 ft above the final at the join: not stable at the gate")
	}
	high := joinFlight(t, 7000)
	high.ctl.mu.Lock()
	unstable = high.ctl.last.Unstable
	high.ctl.mu.Unlock()
	if !unstable {
		t.Error("7000 ft above the final at the join: stable at the gate")
	}
	high.ctl.mu.Lock()
	round := high.ctl.last.CrewGoAround
	high.ctl.mu.Unlock()
	if !round {
		t.Error("7000 ft above the final at the join: its crew did not go round")
	}
	if fpm := steepestFpm(t, high); fpm > joinBlendMaxFpm+50 {
		t.Errorf("written %.0f fpm after the hand-over, want %.0f at most", fpm, joinBlendMaxFpm)
	}
}

// steepestFpm is j's steepest height change written over any second after
// the hand-over, its touchdown included.
func steepestFpm(t *testing.T, j joinRun) float64 {
	t.Helper()
	if len(j.alts) < 60 {
		t.Fatalf("%d placements after the hand-over", len(j.alts))
	}
	worst := 0.0
	prev := joinWrite{j.alts[0].at.Add(-1e9), j.joinAlt}
	for _, x := range j.alts {
		if dt := x.at.Sub(prev.at); dt.Seconds() >= 1 {
			worst = math.Max(worst, math.Abs((x.ft-prev.ft)/dt.Minutes()))
			prev = x
		}
	}
	return worst
}

// The user's own with held gates is never sent round by its crew (#1096):
// it lands from an approach not stable at the gate, the height faded on
// the way down, never a snap onto the runway (#1681). 4,000 ft high it
// fades at the airliner's rate by touchdown; 7,000 ft high, by 50 ft,
// steeper but continuous.
func TestJoinHeldLandsUnstable(t *testing.T) {
	for _, c := range []struct {
		above, maxFpm float64
	}{{4000, joinBlendMaxFpm + 50}, {7000, 2 * joinBlendMaxFpm}} {
		j := joinFlightAt(t, c.above, 0, true)
		j.ctl.mu.Lock()
		unstable, round := j.ctl.last.Unstable, j.ctl.last.CrewGoAround
		j.ctl.mu.Unlock()
		if !unstable || round {
			t.Errorf("%.0f ft high, held: unstable %v, crew round %v; want unstable, landing", c.above, unstable, round)
		}
		if fpm := steepestFpm(t, j); fpm > c.maxFpm {
			t.Errorf("%.0f ft high, held: written %.0f fpm, want %.0f at most", c.above, fpm, c.maxFpm)
		}
	}
}

// A close-in IFR join (3 NM out, some 1,000 ft up) 300 ft above the
// final, as MSFS AI brings them in, is stable: its gate is the visual
// one, the offset faded by there (#1681: every join below 1,000 ft was
// flagged on its first frame).
func TestJoinCloseInStable(t *testing.T) {
	j := joinFlightAt(t, 300, 3, false)
	j.ctl.mu.Lock()
	unstable := j.ctl.last.Unstable
	j.ctl.mu.Unlock()
	if unstable {
		t.Error("joined 3 NM out 300 ft above the final: not stable at the gate")
	}
}

// The gate judged from the blend alone, on a path coming down at 3° and
// 140 kt: a VFR circuit's final joined at 320 ft is not judged, a
// close-in join at 800 ft with a normal offset is stable at 500 ft, the
// 8 NM join 7,000 ft above is not stable at 1,000 ft (#1681).
func TestJoinGate(t *testing.T) {
	const dt, fpm = 1.0 / 60, 740.0
	for _, c := range []struct {
		name            string
		joinFt, offset  float64
		gate            float64
		wantJudged, bad bool
	}{
		{"VFR circuit final", 320, 300, 0, false, false},
		{"VFR circuit final, high", 320, 1500, 0, false, false},
		{"close-in IFR", 800, 300, StabilisedVisualGateFt, true, false},
		{"just above the gate", 1100, 300, StabilisedVisualGateFt, true, false},
		{"8 NM", 2540, 300, StabilisedGateFt, true, false},
		{"8 NM, 7000 ft high", 2540, 7000, StabilisedGateFt, true, true},
	} {
		b := joinBlend{dFt: c.offset, left: JoinBlendSeconds}
		b.ftFor = joinBlendFtSeconds(c.offset, fpm)
		b.ftLeft, b.gateFt = b.ftFor, gateFor(c.joinFt)
		if b.gateFt != c.gate {
			t.Errorf("%s: gate %.0f ft, want %.0f", c.name, b.gateFt, c.gate)
		}
		judged, bad := false, false
		for h := c.joinFt; h > 0; h -= fpm / 60 * dt {
			b.apply(ApproachPose{HeightFt: h}, dt)
			was := b.gateFt
			if b.judgeGate(h) {
				bad = true
			}
			judged = judged || was > 0 && b.gateFt == 0
		}
		if judged != c.wantJudged || bad != c.bad {
			t.Errorf("%s: judged %v, not stable %v; want %v, %v", c.name, judged, bad, c.wantJudged, c.bad)
		}
	}
}
