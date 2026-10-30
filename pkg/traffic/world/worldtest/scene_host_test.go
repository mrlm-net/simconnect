package worldtest

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestPlaySceneFromHost: a host plays a Scene it holds, not a file: the
// camera is acquired and set, progress comes as events in order and on the
// view, a long shot runs past MaxShot, and StopScene gives the camera back
// and ends it (#1324).
func TestPlaySceneFromHost(t *testing.T) {
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

	var mu sync.Mutex
	var got []world.SceneEvent
	h.World.OnSceneEvent(func(e world.SceneEvent) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	})
	kinds := func() string {
		mu.Lock()
		defer mu.Unlock()
		var k []string
		for _, e := range got {
			k = append(k, e.Kind)
		}
		return strings.Join(k, " ")
	}

	if err := h.World.PlayScene("LKPR", world.Scene{Name: "bad", Beats: []world.SceneBeat{{Shots: []world.SceneShot{{Move: "flyover", Who: "nobody"}}}}}); err == nil {
		t.Error("a shot of nobody in the cast was taken")
	}
	sc := world.Scene{Name: "from the host", LimitSec: 600,
		Cast:  []world.SceneCast{{Role: "dep", Kind: "departure", Tail: "HOST1", Models: "a320"}},
		Beats: []world.SceneBeat{{Cue: "start", Shots: []world.SceneShot{{Move: "topOrbit", Who: "dep", Sec: 40, Long: true}}}}}
	if err := h.World.PlayScene("LKPR", sc); err != nil {
		t.Fatal(err)
	}
	if err := h.World.PlayScene("LKPR", sc); err == nil {
		t.Error("a second scene was taken while one plays")
	}
	type view struct {
		Mode     string               `json:"mode"`
		Acquired bool                 `json:"acquired"`
		Scene    *world.SceneProgress `json:"scene"`
	}
	var v view
	played := func() bool {
		if err := h.World.Get("/api/camera", &v); err != nil {
			t.Fatal(err)
		}
		return v.Acquired && h.Sim.CameraSets() > 0 && strings.Contains(kinds(), "shot")
	}
	if !h.WaitFor(30*time.Second, func() bool {
		h.Advance(time.Second)
		return played()
	}) {
		t.Fatalf("not playing: %+v, %d poses, events %q", v, h.Sim.CameraSets(), kinds())
	}
	if v.Mode != "scene" || v.Scene == nil || v.Scene.Name != sc.Name || v.Scene.Beat != 1 || v.Scene.Beats != 1 {
		t.Errorf("view: %+v, progress %+v", v, v.Scene)
	}
	// Still on its one shot past MaxShot: the long shot was not capped.
	for wait := time.Now().Add(12 * time.Second); time.Now().Before(wait); {
		h.Advance(time.Second)
	}
	mu.Lock()
	shots := 0
	for _, e := range got {
		if e.Kind == "shot" {
			shots++
		}
	}
	mu.Unlock()
	if shots != 1 {
		t.Errorf("%d shots in 12 s, want the one long shot; events %q", shots, kinds())
	}
	if err := h.World.StopScene(); err != nil {
		t.Fatal(err)
	}
	if !h.WaitFor(10*time.Second, func() bool { return strings.HasSuffix(kinds(), " end") }) {
		t.Fatalf("no end: %q", kinds())
	}
	if want := "start cue beat shot end"; kinds() != want {
		t.Errorf("events %q, want %q", kinds(), want)
	}
	v = view{}
	if err := h.World.Get("/api/camera", &v); err != nil {
		t.Fatal(err)
	}
	if v.Mode != "off" || v.Acquired || v.Scene != nil {
		t.Errorf("after the stop: %+v", v)
	}
	if err := h.World.StopScene(); err != nil {
		t.Errorf("stop with nothing playing: %v", err)
	}
}
