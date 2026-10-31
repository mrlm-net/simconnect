package worldtest

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestSceneTimeOfDay: a scene plays at its local time, whatever the zone, and the
// clock is back when it ends.
func TestSceneTimeOfDay(t *testing.T) {
	h := spottingStart(t)
	h.Sim.SetLocalOffset(3600)
	var a struct {
		ZuluSec  float64 `json:"zuluSec"`
		LocalSec float64 `json:"localSec"`
	}
	read := func() {
		t.Helper()
		if err := h.World.Get("/api/aircraft", &a); err != nil {
			t.Fatal(err)
		}
	}
	h.Advance(3 * time.Second)
	read()
	zulu := a.ZuluSec
	// Twelve hours away from now, so it cannot pass by chance.
	want := math.Mod(a.LocalSec+12*3600, 86400)
	local := time.Unix(int64(want), 0).UTC().Format("15:04")
	sc := world.Scene{Name: "at night", LimitSec: 60, Local: local,
		Cast:  []world.SceneCast{{Role: "dep", Kind: "departure", Tail: "NITE1", Models: "a320"}},
		Beats: []world.SceneBeat{{Cue: "start", Shots: []world.SceneShot{{Move: "topOrbit", Who: "dep", Sec: 40, Long: true}}}}}
	if err := h.World.PlayScene("LKPR", sc); err != nil {
		t.Fatal(err)
	}
	off := func(a, b float64) float64 { return math.Abs(math.Mod(a-b+86400+43200, 86400) - 43200) }
	if !h.WaitFor(20*time.Second, func() bool {
		h.Advance(time.Second)
		read()
		return off(a.LocalSec, want) < 300
	}) {
		t.Fatalf("local %.0f s, want about %.0f (%s)", a.LocalSec, want, local)
	}
	if off(a.LocalSec-a.ZuluSec, 3600) > 1 {
		t.Errorf("the zone changed: local %.0f, zulu %.0f", a.LocalSec, a.ZuluSec)
	}
	if err := h.World.StopScene(); err != nil {
		t.Fatal(err)
	}
	if !h.WaitFor(20*time.Second, func() bool {
		h.Advance(time.Second)
		read()
		return off(a.ZuluSec, zulu) < 300
	}) {
		t.Fatalf("zulu %.0f s, want about %.0f again", a.ZuluSec, zulu)
	}
}

// TestSceneLeavesTheSimAsItWas: the map closing during a scene gives back the
// simulator's own view and its clock, the scene not having ended to do it.
func TestSceneLeavesTheSimAsItWas(t *testing.T) {
	h := spottingStart(t)
	h.Sim.SetCameraState(3) // the user's in the chase view
	h.Advance(3 * time.Second)
	sc := world.Scene{Name: "cut short", LimitSec: 600, Local: "03:00",
		Cast:  []world.SceneCast{{Role: "dep", Kind: "departure", Tail: "CUT1", Models: "a320"}},
		Beats: []world.SceneBeat{{Cue: "start", Shots: []world.SceneShot{{Move: "topOrbit", Who: "dep", Sec: 300, Long: true}}}}}
	if err := h.World.PlayScene("LKPR", sc); err != nil {
		t.Fatal(err)
	}
	var v struct {
		Acquired bool `json:"acquired"`
	}
	if !h.WaitFor(30*time.Second, func() bool {
		h.Advance(time.Second)
		if err := h.World.Get("/api/camera", &v); err != nil {
			t.Fatal(err)
		}
		return v.Acquired
	}) {
		t.Fatal("the scene's camera never came")
	}
	h.Sim.SetCameraState(2) // the simulator's state under our camera
	h.Close()
	if got := h.Sim.CameraState(); got != 3 {
		t.Errorf("simulator camera %d after closing, want 3", got)
	}
	if s := math.Mod(math.Abs(h.Sim.ZuluShift()), 86400); s > 300 && s < 86100 {
		t.Errorf("the clock is %.0f s off after closing", s)
	}
}
