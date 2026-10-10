package world

import (
	"github.com/mrlm-net/simconnect/pkg/nav"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestJoinAlong: a route east past a STAR point 2 NM off it joins there,
// having flown its points up to it; a point it has passed is not flown;
// a route nowhere near the STARs meets none.
func TestJoinAlong(t *testing.T) {
	at := func(eastNM, northNM float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(50, 14, 90, eastNM*1852)
		lat, lon = calc.DisplaceByHeading(lat, lon, 0, northNM*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	pp := func(p airport.LatLon) traffic.PathPoint {
		return traffic.PathPoint{Lat: p.Lat, Lon: p.Lon, AltFt: 20000}
	}
	pos := at(0, 0)
	route := []traffic.PathPoint{pp(at(-10, 0)), pp(at(20, 0)), pp(at(40, 0)), pp(at(60, 0)), pp(at(80, 0))}
	near := starJoin{pts: []airport.NavPoint{{Ident: "NEAR", Position: at(50, 2)}}, name: "NEAR1A", restNM: 30}
	far := starJoin{pts: []airport.NavPoint{{Ident: "FAR", Position: at(50, 40)}}, name: "FAR1A", restNM: 10}
	rj, ok := joinAlong(pos, route, []starJoin{far, near})
	if !ok || rj.join.name != "NEAR1A" {
		t.Fatalf("joined %q (ok %v), want NEAR1A", rj.join.name, ok)
	}
	if len(rj.flown) != 2 || rj.flown[0] != route[1] {
		t.Errorf("flown %v, want the points at 20 and 40 NM", rj.flown)
	}
	if _, ok := joinAlong(pos, route, []starJoin{far}); ok {
		t.Error("a STAR 40 NM off the route was met")
	}
}

// TestRealOverflightPath: a route crossing the area is flown to its first
// point out of it; without a route, straight on along the track to where
// it leaves.
func TestRealOverflightPath(t *testing.T) {
	centre := airport.LatLon{Lat: 50, Lon: 14}
	at := func(eastNM float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(centre.Lat, centre.Lon, 90, eastNM*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	pp := func(p airport.LatLon) traffic.PathPoint { return traffic.PathPoint{Lat: p.Lat, Lon: p.Lon} }
	pos := at(-50)
	route := []traffic.PathPoint{pp(at(0)), pp(at(80)), pp(at(140)), pp(at(300))}
	path := realOverflightPath(centre, pos, 35000, 90, route)
	if len(path) != 3 || path[2] != (traffic.PathPoint{Lat: route[2].Lat, Lon: route[2].Lon, AltFt: 35000}) {
		t.Errorf("path %v, want the route to its point at 140 NM, at 35000 ft", path)
	}
	straight := realOverflightPath(centre, pos, 35000, 90, nil)
	if len(straight) != 1 {
		t.Fatalf("straight path %v", straight)
	}
	if d := calc.HaversineNM(centre.Lat, centre.Lon, straight[0].Lat, straight[0].Lon); d < overflightRadiusNM+10 || d > overflightRadiusNM+16 {
		t.Errorf("straight on leaves %.0f NM from the centre", d)
	}
}

// TestFlyGiven: a departure's plan keeps its SID and then flies the given
// route from past the SID's end, at the route's levels or the cruise level.
func TestFlyGiven(t *testing.T) {
	at := func(eastNM float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(50, 14, 90, eastNM*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	fp := &nav.FlightPlan{CruiseFL: 350, Waypoints: []nav.Waypoint{
		{Ident: "RW24", Kind: nav.PointRunway, Phase: nav.PhaseSID, Position: at(0)},
		{Ident: "SID1", Kind: "W", Phase: nav.PhaseSID, Position: at(10)},
		{Ident: "SID2", Kind: "W", Phase: nav.PhaseSID, Position: at(20)},
		{Ident: "AWY1", Kind: "W", Phase: nav.PhaseEnroute, Position: at(60)},
	}}
	p, err := plannedFrom(fp, "departure")
	if err != nil {
		t.Fatal(err)
	}
	pp := func(p airport.LatLon, alt float64) traffic.PathPoint {
		return traffic.PathPoint{Lat: p.Lat, Lon: p.Lon, AltFt: alt}
	}
	flyGiven(p, []traffic.PathPoint{pp(at(5), 0), pp(at(40), 24000), pp(at(90), 0)})
	if len(p.route) != 4 || p.route[1].Ident != "SID2" {
		t.Fatalf("route %v, want SID1, SID2 and the two given points past it", p.route)
	}
	if math.Abs(p.route[2].AltMax-7315.2) > 0.1 || math.Abs(p.route[3].AltMax-10668) > 0.1 {
		t.Errorf("levels %.0f, %.0f m", p.route[2].AltMax, p.route[3].AltMax)
	}
}
