package worldtest

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestStaffedDeliveryGivesTheClearance: with delivery staffed, a departure
// asks for its clearance and the World does not answer (#1156); the view
// names delivery, the position that may clear it (#1155), and offers the
// "clearance" action, which gives it. Delivery unstaffed, the World gives
// the next one itself.
func TestStaffedDeliveryGivesTheClearance(t *testing.T) {
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
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "delivery", Mode: "silent"}})
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	cleared := func(tail string) bool {
		for _, tx := range h.Heard() {
			if tx.Callsign == tail && tx.Intent == traffic.IntentDepartureClearance {
				return true
			}
		}
		return false
	}
	var dep world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && !v.Done && slices.Contains(v.Actions, "clearance") {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure waiting for its clearance in 10 simulated minutes")
	}
	h.Advance(30 * time.Second)
	if cleared(dep.Tail) {
		t.Fatalf("%s cleared by the World on a staffed delivery", dep.Tail)
	}
	for _, v := range h.World.Snapshot().Aircraft {
		if v.Tail == dep.Tail && v.ATC != "delivery" {
			t.Errorf("%s shown with %s, still delivery's", v.Tail, v.ATC)
		}
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/clearance?as=delivery", dep.ID), nil); err != nil {
		t.Fatalf("clearance as delivery: %v", err)
	}
	if !h.Until(time.Minute, func() bool { return cleared(dep.Tail) }) {
		t.Fatalf("%s: the player's clearance never said", dep.Tail)
	}
	// Unstaffed: the World clears those still waiting, and the next.
	h.World.SetStaffed(nil)
	h.Advance(2 * time.Minute)
	for _, v := range h.World.Snapshot().Aircraft {
		if v.Kind == "departure" && slices.Contains(v.Actions, "clearance") {
			t.Errorf("%s still waits for its clearance, delivery unstaffed", v.Tail)
		}
	}
}
