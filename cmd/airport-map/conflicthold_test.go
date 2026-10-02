//go:build windows
// +build windows

package main

import (
	"testing"
	"time"
)

// TestConflictHold: an arrival holding for a conflict is kept holding for
// conflictHoldMin and while the pair is still predicted in conflict, then
// released (live: CSA1257 told to hold at ERASU for AFR1552 was let go a
// second later by the sequence's zero delay, and they met at 0.5 NM).
func TestConflictHold(t *testing.T) {
	now := time.Date(2026, 10, 2, 21, 17, 0, 0, time.UTC)
	conflict := true
	q := &sequences{conflictHeld: map[string]conflictHold{"CSA1257": {other: "AFR1552", at: now}},
		inConflict: func(a, b string) bool { return conflict && a == "CSA1257" && b == "AFR1552" }}
	if !q.keepHolding(now.Add(time.Second), "CSA1257") {
		t.Fatal("released a second after the conflict hold")
	}
	if !q.keepHolding(now.Add(3*time.Minute), "CSA1257") {
		t.Fatal("released while still in conflict")
	}
	conflict = false
	if !q.keepHolding(now.Add(time.Minute), "CSA1257") {
		t.Fatal("released before conflictHoldMin")
	}
	if q.keepHolding(now.Add(3*time.Minute), "CSA1257") {
		t.Fatal("kept holding after the conflict was over")
	}
	if q.keepHolding(now.Add(3*time.Minute), "OTHER") {
		t.Fatal("an aircraft not holding for a conflict kept")
	}
}
