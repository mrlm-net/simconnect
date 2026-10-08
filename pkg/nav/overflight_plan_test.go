package nav

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestPlanFromPositionsOnly: an overflight is planned between two airports
// known only by position (no layout loaded): Bucharest to New York over
// the LKPR airways.
func TestPlanFromPositionsOnly(t *testing.T) {
	g, err := LoadAirwayGraph("testdata/LKPR-airways.json")
	if err != nil {
		t.Fatal(err)
	}
	fp, err := Plan(FlightPlanRequest{Type: "B789",
		Departure: AirportInfo{ICAO: "LROP", Position: airport.LatLon{Lat: 44.5711, Lon: 26.085}, ElevationM: 95},
		Arrival:   AirportInfo{ICAO: "KJFK", Position: airport.LatLon{Lat: 40.6398, Lon: -73.7789}, ElevationM: 4},
	}, g)
	if err != nil {
		t.Fatal(err)
	}
	if fp.DistanceNM < 3500 || len(fp.Waypoints) < 2 {
		t.Errorf("plan %.0f NM, %d points", fp.DistanceNM, len(fp.Waypoints))
	}
	t.Logf("%s, FL%03d, %.0f NM", fp.Route, fp.CruiseFL, fp.DistanceNM)
}
