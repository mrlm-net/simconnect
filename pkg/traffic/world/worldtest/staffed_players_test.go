package worldtest

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestStaffedByPlayers: in network play each player staffs their own
// stations (#1096): one playing as tower band-boxes ground, another
// staffs approach, neither replacing the other's. The tower player may
// clear a departure on ground, the approach player and ground's own
// position may not. Before, the second player's stations replaced the
// first's, and a client cleared only on its own position's frequency.
func TestStaffedByPlayers(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	put := func(as string, list []world.StaffedStation) {
		t.Helper()
		if _, err := h.World.Do(http.MethodPut, "/api/stations/staffed?as="+as, list); err != nil {
			t.Fatal(err)
		}
	}
	put("tower", []world.StaffedStation{{ICAO: "LKPR", Position: "tower"}, {ICAO: "LKPR", Position: "ground"}})
	put("approach", []world.StaffedStation{{ICAO: "LKPR", Position: "approach"}})
	by := map[string]string{}
	for _, s := range h.World.Staffed() {
		by[s.Position] = s.By
	}
	if by["tower"] != "tower" || by["ground"] != "tower" || by["approach"] != "approach" {
		t.Fatalf("staffed by %v: want tower and ground by tower, approach by approach", by)
	}
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 4, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	if !h.Until(30*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && v.ATC == "ground" {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure with ground in 30 minutes")
	}
	manual := func(as string) error {
		_, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/manual?on=1&as=%s", dep.ID, as), nil)
		return err
	}
	if err := manual("tower"); err != nil {
		t.Errorf("the tower player band-boxing ground: %v", err)
	}
	for _, as := range []string{"approach", "ground"} {
		if manual(as) == nil {
			t.Errorf("as %s: cleared %s on ground, staffed by the tower player", as, dep.Tail)
		}
	}
	put("approach", nil)
	if n := len(h.World.Staffed()); n != 2 {
		t.Errorf("approach player staffing nothing: %d stations left, want tower's 2", n)
	}
}
