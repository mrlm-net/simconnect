package nav

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestCorridor: LKPR → LPPT, 60 NM either side: a point by the track
// midway is in, one 100 NM off it out, the rounded ends in past the
// airports only within the half width.
func TestCorridor(t *testing.T) {
	k := Corridor{From: airport.LatLon{Lat: 50.1, Lon: 14.26}, To: airport.LatLon{Lat: 38.78, Lon: -9.13}, HalfWidthNM: 60}
	for _, c := range []struct {
		name string
		p    airport.LatLon
		in   bool
	}{
		{"start", k.From, true},
		{"end", k.To, true},
		{"near the track, mid-way", airport.LatLon{Lat: 45.2, Lon: 3.0}, true},
		{"far off the track", airport.LatLon{Lat: 48.5, Lon: 0.5}, false},
		{"40 NM behind the start", airport.LatLon{Lat: 50.4, Lon: 15.2}, true},
		{"200 NM behind the start", airport.LatLon{Lat: 51.5, Lon: 18.5}, false},
		{"100 NM past the end", airport.LatLon{Lat: 37.9, Lon: -11.0}, false},
	} {
		if got := k.Contains(c.p); got != c.in {
			t.Errorf("%s %+v: in %v, want %v", c.name, c.p, got, c.in)
		}
	}
	o := CrawlOptions{Corridor: &k, Center: k.From, RadiusNM: 10}
	if !o.inRange(airport.LatLon{Lat: 45.2, Lon: 3.0}) {
		t.Error("a corridor crawl kept to the radius")
	}
}
