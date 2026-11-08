package jetway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A door variant's door where it is on the narrow type: as far behind the
// nose gear and as high, not copied as written (live, LKPR 22A: the "B748
// D-A320" helper's jetway 0.9 m above the A320's door; C22: the A320 8.5 m
// further into the circle). The FSLTL B748's and A320's own values.
func TestWritePackageDoorOfGeometry(t *testing.T) {
	src := filepath.Join("..", "addons", "testdata", "doors", "FSLTL_A320")
	cfg, err := os.ReadFile(filepath.Join(src, "aircraft.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	model := func(fm string) string {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, "aircraft.cfg"), cfg, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "flight_model.cfg"), []byte(strings.ReplaceAll(fm, "\n", "\r\n")), 0o644); err != nil {
			t.Fatal(err)
		}
		return d
	}
	narrow := model(`[INTERACTIVE POINTS]
interactive_point.0 = 0.4, 37.654, -6.137,  0.203 ,0, 0, 0, -86, 72, 16, 85, 3, -2, 33 ; main door left
[CONTACT_POINTS]
static_pitch = -0.95 ; degrees
static_cg_height = 12.25 ; feet
point.0 = 1,  37.319,   0.00,  -12.3, 1600 ;
point.1 = 1, -4.163, -12.634, -12.48, 1600 ;
point.11 = 2, 48.528, -18.92, -5.67, 1600 ;
[WEIGHT_AND_BALANCE]
reference_datum_position = 0, 0, 0 ; (feet)
`)
	wide := model(`[INTERACTIVE POINTS]
interactive_point.0 = 0.4, -40.17, -10.65, 1.5, 0, 0, 0, -90, 0, 0, 0, 0, 0, 0 ; main door left
[CONTACT_POINTS]
static_pitch = -0.36
static_cg_height = 15.35
point.0 = 1, 9.163, 0, -15.0, 1600 ;
point.1 = 1, -16.33, -12, -15.3, 1600 ;
[WEIGHT_AND_BALANCE]
reference_datum_position = 83.5, 0, 0
`)
	root, err := WritePackage(t.TempDir(), []HelperModel{{Type: "B748", Dir: wide}, {Type: "A320", Dir: narrow}},
		PackageOptions{DoorOf: map[string][]string{"B748": {"A320"}}})
	if err != nil {
		t.Fatal(err)
	}
	fm, _ := os.ReadFile(filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_B748_D-A320", "flight_model.cfg"))
	// Along: 37.654 - 37.319 + 9.163. Up: 12.25 + 0.203 + 37.654 sin(-0.95°)
	// above the ground, less 15.35 + (83.5 + 9.498) sin(-0.36°).
	want := "interactive_point.0 = 0.4, 9.498, -6.137, -2.937,0, 0, 0, -86, 72, 16, 85, 3, -2, 33 ; main door left ; MyCrew: the door of "
	if !strings.Contains(string(fm), want) {
		t.Errorf("door:\n%s\nwant %s", fm, want)
	}
	if !strings.Contains(string(fm), "\r\n") || strings.Contains(string(fm), "-40.17") {
		t.Errorf("line endings or the B748's own door:\n%s", fm)
	}
}

// A 777-300ER's L1 on the B748: 3 ft behind the nose gear (live, LKPR B14:
// the B748's own door had the jetway 15 m aft of an Emirates 777's L1).
func TestDoorFromB77W(t *testing.T) {
	b77w := geometry{nose: 101.725, hasNose: true, cgHeight: 16.3, hasCG: true, pitch: -0.45, hasPitch: true}
	b748 := geometry{datum: [3]float64{83.5, 0, 0}, nose: 9.163, hasNose: true, cgHeight: 15.35, hasCG: true, pitch: -0.36, hasPitch: true}
	l, ok := doorFrom(b77w, b748, "interactive_point.0 = 0.4, 98.6624, -9.512, 0.6229, 0, 0, 0")
	if !ok || !strings.HasPrefix(l, "interactive_point.0 = 0.4, 6.100, -9.512, ") {
		t.Errorf("%q %v", l, ok)
	}
	if _, ok := doorFrom(geometry{}, b748, "interactive_point.0 = 0.4, 98.6624, -9.512, 0.6229"); ok {
		t.Error("moved without the nose gear")
	}
	// Without resting heights: along only.
	l, _ = doorFrom(geometry{nose: 101.725, hasNose: true}, b748, "interactive_point.0 = 0.4, 98.6624, -9.512, 0.6229")
	if l != "interactive_point.0 = 0.4, 6.100, -9.512, 0.6229" {
		t.Errorf("%q", l)
	}
}
