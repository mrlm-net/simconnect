package worldtest

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestSpotsAnyAirport: spots from the airport's data alone at airports of
// every shape, each near the airfield and the local table winning per value;
// one plays on an aircraft: a camera in the world, aiming at it, the lens
// holding its share; in a scene too.
func TestSpotsAnyAirport(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	data := dataDir(t)
	dir := t.TempDir()
	table := `{"LKPR": {"touchdown:24": {"up": 12, "share": 0.5}, "threshold:06": {"off": true},
		"mine": {"lat": 50.1, "lon": 14.26, "up": 4}}}`
	if err := os.WriteFile(filepath.Join(dir, "spots.json"), []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	o := Options{DataDir: data, ICAO: "LKPR", Manual: true, FrameRate: 10}
	o.World.DataDir = dir
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	spots, err := h.World.Spots("LKPR")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]world.CameraSpot{}
	kinds := map[string]int{}
	for _, s := range spots {
		by[s.Name] = s
		kinds[s.Kind]++
	}
	for _, k := range []string{"threshold", "touchdown", "approach", "holding", "apron", "custom"} {
		if kinds[k] == 0 {
			t.Errorf("no %s spot: %v", k, kinds)
		}
	}
	td, ok := by["touchdown:24"]
	if !ok {
		t.Fatalf("no touchdown:24 in %v", kinds)
	}
	if td.Up != 12 || td.Share != 0.5 || td.RailLat == 0 {
		t.Errorf("the table on touchdown:24: %+v", td)
	}
	if _, ok := by["threshold:06"]; ok {
		t.Error("threshold:06 is off in the table")
	}
	if m := by["mine"]; m.Lat != 50.1 || m.AltM == 0 {
		t.Errorf("custom spot: %+v", m)
	}

	if err := h.World.PlaySpot(world.SpotRequest{ICAO: "LKPR", Spot: "nowhere", ID: -1}); err == nil {
		t.Error("a spot that does not exist was taken")
	}
	// On the player, from the scan.
	if !h.WaitFor(60*time.Second, func() bool {
		h.Advance(time.Second)
		return h.World.PlaySpot(world.SpotRequest{ICAO: "LKPR", Spot: "touchdown:24", ID: -1, Sec: 60}) == nil
	}) {
		t.Fatal("the spot never played on the player")
	}
	if !h.WaitFor(30*time.Second, func() bool {
		h.Advance(200 * time.Millisecond)
		return h.Sim.CameraSets() > 0
	}) {
		t.Fatal("no pose set")
	}
	for wait := time.Now().Add(3 * time.Second); time.Now().Before(wait); {
		h.Advance(100 * time.Millisecond)
	}
	p := h.Sim.CameraPose()
	if p.Position.Z < td.AltM-0.5 || p.Position.Z > td.AltM+0.5 {
		t.Errorf("eye at %.1f m, want %.1f", p.Position.Z, td.AltM)
	}
	if fov := p.Fov * 180 / math.Pi; fov < 5.9 || fov > 50.1 {
		t.Errorf("fov %.1f outside the lens", fov)
	}
	if p.RotationReferential != types.SIMCONNECT_POSITION_REFERENTIAL_WORLD {
		t.Errorf("aim not in the world: %+v", p)
	}
	if _, err := h.World.Do("POST", "/api/camera", map[string]string{"mode": "off"}); err != nil {
		t.Fatal(err)
	}
	// In a scene, on one of the cast.
	sc := world.Scene{Name: "spot", LimitSec: 600,
		Cast:  []world.SceneCast{{Role: "dep", Kind: "departure", Tail: "SPOT1", Models: "a320"}},
		Beats: []world.SceneBeat{{Cue: "start", Shots: []world.SceneShot{{Spot: "threshold:24", Who: "dep", Sec: 40}}}}}
	if err := h.World.PlayScene("LKPR", sc); err != nil {
		t.Fatal(err)
	}
	var shot string
	if !h.WaitFor(60*time.Second, func() bool {
		h.Advance(time.Second)
		var v struct {
			Shot string `json:"shot"`
		}
		if err := h.World.Get("/api/camera", &v); err != nil {
			t.Fatal(err)
		}
		shot = v.Shot
		return strings.Contains(shot, "threshold:24")
	}) {
		t.Errorf("the scene's spot shot did not play: %q", shot)
	}
	if err := h.World.StopScene(); err != nil {
		t.Fatal(err)
	}


	for _, icao := range []string{"LOWI", "EDDM", "EGLL", "KJFK", "LFPG"} {
		if _, err := h.World.LoadAirportDir(filepath.Join(data, icao)); err != nil {
			t.Fatal(err)
		}
		list, err := h.World.Spots(icao)
		if err != nil {
			t.Errorf("%s: %v", icao, err)
			continue
		}
		seen := map[string]int{}
		for _, s := range list {
			seen[s.Kind]++
		}
		t.Logf("%s: %v", icao, seen)
		for _, k := range []string{"threshold", "touchdown", "holding", "apron"} {
			if seen[k] == 0 {
				t.Errorf("%s: no %s spot: %v", icao, k, seen)
			}
		}
	}
}
