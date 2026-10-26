package worldtest

import (
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestGroundStatesReportOnGround: an aeroplane on the surface says it is on
// the surface (#1230).
//
// ControlView.OnGround was written only in the frame that drives an aircraft
// along a path, so one that had not moved yet kept the zero value: a departure
// at its stand reported OnGround false while the traffic picture reported true
// for the same aeroplane on the same tick. Measured at LKPR before the fix,
// 38 of 48 control entries whose own state was a ground state said false.
//
// The field is used as a predicate — a host's "is this airborne", the game's
// runway-occupancy check — so one that means "has been driven at least once"
// silently gives every consumer the wrong answer.
func TestGroundStatesReportOnGround(t *testing.T) {
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
	h.SetUser(func(u *Object) { // parked clear of the runways
		u.Lat, u.Lon, u.Heading, u.AltFt, u.OnGround, u.GroundKts = 50.097060, 14.284178, 168.8, h.Layout.Altitude/0.3048, true, 0
	})
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 12}); err != nil {
		t.Fatal(err)
	}

	// The states that ARE the ground, by the engine's own names.
	ground := map[string]bool{
		"awaiting pushback": true, "pushback": true, "awaiting taxi": true,
		"taxiing": true, "holding short": true, "lining up": true, "lined up": true,
	}
	var seen, wrong int
	var firstWrong string
	states := map[string]int{} // every state seen, so a failure says why

	// THE CLOCK IS MANUAL: h.Until steps it. A plain sleep loop advances nothing
	// and the world stays frozen — which is how an earlier version of this test
	// passed against the bug, having never produced a single aircraft.
	h.Until(20*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			states[v.Kind+"/"+v.State]++
			if v.Kind != "departure" || !ground[v.State] {
				continue
			}
			seen++
			if !v.OnGround {
				wrong++
				if firstWrong == "" {
					firstWrong = v.Tail + " in state " + v.State + " at stand " + v.Stand
				}
			}
		}
		return seen >= 40
	})
	if seen == 0 {
		// NOT a silent skip: a test that never reached its subject proves nothing,
		// and the states it DID see are what says whether the names moved.
		t.Fatalf("no departure reached a ground state; states seen: %v", states)
	}
	if wrong != 0 {
		t.Errorf("%d of %d control views in a GROUND state reported OnGround false; first: %s", wrong, seen, firstWrong)
	}
}
