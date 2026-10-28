package worldtest

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestRunwayTooShortRefused: a jet given a runway too short for its
// take-off that day refuses it on the frequency and keeps its own; the
// host is told (#1232). LKPR's runways are all long: the day is made hot
// enough (ISA+87) that 12/30 is not.
func TestRunwayTooShortRefused(t *testing.T) {
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
	wx := nav.StaticWeather(240, 5, 9999, 100, 10, 1013)
	h.World.SetWeather(&wx)
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "delivery", Mode: "silent"}})
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	jet := func(model string) bool {
		m := strings.ToUpper(model)
		return slices.ContainsFunc([]string{"A31", "A32", "A20N", "A21N", "B73", "B38", "B739"}, func(s string) bool { return strings.Contains(m, s) })
	}
	var dep world.ControlView
	if !h.Until(20*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && !v.Done && jet(v.Model) && slices.Contains(v.Actions, "clearance") {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no jet departure waiting for its clearance in 20 simulated minutes")
	}
	err = h.World.GiveClearance(dep.ID, world.ClearanceAmend{Runway: "12"})
	if err == nil || !strings.Contains(err.Error(), traffic.ErrRunwayTooShort.Error()) {
		t.Fatalf("%s (%s) given 12: %v, want ErrRunwayTooShort", dep.Tail, dep.Model, err)
	}
	if !h.Until(time.Minute, func() bool {
		return slices.ContainsFunc(h.Heard(), func(tx traffic.Transmission) bool {
			return tx.Callsign == dep.Tail && tx.Intent == traffic.IntentUnableRunway && tx.Params[traffic.ParamRunway] == dep.Runway
		})
	}) {
		t.Errorf("%s: no refusal heard", dep.Tail)
	}
	for _, v := range h.World.Snapshot().Aircraft {
		if v.ID == dep.ID && v.Runway != dep.Runway {
			t.Errorf("%s on runway %s after refusing 12, want %s", v.Tail, v.Runway, dep.Runway)
		}
	}
}
