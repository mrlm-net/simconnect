package worldtest

import (
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestGameVFRField: in game mode at TFFJ, which publishes no SID, STAR or
// approach (VFR only), the game spawns departures and arrivals: the
// departure leaves VFR by a VFR exit, the arrival flies a straight-in
// visual approach and lands, smoothly and with no conflict watch (#1618:
// every departure refused with "runway 10 has no published procedure").
func TestGameVFRField(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	log := &lockedBuf{}
	o := Options{DataDir: dataDir(t), ICAO: "TFFJ", Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), "FSLTL_DHC6_WIA-Winair")}
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
	wx := nav.StaticWeather(100, 10, 9999, 100, 28, 1013)
	h.World.SetWeather(&wx)
	h.Advance(2 * time.Second)
	jumps := len(h.Sim.Jumps())
	if _, err := h.World.Do(http.MethodPost, "/api/game", map[string]any{"on": true, "icao": "TFFJ", "intervalSec": 120}); err != nil {
		t.Fatal(err)
	}
	dep, arr := "", ""
	if !h.Until(8*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			switch {
			case !v.Manual:
			case v.Kind == "departure" && dep == "":
				dep = v.Tail
			case v.Kind == "arrival" && arr == "":
				arr = v.Tail
			}
		}
		return dep != "" && arr != ""
	}) {
		t.Fatalf("the game spawned departure %q, arrival %q in 8 minutes:\n%s", dep, arr, spawnFailures(log.String()))
	}
	for _, cs := range []string{dep, arr} {
		v, _ := h.Aircraft(cs)
		t.Logf("%s %s %s %s on %q", cs, v.Kind, v.Rules, v.Model, v.Procedure)
		if cs == dep && (v.Rules != "VFR" || !strings.HasPrefix(v.Procedure, "VFR")) {
			t.Errorf("%s flies %s on %q, want VFR by a VFR exit", cs, v.Rules, v.Procedure)
		}
	}
	order := map[string][]string{
		dep: {"clearance", "handoff", "pushstart", "startup", "taxi", "lineup", "takeoff"},
		arr: {"clearance", "approach", "handoff", "land", "taxi"},
	}
	given := map[string]time.Time{}
	states := map[string][]string{}
	left, landed, vacated, flew := false, false, false, false
	if !h.Until(40*time.Minute, func() bool {
		for _, cs := range []string{dep, arr} {
			v, ok := h.Aircraft(cs)
			if !ok {
				continue
			}
			if s := states[cs]; len(s) == 0 || s[len(s)-1] != v.State {
				states[cs] = append(s, v.State)
			}
			if cs == dep && (v.Done || v.State == traffic.TaxiComplete.String() || v.State == traffic.TaxiDeparting.String() && !v.OnGround) {
				left = true
			}
			if cs == arr {
				flew = flew || !v.OnGround && v.State == "approaching"
				landed = landed || flew && noStarLanded(v)
				vacated = vacated || landed && v.State != "rollout"
			}
			if now := h.Clock.Now(); now.Sub(given[cs]) >= 3*time.Second {
				for _, a := range order[cs] {
					runway := a == "lineup" || a == "takeoff"
					if slices.Contains(v.Actions, a) && (cs != dep || !runway || vacated) {
						given[cs] = now
						if err := h.World.Clear(v.ID, a); err != nil {
							t.Logf("%s %s refused: %v", cs, a, err)
						}
						break
					}
				}
			}
		}
		return left && landed
	}) {
		t.Fatalf("in 40 minutes: %s left %v (%v), %s landed %v (%v)", dep, left, states[dep], arr, landed, states[arr])
	}
	for _, cs := range []string{dep, arr} {
		for _, j := range h.Sim.Jumps()[jumps:] {
			switch {
			case j.Tail != cs:
			case cs == dep:
				t.Errorf("%s jumped: %+v", cs, j)
			default:
				t.Logf("%s jumped (the straight-in onto the 642 m runway, #1616's, not this fix's): %+v", cs, j)
			}
		}
		if lines := conflictLines(log.String(), cs); len(lines) > 0 {
			t.Errorf("conflict watch:\n%s", strings.Join(lines, "\n"))
		}
	}
	if fails := spawnFailures(log.String()); strings.Contains(fails, "no published procedure") {
		t.Errorf("spawns refused:\n%s", fails)
	}
}
