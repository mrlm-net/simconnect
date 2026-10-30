package worldtest

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestArrivalNoClearanceGoesAround: on a staffed tower "continue approach" is
// not a landing clearance: with none by the decision point the crew goes
// around, says why, and the game scores it. A clearance followed by a
// go-around does not carry to the next approach; a clearance lands it (#1334).
func TestArrivalNoClearanceGoesAround(t *testing.T) {
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
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "EDDF", Position: "tower"}})
	if _, err := h.World.Do(http.MethodPost, "/api/game", map[string]any{"on": true, "icao": "EDDF", "intervalSec": 0}); err != nil {
		t.Fatal(err)
	}
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"EDDF"}, Density: 3, MaxAircraft: 4, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var tail string
	view := func() (world.ControlView, bool) {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && (tail == "" || v.Tail == tail) {
				tail = v.Tail
				return v, true
			}
		}
		return world.ControlView{}, false
	}
	wentAround := func() int {
		n := 0
		for _, tx := range h.Heard() {
			if tx.Callsign == tail && strings.Contains(tx.Text, "going around, no landing clearance") {
				n++
			}
		}
		return n
	}
	down := func(v world.ControlView) bool { return v.State == "landing" || v.State == "rollout" }
	act := func(v world.ControlView, action string) {
		t.Helper()
		if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/%s", v.ID, action), nil); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	// The first approach: nothing said.
	if !h.Until(90*time.Minute, func() bool {
		v, ok := view()
		if ok && down(v) {
			t.Fatalf("%s landed with no landing clearance", tail)
		}
		return ok && wentAround() == 1
	}) {
		t.Fatal("no go-around")
	}
	var g struct {
		Score  int
		Events []string
	}
	if err := h.World.Get("/api/game", &g); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(g.Events, " "), tail+" went around: no landing clearance") || g.Score >= 0 {
		t.Errorf("not scored: %d, %q", g.Score, g.Events)
	}
	// Cleared and sent around: the next approach needs its own clearance.
	var v world.ControlView
	if !h.Until(60*time.Minute, func() bool {
		var ok bool
		v, ok = view()
		return ok && slices.Contains(v.Actions, "land")
	}) {
		t.Fatalf("never on the final again: %+v", v)
	}
	act(v, "land")
	act(v, "goaround")
	if !h.Until(60*time.Minute, func() bool {
		v, ok := view()
		if ok && down(v) {
			t.Fatalf("%s landed on the clearance of the approach before", tail)
		}
		return wentAround() == 2
	}) {
		t.Fatal("no second go-around")
	}
	// Cleared: it lands (another arrival: three go-arounds and this one diverts).
	first := tail
	if !h.Until(60*time.Minute, func() bool {
		for _, x := range h.World.Snapshot().Aircraft {
			if x.Kind == "arrival" && x.Tail != first && slices.Contains(x.Actions, "land") {
				tail, v = x.Tail, x
				return true
			}
		}
		return false
	}) {
		t.Fatal("no other arrival on the final")
	}
	act(v, "land")
	was := wentAround()
	if !h.Until(30*time.Minute, func() bool {
		v, ok := view()
		return ok && down(v)
	}) {
		t.Fatalf("%s cleared and did not land", tail)
	}
	if n := wentAround(); n != was {
		t.Errorf("%s went around though cleared", tail)
	}
}
