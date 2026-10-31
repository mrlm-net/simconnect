package worldtest

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

func startLKPR(t *testing.T) *Harness {
	t.Helper()
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
	t.Cleanup(h.Close)
	return h
}

// TestLineUpTakeoffLandingWords: a departure is offered line up only with the
// runway next and close, goes on to line up when told while taxiing; its take-off
// takes "immediate" and "no delay", and an arrival cleared to land while it rolls
// hears of it (#1331).
func TestLineUpTakeoffLandingWords(t *testing.T) {
	h := startLKPR(t)
	h.World.UseRunway("LKPR", "24")
	if _, err := h.World.Do(http.MethodPost, "/api/control", map[string]any{"kind": "departure", "icao": "LKPR", "tail": "WORD1", "runway": "24", "stand": -1, "model": "A320"}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	far := false
	if !h.Until(30*time.Minute, func() bool {
		v, ok := h.Aircraft("WORD1")
		if !ok {
			return false
		}
		dep = v
		lineup := slices.Contains(v.Actions, "lineup")
		if v.State == traffic.TaxiTaxiing.String() {
			if slices.Contains(v.Actions, "takeoff") {
				t.Fatalf("take-off offered while taxiing: %v", v.Actions)
			}
			if lineup {
				if err := h.World.Clear(v.ID, "lineup"); err != nil {
					t.Fatalf("line up while taxiing: %v", err)
				}
				return true
			}
			far = true
		}
		for _, a := range []string{"clearance", "pushstart", "pushback", "startup", "taxi", "cross"} {
			if slices.Contains(v.Actions, a) {
				h.World.Clear(v.ID, a)
				break
			}
		}
		return false
	}) {
		t.Fatalf("line up never offered while taxiing: %s %v", dep.State, dep.Actions)
	}
	if !far {
		t.Error("line up offered from the start of the taxi")
	}
	if !h.Until(5*time.Minute, func() bool {
		v, _ := h.Aircraft("WORD1")
		return v.State == traffic.TaxiLinedUp.String()
	}) {
		v, _ := h.Aircraft("WORD1")
		t.Fatalf("told to line up while taxiing, %s", v.State)
	}
	phrase := func(id int, q string) string {
		t.Helper()
		b, err := h.World.Do(http.MethodGet, fmt.Sprintf("/api/control/%d/%s", id, q), nil)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return string(b)
	}
	if got := phrase(dep.ID, "takeoff/phrase?immediate=1"); !strings.Contains(got, "cleared for immediate take-off") {
		t.Errorf("immediate: %s", got)
	}

	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 3, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var arr world.ControlView
	if !h.Until(30*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.Runway == "24" && slices.Contains(v.Actions, "land") {
				arr = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no arrival for 24")
	}
	if got := phrase(arr.ID, "land/phrase"); strings.Contains(got, "departing ahead") {
		t.Errorf("no one rolling: %s", got)
	}
	if err := h.World.Clear(dep.ID, "takeoff?nodelay=1"); err != nil {
		t.Fatal(err)
	}
	h.Advance(time.Second)
	if !h.Said("WORD1", "no delay") {
		t.Error("take-off without no delay")
	}
	heard := false
	h.Until(2*time.Minute, func() bool {
		if strings.Contains(phrase(arr.ID, "land/phrase"), "traffic departing ahead") {
			heard = true
		}
		return heard
	})
	if !heard {
		v, _ := h.Aircraft("WORD1")
		t.Errorf("landing clearance never named the departure, %s on ground %v", v.State, v.OnGround)
	}
}

// TestUptoByName: a limit given by taxiway name is said as that taxiway; one its
// route does not meet is refused (#1331).
func TestUptoByName(t *testing.T) {
	h := startLKPR(t)
	h.World.UseRunway("LKPR", "24")
	if _, err := h.World.Do(http.MethodPost, "/api/control", map[string]any{"kind": "departure", "icao": "LKPR", "tail": "WORD2", "runway": "24", "stand": -1, "model": "A320"}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	if !h.Until(10*time.Minute, func() bool {
		v, ok := h.Aircraft("WORD2")
		dep = v
		if ok && slices.Contains(v.Actions, "upto") {
			return true
		}
		for _, a := range []string{"clearance", "pushstart", "pushback", "startup"} {
			if ok && slices.Contains(v.Actions, a) {
				h.World.Clear(v.ID, a)
				break
			}
		}
		return false
	}) {
		t.Fatalf("upto never offered: %s %v", dep.State, dep.Actions)
	}
	g, err := airport.BuildGraph(h.Layout)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, edges := range g.Adj {
		for _, e := range edges {
			if e.Name != "" {
				names[e.Name] = true
			}
		}
	}
	met := ""
	for name := range names {
		b, err := h.World.Do(http.MethodGet, fmt.Sprintf("/api/control/%d/upto/phrase?taxiway=%s", dep.ID, name), nil)
		if err != nil {
			continue
		}
		if !strings.Contains(string(b), "hold short of "+name) {
			t.Errorf("limit %s said: %s", name, b)
		}
		met = name
	}
	if met == "" {
		t.Fatalf("none of %d taxiways met", len(names))
	}
	if err := h.World.Clear(dep.ID, "upto?taxiway=ZZ9"); err == nil || !strings.Contains(err.Error(), "does not meet") {
		t.Errorf("a taxiway off its route: %v", err)
	}
	if err := h.World.Clear(dep.ID, "upto?taxiway="+met); err != nil {
		t.Errorf("upto %s: %v", met, err)
	}
	h.Advance(time.Second)
	if !h.Said("WORD2", "hold short of "+met) {
		t.Errorf("not told to hold short of %s", met)
	}
}

// TestExpeditedClimb: a staffed departure told to expedite through a level is
// said so and climbs faster than at the usual 2000 ft/min (#1331).
func TestExpeditedClimb(t *testing.T) {
	h := startLKPR(t)
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "departure", Mode: "silent"}})
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 2, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var dep world.ControlView
	if !h.Until(30*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && slices.Contains(v.Actions, "climb") {
				dep = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no departure on the staffed departure")
	}
	alt := func() float64 {
		v, _ := h.Aircraft(dep.Tail)
		return v.AltFt
	}
	h.Advance(3 * time.Minute)
	from := alt()
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/climb?level=30000&through=%d", dep.ID, int(from)+8000), nil); err != nil {
		t.Fatal(err)
	}
	h.Advance(time.Second)
	if !h.Said(dep.Tail, "expedite through") {
		t.Error("not told to expedite")
	}
	h.Advance(time.Minute)
	if got := alt() - from; got < 2400 {
		t.Errorf("%.0f ft in a minute expedited", got)
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/climb?level=31000&through=99000", dep.ID), nil); err == nil {
		t.Error("through a level above the one cleared")
	}
}

// TestExpeditedDescentAndSpeed: on its STAR an arrival is told to expedite the
// descent, and a speed (#1331).
func TestExpeditedDescentAndSpeed(t *testing.T) {
	h := startLKPR(t)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 3, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var arr world.ControlView
	if !h.Until(20*time.Minute, func() bool {
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "arrival" && v.State == "approaching" && v.AltFt > 9000 {
				arr = v
				return true
			}
		}
		return false
	}) {
		t.Fatal("no arrival on its STAR above 9000 ft")
	}
	// In the landing sequence a moment after it appears.
	var err error
	if !h.Until(2*time.Minute, func() bool {
		_, err = h.World.Do(http.MethodPost, "/api/approach/LKPR/"+arr.Tail+"/descend", map[string]any{"ft": 7000, "expedite": true})
		return err == nil
	}) {
		t.Fatal(err)
	}
	h.Advance(time.Second)
	if !h.Said(arr.Tail, "expedite descent to") {
		t.Error("not told to expedite the descent")
	}
	if _, err := h.World.Do(http.MethodPost, "/api/approach/LKPR/"+arr.Tail+"/speed", map[string]any{"kts": 220, "expedite": true}); err != nil {
		t.Fatal(err)
	}
	h.Advance(time.Second)
	if !h.Said(arr.Tail, "knots, expedite") {
		t.Error("not told to expedite the speed")
	}
}
