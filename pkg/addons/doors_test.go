package addons

import (
	"path/filepath"
	"testing"
)

func TestDoorsByType(t *testing.T) {
	doors := DoorsByType(filepath.Join("testdata", "doors"), filepath.Join("testdata", "missing"))
	d, ok := doors["A320"]
	if !ok || len(doors) != 1 {
		t.Fatalf("doors %v", doors)
	}
	if d.Folder != "FSLTL_A320" || d.WingSpanFt != 104 || d.Door.Long != 37.654 || d.Door.Lat != -6.137 || d.Door.Vert != 0.203 {
		t.Errorf("door %+v", d)
	}
	if d.Door.Raw != "0.4, 37.654, -6.137,  0.203 ,0, 0, 0, -86, 72, 16, 85, 3, -2, 33" {
		t.Errorf("raw %q", d.Door.Raw)
	}
	if _, ok := ParseInteractivePoint("0.4, x, 1, 2"); ok {
		t.Error("parsed a bad point")
	}
}

// A livery folder's titles have its base model's door.
func TestDoorsByTitle(t *testing.T) {
	doors := DoorsByTitle(filepath.Join("testdata", "doors"))
	for _, title := range []string{"fsltl a320 dlh lufthansa", "fsltl a320 dlh star alliance"} {
		if d, ok := doors[title]; !ok || d.Folder != "FSLTL_A320" || d.Door.Long != 37.654 {
			t.Errorf("%s: %+v, %v", title, d, ok)
		}
	}
}
