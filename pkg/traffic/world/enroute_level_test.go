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

// TestClimbable: a plan's climb flown at the cruise speed is planned no
// steeper than the AI flies it (offline, LKPR: OKILI, E545, at 7300 ft
// with FL400 51 NM on, a 5000 fpm climb at 460 kt); a climb not finished
// before the route turns down goes no higher than the point after it, one
// not finished at its end levels at a level of its direction; a level way
// and a heavy's slower climb as they are.
func TestClimbable(t *testing.T) {
	a := airport.LatLon{Lat: 49.7, Lon: 17.85}
	at := func(nm float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(a.Lat, a.Lon, 270, nm*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	route := []traffic.RoutePoint{{Position: a, AltFt: 7300, Kts: 250}, {Position: at(51), AltFt: 40000, Kts: 460},
		{Position: at(88), AltFt: 40000, Kts: 460}, {Position: at(113), AltFt: 31500, Kts: 280}}
	r := climbable(route, "E545", 460)
	for i := 1; i < len(r); i++ {
		nm := calc.HaversineNM(r[i-1].Position.Lat, r[i-1].Position.Lon, r[i].Position.Lat, r[i].Position.Lon)
		if fpm := (r[i].AltFt - r[i-1].AltFt) / (nm / r[i].Kts * 60); fpm > enrouteClimbMaxFpm+1 {
			t.Errorf("leg %d: climbs at %.0f fpm, want %.0f at most", i, fpm, enrouteClimbMaxFpm)
		}
	}
	if r[2].AltFt != 31500 {
		t.Errorf("top of the climb at %.0f ft, want 31500 (no higher than the STAR entry after it)", r[2].AltFt)
	}
	// Ending in the climb: at the level of its direction below (westbound).
	if r := climbable(route[:3], "E545", 460); r[2].AltFt != 34000 {
		t.Errorf("climb to the route's end: %.0f ft, want FL340", r[2].AltFt)
	}
	if r[3] != route[3] || route[1].AltFt != 40000 {
		t.Errorf("the descent changed, or the route given: %+v", r[3])
	}
	// A top it can reach, left downward 7 NM on (offline, LKPR: AUA909,
	// E190 LOWW → LKPR): level at its STAR entry's altitude instead; a top
	// flown level longer than the watch's look-ahead as it is.
	peak := []traffic.RoutePoint{{Position: a, AltFt: 22780, Kts: 440}, {Position: at(34), AltFt: 32000, Kts: 440}, {Position: at(41), AltFt: 29454, Kts: 280}}
	if r := climbable(peak, "E190", 440); r[1].AltFt != 29454 || r[2] != peak[2] {
		t.Errorf("a top left 7 NM on: %.0f ft, then %.0f ft; want 29454 level to the entry", r[1].AltFt, r[2].AltFt)
	}
	cruise := []traffic.RoutePoint{{Position: a, AltFt: 22780, Kts: 440}, {Position: at(34), AltFt: 32000, Kts: 440}, {Position: at(84), AltFt: 32000, Kts: 440},
		{Position: at(100), AltFt: 25000, Kts: 280}}
	if r := climbable(cruise, "E190", 440); r[1] != cruise[1] || r[2] != cruise[2] {
		t.Errorf("a cruise of 50 NM: %.0f ft, %.0f ft; want FL320 as planned", r[1].AltFt, r[2].AltFt)
	}
	level := []traffic.RoutePoint{{Position: a, AltFt: 35000, Kts: 450}, {Position: at(20), AltFt: 35000, Kts: 450}}
	if r := climbable(level, "A320", 450); r[1] != level[1] {
		t.Errorf("a level way changed: %+v", r[1])
	}
	// A B744 at 1600 fpm: 20 NM at 490 kt climb 3918 ft.
	heavy := []traffic.RoutePoint{{Position: a, AltFt: 20000, Kts: 490}, {Position: at(20), AltFt: 30000, Kts: 490}, {Position: at(200), AltFt: 30000, Kts: 490}}
	if r := climbable(heavy, "B744", 490); math.Abs(r[1].AltFt-(20000+20.0/490*60*1600)) > 50 || r[2].AltFt != 30000 {
		t.Errorf("B744: %.0f ft 20 NM on, %.0f ft at the end; want %.0f, 30000", r[1].AltFt, r[2].AltFt, 20000+20.0/490*60*1600)
	}
}
