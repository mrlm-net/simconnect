package worldtest

import (
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// sceneRadioRun plays a scene with scripted lines among traffic and gives what
// the host heard and what the scene told.
func sceneRadioRun(t *testing.T, mute bool) (heard []traffic.Transmission, radio []world.SceneEvent, h *Harness) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	var mu sync.Mutex
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	o.World.OnTransmission = func(tx traffic.Transmission) {
		mu.Lock()
		defer mu.Unlock()
		heard = append(heard, tx)
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	if !h.Until(10*time.Minute, func() bool { return len(h.World.Recent("LKPR", 30)) >= 20 }) {
		t.Fatal("the radio stays quiet")
	}
	busy := func(n int) string { // the frequency most is said on
		count := map[string]int{}
		best := ""
		for _, tx := range h.World.Recent("LKPR", n) {
			if count[tx.Frequency]++; tx.Frequency != "" && count[tx.Frequency] > count[best] {
				best = tx.Frequency
			}
		}
		return best
	}
	f := busy(100)
	h.World.OnSceneEvent(func(e world.SceneEvent) {
		if e.Kind == "radio" {
			mu.Lock()
			defer mu.Unlock()
			radio = append(radio, e)
		}
	})
	mu.Lock()
	heard = nil
	mu.Unlock()
	sc := world.Scene{Name: "lines", LimitSec: 120, Mute: mute,
		Cast: []world.SceneCast{{Role: "dep", Kind: "departure", Tail: "LINE1", Models: "a320"}},
		Lines: []world.SceneLine{
			{AtSec: 4, Who: "Film Tower", To: "LINE1", Frequency: f, Text: "Line one, wind calm, hold position"},
			{AtSec: 12, Who: "dep", Frequency: f, Text: "Holding position, line one"},
			{AtSec: 30, Who: "Film Tower", Frequency: f, Text: "All stations, film tower closing"},
		},
		Beats: []world.SceneBeat{{Cue: "start", Shots: []world.SceneShot{{Move: "topOrbit", Who: "dep", Sec: 60, Long: true}}}}}
	if err := h.World.PlayScene("LKPR", sc); err != nil {
		t.Fatal(err)
	}
	scripted := func() int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, e := range radio {
			if e.Scripted {
				n++
			}
		}
		return n
	}
	// The traffic's clock at the speed of the scene's, as in the simulator at 1x.
	last := time.Now()
	real := func() {
		time.Sleep(50 * time.Millisecond)
		now := time.Now()
		h.Advance(now.Sub(last))
		last = now
	}
	for wait := time.Now().Add(90 * time.Second); scripted() < len(sc.Lines); real() {
		if time.Now().After(wait) {
			t.Fatalf("lines told: %d", scripted())
		}
	}
	for wait := time.Now().Add(time.Second); time.Now().Before(wait); {
		real()
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]traffic.Transmission(nil), heard...), append([]world.SceneEvent(nil), radio...), h
}

// TestSceneLinesToTheTimetable: scripted lines go at their time word for word,
// said by whom the script says, and nothing else on the frequency talks over them.
func TestSceneLinesToTheTimetable(t *testing.T) {
	_, radio, h := sceneRadioRun(t, false)
	want := map[string]struct {
		at                float64
		speaker, callsign string
	}{
		"Line one, wind calm, hold position": {4, "Film Tower", "LINE1"},
		"Holding position, line one":         {12, "LINE1", ""},
		"All stations, film tower closing":   {30, "Film Tower", ""},
	}
	var freq string
	type span struct{ from, to float64 }
	var lines []span
	for _, e := range radio {
		if !e.Scripted {
			continue
		}
		w, ok := want[e.Text]
		if !ok {
			t.Errorf("line changed on the way: %q", e.Text)
			continue
		}
		if math.Abs(e.AtSec-w.at) > 1.5 {
			t.Errorf("%q at %.1f s, want %.0f", e.Text, e.AtSec, w.at)
		}
		if e.Speaker != w.speaker || e.Callsign != w.callsign || e.DurSec <= 0 || e.Frequency == "" {
			t.Errorf("%q: %+v", e.Text, e)
		}
		freq = e.Frequency
		lines = append(lines, span{e.AtSec, e.AtSec + e.DurSec})
	}
	others := 0
	for _, e := range radio {
		if e.Scripted {
			continue
		}
		others++
		if e.Muted {
			t.Errorf("not muted, told muted: %+v", e)
		}
		if e.Frequency != freq {
			continue
		}
		for _, l := range lines {
			if e.AtSec < l.to && e.AtSec+e.DurSec > l.from {
				t.Errorf("%s at %.1f–%.1f over a line at %.1f–%.1f: %q", e.Speaker, e.AtSec, e.AtSec+e.DurSec, l.from, l.to, e.Text)
			}
		}
	}
	if others == 0 {
		t.Error("no other traffic on the radio, nothing proved")
	}
	_ = h
}

// TestSceneMute: muted, the host hears the lines alone; the rest is still told.
func TestSceneMute(t *testing.T) {
	heard, radio, _ := sceneRadioRun(t, true)
	said := 0
	for _, tx := range heard {
		if !tx.Scripted {
			t.Errorf("the host heard %q from %s", tx.Text, tx.Callsign)
			continue
		}
		said++
	}
	if said != 3 {
		t.Errorf("host heard %d lines", said)
	}
	muted := 0
	for _, e := range radio {
		if e.Muted {
			muted++
		}
	}
	if muted == 0 {
		t.Error("nothing told as muted")
	}
}
