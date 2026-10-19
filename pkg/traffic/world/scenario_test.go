package world

import (
	"github.com/mrlm-net/simconnect/pkg/nav"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestCastTrackCues: a cast aircraft's cues hold once reached: a state
// passed, airborne and its height on the climb, a light once on, the
// distance from the airport closing in and going out.
func TestCastTrackCues(t *testing.T) {
	tr := &castTrack{seen: map[string]bool{}}
	tr.update(ControlView{State: "taxiing", OnGround: true, Lights: "NB.T...", Position: airport.LatLon{Lat: 50.1, Lon: 14.26}}, 0, 0.5)
	tr.update(ControlView{State: "departing", Lights: "NBS.L..", Position: airport.LatLon{Lat: 50.2, Lon: 14.4}}, 3200, 12)
	tr.update(ControlView{State: "complete", Position: airport.LatLon{Lat: 50.3, Lon: 14.6}}, 9000, 25)
	for cue, want := range map[string]bool{
		"taxiing": true, "departing": true, "airborne": true, "lined up": false,
		"lights:T": true, "lights:L": true, "lights:W": false,
		"height:3000": true, "height:4000": false,
		"distance:1": true, "distance:0.2": false, "distance:>20": true, "distance:>30": false,
	} {
		if got, err := tr.met(cue); err != nil || got != want {
			t.Errorf("%s: %v %v, want %v", cue, got, err, want)
		}
	}
	if _, err := tr.met("height:high"); err == nil {
		t.Error("height:high: no error")
	}
}

// TestScenarioValidate: roles once each, a known kind, delays that parse,
// actions on a role of the cast; an unknown action only with a host.
func TestScenarioValidate(t *testing.T) {
	ok := Scenario{Cast: []ScenarioCast{{Role: "dep", Kind: "departure"}}, Steps: []ScenarioStep{
		{ID: "a", Cue: "start", Delay: "40s", Actions: []ScenarioAction{{Do: "push", Who: "dep"}}},
		{Cue: "after:a", Actions: []ScenarioAction{{Do: "runway", Runway: "12/30"}}},
	}}
	if err := ok.validate(ScenarioHooks{}); err != nil {
		t.Fatal(err)
	}
	bad := []Scenario{
		{Cast: []ScenarioCast{{Role: "dep", Kind: "departure"}, {Role: "dep", Kind: "arrival"}}},
		{Cast: []ScenarioCast{{Role: "dep", Kind: "vfr"}}},
		{Steps: []ScenarioStep{{Cue: "start", Delay: "soon"}}},
		{Steps: []ScenarioStep{{Cue: "start", Actions: []ScenarioAction{{Do: "push", Who: "nobody"}}}}},
		{Steps: []ScenarioStep{{Cue: "start", Actions: []ScenarioAction{{Do: "failure"}}}}},
	}
	for i, sc := range bad {
		if sc.validate(ScenarioHooks{}) == nil {
			t.Errorf("bad scenario %d passed", i)
		}
	}
	host := Scenario{Steps: []ScenarioStep{{Cue: "player:stage:taxi_out", Actions: []ScenarioAction{{Do: "failure"}}}}}
	if err := host.validate(ScenarioHooks{Action: func(ScenarioAction) (bool, error) { return true, nil }}); err != nil {
		t.Errorf("host action: %v", err)
	}
}

// TestUseRunway: a runway made the one in use is chosen whatever the wind
// (LKPR 24 in a 15 kt wind from 060), and the wind chooses again once it
// is given back.
func TestUseRunway(t *testing.T) {
	l := lkprLayout(t)
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	w := &World{st: &state{core: newCore(&trafficLog{})}}
	wx := nav.Weather{WindDirTrue: 60, WindKts: 15, VisibilityM: 9999}
	lim := airport.LimitsFor(l, nil)
	choose := func() string { return nav.ActiveRunways(l, wx, w.st.core.runwayLimits(g, lim)).Departure.Name }
	if got := choose(); got != "06" {
		t.Fatalf("by the wind: %s, want 06", got)
	}
	w.UseRunway("LKPR", "24")
	if got := choose(); got != "24" {
		t.Errorf("made the runway in use: %s, want 24", got)
	}
	w.UseRunway("LKPR", "")
	if got := choose(); got != "06" {
		t.Errorf("given back: %s, want 06", got)
	}
}
