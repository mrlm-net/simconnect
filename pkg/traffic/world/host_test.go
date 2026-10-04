//go:build windows
// +build windows

package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestHostAPI: a World not connected answers its API in process, counts
// what Feed drops, and keeps the host's player clearance.
func TestHostAPI(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	var status struct {
		Connected bool `json:"connected"`
	}
	if err := w.Get("/api/status", &status); err != nil || status.Connected {
		t.Fatalf("status %+v %v", status, err)
	}
	if s := w.Snapshot(); s.Connected || len(s.Aircraft) != 0 {
		t.Errorf("snapshot %+v", s)
	}
	if _, err := w.Do("POST", "/api/control/1/taxi", nil); err == nil {
		t.Error("a clearance with nothing connected: no error")
	}
	for range DefaultQueueSize + 3 {
		w.Feed(engine.Message{})
	}
	if d := w.Snapshot().Dropped; d != 3 {
		t.Errorf("dropped %d, want 3", d)
	}
	w.ClearPlayer(PlayerClearance{ICAO: "lkpr", Runway: "24", Phase: PlayerLanding})
	if _, ok := w.st.core.playerOn("LKPR", "24"); !ok {
		t.Error("player not on 24")
	}
	if _, ok := w.st.core.playerLanding(); !ok {
		t.Error("player not landing")
	}
	w.ClearPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerVacated})
	if _, ok := w.st.core.playerOn("LKPR", "24"); ok {
		t.Error("vacated: still on 24")
	}
	w.Heard(traffic.Transmission{Airport: "LKPR", Frequency: "118.100", Text: "x"}) // not connected: nothing
}
