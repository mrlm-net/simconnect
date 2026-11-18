package world

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestEnrouteLevelClear: two flights crossing at FL400 meet; the one
// appearing is clear at FL380, the level a step down its direction gives
// (live, DLH1050 and THY1880 created in the same second at FL400 on
// converging tracks).
func TestEnrouteLevelClear(t *testing.T) {
	way := func(from, to airport.LatLon, ft float64) []traffic.RoutePoint {
		return []traffic.RoutePoint{{Position: from, AltFt: ft, Kts: 450}, {Position: to, AltFt: ft, Kts: 450}}
	}
	cross := airport.LatLon{Lat: 50.3, Lon: 14.0}
	off := func(brg, nm float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(cross.Lat, cross.Lon, brg, nm*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	// THY1880 there already, 60 NM before the crossing, flying 300°.
	other := way(off(120, 60), off(300, 60), 40000)
	thy := traffic.TrackedAircraft{Observation: traffic.Observation{Tail: "THY1880", Position: other[0].Position, AltFt: 40000, GroundKts: 450,
		Heading: 300}, Phase: traffic.PhaseEnroute, Ours: true}
	plans := map[string][]traffic.RoutePoint{"THY1880": other}
	// DLH1050 appearing 60 NM before it too, flying 330°.
	mine := way(off(150, 60), off(330, 60), 40000)
	if who := meetsAlong("DLH1050", mine, []traffic.TrackedAircraft{thy}, plans); who != "THY1880" {
		t.Fatalf("at FL400: meets %q, want THY1880", who)
	}
	if who := meetsAlong("DLH1050", atLevel(mine, 40000, 38000), []traffic.TrackedAircraft{thy}, plans); who != "" {
		t.Errorf("at FL380: meets %q, want nobody", who)
	}
}

// TestFlownAlong: a way planned clear starts where the flight is by the
// time it appears (offline, LKPR: BAW1153 planned at 07:07:00 appearing at
// its first point 26 s later), on along its legs, its first leg's speed the
// point it heads for's.
func TestFlownAlong(t *testing.T) {
	a := airport.LatLon{Lat: 50, Lon: 14}
	at := func(brg, nm float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(a.Lat, a.Lon, brg, nm*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	route := []traffic.RoutePoint{{Position: a, AltFt: 7000, Kts: 250}, {Position: at(270, 20), AltFt: 37000, Kts: 450},
		{Position: at(270, 40), AltFt: 37000, Kts: 450}, {Position: at(270, 60), AltFt: 37000, Kts: 450}}
	if k := legKts(route); k != 450 {
		t.Errorf("first leg flown at %.0f kt, want 450 (the point it heads for)", k)
	}
	nm := func(p, q airport.LatLon) float64 { return calc.HaversineNM(p.Lat, p.Lon, q.Lat, q.Lon) }
	r := flownAlong(route, 3)
	if len(r) != 4 || math.Abs(nm(a, r[0].Position)-3) > 0.01 || r[0].AltFt != 7000 || r[1] != route[1] {
		t.Errorf("3 NM on: %d points, first %.2f NM on at %.0f ft; want 4, 3 NM on at 7000 ft", len(r), nm(a, r[0].Position), r[0].AltFt)
	}
	r = flownAlong(route, 25)
	if len(r) != 3 || math.Abs(nm(a, r[0].Position)-25) > 0.05 || r[0].AltFt != 37000 || r[1] != route[2] {
		t.Errorf("25 NM on: %d points, first %.2f NM on at %.0f ft; want 3, 25 NM on at 37000 ft", len(r), nm(a, r[0].Position), r[0].AltFt)
	}
	if r := flownAlong(route, 500); len(r) != 2 || nm(r[0].Position, r[1].Position) < 1 {
		t.Errorf("past its end: %d points, the first %.1f NM before the last; want 2 apart", len(r), nm(r[0].Position, r[1].Position))
	}
	if r := flownAlong(route, 0); &r[0] != &route[0] {
		t.Errorf("not late: the route changed")
	}
	if route[0].Position != a {
		t.Errorf("the route given was changed")
	}
}
