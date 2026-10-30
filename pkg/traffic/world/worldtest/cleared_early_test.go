package worldtest

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestClearedEarly: a player's clearance ahead of the crew is taken and
// shown; one in the crew's order is not shown (#1308).
func TestClearedEarly(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.SetStaffed([]world.StaffedStation{
		{ICAO: "LKPR", Position: "delivery", Mode: "silent"},
		{ICAO: "LKPR", Position: "ground", Mode: "silent"},
	})
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	view := func(id int) world.ControlView {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.ID == id {
				return v
			}
		}
		t.Fatalf("aircraft %d gone", id)
		return world.ControlView{}
	}
	waiting := func(not int) (world.ControlView, bool) {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.ID != not && v.Kind == "departure" && !v.Done && slices.Contains(v.Actions, "clearance") {
				return v, true
			}
		}
		return world.ControlView{}, false
	}
	var early world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		v, ok := waiting(-1)
		early = v
		return ok
	}) {
		t.Fatal("no departure waiting for its clearance")
	}
	// With delivery, not called ground: only its clearance is on offer.
	if !slices.Equal(early.Actions, []string{"clearance"}) {
		t.Errorf("%s before its clearance: offered %v", early.Tail, early.Actions)
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/taxi", early.ID), nil); err != nil {
		t.Fatalf("taxi ahead of its clearance refused: %v", err)
	}
	if got := view(early.ID).ClearedEarly; !slices.Equal(got, []string{"taxi"}) {
		t.Errorf("cleared early %v, want taxi", got)
	}
	var dep world.ControlView
	if !h.Until(30*time.Minute, func() bool {
		v, ok := waiting(early.ID)
		dep = v
		return ok
	}) {
		t.Fatal("no second departure waiting for its clearance")
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/clearance", dep.ID), nil); err != nil {
		t.Fatalf("clearance: %v", err)
	}
	if !h.Until(15*time.Minute, func() bool { return view(dep.ID).Request == "pushback" && view(dep.ID).ATC == "ground" }) {
		t.Fatalf("%s never asked ground for its pushback: %+v", dep.Tail, view(dep.ID).Request)
	}
	if !h.Until(time.Minute, func() bool { return slices.Contains(view(dep.ID).Actions, "pushback") }) {
		t.Errorf("%s called for its pushback: offered %v", dep.Tail, view(dep.ID).Actions)
	}
	if a := view(dep.ID).Actions; slices.Contains(a, "taxi") || slices.Contains(a, "upto") {
		t.Errorf("%s called for its pushback: offered %v", dep.Tail, a)
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/pushback", dep.ID), nil); err != nil {
		t.Fatalf("pushback: %v", err)
	}
	if got := view(dep.ID).ClearedEarly; len(got) != 0 {
		t.Errorf("in the crew's order, shown as early: %v", got)
	}
	// The first one's clearance, late: read back, no transfer to ground.
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/clearance", early.ID), nil); err != nil {
		t.Fatalf("late clearance: %v", err)
	}
	if !h.Until(10*time.Minute, func() bool { return view(early.ID).ATC != "delivery" }) {
		t.Fatalf("%s stayed with delivery after its clearance", early.Tail)
	}
	correct, contact := false, false
	for _, tx := range h.Heard() {
		if tx.Callsign != early.Tail {
			continue
		}
		correct = correct || tx.Intent == traffic.IntentReadbackCorrect
		contact = contact || tx.Intent == traffic.IntentContact && !tx.Pilot
	}
	if !correct || contact {
		t.Errorf("%s late clearance: readback correct %v, contact %v", early.Tail, correct, contact)
	}
}
