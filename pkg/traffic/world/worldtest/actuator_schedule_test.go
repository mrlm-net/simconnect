package worldtest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestActuatorNoLocalSchedule: a World linked as an actuator flies its
// director's traffic only (#1284): its own schedule, set while linked,
// spawns nothing, and goes on once the link ends. Before, its local
// schedule (set, or replayed on a connection) spawned beside the
// director's.
func TestActuatorNoLocalSchedule(t *testing.T) {
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
	// Linked to a director that never answers: an actuator all the same.
	ctx, unlink := context.WithCancel(context.Background())
	linked := make(chan struct{})
	go func() {
		defer close(linked)
		_ = h.World.LinkDirector(ctx, "127.0.0.1:9", "")
	}()
	time.Sleep(200 * time.Millisecond)
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}); err != nil {
		t.Fatal(err)
	}
	count := func() int { return len(h.World.Snapshot().Aircraft) }
	for range 6 * 10 {
		h.Advance(10 * time.Second)
		if n := count(); n > 0 {
			t.Fatalf("%d aircraft of its own schedule spawned on an actuator", n)
		}
	}
	unlink()
	<-linked
	if !h.Until(10*time.Minute, func() bool { return count() > 0 }) {
		t.Error("no traffic in 10 minutes once unlinked: the schedule set while linked was lost")
	}
}
