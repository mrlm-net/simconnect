package world

import (
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
	pp := func(p airport.LatLon) traffic.PathPoint { return traffic.PathPoint{Lat: p.Lat, Lon: p.Lon, AltFt: 20000} }
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
