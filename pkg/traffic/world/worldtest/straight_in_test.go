package worldtest

import (
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestStraightInSmooth: an arrival flown straight in on the final (#1618)
// appears where it flies on, lands within its runway's first part and
// rolls out on the centreline to its exit and stand with no jump, at TFFJ
// (no procedure: 642 m, a DHC6) and LKPR (an A320 spawned on the final).
// Before, it moved 32 m in its first frame (the touchdown spread applied
// at once), touched down 386 m into TFFJ's 642 m and swung 6–26° a frame
// at 64 kt; its first call at TFFJ said it was "3858 miles east".
func TestStraightInSmooth(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	cases := []struct {
		icao, rwy, title string
		maxTouchdown     float64 // share of the runway's length
		procedure        bool    // asked for its procedures: none at TFFJ
	}{
		{"TFFJ", "10", "FSLTL_DHC6_WIA-Winair", 0.3, true},
		{"LKPR", "24", "FSLTL A320 Air France SL", 0.25, false},
	}
	for _, tc := range cases {
		t.Run(tc.icao, func(t *testing.T) {
			straightInRun(t, tc.icao, tc.rwy, tc.title, tc.maxTouchdown, tc.procedure)
		})
	}
}

var touchdownLine = regexp.MustCompile(`touchdown (\d+) m past the threshold`)

func straightInRun(t *testing.T, icao, rwy, title string, maxTouchdown float64, procedure bool) {
	d := dataDir(t)
	if m, _ := filepath.Glob(filepath.Join(d, "fields", "*", icao, "layout.json")); len(m) == 0 && !fileExists(filepath.Join(d, icao, "layout.json")) {
		t.Skipf("no %s data", icao) // TFFJ on the Linux director's LKPR-only data
	}
	log := &lockedBuf{}
	o := Options{DataDir: d, ICAO: icao, Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), title)}
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
	rw, end, ok := h.Layout.RunwayEnd(rwy)
	if !ok {
		t.Fatalf("no runway %s at %s", rwy, icao)
	}
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 100, 25, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway(icao, rwy)
	h.Advance(2 * time.Second)
	jumps := len(h.Sim.Jumps())
	const cs = "SIN1"
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "arrival", ICAO: icao, Tail: cs, Runway: rwy, Stand: -1, Model: title, Procedure: procedure, InjectApproach: true}); err != nil {
		t.Fatal(err)
	}
	var states []string
	if !h.Until(30*time.Minute, func() bool {
		v, ok := h.Aircraft(cs)
		if !ok {
			return false
		}
		if len(states) == 0 || states[len(states)-1] != v.State {
			states = append(states, v.State)
		}
		return v.State == "parked" || v.Done
	}) {
		t.Fatalf("%s not on its stand in 30 minutes: %v", cs, states)
	}
	t.Logf("%s: %v", cs, states)
	for _, j := range h.Sim.Jumps()[jumps:] {
		if j.Tail == cs {
			t.Errorf("%s jumped: %+v", cs, j)
		}
	}
	if lines := conflictLines(log.String(), cs); len(lines) > 0 {
		t.Errorf("conflict watch:\n%s", strings.Join(lines, "\n"))
	}
	m := touchdownLine.FindStringSubmatch(log.String())
	if m == nil {
		t.Fatalf("%s: no touchdown logged", cs)
	}
	td, _ := strconv.ParseFloat(m[1], 64)
	t.Logf("touchdown %.0f m past the threshold of %.0f m", td, rw.Length)
	if td > maxTouchdown*rw.Length {
		t.Errorf("touchdown %.0f m past the threshold, want within the first %.0f%% of %.0f m", td, maxTouchdown*100, rw.Length)
	}
	for _, tx := range h.Heard() {
		if tx.Callsign != cs || !tx.Pilot || !strings.Contains(tx.Text, "inbound for landing") {
			continue
		}
		f := strings.Fields(tx.Text)
		for i, w := range f {
			if strings.HasPrefix(w, "mile") && i > 0 {
				if n, err := strconv.Atoi(f[i-1]); err == nil && n > 30 {
					t.Errorf("first call %q: %d miles out", tx.Text, n)
				}
			}
		}
	}
}
