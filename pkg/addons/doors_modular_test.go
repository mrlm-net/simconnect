package addons

import (
	"os"
	"path/filepath"
	"testing"
)

// A modular aircraft (MSFS 2024, laid out as the Fenix A32X): its preset's
// title has the door of its exterior attachment, the type from the
// attachment's [Inherit] base, across two packages (the A319 preset in one,
// its attachments beside the A320's in the other is the same).
func TestDoorsByTitleModular(t *testing.T) {
	pkg := t.TempDir()
	air := filepath.Join(pkg, "SimObjects", "Airplanes")
	put := func(rel, body string) {
		t.Helper()
		f := filepath.Join(air, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("FNX_32X/presets/fnx/FNX_320_CFM_SL/config/aircraft.cfg", "[fltsim.0]\ntitle = \"FenixA320 CFM SL\"\nisuserselectable = 1\n")
	put("FNX_32X/presets/fnx/FNX_320_CFM_SL/config/attached_objects.cfg",
		"[sim_attachment.0]\nattachment_root = \"SimObjects/Airplanes/FNX_32X/attachments/fnx/Function_Exterior_A320_CFM_SL\"\nattach_to_model = \"exterior\"\n"+
			"[sim_attachment.1]\nattachment_root = \"SimObjects/Airplanes/FNX_32X/attachments/fnx/Function_Interior_A320\"\nattach_to_model = \"interior\"\n")
	put("FNX_32X/attachments/fnx/Function_Exterior_A320_CFM_SL/attachment.cfg", "[Inherit]\nbase = \"SimObjects/Airplanes/FNX_32X/attachments/fnx/Part_Exterior_Fuselage_A320\"\n")
	put("FNX_32X/attachments/fnx/Function_Exterior_A320_CFM_SL/config/flight_model.cfg",
		"[CONTACT_POINTS]\npoint.0 = 1,27.690001,0,-10.533,720\n[INTERACTIVE POINTS]\ninteractive_point.0 = 0.400000 , 29.000000 , -6.049819, 1.842220, 0 ;L1\n[AIRPLANE_GEOMETRY]\nwing_span =111\n")
	put("FNX_32X/attachments/fnx/Part_Exterior_Fuselage_A320/config/aircraft.cfg", "[GENERAL]\nicao_type_designator = \"A320\"\n")

	d, ok := DoorsByTitle(air)["fenixa320 cfm sl"]
	if !ok {
		t.Fatal("no door for the modular preset FenixA320 CFM SL")
	}
	if d.Type != "A320" || d.First.Long != 29 || !d.HasNoseGear || d.NoseGearFt != 27.690001 || d.WingSpanFt != 111 {
		t.Errorf("door %+v, want A320, L1 29 ft, nose gear 27.69 ft, span 111", d)
	}
}

// A modular aircraft's preset titles are its package's, flyable as set.
func TestTitleSourcesModular(t *testing.T) {
	pkg := t.TempDir()
	f := filepath.Join(pkg, "SimObjects", "Airplanes", "FNX_32X", "presets", "fnx", "FNX_320_CFM_SL", "config", "aircraft.cfg")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("[fltsim.0]\ntitle = \"FenixA320 CFM SL\"\nisuserselectable = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, ok := TitleSources([]Package{{Folder: "fnx-aircraft-320", Path: pkg}})["fenixa320 cfm sl"]
	if !ok || s.Package != "fnx-aircraft-320" || !s.Flyable {
		t.Errorf("source %+v %v, want fnx-aircraft-320, flyable", s, ok)
	}
}
