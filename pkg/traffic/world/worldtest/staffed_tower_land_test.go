package worldtest

import (
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestStaffedTowerLandStands (#1540): on a staffed tower an arrival the
// player clears to land never hears the AI's tower say "continue approach"
// after it (the AI answered its check-in in its turn, whatever the player
// had said).
func TestStaffedTowerLandStands(t *testing.T) {
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
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "tower"}})
	if _, err := h.World.Do(http.MethodPost, "/api/schedule", map[string]any{"enabled": true, "icao": "LKPR", "vfr": false}); err != nil {
		t.Fatal(err)
	}
	clearedAt := map[string]int{} // tail: transmissions heard when cleared
	if !h.Until(90*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if _, done := clearedAt[v.Tail]; done || v.Kind != "arrival" || !slices.Contains(v.Actions, "land") {
				continue
			}
			if err := h.World.Clear(v.ID, "land"); err == nil {
				clearedAt[v.Tail] = len(h.Heard())
			}
		}
		return len(clearedAt) >= 3
	}) {
		t.Fatalf("%d arrivals cleared to land in 90 simulated minutes, want 3", len(clearedAt))
	}
	h.Advance(30 * time.Second)
	heard := h.Heard()
	for tail, n := range clearedAt {
		for _, tx := range heard[n:] {
			if tx.Callsign == tail && !tx.Pilot && tx.Intent == traffic.IntentCircuitInstr && tx.Params[traffic.ParamInstr] == traffic.InstrContinue {
				t.Errorf("%s cleared to land by the player, then the AI's %q", tail, tx.Text)
			}
		}
	}
}
