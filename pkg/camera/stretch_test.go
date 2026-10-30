package camera

import (
	"testing"
	"time"
)

// TestStretch: a shot played longer goes through the same poses, slower;
// not shorter than it is, nor past MaxStretch; the cap stays on the rest.
func TestStretch(t *testing.T) {
	from, to := Pose{FovDeg: 20}, Pose{FovDeg: 60}
	m := Move("m", from, to, 8*time.Second, Linear)
	s := Stretch(m, 80*time.Second)
	if s.Length() != 80*time.Second || s.Name() != "m" {
		t.Fatalf("%v %q", s.Length(), s.Name())
	}
	for _, f := range []float64{0, 0.25, 0.5, 1} {
		got := s.PoseAt(time.Duration(f * float64(80*time.Second))).FovDeg
		want := m.PoseAt(time.Duration(f * float64(8*time.Second))).FovDeg
		if d := got - want; d > 1e-6 || d < -1e-6 {
			t.Errorf("at %.2f: fov %v, want %v", f, got, want)
		}
	}
	if got := Stretch(m, 4*time.Second); got.Length() != 8*time.Second {
		t.Errorf("shorter: %v", got.Length())
	}
	if got := Stretch(m, time.Hour); got.Length() != MaxStretch {
		t.Errorf("past the limit: %v", got.Length())
	}
	if got := Move("m", from, to, time.Minute, nil); got.Length() != MaxShot {
		t.Errorf("a move not stretched: %v, want MaxShot", got.Length())
	}
}
