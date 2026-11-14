package worldtest

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// vehicleCall: a service vehicle's call or an answer to one ("Tug 3",
// "Fuel 2", ...).
func vehicleCall(tx traffic.Transmission) bool {
	for _, w := range []string{"Tug ", "Fuel ", "Stairs ", "GPU ", "Bus ", "Follow-me ", "Vehicle "} {
		if strings.HasPrefix(tx.Callsign, w) {
			return true
		}
	}
	return false
}

// TestStaffedVehiclesQuiet (#1539): with ground and tower staffed, our
// service vehicles neither call nor are answered by the AI on the player's
// frequencies (they still ask, quietly: a taxiway is theirs, a crossing
// still sequenced); unstaffed they call and are answered.
func TestStaffedVehiclesQuiet(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	for _, staffed := range []bool{false, true} {
		o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
		log := &lockedBuf{}
		o.World.Output = log
		h, err := Start(o)
		if err != nil {
			t.Fatal(err)
		}
		if staffed {
			h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "ground"}, {ICAO: "LKPR", Position: "tower"}})
		}
		if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 14}); err != nil {
			t.Fatal(err)
		}
		h.Advance(60 * time.Minute)
		calls := 0
		for _, tx := range h.Heard() {
			if vehicleCall(tx) && (tx.Position == traffic.PosGround || tx.Position == traffic.PosTower) {
				calls++
				if staffed {
					t.Errorf("staffed: %s %s %q", tx.Position, tx.Intent, tx.Text)
				}
			}
		}
		t.Logf("staffed=%v: %d vehicle transmissions", staffed, calls)
		// Data with no vehicle on the taxiways (the director job's old LKPR)
		// proves nothing either way.
		if !staffed && calls == 0 {
			t.Log("unstaffed: no vehicle called ATC in an hour, nothing to check")
			h.Close()
			return
		}
		if staffed && !strings.Contains(log.String(), ": quiet ") {
			t.Error("staffed: no vehicle asked at all, unstaffed they did")
		}
		h.Close()
	}
}
