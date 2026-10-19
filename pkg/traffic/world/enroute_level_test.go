package world

import (
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
