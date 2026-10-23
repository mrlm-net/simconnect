package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestStaffedAt: a whole position staffed covers its stations; a single
// station only itself; another airport nothing (#1096).
func TestStaffedAt(t *testing.T) {
	w := &World{st: &state{core: newCore(&trafficLog{})}}
	w.SetStaffed([]StaffedStation{{ICAO: "lkpr", Position: "Tower"}, {ICAO: "LKPR", Station: "Ruzyne Ground North"}})
	for _, c := range []struct {
		icao    string
		pos     traffic.Position
		station string
		want    bool
	}{
		{"LKPR", traffic.PosTower, "Ruzyne Tower", true},
		{"LKPR", traffic.PosGround, "Ruzyne Ground North", true},
		{"LKPR", traffic.PosGround, "Ruzyne Ground South", false},
		{"LKPR", traffic.PosApproach, "Praha Approach", false},
		{"EDDF", traffic.PosTower, "Frankfurt Tower", false},
	} {
		if _, got := w.st.core.staffedAt(c.icao, c.pos, c.station); got != c.want {
			t.Errorf("%s %s %q: staffed %v, want %v", c.icao, c.pos, c.station, got, c.want)
		}
	}
}

// TestAmendNeedsAClearance: an amendment is refused when none is waiting and
// on an arrival, and asking for nothing is not an error — the clearance then
// goes as filed (#1164).
func TestAmendNeedsAClearance(t *testing.T) {
	dep := &controlled{Tail: "CSA123", dep: &fakeDep{}}
	if err := dep.amendClearance("24", "", ""); err == nil {
		t.Error("amend with none waiting: no error")
	}
	if err := dep.amendClearance("", "", ""); err != nil {
		t.Errorf("amend asking for nothing: %v", err)
	}
	arr := &controlled{Tail: "CSA456"}
	if err := arr.amendClearance("24", "", ""); err == nil {
		t.Error("amend on an arrival: no error")
	}
	// Held but no airport: refused rather than panicking on a nil graph.
	clr := traffic.ClearedDeparture("CSA123", traffic.DepartureClearance{Runway: "24"})
	dep.clearanceDue = &clr
	if err := dep.amendClearance("", "", "4000"); err == nil {
		t.Error("amend with no airport: no error")
	}
}
