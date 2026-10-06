package nav

import "testing"

// Two airports' airway graphs merged: the fixes and airways of both, a
// segment read from both sides once, routes across the seam.
func TestMergeAirwayGraphs(t *testing.T) {
	g, err := LoadAirwayGraph("testdata/LKPR-airways.json")
	if err != nil {
		t.Fatal(err)
	}
	m := MergeAirwayGraphs(nil, g, g)
	if len(m.Fixes) != len(g.Fixes) || m.SegmentCount() != g.SegmentCount() {
		t.Errorf("merged with itself: %d fixes, %d segments; want %d, %d", len(m.Fixes), m.SegmentCount(), len(g.Fixes), g.SegmentCount())
	}
	if m.Center != g.Center {
		t.Errorf("center %v, want the first's %v", m.Center, g.Center)
	}
	// Each half of the airways alone, merged: the whole again.
	a, b := *g, *g
	a.Airways, b.Airways = g.Airways[:len(g.Airways)/2], g.Airways[len(g.Airways)/2:]
	if n := MergeAirwayGraphs(&a, &b).SegmentCount(); n != g.SegmentCount() {
		t.Errorf("halves merged: %d segments, want %d", n, g.SegmentCount())
	}
}
