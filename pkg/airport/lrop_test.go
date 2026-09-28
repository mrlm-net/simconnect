//go:build windows
// +build windows

package airport

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// LROP-layout.json is a Layout (not raw records) fetched from MSFS 2024.
func lropGraph(t *testing.T) *Graph {
	t.Helper()
	b, err := os.ReadFile("testdata/LROP-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var l Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	g, err := BuildGraph(&l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestLROPRoutes: at LROP the taxiways cross runway 08R/26L at its end
// where they run a short way along the runway surface; routes must still
// reach every runway without runway paths, and through traffic keeps to
// taxiway N rather than the stand taxilane M.
func TestLROPRoutes(t *testing.T) {
	g := lropGraph(t)
	n8, err := g.Layout.ParkingIndex("N8")
	if err != nil {
		t.Fatal(err)
	}
	for _, end := range []string{"08L", "26R", "08R", "26L"} {
		r, err := g.RouteToRunway(n8, end, RouteOptions{})
		if err != nil {
			t.Errorf("N8 → %s: %v", end, err)
			continue
		}
		t.Logf("N8 → %s: %.0f m via %v, crossings %v", end, r.Length, r.Taxiways, r.RunwayCrossings)
		if end == "08L" && (slices.Contains(r.Taxiways, "M") || !slices.Contains(r.Taxiways, "N")) {
			t.Errorf("N8 → 08L via %v, want N and not the stand taxilane M", r.Taxiways)
		}
	}
}
