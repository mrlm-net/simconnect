//go:build windows

package checklist

import (
	"slices"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/flight"
	"github.com/mrlm-net/simconnect/pkg/systems"
)

// TestFor: a stock A320 and the Fenix get the family's lists; another
// type none; the shipped sets are valid.
func TestFor(t *testing.T) {
	for _, s := range Shipped() {
		if err := s.Validate(); err != nil {
			t.Error(err)
		}
	}
	a320, ok := For(systems.Aircraft{Title: "Airbus A320neo Asobo", ATCType: "A20N"})
	if !ok || len(a320.Lists) < 8 {
		t.Fatalf("A320: %v, %d lists", ok, len(a320.Lists))
	}
	fenix, ok := For(systems.Aircraft{Package: "fnx-aircraft-320", Title: "FenixA319 CFM"})
	if !ok || fenix.Name != "Fenix A320" || len(fenix.Lists) != len(a320.Lists) {
		t.Errorf("Fenix: %v %q, %d lists", ok, fenix.Name, len(fenix.Lists))
	}
	if _, ok := For(systems.Aircraft{Title: "Cessna 172", ATCType: "C172"}); ok {
		t.Error("lists for a C172")
	}
}

// TestValidateActions: gear, flaps, autopilot and thrust are never a
// checklist's actions.
func TestValidateActions(t *testing.T) {
	for _, a := range []string{systems.GearDown, systems.FlapsDown, systems.APMaster, systems.Throttle, systems.ThrottleN(1)} {
		s := Set{Name: "x", Lists: []List{{Name: "l", Items: []Item{{Challenge: "X", Action: a}}}}}
		if s.Validate() == nil {
			t.Errorf("action %s allowed", a)
		}
	}
	ok := Set{Name: "x", Lists: []List{{Name: "l", Items: []Item{{Challenge: "X", Action: systems.Seatbelts}}}}}
	if err := ok.Validate(); err != nil {
		t.Error(err)
	}
}

// TestLocalOverride: a local set changes a response, removes an item, adds
// one and a list; the rest stays.
func TestLocalOverride(t *testing.T) {
	local, err := ReadSet(strings.NewReader(`{"name": "mine", "lists": [
		{"name": "landing", "items": [
			{"challenge": "autobrake", "response": "MED"},
			{"challenge": "CABIN CREW", "remove": true},
			{"challenge": "TCAS", "response": "TA/RA"}]},
		{"name": "taxi", "stages": ["taxi"], "items": [{"challenge": "BRAKES", "response": "CHECKED"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	s, _ := For(systems.Aircraft{ATCType: "A320"}, local)
	l, _ := s.List("landing")
	var got []string
	for _, it := range l.Items {
		got = append(got, it.Challenge+" "+it.Response)
	}
	if !slices.Contains(got, "AUTOBRAKE MED") || slices.Contains(got, "CABIN CREW ADVISED") || got[len(got)-1] != "TCAS TA/RA" {
		t.Errorf("landing %v", got)
	}
	if len(s.Due("taxi")) != 1 || len(s.Due("approach")) != 1 {
		t.Errorf("due taxi %d, approach %d", len(s.Due("taxi")), len(s.Due("approach")))
	}
}

// TestRunner: the PF calls, the PM reads, each item's role resolved by who
// is PF now; items checked against the state.
func TestRunner(t *testing.T) {
	s, _ := For(systems.Aircraft{ATCType: "A320"})
	l, _ := s.List("landing")
	r := Run(l)
	if r.CalledBy(true) != Copilot || r.ReadBy(true) != Player || r.CalledBy(false) != Player || r.ReadBy(false) != Copilot {
		t.Error("caller and reader not from the PF")
	}
	st := systems.State{Values: map[string]float64{systems.GearDown: 1, systems.FlapsIndex: 4, systems.SpoilersArmed: 0}}
	var gear, spoilers, cabin Status
	for !r.Done() {
		if it, _ := r.Current(); it.Challenge == "LDG GEAR" && r.Responder(true) != Copilot {
			t.Error("gear not the PF's (copilot)")
		}
		res, _ := r.Next(st)
		switch res.Item.Challenge {
		case "LDG GEAR":
			gear = res.Status
		case "SPLRS":
			spoilers = res.Status
		case "CABIN CREW":
			cabin = res.Status
		}
	}
	if gear != Done || spoilers != NotDone || cabin != CantCheck {
		t.Errorf("gear %s, spoilers %s, cabin %s", gear, spoilers, cabin)
	}
	if open := r.Open(); len(open) != 1 || open[0].Item.Challenge != "SPLRS" || open[0].Item.Action != systems.SpoilersArmed {
		t.Errorf("open %v", open)
	}
}

// TestForAssess: on a recorded flight the landing checklist completes when
// the gear, spoilers and landing flaps are set; Assess flags it when that
// is after 1000 ft.
func TestForAssess(t *testing.T) {
	s, _ := For(systems.Aircraft{ATCType: "A320"})
	tr := &flight.Track{}
	add := func(agl float64, ground bool, gear, armed bool, flaps int) {
		n := len(tr.Samples)
		tr.Samples = append(tr.Samples, flight.Sample{T: float64(n), AltFt: 500 + agl, GroundFt: 500, IAS: 150, VS: -700,
			OnGround: ground, GearHandle: gear, SpoilersArmed: armed, FlapsIndex: flaps, EngineCount: 2,
			Lights: flight.LightLanding | flight.LightStrobe | flight.LightBeacon})
	}
	add(0, true, true, false, 2) // the take-off roll
	add(0, true, true, false, 2)
	for h := 50.0; h < 3000; h += 50 {
		add(h, false, false, false, 0)
	}
	for h := 3000.0; h > 0; h -= 50 {
		late := h < 800 // everything set only below 800 ft
		add(h, false, late, late, map[bool]int{true: 4, false: 2}[late])
	}
	add(0, true, true, true, 4)
	done := ForAssess(s, tr)
	var landing *flight.ChecklistDone
	for i := range done {
		if done[i].Name == "landing" {
			landing = &done[i]
		}
	}
	if landing == nil || !landing.Done {
		t.Fatalf("landing %+v", done)
	}
	a := flight.Assess(tr, flight.AssessOptions{Checklists: done})
	found := false
	for _, f := range a.Findings {
		found = found || f.Code == "landing-checklist-late"
	}
	if !found {
		t.Errorf("no landing-checklist-late in %+v", a.Findings)
	}
}
