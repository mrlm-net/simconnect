package nav

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A short hop (LKPR to LKKB, 10 NM; MyCrew: flown 120 NM out on VENO4M
// and back on GOLO4W) goes from the runway onto the approach, without a
// SID or a STAR; a player's SID is still flown.
func TestPlanShortHop(t *testing.T) {
	b, err := os.ReadFile("../airport/testdata/LKKB-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	lkkb := AirportInfo{ICAO: "LKKB", Position: airport.LatLon{Lat: 50.1214, Lon: 14.5436}, ElevationM: 285, Procedures: &p}
	fp, err := Plan(FlightPlanRequest{Departure: lkprInfo(t), Arrival: lkkb, Type: "A320", DepartureRunway: "24", ArrivalRunway: "24"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %s, %.0f NM", fp.Route, planIdents(fp.Waypoints), fp.DistanceNM)
	if fp.SID != "" || fp.STAR != "" {
		t.Errorf("short hop on SID %q, STAR %q", fp.SID, fp.STAR)
	}
	if fp.DistanceNM > 60 {
		t.Errorf("flown %.0f NM for a 10 NM hop", fp.DistanceNM)
	}
	fp, err = Plan(FlightPlanRequest{Departure: lkprInfo(t), Arrival: lkkb, Type: "A320", DepartureRunway: "24", ArrivalRunway: "24", SID: "VENO4M"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fp.SID != "VENO4M" {
		t.Errorf("the player's SID VENO4M not flown: %q (%v)", fp.SID, fp.Notes)
	}
}
