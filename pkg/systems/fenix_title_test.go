package systems

import "testing"

// TestFenixByTitle: its package unknown, a Fenix is told by the title it
// reports (FenixA319…, no "FNX" in it), not left on the A320 family's base.
func TestFenixByTitle(t *testing.T) {
	if p := For(Aircraft{Title: "FenixA319 CFM WF HD"}); p.Name != "Fenix A320 family" {
		t.Errorf("got profile %q", p.Name)
	}
	if p := For(Aircraft{Title: "Airbus A320neo"}); p.Name == "Fenix A320 family" {
		t.Error("a stock A320 took the Fenix profile")
	}
}
