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
