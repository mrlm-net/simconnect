package worldtest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// TestDepartureRunwayHasSID: with the 25s in use at EDDF no stand gets 25R,
// which publishes no SID, however near it; a departure sent there by name is
// marked NoSID, one on 25C is not (#1329).
func TestDepartureRunwayHasSID(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "EDDF", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	wx := nav.StaticWeather(250, 8, 9999, 100, 10, 1013)
	h.World.SetWeather(&wx)
	h.Advance(2 * time.Second)

	got := map[string]int{}
	b, err := os.ReadFile(filepath.Join(dataDir(t), "EDDF", "layout.json"))
	if err != nil {
		t.Fatal(err)
	}
	var l struct {
		Parking []airport.Parking `json:"parking"`
	}
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	for _, p := range l.Parking {
		got[h.World.DepartureRunway("EDDF", p.Label())]++
	}
	t.Logf("departure runways: %v", got)
	if got["25R"] > 0 {
		t.Errorf("25R given, it publishes no SID: %v", got)
	}
	if got["25C"] == 0 {
		t.Errorf("no departure on 25C: %v", got)
	}

	spawn := func(tail, runway string) (noSID bool) {
		t.Helper()
		if _, err := h.World.Do(http.MethodPost, "/api/control", map[string]any{"kind": "departure", "icao": "EDDF", "tail": tail, "runway": runway, "stand": -1, "model": "A320"}); err != nil {
			t.Fatal(err)
		}
		for range 30 {
			h.Advance(time.Second)
			if v, ok := h.Aircraft(tail); ok {
				return v.NoSID
			}
		}
		t.Fatalf("%s not spawned", tail)
		return false
	}
	if !spawn("NOSID1", "25R") {
		t.Error("a departure off 25R not marked NoSID")
	}
	if spawn("NOSID2", "25C") {
		t.Error("a departure off 25C marked NoSID")
	}
}
