package traffic

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestJumpAllowedAfterALateFrame: 8.5 m in 59 ms at 135 kt, right after a late
// frame, is simulator time catching up; the same with no late frame is a jump.
func TestJumpAllowedAfterALateFrame(t *testing.T) {
	const d, kts, dt = 8.5, 135.0, 59 * time.Millisecond
	if a := jumpAllowedM(kts, dt, 120*time.Millisecond); d > a {
		t.Errorf("after a late frame: %.1f m allowed", a)
	}
	if a := jumpAllowedM(kts, dt, 33*time.Millisecond); d <= a {
		t.Errorf("no late frame: %.1f m allowed", a)
	}
}

// TestJumpTurnScalesWithTheFrame: 3.3° in a frame of 106 ms is a tug's steady
// turn; the same in one frame at 60 Hz is a snap.
func TestJumpTurnScalesWithTheFrame(t *testing.T) {
	if a := jumpTurnAllowed(106 * time.Millisecond); a < 3.3 {
		t.Errorf("106 ms: %.1f° allowed", a)
	}
	if a := jumpTurnAllowed(17 * time.Millisecond); a >= 3.3 {
		t.Errorf("17 ms: %.1f° allowed", a)
	}
}

// TestJumpTraceNamesTheSimStep: a line says how far the simulator's clock moved
// the aircraft, as frameStep noted it, to tell its hitch from a wrong placement.
func TestJumpTraceNamesTheSimStep(t *testing.T) {
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	o := &injected{}
	i := &Injector{now: func() time.Time { return at }, objects: map[uint32]*injected{7: o}}
	d := &groundDrive{injector: i, object: 7, simNow: 100.377, simLast: 100, lastStep: at, simAt: at}
	if got := d.frameStep(at.Add(135 * time.Millisecond)); got < 0.376 || got > 0.378 {
		t.Fatalf("stepped %.3f s, want the simulator's 0.377", got)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()
	start := airport.LatLon{Lat: 50, Lon: 14}
	i.traceJump(7, o, "air", start, 240, 144)
	at = at.Add(17 * time.Millisecond)
	i.traceJump(7, o, "air", start, 240, 144) // the gap before now a normal frame
	at = at.Add(135 * time.Millisecond)
	i.traceJump(7, o, "air", airport.LatLon{Lat: 50.00025, Lon: 14}, 240, 144)
	w.Close()
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "in 135ms, sim step 377ms at 144 kt") {
		t.Errorf("line: %q", out)
	}
}
