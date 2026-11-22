package worldtest

import (
	"math"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestWayOffLeavesPlayerAlone (#1640, #1096): with a departure holding
// short on its way off, the AI plans its own arrival clear, and never
// plans, re-stands or sends around a player's.
func TestWayOffLeavesPlayerAlone(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	data := dataDir(t)
	if m, _ := filepath.Glob(filepath.Join(data, "fields", "*", "TFFJ", "layout.json")); len(m) == 0 {
		t.Skip("no TFFJ data")
	}
	for _, c := range []struct {
		name   string
		player bool
	}{{"ai", false}, {"player", true}} {
		t.Run(c.name, func(t *testing.T) { wayOffPlayerRun(t, data, c.player) })
	}
}

func wayOffPlayerRun(t *testing.T, data string, player bool) {
	const title = "FSLTL_DHC6_WIA-Winair"
	log := &lockedBuf{}
	o := Options{DataDir: data, ICAO: "TFFJ", Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), title)}
	o.World.Output = log
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, end, ok := h.Layout.RunwayEnd("10")
	if !ok {
		t.Fatal("no runway 10 at TFFJ")
	}
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 100, 25, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway("TFFJ", "10")
	h.Advance(2 * time.Second)
	stand := -1
	for i, p := range h.Layout.Parking {
		if p.Label() == "4P" {
			stand = i
		}
	}
	if stand < 0 {
		t.Fatal("no stand 4P at TFFJ")
	}
	spawn := func(r world.SpawnRequest) {
		t.Helper()
		r.ICAO, r.Runway, r.Model = "TFFJ", "10", title
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			t.Fatalf("%s: %v", r.Kind, err)
		}
	}
	spawn(world.SpawnRequest{Kind: "departure", Tail: "MTD1", Stand: stand})
	if !h.Until(15*time.Minute, func() bool {
		v, ok := h.Aircraft("MTD1")
		return ok && v.State == "holding short"
	}) {
		t.Fatal("the departure never held short")
	}
	spawn(world.SpawnRequest{Kind: "arrival", Tail: "MTA1", Stand: -1, InjectApproach: true, Gates: player})
	was := ""
	planned := h.Until(2*time.Minute, func() bool {
		if v, ok := h.Aircraft("MTA1"); ok && was == "" {
			was = v.Stand
		}
		return strings.Contains(log.String(), "way off the runway")
	})
	switch {
	case !player && !planned:
		t.Error("the AI's arrival was not planned clear of the departure")
	case player && planned:
		t.Error("a player's arrival was planned by the AI")
	case player:
		if v, ok := h.Aircraft("MTA1"); !ok || v.Stand != was {
			t.Errorf("a player's arrival moved from stand %s to %s", was, v.Stand)
		}
		if strings.Contains(log.String(), "departing traffic") {
			t.Error("a player's arrival was sent around")
		}
	}
}
