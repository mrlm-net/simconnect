package world

import (
	"testing"
	"time"
)

// A change for a conflict is looked at again once flown a minute (live,
// EZY1384 turned for OKIXH, nothing more done while they lost separation).
func TestConflictChangedBefore(t *testing.T) {
	now := time.Now()
	w := &conflictWatch{busy: map[string]time.Time{}, changed: map[string]time.Time{}}
	if w.changedBefore(now, "A", "B") {
		t.Error("neither changed: want false")
	}
	w.busy["A"], w.changed["A"] = now.Add(4*time.Minute+30*time.Second), now.Add(-30*time.Second)
	if w.changedBefore(now, "A", "B") {
		t.Error("changed 30 s ago: want false, time to see it work")
	}
	w.changed["A"] = now.Add(-90 * time.Second)
	if !w.changedBefore(now, "A", "B") {
		t.Error("changed 90 s ago: want true")
	}
	w.busy["B"] = now.Add(time.Minute) // an arrival held on its own check, no change given
	if w.changedBefore(now, "A", "B") {
		t.Error("B busy without a change of the watch: want false")
	}
}
