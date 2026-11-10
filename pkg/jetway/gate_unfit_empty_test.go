package jetway

import (
	"errors"
	"testing"
	"time"
)

// A gate whose helpers go elsewhere more than they dock (GateUnfitMin),
// left empty after its last helper went elsewhere, holds up no other dock:
// the sim does not give it the next helper, and a refill only put a
// placeholder on a docked gate, over and over (live LKPR 2026-11-09: stand
// 19, 11 misses and 3 docks; every arrival refused "gates still filling"
// from 18:46 for over ten minutes, stairs instead).
func TestHelpersUnfitEmptyGateHoldsNoDock(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"A320", "BCS3"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	h.see(jw(7, 0, 0), jw(9, 0, 0), jw(11, 0, 0))
	h.GateRecord("EDDM", 9, 3, 11)
	h.mu.Lock()
	h.gate("EDDM", 7).placeholder, h.gate("EDDM", 11).placeholder = 101, 103
	for _, s := range []int{7, 9, 11} {
		if h.accepted["EDDM"] == nil {
			h.accepted["EDDM"] = map[int]bool{}
		}
		h.accepted["EDDM"][s] = true
	}
	g := h.gate("EDDM", 9) // its helper went elsewhere: failed, nothing on it
	g.status, g.since = StatusFailed, now
	h.mu.Unlock()
	h.Update(now)
	if err := h.Dock("EDDM", 11, "FSLTL_FAIB_A320_CSA"); err != nil {
		t.Fatalf("dock at 11 with unfit 9 empty: %v", err)
	}
	// A gate not known unfit, empty, still holds docks until filled again.
	h.GateRecord("EDDM", 9, 3, 1)
	h.mu.Lock()
	h.gate("EDDM", 11).status, h.gate("EDDM", 11).helper = StatusReserved, 0
	h.mu.Unlock()
	if err := h.Dock("EDDM", 7, "FSLTL_FAIB_A320_CSA"); !errors.Is(err, ErrFilling) {
		t.Fatalf("dock at 7 with fit 9 empty: %v, want ErrFilling", err)
	}
}
