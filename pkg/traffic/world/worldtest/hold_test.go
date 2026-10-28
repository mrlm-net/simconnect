package worldtest

import (
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestHoldKeepsTrafficOut: traffic held for a replay stays out whatever
// asks for it meanwhile (#1283): a schedule set again (enabled) and a
// corridor started spawn nothing while held, holding again removes what
// is there, and let go, both fill the scene. Before, SetSchedule started
// the schedule beside the replay's traffic, and Hold(true) once held
// returned without removing it.
func TestHoldKeepsTrafficOut(t *testing.T) {
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
	on := world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6}
	count := func() int { return len(h.World.Snapshot().Aircraft) }
	if err := h.World.SetSchedule(on); err != nil {
		t.Fatal(err)
	}
	if !h.Until(10*time.Minute, func() bool { return count() > 0 }) {
		t.Fatal("no traffic in 10 minutes")
	}
	if r, err := h.World.Hold(true); err != nil || !r.Held || r.Removed == 0 {
		t.Fatalf("hold: %+v, %v", r, err)
	}
	h.Advance(10 * time.Second)
	// Asked for while held: the schedule again, and a corridor round the
	// user aircraft.
	if err := h.World.SetSchedule(on); err != nil {
		t.Fatal(err)
	}
	u := h.Layout.Parking[0].Position
	lat, lon := calc.DisplaceByHeading(u.Lat, u.Lon, 90, 200*1852)
	if err := h.World.SetCorridor(world.CorridorSettings{Enabled: true, Route: []airport.LatLon{u, {Lat: lat, Lon: lon}}, LevelFt: 36000}); err != nil {
		t.Fatal(err)
	}
	for range 6 * 10 {
		h.Advance(10 * time.Second)
		if n := count(); n > 0 {
			t.Fatalf("%d aircraft spawned while held", n)
		}
	}
	if r, err := h.World.Hold(true); err != nil || !r.Held {
		t.Fatalf("held again: %+v, %v", r, err)
	}
	if r, err := h.World.Hold(false); err != nil || r.Held {
		t.Fatalf("let go: %+v, %v", r, err)
	}
	if !h.Until(10*time.Minute, func() bool { return count() > 0 }) {
		t.Error("no traffic in 10 minutes after the hold: the schedule set while held was not given back")
	}
}
