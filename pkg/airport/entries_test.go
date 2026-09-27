//go:build windows
// +build windows

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
			if e.Angle > MaxExitAngle {
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
