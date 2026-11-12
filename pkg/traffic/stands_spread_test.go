package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A free stand next to one of ours ranks behind one with no neighbour of
// ours: departures spread over the apron, not side by side (user, LKPR
// 2026-11-11). Packed once nothing else is free.
func TestStandsSpreadFromNeighbours(t *testing.T) {
	l := &airport.Layout{ICAO: "ZZZZ", Latitude: 50, Longitude: 14}
	for k := range 4 {
		lat, lon := 50.0, 14.0+float64(k)*0.0006 // about 43 m apart, radius 18: 7 m edge to edge
		l.Parking = append(l.Parking, airport.Parking{Index: k, Radius: 18, Position: airport.LatLon{Lat: lat, Lon: lon}, Type: 9})
	}
	a := NewStandAllocator(nil, &airport.Graph{Layout: l})
	if err := a.Occupy(1, "AAA1", 17); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		r := a.rank([]int{0, 2, 3}, "", t0, nil)
		if r[0] != 3 {
			t.Fatalf("ranked %v: next to stand 1 before stand 3", r)
		}
	}
}
