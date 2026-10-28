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

// TestStationMode: a station's mode as set, "safety-net" spelt any way as
// the safety net, "" and anything unknown as assist (#1096).
func TestStationMode(t *testing.T) {
	for in, want := range map[string]string{"": ModeAssist, "Assist": ModeAssist, "silent": ModeSilent, "safety-net": ModeSafetyNet, "Safety_Net": ModeSafetyNet, "safetynet": ModeSafetyNet, "loud": ModeAssist} {
		if got := stationMode(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	w := &World{st: &state{core: newCore(&trafficLog{})}}
	w.SetStaffed([]StaffedStation{{ICAO: "LKPR", Position: "approach", Mode: "Safety-Net"}})
	if m := w.Staffed()[0].Mode; m != ModeSafetyNet {
		t.Errorf("kept as %q, want %q", m, ModeSafetyNet)
	}
}

// TestStaffedByPlayer: a player's stations replace only their own; one
// station staffed by a player wins over its whole position staffed by
// another (#1096).
func TestStaffedByPlayer(t *testing.T) {
	w := &World{st: &state{core: newCore(&trafficLog{})}}
	w.SetStaffedBy("Ground", []StaffedStation{{ICAO: "LKPR", Position: "ground"}})
	w.SetStaffedBy("alice", []StaffedStation{{ICAO: "LKPR", Station: "Ruzyne Ground North"}})
	w.SetStaffedBy("ground", []StaffedStation{{ICAO: "LKPR", Position: "ground"}})
	if n := len(w.Staffed()); n != 2 {
		t.Fatalf("%d stations, want 2: %+v", n, w.Staffed())
	}
	if s, _ := w.st.core.staffedAt("LKPR", traffic.PosGround, "Ruzyne Ground North"); s.By != "alice" {
		t.Errorf("Ground North by %q, want alice", s.By)
	}
	if s, _ := w.st.core.staffedAt("LKPR", traffic.PosGround, "Ruzyne Ground South"); s.By != "ground" {
		t.Errorf("Ground South by %q, want ground", s.By)
	}
}
