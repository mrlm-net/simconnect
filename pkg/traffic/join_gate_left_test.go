package traffic

import "testing"

// The gate judged ahead of the join as the join's blend judges it: 300 ft
// above an 8 NM final fades by the gate, 4,000 ft does not, a VFR
// circuit's final has none (#1681 follow-up).
func TestGateLeftFt(t *testing.T) {
	const fpm = 740.0
	for _, c := range []struct {
		name          string
		dFt, heightFt float64
		high          bool
	}{
		{"8 NM, 300 ft above", 300, 2540, false},
		{"8 NM, 4000 ft above", 4000, 2540, true},
		{"VFR circuit final, 1500 ft above", 1500, 320, false},
	} {
		if high := gateLeftFt(c.dFt, c.heightFt, fpm) > stableOffsetFt; high != c.high {
			t.Errorf("%s: too high %v, want %v", c.name, high, c.high)
		}
	}
}
