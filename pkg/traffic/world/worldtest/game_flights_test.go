package worldtest

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestGameFlightsAreScheduled: the game's flights are the schedule's: a
// departure asks for its clearance to somewhere, and each flies a model of its
// own airline, not one Air France A320 for every call sign (#1408).
func TestGameFlightsAreScheduled(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if _, err := h.World.Do(http.MethodPost, "/api/game", map[string]any{"on": true, "icao": "LKPR", "intervalSec": 30}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]world.ControlView{}
	h.Until(20*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Manual {
				seen[v.Tail] = v
			}
		}
		return len(seen) >= 6
	})
	if len(seen) < 6 {
		t.Fatalf("%d flights in 20 minutes", len(seen))
	}
	models := map[string]bool{}
	for tail, v := range seen {
		t.Logf("%s %s %s", tail, v.Kind, v.Model)
		models[v.Model] = true
	}
	if len(models) < 2 {
		t.Errorf("every flight the same model: %v", models)
	}
	asked := 0
	for _, tx := range h.Heard() {
		if strings.Contains(tx.Text, "request clearance") {
			asked++
			if !strings.Contains(tx.Text, "request clearance to ") {
				t.Errorf("clearance to nowhere: %q", tx.Text)
			}
		}
	}
	if asked == 0 {
		t.Error("no departure asked for its clearance")
	}
}
