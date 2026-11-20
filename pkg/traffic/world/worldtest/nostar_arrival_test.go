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
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// startTNCM starts TNCM, whose runway 10 has approaches but no STAR, with
// the wind down runway 10.
func startTNCM(t *testing.T) (*Harness, *lockedBuf) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	log := &lockedBuf{}
	o := Options{DataDir: dataDir(t), ICAO: "TNCM", Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), "FSLTL_B744_KLM")}
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
	return h, log
}

// noStarSmooth reports cs's jumps from the jump trace from n on and its
// conflict watch lines.
func noStarSmooth(t *testing.T, h *Harness, log *lockedBuf, n int, cs string) {
	t.Helper()
	for _, j := range h.Sim.Jumps()[n:] {
		if j.Tail == cs {
			t.Errorf("%s jumped: %+v", cs, j)
		}
	}
	if lines := conflictLines(log.String(), cs); len(lines) > 0 {
		t.Errorf("conflict watch:\n%s", strings.Join(lines, "\n"))
	}
}

// TestGameArrivalWithoutSTAR: in game mode at TNCM, whose runway 10 has
// approaches but no STAR, arrivals are spawned on an approach and land
// (#1616: each refused with "runway 10 has no published procedure").
func TestGameArrivalWithoutSTAR(t *testing.T) {
	h, log := startTNCM(t)
	jumps := len(h.Sim.Jumps())
	if _, err := h.World.Do(http.MethodPost, "/api/game", map[string]any{"on": true, "icao": "TNCM", "intervalSec": 30}); err != nil {
		t.Fatal(err)
	}
	tail := ""
	if !h.Until(5*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.Manual {
				tail = v.Tail
			}
		}
		return tail != ""
	}) {
		t.Fatalf("the game spawned no arrival in 5 minutes:\n%s", spawnFailures(log.String()))
	}
	v, _ := h.Aircraft(tail)
	t.Logf("%s inbound on %q", tail, v.Procedure)
	cleared, flew := map[string]bool{}, false
	if !h.Until(40*time.Minute, func() bool {
		v, ok := h.Aircraft(tail)
		if !ok {
			return false
		}
		for _, act := range []string{"clearance", "approach", "handoff", "land"} {
			if !cleared[act] && slices.Contains(v.Actions, act) && h.World.Clear(v.ID, act) == nil {
				cleared[act] = true
			}
		}
		flew = flew || !v.OnGround && v.State == "approaching"
		return flew && noStarLanded(v)
	}) {
		v, _ := h.Aircraft(tail)
		t.Fatalf("%s never landed: %s (cleared %v)", tail, v.State, cleared)
	}
	noStarSmooth(t, h, log, jumps, tail)
}

// spawnFailures are the log's lines of spawns refused.
func spawnFailures(log string) string {
	var out []string
	for _, l := range strings.Split(log, "\n") {
		if strings.Contains(l, "could not be spawned") || strings.Contains(l, "no published procedure") {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// TestScheduleEnrouteArrivalWithoutSTAR: a schedule arrival to TNCM flown
// en route is handed over at its entry, flies the approach and lands, not
// cancelled (#1616: "the procedure has no point before the final").
func TestScheduleEnrouteArrivalWithoutSTAR(t *testing.T) {
	h, log := startTNCM(t)
	const cs = "KLM102"
	off := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"TNCM"}, Density: 1, MaxAircraft: 4, Generator: &off}); err != nil {
		t.Fatal(err)
	}
	sta := h.Clock.Now().Add(80 * time.Minute)
	if err := h.World.AddFlights([]traffic.Flight{
		{Callsign: cs, Airline: "KLM", Type: "B738", Origin: "EHAM", Destination: "TNCM", STD: sta.Add(-9 * time.Hour), STA: sta, DistanceNM: 3780},
	}); err != nil {
		t.Fatal(err)
	}
	if !h.Until(70*time.Minute, func() bool {
		return strings.Contains(log.String(), cs+" schedule: arrival") || strings.Contains(log.String(), "handed to the arrival controller")
	}) {
		t.Fatalf("%s never en route", cs)
	}
	jumps, flew := len(h.Sim.Jumps()), false
	if !h.Until(90*time.Minute, func() bool {
		v, ok := h.Aircraft(cs)
		flew = flew || ok && !v.OnGround && v.State == "approaching"
		return ok && flew && noStarLanded(v) || strings.Contains(log.String(), cs+" schedule: cancelled")
	}) {
		v, _ := h.Aircraft(cs)
		t.Fatalf("%s never landed: %q", cs, v.State)
	}
	for _, l := range strings.Split(log.String(), "\n") {
		if strings.Contains(l, cs) && (strings.Contains(l, "handover failed") || strings.Contains(l, "cancelled")) {
			t.Error(l)
		}
	}
	if v, ok := h.Aircraft(cs); !ok || !flew || !noStarLanded(v) {
		t.Fatalf("%s not landed", cs)
	}
	if !strings.Contains(log.String(), cs+" schedule: at ") {
		t.Errorf("%s not handed over en route", cs)
	}
	noStarSmooth(t, h, log, jumps, cs)
}

// noStarLanded: on the runway after its landing, or past it.
func noStarLanded(v world.ControlView) bool {
	switch v.State {
	case "rollout", "vacating", "awaiting taxi", "taxiing", "holding short", "parked":
		return true
	}
	return false
}
