package nav

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/dict"
)

// TestRouteStructure: an area "free route above FL245" is flown direct
// above it and along the airways below; an airway-based area and an area
// with no entry along the airways (#1484).
func TestRouteStructure(t *testing.T) {
	t.Cleanup(func() { _ = dict.Reset("nav.routeStructure") })
	if err := dict.Use("nav.routeStructure", []byte(`[
		{"area": "FRA", "north": 50, "south": 48, "west": 10, "east": 12, "freeRouteFt": 24500,
		 "points": [{"ident": "AAAAA", "position": {"lat": 49, "lon": 10}}, {"ident": "BBBBB", "position": {"lat": 49.2, "lon": 10}}, {"ident": "CCCCC", "position": {"lat": 49, "lon": 12}}]},
		{"area": "AWY", "north": 46, "south": 44, "west": 10, "east": 12, "airwaysOnly": true}]`)); err != nil {
		t.Fatal(err)
	}
	fra, ok := RouteStructureAt(airport.LatLon{Lat: 49, Lon: 11})
	if !ok || fra.Area != "FRA" {
		t.Fatalf("FRA: %+v %v", fra, ok)
	}
	if !fra.Direct(37000) || !fra.Direct(24500) || fra.Direct(20000) {
		t.Errorf("free route above FL245: FL370 %v, FL245 %v, FL200 %v", fra.Direct(37000), fra.Direct(24500), fra.Direct(20000))
	}
	awy, _ := RouteStructureAt(airport.LatLon{Lat: 45, Lon: 11})
	if awy.Direct(37000) {
		t.Error("an airway-based area flown direct")
	}
	none, ok := RouteStructureAt(airport.LatLon{Lat: -30, Lon: 11})
	if ok || none.Direct(37000) {
		t.Errorf("no entry: %+v %v, want the airways", none, ok)
	}
	// Entry points: the nearest, sometimes the next.
	rng := rand.New(rand.NewPCG(1, 2))
	seen := map[string]int{}
	for range 300 {
		e, ok := fra.EntryNear(airport.LatLon{Lat: 49.05, Lon: 10.1}, 30, 2, rng)
		if !ok {
			t.Fatal("no entry point near")
		}
		seen[e.Ident]++
	}
	if seen["AAAAA"] < seen["BBBBB"] || seen["BBBBB"] == 0 || seen["CCCCC"] > 0 {
		t.Errorf("entry points drawn %v", seen)
	}
	if _, ok := fra.EntryNear(airport.LatLon{Lat: 45, Lon: 0}, 30, 2, rng); ok {
		t.Error("an entry point far away")
	}
}

// TestStrategicOffset: points moved to the right of the track by the
// offset.
func TestStrategicOffset(t *testing.T) {
	pts := []airport.LatLon{{Lat: 50, Lon: 10}, {Lat: 50, Lon: 11}, {Lat: 50, Lon: 12}} // eastbound
	off := StrategicOffset(pts, 2)
	for i, p := range off {
		d := calc.HaversineNM(pts[i].Lat, pts[i].Lon, p.Lat, p.Lon)
		if math.Abs(d-2) > 0.05 || p.Lat >= pts[i].Lat {
			t.Errorf("point %d moved %.2f NM to %v: want 2 NM south (right of an eastbound track)", i, d, p)
		}
	}
	if got := StrategicOffset(pts, 0); &got[0] != &pts[0] {
		t.Error("no offset: a copy")
	}
}
