package jetway

import (
	"slices"
	"testing"
	"time"
)

// A helper's AI control is released once its jetway docks, as a
// placeholder's is, so the sim keeps its gate taken: left with the AI, the
// next helper went to that docked gate (live, LKPR C23 and B15). Once, not
// at every list.
func TestHelpersDockedReleased(t *testing.T) {
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"A320"}, RetractPlan: func(string) (string, error) { return "out.pln", nil }})
	now := time.Now()
	h.Update(now)
	h.see(jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	h.gate("EDDM", 9).placeholder, h.gate("EDDM", 9).status = 600, StatusReserved
	h.mu.Unlock()
	if err := h.Dock("EDDM", 9, "A320"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	h.assign(f, 201)
	h.see(jw(9, 1, 201))
	if slices.Contains(f.released, 201) {
		t.Fatal("released before its jetway docked")
	}
	h.see(jw(9, 7, 201))
	h.see(jw(9, 7, 201))
	if n := countOf(f.released, 201); n != 1 {
		t.Fatalf("released %d times, want once (%v)", n, f.released)
	}
	// Undocked as before: the retract plan, kept as the placeholder.
	h.Undock("EDDM", 9)
	if h.Status("EDDM", 9) != StatusUndocking || f.plans[len(f.plans)-1] != 201 {
		t.Fatalf("stand 9 %s, plans %v", h.Status("EDDM", 9), f.plans)
	}
}

func countOf(s []uint32, v uint32) int {
	n := 0
	for _, x := range s {
		if x == v {
			n++
		}
	}
	return n
}
