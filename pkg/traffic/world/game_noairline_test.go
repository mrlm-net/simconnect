package world

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestGameNoAirlineNamesTheMissingData: a game at a field no airline of the
// schedule serves fails with an error that says what is missing (#1610):
// the airport, the schedule, its region and the runway a type must fit.
func TestGameNoAirlineNamesTheMissingData(t *testing.T) {
	cc := &controlCenter{}
	g := &airport.Graph{Layout: &airport.Layout{ICAO: "ZZZZ", Latitude: -60, Longitude: -150,
		Runways: []airport.Runway{{Length: 250}}}}
	r := SpawnRequest{Kind: "arrival", ICAO: "ZZZZ"}
	err := cc.gameFlight(&r, g, time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC), map[string]bool{})
	if err == nil {
		t.Fatalf("a flight at a 250 m field no airline serves: %+v", r)
	}
	for _, want := range []string{"no airline flies to ZZZZ", "DefaultScheduleConfig", `"ZZ"`, "250 m runway"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
}
