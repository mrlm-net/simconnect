package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The route the map draws for an aircraft flown by MSFS AI starts at the
// point it flies to, not at the points behind it (the user: WZZ1467's route
// drawn twice, back to where it appeared and out again).
func TestPointsAheadNotBehind(t *testing.T) {
	route := []airport.LatLon{{Lat: 50, Lon: 12}, {Lat: 50, Lon: 13}, {Lat: 50, Lon: 14}, {Lat: 50, Lon: 15}}
	got := pointsAhead(airport.LatLon{Lat: 50, Lon: 13.6}, route)
	if len(got) != 2 || got[0] != route[2] {
		t.Fatalf("route ahead of 13.6°E starts %v (%d points), want %v", got[0], len(got), route[2])
	}
	if got := pointsAhead(airport.LatLon{Lat: 50, Lon: 12.1}, route); got[0] != route[1] {
		t.Fatalf("just past the first point the route ahead starts %v, want %v", got[0], route[1])
	}
}

// An overflight due in the area some time ago appears that far on along
// its great circle, not at the entry.
func TestFlownOn(t *testing.T) {
	var gc []traffic.RoutePoint
	for lon := 12.0; lon <= 16; lon += 0.5 { // about 19 NM apart at 50°N
		gc = append(gc, traffic.RoutePoint{Position: airport.LatLon{Lat: 50, Lon: lon}})
	}
	if got := flownOn(gc, 0); len(got) != len(gc) {
		t.Fatalf("not flown yet: %d points of %d", len(got), len(gc))
	}
	if got := flownOn(gc, 50); got[0].Position.Lon != 13.0 {
		t.Fatalf("50 NM on it starts at %.1f°E, want 13.0", got[0].Position.Lon)
	}
	if got := flownOn(gc, 1e6); len(got) != 2 {
		t.Fatalf("flown past the end: %d points, want the last two", len(got))
	}
}
