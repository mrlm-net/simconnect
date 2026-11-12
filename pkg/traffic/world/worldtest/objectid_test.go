package worldtest

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestControlViewObjectID: an aircraft whose object exists reports it in
// its ControlView, the same object the simulator holds under its tail; one
// not created yet reports none.
func TestControlViewObjectID(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 4}); err != nil {
		t.Fatal(err)
	}
	var got world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.ObjectID != 0 {
				got = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no aircraft with an object in 10 simulated minutes")
	}
	for _, o := range h.Sim.Objects() {
		if o.ID == got.ObjectID {
			if o.ATCID != got.Tail {
				t.Errorf("objectId %d is %q in the simulator, the view says %q", got.ObjectID, o.ATCID, got.Tail)
			}
			return
		}
	}
	t.Errorf("%s objectId %d: no such object in the simulator", got.Tail, got.ObjectID)
}
