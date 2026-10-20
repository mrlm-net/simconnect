package worldtest

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"

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

// TestHarnessManualClock: on the manual clock the World's time moves only
// with Advance, frame by frame, much faster than the wall clock: the
// schedule spawns and the radio talks within simulated minutes.
func TestHarnessManualClock(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	wall := time.Now()
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	start := h.Clock.Now()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10}); err != nil {
		t.Fatal(err)
	}
	if !h.Until(10*time.Minute, func() bool { return len(h.World.Snapshot().Aircraft) > 0 && len(h.Heard()) > 1 }) {
		t.Fatalf("no aircraft or radio in 10 simulated minutes (%d aircraft, %d heard)", len(h.World.Snapshot().Aircraft), len(h.Heard()))
	}
	h.Advance(2 * time.Minute)
	sim := h.Clock.Now().Sub(start)
	if at := h.World.Snapshot().At; at.Before(start.Add(sim - time.Second)) {
		t.Errorf("the World's time %v, want about %v on", at.Sub(start), sim)
	}
	t.Logf("%v simulated in %v of wall time", sim.Round(time.Second), time.Since(wall).Round(time.Millisecond))
	for _, v := range h.World.Snapshot().Aircraft {
		t.Logf("%s %s %s", v.Tail, v.Kind, v.State)
	}
	for _, tx := range h.Heard() {
		t.Logf("%s", tx.Text)
	}
}

// TestPlayerRescue: the user aircraft flown in on the LKPR 24 final with
// an engine failure declared gets the rescue crew out (fire trucks on the
// loopback) once within 20 NM, and they follow it after touchdown.
func TestPlayerRescue(t *testing.T) {
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
	_, end, ok := h.Layout.RunwayEnd("24")
	if !ok {
		t.Fatal("no runway 24")
	}
	field := h.Layout.Altitude / 0.3048
	nm := 25.0
	fly := func() { // the user on the final at 160 kt, 3°
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading+180, nm*1852)
		h.SetUser(func(u *Object) {
			u.Lat, u.Lon, u.Heading, u.GroundKts, u.VSFpm = lat, lon, end.Heading, 160, -800
			u.AltFt, u.OnGround = field+50+nm*318, false
			if nm <= 0 {
				u.AltFt, u.OnGround, u.VSFpm, u.GroundKts = field, true, 0, 60
			}
		})
	}
	trucks := func() int {
		n := 0
		for _, ob := range h.Sim.Objects() {
			if strings.Contains(ob.Title, "Fire") || strings.Contains(ob.Title, "Medic") {
				n++
			}
		}
		return n
	}
	declared := false
	for step := 0; step < 12*60 && trucks() == 0; step++ {
		fly()
		h.Advance(time.Second)
		nm = math.Max(0, nm-160.0/3600)
		if step == 10 && !declared {
			h.World.PriorityFor("", traffic.EmergencyEngineFailure)
			declared = true
		}
	}
	if b, err := h.World.Do("GET", "/api/traffic", nil); err == nil {
		t.Logf("traffic: %.600s", b)
	}
	if trucks() == 0 {
		t.Fatalf("no rescue vehicle out by %.1f NM", nm)
	}
	t.Logf("rescue out at %.1f NM", nm)
}
