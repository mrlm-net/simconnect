package nav

import (
	"slices"
	"testing"
)

// TestLevelRule: the cruising levels of the semicircular rule as
// Wikipedia's "Flight level" gives them, below FL290, in RVSM, above FL410,
// without RVSM, north/south (Italy, France, Portugal, Spain: southbound
// odd; New Zealand: southbound even), and VFR's +500 ft.
func TestLevelRule(t *testing.T) {
	levels := func(r LevelRule, track float64, vfr bool, from, to int) []int {
		var out []int
		for fl := from; fl <= to; fl++ {
			if r.Valid(fl, track, vfr) {
				out = append(out, fl)
			}
		}
		return out
	}
	ew := LevelRule{}
	for _, c := range []struct {
		name     string
		r        LevelRule
		track    float64
		vfr      bool
		from, to int
		want     []int
	}{
		{"east below FL290", ew, 90, false, 240, 280, []int{250, 270}},
		{"west below FL290", ew, 270, false, 240, 280, []int{240, 260, 280}},
		{"east RVSM", ew, 45, false, 290, 410, []int{290, 310, 330, 350, 370, 390, 410}},
		{"west RVSM", ew, 225, false, 290, 410, []int{300, 320, 340, 360, 380, 400}},
		{"east above FL410", ew, 10, false, 411, 500, []int{450, 490}},
		{"west above FL410", ew, 190, false, 411, 520, []int{430, 470, 510}},
		{"east no RVSM", LevelRule{NoRVSM: true}, 90, false, 290, 410, []int{290, 330, 370, 410}},
		{"west no RVSM", LevelRule{NoRVSM: true}, 270, false, 290, 410, []int{310, 350, 390}},
		{"southbound Italy", LevelRuleFor("LIPZ"), 170, false, 330, 380, []int{330, 350, 370}},
		{"northbound Italy", LevelRuleFor("LIPZ"), 350, false, 330, 380, []int{340, 360, 380}},
		{"southbound New Zealand", LevelRuleFor("NZAA"), 180, false, 330, 380, []int{340, 360, 380}},
		{"east VFR", ew, 90, true, 30, 100, []int{35, 55, 75, 95}},
		{"west VFR", ew, 270, true, 30, 100, []int{45, 65, 85}},
		{"no VFR from FL200", ew, 90, true, 195, 260, []int{195}},
	} {
		if got := levels(c.r, c.track, c.vfr, c.from, c.to); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if LevelRuleFor("LKPR") != (LevelRule{}) || LevelRuleFor("LFPG") != (LevelRule{NorthSouth: true, SouthOdd: true}) {
		t.Error("rules by country")
	}
}

// TestCruiseLevelPlanned: an airliner on a long flight is planned at its
// normal ceiling, not its certified one (a 737 at FL400 was too high), a
// business turboprop well below the airliners, a business jet above them
// on the 4,000 ft levels.
func TestCruiseLevelPlanned(t *testing.T) {
	for _, c := range []struct {
		typ   string
		track float64
		want  int
	}{
		{"B738", 270, 380},
		{"B738", 90, 370},
		{"A320", 270, 360},
		{"B350", 180, 260},
		{"C750", 90, 450},
		{"C700", 270, 430},
	} {
		if got := CruiseLevel(PerformanceFor(c.typ), c.track, 900, 1200, 1200); got != c.want {
			t.Errorf("%s on %.0f°: FL%d, want FL%d", c.typ, c.track, got, c.want)
		}
	}
}
