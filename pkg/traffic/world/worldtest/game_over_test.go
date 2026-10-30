package worldtest

import (
	"net/http"
	"os"
	"testing"
	"time"
)

// TestGameOverReleasesFlights: the game's departure, waiting for the
// player's pushback, pushes under automatic control once the game is over
// (live: the game was on for 7 s and RYR232 asked for its push at B15 for
// an hour and a half, nobody to answer).
func TestGameOverReleasesFlights(t *testing.T) {
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
	game := func(on bool) {
		if _, err := h.World.Do(http.MethodPost, "/api/game", map[string]any{"on": on, "icao": "LKPR"}); err != nil {
			t.Fatal(err)
		}
	}
	game(true)
	tail := ""
	if !h.Until(time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && v.Manual {
				tail = v.Tail
			}
		}
		return tail != ""
	}) {
		t.Fatal("the game spawned no departure in a minute")
	}
	h.Advance(10 * time.Second)
	game(false)
	state := ""
	if !h.Until(20*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Tail == tail {
				state = v.State
				return !v.Manual && v.State != "awaiting pushback" && v.State != "spawning"
			}
		}
		return false
	}) {
		t.Fatalf("%s still %s 20 minutes after the game was over", tail, state)
	}
}
