package jetway

import (
	"testing"
	"time"
)

// OnDock hears a dock whose jetway never came (docking timed out, the
// helper on its gate) with the aircraft's model, and a dock done; a helper
// the sim put on another gate is not told (live, LKPR C21 2026-11-08:
// "docking timed out", the aircraft waited for a jetway that never came).
func TestHelpersOnDock(t *testing.T) {
	type heard struct {
		stand  int
		model  string
		docked bool
	}
	var got []heard
	f := &fakeSim{}
	h := NewHelpers(f, HelperOptions{Types: []string{"A321"}, RetractPlan: func(string) (string, error) { return "out.pln", nil },
		OnDock: func(icao string, stand int, model string, docked bool) {
			if icao != "EDDM" {
				t.Errorf("icao %q", icao)
			}
			got = append(got, heard{stand, model, docked})
		}})
	now := time.Now()
	h.Update(now)
	h.see(jw(7, 0, 0), jw(9, 0, 0))
	h.mu.Lock()
	h.gate("EDDM", 7).placeholder, h.gate("EDDM", 7).status = 600, StatusReserved
	h.gate("EDDM", 9).placeholder, h.gate("EDDM", 9).status = 601, StatusReserved
	h.mu.Unlock()

	// Stand 9: on its gate, never docks.
	if err := h.Dock("EDDM", 9, "A321"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	h.assign(f, 700)
	h.see(jw(9, 1, 700))
	h.Update(now)
	if len(got) != 0 {
		t.Fatalf("heard %v before the timeout", got)
	}
	now = now.Add(DockTimeout + time.Second)
	h.Update(now)
	if len(got) != 1 || got[0] != (heard{9, "A321", false}) {
		t.Fatalf("heard %v, want stand 9 A321 not docked", got)
	}

	// Stand 7: docks.
	now = now.Add(DockSettle + time.Second)
	h.Update(now)
	if err := h.Dock("EDDM", 7, "A321"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(HelperDelay)
	h.Update(now)
	h.assign(f, 701)
	h.see(jw(7, 1, 701))
	h.see(jw(7, 7, 701))
	h.see(jw(7, 7, 701)) // listed docked again: heard once
	if len(got) != 2 || got[1] != (heard{7, "A321", true}) {
		t.Fatalf("heard %v, want stand 7 A321 docked once", got)
	}
}
