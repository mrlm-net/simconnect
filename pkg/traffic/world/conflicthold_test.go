package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
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

// An aircraft in a conflict, or just resolved, is engaged: the sequencer
// gives it no shortcut (#785: TVS979 sent direct RATEV after it was slowed
// for THY319).
func TestEngaged(t *testing.T) {
	now := time.Date(2026, 10, 5, 21, 39, 0, 0, time.UTC)
	w := &conflictWatch{busy: map[string]time.Time{}, slowed: map[string]bool{}, leveled: map[string]bool{}, stopped: map[string]stoppedLevel{}}
	w.now = []traffic.Conflict{{A: "TVS979", B: "THY319"}}
	if !w.engaged("TVS979", now) || !w.engaged("THY319", now) || w.engaged("RYR657", now) {
		t.Error("in a predicted conflict")
	}
	w.now = nil
	w.busy["TVS979"] = now.Add(-time.Minute)
	if !w.engaged("TVS979", now) {
		t.Error("a minute after its resolution: no longer engaged")
	}
	if w.engaged("TVS979", now.Add(5*time.Minute)) {
		t.Error("still engaged 6 minutes after")
	}
	w.slowed["THY319"] = true
	if !w.engaged("THY319", now) {
		t.Error("slowed for a conflict: not engaged")
	}
}
