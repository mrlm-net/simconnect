//go:build windows
// +build windows

package systems

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Take-off speeds: each A320-family variant from its own table by weight
// and flaps (the Fenix A319 too, by its title), a model reading its FMS
// speeds those, an unknown type none; the speed check by type.
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
	if s := read(a320, map[string]float64{Weight: 62500, FlapsIndex: 0}); s.V1Kt != 135 {
		t.Errorf("flaps not set yet: V1 %.0f, want CONF 1+F's 135", s.V1Kt)
	}
	fenix := For(Aircraft{Package: "fnx-aircraft-320", Title: "FenixA319 CFM WF HD"})
	if s := read(fenix, map[string]float64{Weight: 62500, FlapsIndex: 1}); s.SpeedsFrom != SpeedsTable || s.V1Kt != 134 || s.VRKt != 135 || s.V2Kt != 139 || s.SpeedCheckKt != 100 {
		t.Errorf("Fenix A319 at 62.5 t: %+v, want the A319's table", s)
	}
	a321 := For(Aircraft{Title: "Airbus A321neo", ATCType: "A21N"})
	if s := read(a321, map[string]float64{Weight: 80000, FlapsIndex: 1}); s.V1Kt != 147 || s.VRKt != 150 || s.V2Kt != 153 {
		t.Errorf("A321 at 80 t: %+v", s)
	}
	fms := Merge(a320, Profile{Values: map[string]Value{V1: {Vars: []string{"L:V1"}}, VR: {Vars: []string{"L:VR"}}, V2: {Vars: []string{"L:V2"}}}})
	if s := read(fms, map[string]float64{V1: 141, VR: 142, V2: 147, Weight: 62500, FlapsIndex: 1}); s.SpeedsFrom != SpeedsFMS || s.V1Kt != 141 || s.V2Kt != 147 {
		t.Errorf("FMS speeds: %+v", s)
	}
	if s := read(For(Aircraft{Title: "Unknown Jet", ATCType: "ZZZZ"}), map[string]float64{Weight: 62500}); s.SpeedsFrom != "" || s.SpeedCheckKt != 80 {
		t.Errorf("unknown type: %+v", s)
	}
}

// The MyCrew API's "aircraft-speeds" set corrects a type's table without a
// release.
func TestSpeedsSet(t *testing.T) {
	defer dict.Reset("systems.speeds")
	fed, err := dict.UseSet("aircraft-speeds", []byte(`{"items": [{"key": "A319", "payload": {"type": "A319", "weightsKg": [60000], "flaps": {"*": {"v1": [130], "vr": [131], "v2": [135]}}}}]}`))
	if err != nil || len(fed) != 1 || fed[0] != "systems.speeds" {
		t.Fatalf("fed %v, %v", fed, err)
	}
	i, ok := SpeedsFor(Aircraft{Title: "FenixA319"})

	if v1, _, _, got := i.Speeds(1, 70000); !ok || !got || v1 != 130 {
		t.Errorf("A319 V1 %.0f after the set, want 130", v1)
	}
	if _, ok := SpeedsFor(Aircraft{ATCType: "A321"}); !ok {
		t.Error("the A321 dropped: the set replaced the table")
	}
}

// No FMS speeds and no table: computed from the aircraft's design speeds
// (the Fenix A319's, read live), scaled by weight; a light aircraft
// rotates only.
func TestCalcSpeeds(t *testing.T) {
	read := func(vals map[string]float64) State {
		s := State{Values: vals}
		takeoffSpeeds(Default(), &s)
		return s
	}
	a319 := map[string]float64{DesignVS0: 119, DesignVS1: 148, DesignVR: 146, DesignV2: 150, MaxWeight: 75500, Weight: 62500}
	if s := read(a319); s.SpeedsFrom != SpeedsCalc || s.V1Kt != 131 || s.VRKt != 133 || s.V2Kt != 137 {
		t.Errorf("A319 design speeds at 62.5 t: %+v", s)
	}
	noRot := map[string]float64{DesignVS0: 119, DesignVS1: 148, MaxWeight: 75500, Weight: 75500}
	if s := read(noRot); s.VRKt != 139 || s.V2Kt != 152 {
		t.Errorf("from the stall speeds: %+v, want VR 139, V2 152", s)
	}
	c172 := map[string]float64{DesignVS0: 40, DesignVS1: 48, DesignVR: 55, MaxWeight: 1157, Weight: 1100}
	if s := read(c172); s.V1Kt != 0 || s.VRKt != 54 || s.V2Kt != 0 {
		t.Errorf("C172: %+v, want VR 54 only", s)
	}
	if s := read(map[string]float64{Weight: 62500}); s.SpeedsFrom != "" {
		t.Errorf("nothing known: %+v", s)
	}
}
