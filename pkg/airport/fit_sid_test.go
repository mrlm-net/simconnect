package airport

import (
	"encoding/json"
	"os"
	"testing"
)

// The SID for the runway in use on the filed route (LKPR, from the
// facility data): the filed one where it serves the runway; else the one
// ending on the route, the closest name first.
func TestFitSID(t *testing.T) {
	b, err := os.ReadFile("testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	route := []string{"VOZ", "BODAL", "BNO"}
	for _, c := range []struct{ filed, runway, want string }{
		{"VOZ5M", "24", "VOZ5M"}, // serves 24: kept
		{"VOZ5M", "06", "VOZ5D"}, // 06: the closest name to VOZ
		{"VOZ4A", "06", "VOZ4E"},
		{"", "30", "VOZ4B"}, // none filed: one to VOZ (VOZ4B and VOZ5N tie; first)
	} {
		s, _, ok := p.FitSID(c.filed, c.runway, route)
		if !ok || s.Name != c.want {
			t.Errorf("%s for %s: %q (%v), want %s", c.filed, c.runway, s.Name, ok, c.want)
		}
	}
	if _, _, ok := p.FitSID("VOZ5M", "06", []string{"ZZZZZ"}); ok {
		t.Error("a SID for a route none of them reaches")
	}
}
