package worldtest

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestArrivalNoClearanceDiverts: a staffed tower that never clears an arrival
// has it go around, is offered the handover to approach (the player's to give),
// and on the third go-around the crew diverts and leaves, rather than coming
// back for ever (#1411).
func TestArrivalNoClearanceDiverts(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "EDDF", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "EDDF", Position: "tower"}})
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"EDDF"}, Density: 3, MaxAircraft: 4, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	tail := ""
	calls := func() (n int, diverting string) {
		for _, tx := range h.Heard() {
			if tx.Callsign == tail && strings.Contains(tx.Text, "going around, no landing clearance") {
				n++
				if strings.Contains(tx.Text, "diverting to ") {
					diverting = tx.Text
				}
			}
		}
		return n, diverting
	}
	view := func() (world.ControlView, bool) {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && (tail == "" || v.Tail == tail) {
				tail = v.Tail
				return v, true
			}
		}
		return world.ControlView{}, false
	}
	offered := false
	if !h.Until(4*time.Hour, func() bool {
		v, ok := view()
		if !ok {
			return false
		}
		if v.State == "landing" || v.State == "rollout" {
			t.Fatalf("%s landed with no landing clearance", tail)
		}
		n, d := calls()
		offered = offered || (n > 0 && slices.Contains(v.Actions, "handoff"))
		return d != ""
	}) {
		n, _ := calls()
		t.Fatalf("%s: %d go-arounds and no diversion", tail, n)
	}
	n, said := calls()
	t.Log(said)
	if n != 3 {
		t.Errorf("diverted on go-around %d, want 3: %q", n, said)
	}
	if !offered {
		t.Error("the handover to approach never offered after a go-around")
	}
	if !h.Until(30*time.Minute, func() bool {
		_, ok := view()
		return !ok
	}) {
		v, _ := view()
		t.Fatalf("still here 30 minutes after diverting: %s at %+v", v.State, v.Position)
	}
	if n2, _ := calls(); n2 != n {
		t.Errorf("went around again after diverting: %d", n2)
	}
}
