package worldtest

import (
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestSpawnNeverAnswered: an aircraft the simulator does not create is given
// up within the limit, not left "spawning" for good, and frees its stand. If
// it comes after all it is removed, and its answer does not reach the
// aircraft spawned since (#1396; live: LNX03 sat 15 minutes on B11).
func TestSpawnNeverAnswered(t *testing.T) {
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
	spawn := func(kind, tail string, stand int) {
		t.Helper()
		if _, err := h.World.Do(http.MethodPost, "/api/control", map[string]any{"kind": kind, "icao": "LKPR", "tail": tail, "runway": "24", "stand": stand, "model": "A320"}); err != nil {
			t.Fatal(err)
		}
	}
	h.Sim.HoldCreates()
	spawn("departure", "STK1", -1)
	spawn("arrival", "STK2", -1)
	h.Advance(time.Minute)
	for _, tail := range []string{"STK1", "STK2"} {
		if v, ok := h.Aircraft(tail); !ok || v.State != "spawning" {
			t.Fatalf("%s after a minute of a slow creation: %+v, %v; it should still wait", tail, v, ok)
		}
	}
	first, _ := h.Aircraft("STK1")
	stand := slices.IndexFunc(h.Layout.Parking, func(p airport.Parking) bool { return p.Label() == first.Stand })
	if stand < 0 {
		t.Fatalf("no stand %q", first.Stand)
	}
	gone := func() bool {
		_, a := h.Aircraft("STK1")
		_, b := h.Aircraft("STK2")
		return !a && !b
	}
	if !h.Until(3*time.Minute, gone) {
		v, _ := h.Aircraft("STK1")
		t.Fatalf("still there, state %q, after four minutes", v.State)
	}
	// Spawned before the late answers: they must not become this aircraft.
	spawn("departure", "STK3", stand)
	h.Sim.ReleaseCreates()
	tails := func() []string {
		var out []string
		for _, o := range h.Sim.Objects() {
			if o.ID != UserObjectID {
				out = append(out, o.ATCID)
			}
		}
		slices.Sort(out)
		return out
	}
	if !h.Until(time.Minute, func() bool { return slices.Equal(tails(), []string{"STK3"}) }) {
		t.Fatalf("objects in the simulator: %v, want only STK3", tails())
	}
	v, ok := h.Aircraft("STK3")
	if !ok || v.State == "spawning" {
		t.Fatalf("STK3 %+v, %v: its own answer did not reach it", v, ok)
	}
	if v.Stand != first.Stand {
		t.Fatalf("STK3 on %q, asked for the freed %q", v.Stand, first.Stand)
	}
}
