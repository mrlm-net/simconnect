package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestGoAroundFixes: after its go-around an IFR arrival shows its fixes,
// not the circuit's turning points (live, DLH792 shown "BASE" on vectors);
// a VFR circuit arrival shows its circuit.
func TestGoAroundFixes(t *testing.T) {
	fx := []airport.NavPoint{{Ident: "CROSSWIND"}, {Ident: "DOWNWIND"}, {Ident: "PR532"}, {Ident: "BASE"}, {Ident: "FINAL"}}
	if got := goAroundFixes(fx, false); len(got) != 1 || got[0].Ident != "PR532" {
		t.Errorf("IFR: %+v, want PR532 only", got)
	}
	if got := goAroundFixes(fx, true); len(got) != len(fx) {
		t.Errorf("VFR circuit: %+v, want all %d", got, len(fx))
	}
}
