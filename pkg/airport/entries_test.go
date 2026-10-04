package airport

import (
	"errors"
	"testing"
)

func TestRunwayEntries(t *testing.T) {
	g := lkprGraph(t)
	for _, end := range []string{"06", "24", "12", "30"} {
		entries, err := g.RunwayEntries(end)
		if err != nil || len(entries) == 0 {
			t.Fatalf("%s: %d entries, %v", end, len(entries), err)
		}
		rwy, _, _ := g.Layout.RunwayEnd(end)
		var names []string
		for i, e := range entries {
			if e.Angle > MaxEntryAngle {
				t.Errorf("%s at %s: turn %.0f° onto the runway", end, e.Taxiway, e.Angle)
			}
			if d := e.FromThreshold + e.Remaining - rwy.Length; d > 1 || d < -1 {
				t.Errorf("%s at %s: %.0f + %.0f m ≠ runway %.0f m", end, e.Taxiway, e.FromThreshold, e.Remaining, rwy.Length)
			}
			if i > 0 && e.FromThreshold < entries[i-1].FromThreshold {
				t.Errorf("%s: entries not sorted from the threshold", end)
			}
			if e.Path[0] != e.Node || e.Path[len(e.Path)-1] != e.RunwayNode {
				t.Errorf("%s at %s: path does not run from the taxiway onto the runway", end, e.Taxiway)
			}
			names = append(names, e.Taxiway)
		}
		t.Logf("RWY %s entries: %v", end, names)
	}
}

func TestRouteToRunwayEntry(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	full, _ := g.RouteToRunway(c22, "24", RouteOptions{})
	same, err := g.RouteToRunwayEntry(c22, "24", "", RouteOptions{})
	if err != nil || same.Length != full.Length {
		t.Fatalf("empty entry should be full length: %v", err)
	}
	entries, _ := g.RunwayEntries("24")
	routed := 0
	for _, e := range entries {
		if e.Taxiway == "" {
			continue
		}
		r, err := g.RouteToRunwayEntry(c22, "24", e.Taxiway, RouteOptions{})
		if errors.Is(err, ErrNoRoute) {
			continue
		}
		if err != nil {
			t.Fatalf("24 at %s: %v", e.Taxiway, err)
		}
		routed++
		checkRoute(t, g, r)
		if r.Entry != e.Taxiway || r.RunwayEnd != "24" {
			t.Errorf("24 at %s: route entry %q end %q", e.Taxiway, r.Entry, r.RunwayEnd)
		}
		t.Logf("C22 → 24 at %s: %.0f m via %v", e.Taxiway, r.Length, r.Taxiways)
	}
	if routed < 2 {
		t.Errorf("only %d named entries routable", routed)
	}
	if _, err := g.RouteToRunwayEntry(c22, "24", "ZZZ", RouteOptions{}); !errors.Is(err, ErrUnknownEntry) {
		t.Errorf("unknown entry: %v", err)
	}
}

// Threshold entries at LKPR: F onto 06 ends on the centreline beside the
// runway node (joined), L onto 12 meets it at 120° (MaxEntryAngle), Z onto
// 24 joins A's long lead-in at the runway edge (the way across the surface
// is bounded, not the edge leaving it).
func TestRunwayEntriesAtThresholds(t *testing.T) {
	g := lkprGraph(t)
	for _, c := range []struct {
		end, taxiway string
		maxFrom      float64
	}{{"06", "F", 250}, {"12", "L", 100}, {"24", "Z", 250}, {"24", "A", 250}} {
		entries, err := g.RunwayEntries(c.end)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range entries {
			if e.Taxiway == c.taxiway && e.FromThreshold < c.maxFrom {
				found = true
			}
		}
		if !found {
			t.Errorf("no entry onto %s at %s within %.0f m of the threshold: %+v", c.end, c.taxiway, c.maxFrom, entries)
		}
	}
}
