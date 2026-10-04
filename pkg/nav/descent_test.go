//go:build windows

package nav

import (
	"math"
	"testing"
)

// TestTopOfDescent: FL360 to 4000 ft at 1800 fpm and 420 kt: 26000 ft above
// FL100 at 420 kt (101.1 NM), 6000 ft below at 250 kt (13.9 NM), 17 NM to
// slow from 420 to 250 kt, plus the margin (#693).
func TestTopOfDescent(t *testing.T) {
	p := PerformanceFor("A320")
	got := TopOfDescent(p, 36000, 4000, 420, 5)
	want := 26000.0/1800/60*420 + 6000.0/1800/60*250 + 17 + 5
	if math.Abs(got-want) > 0.01 {
		t.Errorf("%.1f NM, want %.1f", got, want)
	}
	if d := TopOfDescent(p, 8000, 3000, 200, 0); math.Abs(d-5000.0/1800/60*200) > 0.01 {
		t.Errorf("below FL100 at 200 kt: %.1f NM", d)
	}
	if d := TopOfDescent(p, 3000, 4000, 300, 5); d != 0 {
		t.Errorf("below the target: %.1f, want 0", d)
	}
	if fast, slow := TopOfDescent(p, 36000, 4000, 480, 0), TopOfDescent(p, 36000, 4000, 380, 0); fast <= slow {
		t.Errorf("a tailwind (480 kt) %.1f NM not further out than 380 kt %.1f", fast, slow)
	}
}
