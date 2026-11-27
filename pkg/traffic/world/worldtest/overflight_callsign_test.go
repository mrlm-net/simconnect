package worldtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// An overflight never takes the call sign of an airport's flight on the
// board, nor one the other way round (#1708): with this seed the
// overflight DLH446 and an LKPR flight DLH446 were both planned, two
// aircraft with one call sign, and the tiers' cap counted one the test
// could not tell apart.
func TestOverflightCallsignsUnique(t *testing.T) {
	root := dataDir(t)
	if _, err := os.Stat(filepath.Join(root, "LKPR", "layout.json")); err != nil {
		t.Skipf("no offline data for LKPR in %s", root)
	}
	h, err := Start(Options{DataDir: root, ICAO: "LKPR", Manual: true, FrameRate: 5, Start: overflightStart, AircraftTitles: overflightTitles(),
		Seed: 14997800560881590864})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	overflightAirportRefs(t, h, root)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 1, MaxAircraft: 12, VFR: &no, Seed: h.Seed,
		OverflightsPerHour: 60, MaxOverflights: 4, OverflightMaterialiseNM: 120, OverflightDematerialiseNM: 130}); err != nil {
		t.Fatal(err)
	}
	h.Advance(10 * time.Second)
	var v struct {
		Flights []boardFlight `json:"flights"`
	}
	if err := h.World.Get("/api/schedule", &v); err != nil {
		t.Fatal(err)
	}
	over, other := map[string]bool{}, map[string]bool{}
	for _, f := range v.Flights {
		if f.Kind == "overflight" {
			over[f.Callsign] = true
		} else {
			other[f.Callsign] = true
		}
	}
	if len(over) == 0 || len(other) == 0 {
		t.Fatalf("board: %d overflights, %d airport flights; want both", len(over), len(other))
	}
	for cs := range over {
		if other[cs] {
			t.Errorf("%s: an overflight and an airport flight", cs)
		}
	}
}
