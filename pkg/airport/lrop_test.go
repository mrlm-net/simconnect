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

// TestTaxiwayGapBridged: LROP's taxiway C stops at the U junction and goes
// on 21 m further (the scenery's gap); bridged, stand 214 reaches 08R and
// 08L along C, not round the airport by P and Q (live, 3.6 km to 08R).
func TestTaxiwayGapBridged(t *testing.T) {
	g := lropGraph(t)
	s, err := g.Layout.ParkingIndex("214")
	if err != nil {
		t.Skip("no stand 214 in this capture:", err)
	}
	for _, end := range []string{"08R", "08L"} {
		r, err := g.RouteToRunway(s, end, RouteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if via := r.SpokenTaxiways(-1); r.Length > 2500 || slices.Contains(via, "Q") {
			t.Errorf("214 → %s: %.0f m via %v, want the short way along C", end, r.Length, via)
		}
	}
}

// TestTaxiwayBridgesDrawn: the joined gap is given for maps, by the graph,
// the layout and the GeoJSON.
func TestTaxiwayBridgesDrawn(t *testing.T) {
	g := lropGraph(t)
	br := g.Bridges()
	if len(br) != 1 || br[0].Name != "C" || br[0].Length < 15 || br[0].Length > 30 {
		t.Fatalf("bridges %+v, want C's 21 m", br)
	}
	if got := g.Layout.TaxiwayBridges(); len(got) != 1 || got[0] != br[0] {
		t.Errorf("layout bridges %+v", got)
	}
	n := 0
	for _, f := range g.Layout.FeatureCollection().Features {
		if f.Properties["bridge"] == true && f.Properties["name"] == "C" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d bridge features", n)
	}
}
