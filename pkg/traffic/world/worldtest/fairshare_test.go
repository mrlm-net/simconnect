package worldtest

import (
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestMorningPeakHasArrivals: the morning peak at density 3 with a cap on
// the aircraft: the departures, on their stands early, do not take the
// whole cap; arrivals come in too (live, LKPR: 41 of ours, all departures,
// 12 waiting to take off and no arrival in an hour).
func TestMorningPeakHasArrivals(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10,
		Start: time.Date(2026, 10, 20, 4, 30, 0, 0, time.UTC)} // 06:30 CEST: the morning wave
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 15, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	maxDeps := 0
	arrivals := map[string]bool{}
	for range 40 * 6 {
		h.Advance(10 * time.Second)
		deps := 0
		for _, v := range h.World.Snapshot().Aircraft {
			switch v.Kind {
			case "arrival":
				arrivals[v.Tail] = true
			case "departure":
				deps++
			}
		}
		maxDeps = max(maxDeps, deps)
	}
	t.Logf("arrivals %d, departures at most %d at once", len(arrivals), maxDeps)
	if len(arrivals) < 3 {
		t.Errorf("%d arrivals in 40 minutes of the morning peak", len(arrivals))
	}
	// Two thirds of the cap: no share rule holds it (reverted with 0.118.3),
	// the timetable's mix does; the live fault was all of the cap.
	if maxDeps > 10 {
		t.Errorf("%d departures at once of a cap of 15", maxDeps)
	}
}
