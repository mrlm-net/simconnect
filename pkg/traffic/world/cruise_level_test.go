package world

import (
	"math"
	"testing"
)

// En route levels are the semicircular rule's for the leg's track: two
// opposite ways at one planned level end a level apart, never on one (live:
// KLM1433 westbound stepped up to FL390 into DLH1506 eastbound at FL390;
// DLH320 and ENT384, opposite ways, both at FL356).
func TestCruiseLevelSemicircular(t *testing.T) {
	for _, c := range []struct{ ft, trk, want float64 }{
		{39000, 290, 38000}, // KLM1433 westbound: not FL390
		{39000, 110, 39000}, // DLH1506 eastbound
		{35600, 40, 35000},  // DLH320 north-east
		{35600, 300, 36000}, // ENT384 north-west
		{28000, 10, 27000},
		{29400, 200, 30000},
		{44000, 90, 45000},  // above FL410: 4000 ft apart
		{43000, 270, 43000}, //
		{8000, 90, 8000},    // below FL100: as planned
		{36000, -10, 36000}, // 350°: westbound
	} {
		if got := cruiseLevelFt(c.ft, c.trk); got != c.want {
			t.Errorf("%.0f ft on %.0f°: FL%.0f, want FL%.0f", c.ft, c.trk, got/100, c.want/100)
		}
	}
	for ft := 10000.0; ft <= 41000; ft += 100 {
		if e, w := cruiseLevelFt(ft, 90), cruiseLevelFt(ft, 270); math.Abs(e-w) < 1000 {
			t.Fatalf("%.0f ft: east FL%.0f and west FL%.0f under 1000 ft apart", ft, e/100, w/100)
		}
	}
}
