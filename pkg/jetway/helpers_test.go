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
