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
	// A route from a fix only the first half has to one only the second has.
	first, second := *g, *g
	first.Airways, second.Airways = a.Airways, b.Airways
	in := func(o *AirwayGraph) map[FixKey]bool {
		keys := map[FixKey]bool{}
		for _, w := range o.Airways {
			for _, s := range w.Segments {
				keys[s.From], keys[s.To] = true, true
			}
		}
		return keys
	}
	inA, inB := in(&first), in(&second)
	merged := MergeAirwayGraphs(&first, &second)
	routed := 0
	for from := range inA {
		for to := range inB {
			if inB[from] || inA[to] || from == to {
				continue
			}
			if steps, err := merged.Route(from, to); err == nil && len(steps) > 0 {
				routed++
			}
			if routed > 0 {
				break
			}
		}
		if routed > 0 {
			break
		}
	}
	if routed == 0 {
		t.Error("no route across the seam")
	}
}
