package worldtest

import (
	"net/http"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestEmergencyCancelClearsSquawk (#1533): a cancelled emergency gives the
// arrival its code back ("": none), not 7700 for the rest of the flight.
func TestEmergencyCancelClearsSquawk(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 8}); err != nil {
		t.Fatal(err)
	}
	var tail, before string
	if !h.Until(30*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.State == "approaching" {
				tail, before = v.Tail, v.Squawk
				return true
			}
		}
		return false
	}) {
		t.Fatal("no arrival approaching")
	}
	squawk := func() string {
		v, ok := h.Aircraft(tail)
		if !ok {
			t.Fatalf("%s gone", tail)
		}
		return v.Squawk
	}
	if _, err := h.World.Do(http.MethodPost, "/api/emergency", map[string]any{"tail": tail, "kind": "medical"}); err != nil {
		t.Fatal(err)
	}
	if s := squawk(); s != "7700" {
		t.Fatalf("%s declared: squawk %q, want 7700", tail, s)
	}
	if _, err := h.World.Do(http.MethodPost, "/api/emergency", map[string]any{"tail": tail, "on": false}); err != nil {
		t.Fatal(err)
	}
	if s := squawk(); s != before {
		t.Fatalf("%s cancelled: squawk %q, want %q as before", tail, s, before)
	}
}
