package traffic

import "testing"

// Every model in the table: a tug fitted to its title stands on its nose
// gear, and the table is sane (nose gear ahead, wheelbase positive).
func TestGearGeometryTable(t *testing.T) {
	if len(gearGeometry) < 50 {
		t.Fatalf("only %d models", len(gearGeometry))
	}
	for k, g := range gearGeometry {
		if g.NoseM <= 0 || g.NoseM+g.MainM <= 0.5 {
			t.Errorf("%s: %+v", k, g)
		}
		title := "FSLTL_" + k + "_TEST"
		got, ok := gearFor(title)
		if !ok || got != g {
			t.Errorf("%s: gearFor %+v %v, want %+v", title, got, ok, g)
		}
	}
}
