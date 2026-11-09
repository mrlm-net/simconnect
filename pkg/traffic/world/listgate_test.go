package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Parked for an hour, the airport list is asked for every 10 minutes, not
// every minute; flying, once the user is 10 NM on.
func TestAirportListOnlyWhenMoved(t *testing.T) {
	var g listGate
	start := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	lkpr := airport.LatLon{Lat: 50.1008, Lon: 14.26}
	asks := 0
	for min := 0; min < 60; min++ {
		now := start.Add(time.Duration(min) * time.Minute)
		if g.due(now, lkpr) {
			asks++
			g.done(now, lkpr)
		}
	}
	if asks != 6 {
		t.Errorf("parked an hour: asked %d times, want 6 (every 10 min)", asks)
	}
	now := start.Add(61 * time.Minute)
	g.done(now, lkpr)
	if g.due(now.Add(time.Minute), airport.LatLon{Lat: lkpr.Lat + 0.1, Lon: lkpr.Lon}) {
		t.Error("asked again 6 NM on")
	}
	if !g.due(now.Add(2*time.Minute), airport.LatLon{Lat: lkpr.Lat + 0.2, Lon: lkpr.Lon}) {
		t.Error("not asked again 12 NM on")
	}
}
