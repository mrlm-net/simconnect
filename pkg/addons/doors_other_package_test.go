package addons

import (
	"os"
	"path/filepath"
	"testing"
)

// A livery package of its own whose base_container ("..\FSLTL_A320") is in
// another package has that base's door: the simulator resolves it across
// all packages (live, LKPR C30: "FSLTL_WANTED_A20N_Sweep" in
// wanted-fsltl-traffic, its base FSLTL_A20N in fsltl-traffic-base, no
// door, the jetway off it).
func TestDoorsByTitleBaseInOtherPackage(t *testing.T) {
	livery := filepath.Join(t.TempDir(), "SimObjects", "Airplanes")
	dir := filepath.Join(livery, "FSLTL_A320_WTD")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `[VARIATION]
base_container = "..\FSLTL_A320"

[FLTSIM.0]
title = "FSLTL_WANTED_A320_Sweep"
`
	if err := os.WriteFile(filepath.Join(dir, "aircraft.cfg"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	doors := DoorsByTitle(livery, filepath.Join("testdata", "doors"))
	if d, ok := doors["fsltl_wanted_a320_sweep"]; !ok || d.Folder != "FSLTL_A320" || d.Door.Long != 37.654 {
		t.Fatalf("livery in its own package: %+v, %v", d, ok)
	}
	if _, ok := DoorsByTitle(livery)["fsltl_wanted_a320_sweep"]; ok {
		t.Error("a door without its base installed")
	}
}
