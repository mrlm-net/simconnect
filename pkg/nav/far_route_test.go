package nav

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

var farAirports = []AirportInfo{
	{ICAO: "LFPG", Position: airport.LatLon{Lat: 49.0097, Lon: 2.5479}, ElevationM: 119},
	{ICAO: "LEMD", Position: airport.LatLon{Lat: 40.4719, Lon: -3.5626}, ElevationM: 610},
	{ICAO: "EGLL", Position: airport.LatLon{Lat: 51.4706, Lon: -0.4619}, ElevationM: 25},
	{ICAO: "LIRF", Position: airport.LatLon{Lat: 41.8003, Lon: 12.2389}, ElevationM: 5},
	{ICAO: "EKCH", Position: airport.LatLon{Lat: 55.6181, Lon: 12.6561}, ElevationM: 5},
	{ICAO: "UUEE", Position: airport.LatLon{Lat: 55.9726, Lon: 37.4146}, ElevationM: 192},
}

func airwayLegs(fp *FlightPlan) int {
	n := 0
	for _, w := range fp.Waypoints {
		if w.Phase == PhaseEnroute && w.Airway != "" && w.Airway != Direct {
			n++
		}
	}
	return n
}

// TestPlanFarAirportsFlyAirways: flights between LKPR and airports beyond
// its airways fly them to their edge, not direct after the SID.
func TestPlanFarAirportsFlyAirways(t *testing.T) {
	g := loadLKPRAirways(t)
	for _, far := range farAirports {
		dep, err := Plan(FlightPlanRequest{Departure: lkprInfo(t), Arrival: far, Type: "A320", DepLimits: lkprLimits}, g)
		if err != nil {
			t.Fatal(err)
		}
		arr, err := Plan(FlightPlanRequest{Departure: far, Arrival: lkprInfo(t), Type: "A320"}, g)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("LKPR→%s: %s", far.ICAO, dep.Route)
		t.Logf("%s→LKPR: %s", far.ICAO, arr.Route)
		if airwayLegs(dep) < 3 {
			t.Errorf("LKPR→%s: %d airway legs: %s", far.ICAO, airwayLegs(dep), dep.Route)
		}
		if airwayLegs(arr) < 3 {
			t.Errorf("%s→LKPR: %d airway legs: %s", far.ICAO, airwayLegs(arr), arr.Route)
		}
	}
}

// TestPlanOverflightFliesAirways: an overflight between two airports
// beyond the LKPR airways joins them on its way across.
func TestPlanOverflightFliesAirways(t *testing.T) {
	g := loadLKPRAirways(t)
	for _, p := range [][2]AirportInfo{
		{farAirports[0], farAirports[5]}, // LFPG → UUEE
		{farAirports[4], farAirports[3]}, // EKCH → LIRF
		{{ICAO: "LROP", Position: airport.LatLon{Lat: 44.5711, Lon: 26.085}}, farAirports[2]},
	} {
		fp, err := Plan(FlightPlanRequest{Departure: p[0], Arrival: p[1], Type: "A320"}, g)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s→%s: %s", p[0].ICAO, p[1].ICAO, fp.Route)
		if airwayLegs(fp) < 3 {
			t.Errorf("%s→%s: %d airway legs: %s", p[0].ICAO, p[1].ICAO, airwayLegs(fp), fp.Route)
		}
	}
}
