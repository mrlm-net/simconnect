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

// TestTaxiClearanceNotRepeated: a taxi clearance given again, the same as
// the one in force, is not said again nor read back (#1269: CSA754 heard
// the same one three times); a hold position and a taxi after it still go
// out.
func TestTaxiClearanceNotRepeated(t *testing.T) {
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
	said := func(intents ...traffic.Intent) (atc, crew int) {
		for _, tx := range h.Heard() {
			if tx.Callsign != dep.Tail || !slices.Contains(intents, tx.Intent) {
				continue
			}
			if tx.Pilot {
				crew++
			} else {
				atc++
			}
		}
		return
	}
	do := func(action string) {
		t.Helper()
		if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/%s", dep.ID, action), nil); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		h.Advance(20 * time.Second)
	}
	do("pushstart")
	do("taxi")
	if atc, _ := said(traffic.IntentTaxi); atc != 1 {
		t.Fatalf("%s: taxi said %d times, want once", dep.Tail, atc)
	}
	do("taxi")
	do("taxi")
	if atc, crew := said(traffic.IntentTaxi); atc != 1 || crew > 1 {
		t.Errorf("%s: the same taxi clearance said %d times, read back %d", dep.Tail, atc, crew)
	}
	// Taxiing: stopped and sent on again, both said.
	if !h.Until(15*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.ID == dep.ID {
				return v.State == traffic.TaxiTaxiing.String()
			}
		}
		return false
	}) {
		t.Skipf("%s never taxied", dep.Tail)
	}
	do("hold")
	h.Advance(30 * time.Second)
	do("taxi")
	if atc, _ := said(traffic.IntentTaxi, traffic.IntentContinueTaxi); atc != 2 {
		t.Errorf("%s: taxi after a hold position said %d times in all, want 2", dep.Tail, atc)
	}
}
