package worldtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// dataDir is the offline data (a simconnect-data checkout): $SIMCONNECT_DATA,
// else beside this repository's checkout. The test skips without it.
func dataDir(t *testing.T) string {
	d := os.Getenv("SIMCONNECT_DATA")
	if d == "" {
		d = filepath.Join("..", "..", "..", "..", "..", "simconnect-data")
	}
	if _, err := os.Stat(filepath.Join(d, "LKPR", "layout.json")); err != nil {
		t.Skipf("no offline data at %s (SIMCONNECT_DATA)", d)
	}
	return d
}

// TestHarnessRunsTraffic: a World with no simulator starts at LKPR, the
// schedule creates our aircraft on the loopback, and ground talks to them.
func TestHarnessRunsTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("runs in real time")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR"}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10}); err != nil {
		t.Fatal(err)
	}
	if !h.WaitFor(40*time.Second, func() bool { return len(h.World.Snapshot().Aircraft) > 0 }) {
		t.Fatal("no aircraft in 90 s")
	}
	if !h.WaitFor(40*time.Second, func() bool { return len(h.Heard()) > 0 }) {
		t.Fatal("nothing said in 90 s")
	}
	for _, v := range h.World.Snapshot().Aircraft {
		t.Logf("%s %s %s at %v", v.Tail, v.Kind, v.State, v.Position)
	}
	for _, tx := range h.Heard() {
		t.Logf("%s", tx.Text)
	}
}
