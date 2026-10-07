//go:build windows
// +build windows

package systems

import "testing"

// Take-off speeds: an A320 from its type's table by weight and flaps, the
// Fenix none (its own FMS speeds not read yet), a model reading its FMS
// speeds those; the speed check by profile.
func TestTakeoffSpeeds(t *testing.T) {
	read := func(p Profile, vals map[string]float64) State {
		s := State{Values: vals}
		takeoffSpeeds(p, &s)
		return s
	}
	a320 := For(Aircraft{Title: "Airbus A320neo Asobo", ATCType: "A20N"})
	s := read(a320, map[string]float64{Weight: 62500, FlapsIndex: 1})
	if s.SpeedsFrom != SpeedsTable || s.V1Kt != 135 || s.VRKt != 136 || s.V2Kt != 140 || s.SpeedCheckKt != 100 {
		t.Errorf("A320 at 62.5 t, flaps 1: %+v", s)
	}
	if s := read(a320, map[string]float64{Weight: 90000, FlapsIndex: 3}); s.V1Kt != 146 {
		t.Errorf("above the table: V1 %.0f, want held at 146", s.V1Kt)
	}
	if s := read(a320, map[string]float64{Weight: 62500, FlapsIndex: 0}); s.SpeedsFrom != "" || s.V1Kt != 0 {
		t.Errorf("flaps up: %+v, want no speeds", s)
	}
	fenix := For(Aircraft{Package: "fnx-aircraft-320", Title: "FenixA319 CFM", ATCType: "A319"})
	if s := read(fenix, map[string]float64{Weight: 62500, FlapsIndex: 1}); s.SpeedsFrom != "" || s.SpeedCheckKt != 100 {
		t.Errorf("Fenix: %+v, want no speeds yet and the Airbus speed check", s)
	}
	fms := Merge(a320, Profile{Values: map[string]Value{V1: {Vars: []string{"L:V1"}}, VR: {Vars: []string{"L:VR"}}, V2: {Vars: []string{"L:V2"}}}})
	if s := read(fms, map[string]float64{V1: 141, VR: 142, V2: 147, Weight: 62500, FlapsIndex: 1}); s.SpeedsFrom != SpeedsFMS || s.V1Kt != 141 || s.V2Kt != 147 {
		t.Errorf("FMS speeds: %+v", s)
	}
	if s := read(Default(), map[string]float64{Weight: 62500}); s.SpeedsFrom != "" || s.SpeedCheckKt != 80 {
		t.Errorf("default: %+v", s)
	}
}
