package worldtest

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestVFRCircuitSpeed: a VFR arrival round the TFFJ circuit is shown
// moving at the speed it is written at, from its circuit to its
// touchdown: within 1.3 times it either way, and never more than 2 kt
// faster or slower from one frame to the next, while the World places it
// (SimlessAI flies its circuit in once-a-second moves). Before, the injected
// approach took over in the turn from downwind to base, 1.4 km off the
// centreline, and faded that out over 20 s: shown at up to 212 kt while
// written at 73; then it flew 54 kt for its touchdown 240 m early and went
// to 73 in one frame (#1713).
func TestVFRCircuitSpeed(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	const icao, rwy, cs, title = "TFFJ", "10", "VFR1", "FSLTL_DHC6_WIA-Winair"
	d := dataDir(t)
	if m, _ := filepath.Glob(filepath.Join(d, "fields", "*", icao, "layout.json")); len(m) == 0 && !fileExists(filepath.Join(d, icao, "layout.json")) {
		t.Skipf("no %s data", icao)
	}
	log := &lockedBuf{}
	o := Options{DataDir: d, ICAO: icao, Manual: true, FrameRate: 10, AircraftTitles: append(slices.Clone(DefaultAircraftTitles), title)}
	var out io.Writer = log
	if testing.Verbose() {
		out = io.MultiWriter(log, os.Stderr)
	}
	o.World.Output = out
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, end, ok := h.Layout.RunwayEnd(rwy)
	if !ok {
		t.Fatalf("no runway %s at %s", rwy, icao)
	}
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 100, 25, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway(icao, rwy)
	h.Advance(2 * time.Second)
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "arrival", ICAO: icao, Tail: cs, Runway: rwy, Stand: -1, Model: title, Circuit: true, InjectApproach: true}); err != nil {
		t.Fatal(err)
	}
	var last Object
	var lastAt time.Time
	var seen, flew, lastWritten bool
	lastPos := -1
	lastV := -1.0
	worstRatio, worstStep := 1.0, 0.0
	var ratioAt, stepAt string
	frames := 0
	if !h.Until(30*time.Minute, func() bool {
		now := h.Clock.Now()
		for _, o := range h.Sim.Objects() {
			if o.ATCID != cs {
				continue
			}
			if o.OnGround {
				return flew
			}
			flew = true
			// Placed by the World this frame and the last: not flown by
			// SimlessAI, whose moves come once a second.
			pos := h.Sim.CallsOf(cs)[CallPosition]
			written := pos > lastPos && lastPos >= 0
			if seen && written && lastWritten && now.After(lastAt) && o.GroundKts > 30 {
				dt := now.Sub(lastAt).Seconds()
				v := groundMeters(last.Lat, last.Lon, o.Lat, o.Lon) / dt / 0.514444
				frames++
				if r := math.Max(v/o.GroundKts, o.GroundKts/math.Max(v, 1e-6)); r > worstRatio {
					worstRatio, ratioAt = r, fmt.Sprintf("%.0f kt shown, %.0f kt written, %.0f ft", v, o.GroundKts, o.AltFt)
				}
				if lastV >= 0 && math.Abs(v-lastV) > worstStep {
					worstStep, stepAt = math.Abs(v-lastV), fmt.Sprintf("%.1f to %.1f kt, %.0f ft", lastV, v, o.AltFt)
				}
				lastV = v
			}
			last, lastAt, seen, lastPos, lastWritten = o, now, true, pos, written
		}
		return false
	}) {
		t.Fatalf("%s did not land in 30 minutes", cs)
	}
	if frames < 100 {
		t.Fatalf("only %d frames flown", frames)
	}
	t.Logf("%d frames: shown speed at most %.2f times off the written (%s); a frame changed it by %.1f kt at most (%s)", frames, worstRatio, ratioAt, worstStep, stepAt)
	if worstRatio > 1.3 {
		t.Errorf("shown %.2f times off its written speed (%s)", worstRatio, ratioAt)
	}
	if worstStep > 2 {
		t.Errorf("shown speed changed %.1f kt in a frame (%s)", worstStep, stepAt)
	}
}

// groundMeters is the distance between two points, metres (flat earth:
// one frame apart).
func groundMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const m = 111319.49
	dx := (lon2 - lon1) * m * math.Cos((lat1+lat2)/2*math.Pi/180)
	dy := (lat2 - lat1) * m
	return math.Hypot(dx, dy)
}
