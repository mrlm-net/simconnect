//go:build windows
// +build windows

package world

import (
	"testing"
	"time"
)

// A take-off clearance given is cancelled for someone on the runway, never
// for an arrival closing in.
func TestCancelTakeoffFor(t *testing.T) {
	for why, want := range map[string]bool{
		"AFR558 on the runway":      true,
		"DLH1402 on a 2.4 NM final": false, // lined up, it goes (WZZ1387)
		"DLH1402 on a 3.9 NM final": false,
		"KLM628 lands in 1m10s":     false,
		"1m20s behind QTR1":         false,
		"number 2 for departure":    false,
	} {
		if got := cancelTakeoffFor(why); got != want {
			t.Errorf("%q: %v, want %v", why, got, want)
		}
	}
}

// The vertical speed from the altitude change, smoothed: a descent is a
// descent whatever VERTICAL SPEED reports for an aircraft we place.
func TestDerivedFpm(t *testing.T) {
	t0 := time.Now()
	first := fix{at: t0, altFt: 3000}
	if vs := derivedFpm(fix{}, 3000, t0); vs != 0 {
		t.Errorf("no previous fix: %.0f", vs)
	}
	vs := derivedFpm(first, 2988, t0.Add(time.Second)) // 720 fpm down, half of it at first
	if vs > -300 || vs < -400 {
		t.Errorf("descending 720 fpm: %.0f after one second", vs)
	}
	second := fix{at: t0.Add(time.Second), altFt: 2988, vs: vs}
	if vs = derivedFpm(second, 2976, t0.Add(2*time.Second)); vs > -500 {
		t.Errorf("still descending: %.0f", vs)
	}
}
