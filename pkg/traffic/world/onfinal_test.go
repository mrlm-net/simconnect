package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestOnFinalNear: lined up on the final close in counts as on the final
// (live, OKUFC on a 1 NM final after another circuit, taken for minutes
// away by its route); abeam, heading away or far out does not.
func TestOnFinalNear(t *testing.T) {
	end := airport.RunwayEnd{Name: "24", Heading: 243, Threshold: airport.LatLon{Lat: 50.1, Lon: 14.28}}
	at := func(bearing, nm float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, bearing, nm*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	for _, c := range []struct {
		name        string
		bearing, nm float64
		hdg         float64
		want        bool
	}{
		{"1 NM final", 63, 1, 243, true},
		{"downwind abeam", 333, 1, 63, false},
		{"over the final heading away", 63, 1, 63, false},
		{"6 NM final", 63, 6, 243, false},
	} {
		if got := onFinalNear(at(c.bearing, c.nm), c.hdg, c.nm, end); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
