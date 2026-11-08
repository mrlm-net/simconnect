package addons

import (
	"os"
	"path/filepath"
	"testing"
)

// The reference datum is read (the FSLTL B748's and its helpers' 83.5 ft);
// none given, 0.
func TestModelDoorDatum(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "MyCrew_JetwayHelper_B748_D-A320")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("aircraft.cfg", "[GENERAL]\nicao_type_designator = \"B748\"\n")
	fm := "[INTERACTIVE POINTS]\ninteractive_point.0 = 0.4, 37.654, -6.137,  0.203 ,0, 0, 0, -86 ; MyCrew: the door of FSLTL_A320\n" +
		"[CONTACT_POINTS]\npoint.0 = 1,  9.163,  0.0, -14.901, 1600 ;\n"
	write("flight_model.cfg", fm+"[WEIGHT_AND_BALANCE]\nreference_datum_position = 83.5, 0, 0 ; (feet) distance from FlightSim Reference position\n")
	d, ok := ReadModelDoor(dir)
	if !ok || d.DatumFt != 83.5 || d.First.Long != 37.654 || d.NoseGearFt != 9.163 {
		t.Errorf("datum %v, door %v, nose gear %v (%v)", d.DatumFt, d.First.Long, d.NoseGearFt, ok)
	}
	write("flight_model.cfg", fm)
	if d, _ := ReadModelDoor(dir); d.DatumFt != 0 {
		t.Errorf("no datum given: %v", d.DatumFt)
	}
}
