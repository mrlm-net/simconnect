package traffic

import (
	"math"
	"testing"
	"time"
)

// A frame moves on by the simulator's clock: uneven arrival of the frames
// (our clock) does not make uneven steps; our clock where the simulator's
// is unknown or the step was restarted.
func TestFrameStepBySimulatorClock(t *testing.T) {
	var d groundDrive
	t0 := time.Unix(1000, 0)
	d.lastStep = t0
	d.simNow = 500
	if dt := d.frameStep(t0.Add(20 * time.Millisecond)); math.Abs(dt-0.02) > 1e-9 {
		t.Fatalf("first frame: %.3f s, want our clock's 0.020", dt)
	}
	// Frames 1/60 s apart on the simulator's clock, arriving 5 and 30 ms apart.
	d.simNow = 500 + 1.0/60
	if dt := d.frameStep(t0.Add(25 * time.Millisecond)); math.Abs(dt-1.0/60) > 1e-9 {
		t.Errorf("frame arriving early: %.4f s, want %.4f", dt, 1.0/60)
	}
	d.simNow = 500 + 2.0/60
	if dt := d.frameStep(t0.Add(55 * time.Millisecond)); math.Abs(dt-1.0/60) > 1e-9 {
		t.Errorf("frame arriving late: %.4f s, want %.4f", dt, 1.0/60)
	}
	// The same simulator frame again: no move.
	if dt := d.frameStep(t0.Add(60 * time.Millisecond)); dt != 0 {
		t.Errorf("same frame again: %.4f s, want 0", dt)
	}
	// The step restarted (a new path): our clock from then.
	d.lastStep = t0.Add(100 * time.Millisecond)
	d.simNow = 500 + 10.0/60
	if dt := d.frameStep(t0.Add(110 * time.Millisecond)); math.Abs(dt-0.01) > 1e-9 {
		t.Errorf("after a restart: %.4f s, want our clock's 0.010", dt)
	}
}
