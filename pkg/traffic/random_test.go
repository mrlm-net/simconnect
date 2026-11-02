package traffic

import "testing"

// TestRandomSeed: seeded, the draws and the generators repeat; unseeded they
// come from elsewhere and do not advance the seeded stream (#1390).
func TestRandomSeed(t *testing.T) {
	defer SetRandomSeed(0)
	draw := func() [4]float64 {
		SetRandomSeed(42)
		a, b := NewRandom(1), NewRandom(1)
		return [4]float64{RandomFloat64(), float64(RandomIntN(1000)), a.Float64(), b.Float64()}
	}
	x, y := draw(), draw()
	if x != y {
		t.Errorf("same seed: %v, then %v", x, y)
	}
	if x[2] == x[3] {
		t.Errorf("two generators drew the same %v", x[2])
	}
	SetRandomSeed(0)
	if a, b := NewRandom(1), NewRandom(2); a == nil || b == nil {
		t.Error("no generator unseeded")
	}
	if n := RandomIntN(5); n < 0 || n >= 5 {
		t.Errorf("unseeded draw %d", n)
	}
}
