package traffic

import (
	"math"
	"testing"
)

// goAroundDescent: a circuit at circuit height keeps its downwind; one far
// above its join comes down at the approach's descent, its downwind and
// final extended for it.
func TestGoAroundDescentPlan(t *testing.T) {
	for _, c := range []struct {
		name               string
		circuitFt, wantExt float64
	}{{"circuit height", 3000, 0}, {"LKPR 5000", 5000, 0}, {"14000 (TNCM before #1659)", 14000, 8}} {
		ext, down, align := goAroundDescent(8, 3800, c.circuitFt)
		if math.Abs(ext-c.wantExt) > 0.5 {
			t.Errorf("%s: extended %.1f NM, want about %.0f", c.name, ext, c.wantExt)
		}
		base := math.Hypot(ProcedureAlignNm-1, GoAroundOffsetNm)
		if g := (down - align) / base; g > ProcedureDescentFtPerNm+1 {
			t.Errorf("%s: base %.0f ft/NM", c.name, g)
		}
		if g := (c.circuitFt - down) / (8 + 1 + ext); g > ProcedureDescentFtPerNm+1 {
			t.Errorf("%s: downwind %.0f ft/NM", c.name, g)
		}
		if g := (align - 3800) / (ProcedureAlignNm + ext); g > ProcedureDescentFtPerNm+1 {
			t.Errorf("%s: final %.0f ft/NM", c.name, g)
		}
	}
}
