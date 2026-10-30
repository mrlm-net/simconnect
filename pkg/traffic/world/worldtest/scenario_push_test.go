package worldtest

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestScenarioPushAlreadyCleared: a scenario's push on a departure already
// cleared to push is done, with no error and nothing said again (#1321).
func TestScenarioPushAlreadyCleared(t *testing.T) {
	pushAgain(t, func(h *Harness, dep world.ControlView) {
		if err := h.World.Act("LKPR", world.ScenarioAction{Do: "push", Who: "dep"}, map[string]string{"dep": dep.Tail}); err != nil {
			t.Errorf("push when already cleared: %v", err)
		}
	})
}

// TestRefusedPushNotSaidAgain: a host's push on a departure already cleared
// to push is refused, and the approval is not said again when the state
// changes: the refusal dropped the mark that it had been said (#1321).
func TestRefusedPushNotSaidAgain(t *testing.T) {
	pushAgain(t, func(h *Harness, dep world.ControlView) {
		if err := h.World.Clear(dep.ID, "pushback"); err == nil || !strings.Contains(err.Error(), "already cleared") {
			t.Errorf("push when already cleared: %v, want refused", err)
		}
	})
}

// pushAgain clears a departure to push and start, runs again a moment
// later, and checks that the approval was said once.
func pushAgain(t *testing.T, again func(*Harness, world.ControlView)) {
	t.Helper()
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
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	if !h.Until(30*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && !v.Done && slices.Contains(v.Actions, "pushstart") {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure to push in 30 simulated minutes")
	}
	if err := h.World.Clear(dep.ID, "pushstart"); err != nil {
		t.Fatal(err)
	}
	h.Advance(5 * time.Second)
	said := func() int {
		n := 0
		for _, tx := range h.Heard() {
			if tx.Callsign == dep.Tail && !tx.Pilot && tx.Intent == traffic.IntentPushback {
				n++
			}
		}
		return n
	}
	before := said()
	again(h, dep)
	h.Advance(30 * time.Second)
	if n := said(); n != before {
		t.Errorf("push approval said %d times, %d before the step", n, before)
	}
}
