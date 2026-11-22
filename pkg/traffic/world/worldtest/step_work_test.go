package worldtest

import (
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// A step waits for the work of the ticks it fired, not only for the ticks
// to be taken (#1645: the race detector's slow run moved the clock on
// between a virtual overflight's pose and its creation, BAW1760 "created
// 50.3 m off its track", one 200 ms frame at 490 kt). Each slow call here
// takes longer than a step waits by itself.
func TestStepWaitsForTickWork(t *testing.T) {
	root := dataDir(t)
	h, err := Start(Options{DataDir: root, ICAO: "LKPR", Manual: true, FrameRate: 5, Start: overflightStart, Seed: 1645, AircraftTitles: overflightTitles()})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	overflightAirportRefs(t, h, root)

	var mu sync.Mutex
	wasVirtual, moved, calls := map[string]bool{}, map[string]int{}, map[string]int{}
	slow := func(call, tail string) {
		mu.Lock()
		// A creation of a track (the tick's); a spawn runs on its own.
		look := call == CallScan || wasVirtual[tail]
		mu.Unlock()
		if !look {
			return
		}
		before := traffic.Now()
		time.Sleep(10 * time.Millisecond)
		after := traffic.Now()
		mu.Lock()
		calls[call]++
		if !after.Equal(before) {
			moved[call]++
		}
		mu.Unlock()
	}
	h.Sim.slowCall.Store(&slow)
	defer h.Sim.slowCall.Store(nil)

	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 1, MaxAircraft: 1, VFR: &no, Seed: h.Seed,
		OverflightsPerHour: 120, OverflightMaterialiseNM: 60}); err != nil {
		t.Fatal(err)
	}
	creations := func() int {
		mu.Lock()
		defer mu.Unlock()
		return calls[CallCreate]
	}
	for s := 0; s < 30*60 && creations() < 3; s++ {
		h.Advance(time.Second)
		var list []world.Traffic
		if err := h.World.Get("/api/traffic", &list); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		for _, v := range list {
			if v.Virtual {
				wasVirtual[v.Tail] = true
			}
		}
		mu.Unlock()
	}
	mu.Lock()
	defer mu.Unlock()
	for _, call := range []string{CallScan, CallCreate} {
		if calls[call] == 0 {
			t.Errorf("no %s looked at", call)
		}
		if moved[call] > 0 {
			t.Errorf("%s: the clock moved on during %d of %d: the tick's work was not waited for", call, moved[call], calls[call])
		}
	}
}
