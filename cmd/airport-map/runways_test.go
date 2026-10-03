//go:build windows
// +build windows

package main

import "testing"

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
