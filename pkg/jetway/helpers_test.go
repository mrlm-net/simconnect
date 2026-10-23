package jetway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A helper per type: the base model's flight model copied (the door the
// jetway docks to), our own FLTSIMs (gates only, invisible), the empty
// model, and a layout listing every file.
func TestWritePackage(t *testing.T) {
	base := filepath.Join("..", "addons", "testdata", "doors", "FSLTL_A320")
	out := t.TempDir()
	root, err := WritePackage(out, []HelperModel{{Type: "A320", Dir: base}}, PackageOptions{Creator: "MyCrew", Codes: []string{"DLH", "AFR"}})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_A320")
	cfg, err := os.ReadFile(filepath.Join(dir, "aircraft.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(cfg)
	for _, want := range []string{`title = "MyCrew Jetway Helper A320"`, `title = "MyCrew Jetway Helper A320 ALL"`, `atc_parking_codes = "DLH,AFR"`, `atc_parking_types = "GATE"`, `isuserselectable = 0`, `icao_type_designator = "A320"`} {
		if !strings.Contains(s, want) {
			t.Errorf("aircraft.cfg without %s", want)
		}
	}
	fm, _ := os.ReadFile(filepath.Join(dir, "flight_model.cfg"))
	if !strings.Contains(string(fm), "interactive_point.0 = 0.4, 37.654") {
		t.Error("the door not copied")
	}
	var layout struct {
		Content []struct{ Path string }
	}
	b, _ := os.ReadFile(filepath.Join(root, "layout.json"))
	if err := json.Unmarshal(b, &layout); err != nil || len(layout.Content) < 4 {
		t.Fatalf("layout %v: %v", layout, err)
	}
	if !IsHelper("MyCrew Jetway Helper A320 ALL") || IsHelper("FSLTL A320") {
		t.Error("IsHelper")
	}
}

// Side variants: each type's helper again with its door moved sideways,
// k steps to the right, its own title and folder.
func TestWritePackageSideVariants(t *testing.T) {
	base := filepath.Join("..", "addons", "testdata", "doors", "FSLTL_A320")
	root, err := WritePackage(t.TempDir(), []HelperModel{{Type: "A320", Dir: base}}, PackageOptions{SideSteps: 2, SideStepM: 0.1})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_A320_R+1")
	cfg, _ := os.ReadFile(filepath.Join(dir, "aircraft.cfg"))
	if !strings.Contains(string(cfg), `title = "MyCrew Jetway Helper A320 R+1"`) || strings.Contains(string(cfg), " ALL") {
		t.Errorf("variant aircraft.cfg:\n%s", cfg)
	}
	fm, _ := os.ReadFile(filepath.Join(dir, "flight_model.cfg"))
	if !strings.Contains(string(fm), "interactive_point.0 = 0.4, 37.654, -5.809,") {
		t.Errorf("door not moved 0.1 m right: %s", fm[strings.Index(string(fm), "interactive_point.0"):][:80])
	}
	if _, err := os.Stat(filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_A320_R-2", "aircraft.cfg")); err != nil {
		t.Error("no R-2 variant")
	}
	if HelperVariantTitle("A320", -2) != "MyCrew Jetway Helper A320 R-2" || HelperVariantTitle("A320", 0) != "MyCrew Jetway Helper A320" {
		t.Error("HelperVariantTitle")
	}
	if !IsHelperVariant("MyCrew Jetway Helper A320 R-2") || IsHelperVariant("MyCrew Jetway Helper A320") || IsHelperVariant("MyCrew Jetway Helper A320 ALL") {
		t.Error("IsHelperVariant")
	}
}

// Ahead variants: the door moved along the fuselage, for a gate where our
// aircraft stands off the simulator's place along its line (LKPR C22:
// narrowbodies 9 m back); the helper parked at the simulator's place has
// its door where ours is.
func TestWritePackageAheadVariants(t *testing.T) {
	base := filepath.Join("..", "addons", "testdata", "doors", "FSLTL_A320")
	root, err := WritePackage(t.TempDir(), []HelperModel{{Type: "A320", Dir: base}}, PackageOptions{AheadM: []float64{-9}})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_A320_A-9.0")
	cfg, _ := os.ReadFile(filepath.Join(dir, "aircraft.cfg"))
	if !strings.Contains(string(cfg), `title = "MyCrew Jetway Helper A320 A-9.0"`) {
		t.Errorf("variant aircraft.cfg:\n%s", cfg)
	}
	fm, _ := os.ReadFile(filepath.Join(dir, "flight_model.cfg"))
	// 37.654 ft ahead less 9 m (29.528 ft): 8.126; the side as it was.
	if !strings.Contains(string(fm), "interactive_point.0 = 0.4, 8.126, -6.137,") {
		t.Errorf("door not moved 9 m back: %s", fm[strings.Index(string(fm), "interactive_point.0"):][:80])
	}
	if HelperAheadTitle("A320", -9) != "MyCrew Jetway Helper A320 A-9.0" || HelperAheadTitle("A320", 0) != "MyCrew Jetway Helper A320" {
		t.Error("HelperAheadTitle")
	}
	if !IsHelperVariant("MyCrew Jetway Helper A320 A-9.0") {
		t.Error("IsHelperVariant")
	}
}

// A wide type's variant with a narrow type's door (DoorOf): the wide
// type's model, the narrow one's interactive_point.0 (LKPR B14: a B748
// helper for a B738 docked its jetway above and beside the door).
func TestWritePackageDoorOf(t *testing.T) {
	narrow := filepath.Join("..", "addons", "testdata", "doors", "FSLTL_A320")
	wide := t.TempDir()
	for _, f := range []string{"aircraft.cfg", "flight_model.cfg"} {
		b, err := os.ReadFile(filepath.Join(narrow, f))
		if err != nil {
			t.Fatal(err)
		}
		s := strings.Replace(string(b), "interactive_point.0 = 0.4, 37.654, -6.137,", "interactive_point.0 = 0.4, 80.000, -9.500,", 1)
		if err := os.WriteFile(filepath.Join(wide, f), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root, err := WritePackage(t.TempDir(), []HelperModel{{Type: "B748", Dir: wide}, {Type: "A320", Dir: narrow}},
		PackageOptions{DoorOf: map[string][]string{"B748": {"A320", "B738"}}})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_B748_D-A320")
	cfg, _ := os.ReadFile(filepath.Join(dir, "aircraft.cfg"))
	if !strings.Contains(string(cfg), `title = "`+HelperDoorTitle("B748", "A320")+`"`) {
		t.Errorf("variant aircraft.cfg:\n%s", cfg)
	}
	fm, _ := os.ReadFile(filepath.Join(dir, "flight_model.cfg"))
	if !strings.Contains(string(fm), "interactive_point.0 = 0.4, 37.654, -6.137,") || strings.Contains(string(fm), "80.000") {
		t.Errorf("the variant's door not the A320's:\n%s", fm)
	}
	if _, err := os.Stat(filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_B748_D-B738")); err == nil {
		t.Error("a variant for a door type not among the models")
	}
	if !IsHelperVariant(HelperDoorTitle("B748", "A320")) || IsHelperVariant(HelperTitle("B748", false)) {
		t.Error("IsHelperVariant")
	}
}
