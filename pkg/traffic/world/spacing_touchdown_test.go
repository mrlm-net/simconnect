package world

import "testing"

// CSA814 (live): 7.2 NM to go at final approach speed, 4.6 NM behind a
// Citation 2.6 NM from touchdown at 110 kt: 3.8 NM behind when it lands,
// beyond the 3 NM minimum — it lands, the departure gap is what is lost.
// One far too close still goes around.
func TestSpacingAtTouchdown(t *testing.T) {
	if got := spacingAtTouchdown(7.2, 140, 2.6, 110); got < 3 {
		t.Errorf("CSA814: %.1f NM at the Citation's touchdown, want 3 or more (no go-around)", got)
	}
	if got := spacingAtTouchdown(4.0, 160, 2.0, 100); got >= 3 {
		t.Errorf("closing fast: %.1f NM at touchdown, want under 3 (go-around)", got)
	}
}
