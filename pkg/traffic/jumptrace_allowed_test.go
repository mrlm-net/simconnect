package traffic

import (
	"testing"
	"time"
)

// TestJumpAllowedAfterALateFrame: 8.5 m in 59 ms at 135 kt, right after a late
// frame, is simulator time catching up; the same with no late frame is a jump.
func TestJumpAllowedAfterALateFrame(t *testing.T) {
	const d, kts, dt = 8.5, 135.0, 59 * time.Millisecond
	if a := jumpAllowedM(kts, dt, 120*time.Millisecond); d > a {
		t.Errorf("after a late frame: %.1f m allowed", a)
	}
	if a := jumpAllowedM(kts, dt, 33*time.Millisecond); d <= a {
		t.Errorf("no late frame: %.1f m allowed", a)
	}
}
