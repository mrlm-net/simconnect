package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestOnFinalLined: an arrival on the extended centreline, heading down
// it, is on the final further out too — its distance straight in, not
// along its route (offline LKPR, TVS499 on the localizer at 7.2 NM counted
// 8); beside the centreline, heading across or away, or far out, it is not.
func TestOnFinalLined(t *testing.T) {
	end := airport.RunwayEnd{Name: "24", Heading: 243, Threshold: airport.LatLon{Lat: 50.1, Lon: 14.28}}
	at := func(alongNM, sideNM float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, 63, alongNM*1852)
		lat, lon = calc.DisplaceByHeading(lat, lon, 153, sideNM*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	for _, c := range []struct {
		name        string
		along, side float64
		hdg         float64
		want        bool
	}{
		{"7 NM on the localizer", 7, 0.1, 243, true},
		{"1 NM final", 1, 0, 243, true},
		{"7 NM, 2 NM beside", 7, 2, 243, false},
		{"7 NM on the centreline, heading across", 7, 0, 333, false},
		{"7 NM on the centreline, heading away", 7, 0, 63, false},
		{"20 NM on the centreline", 20, 0, 243, false},
	} {
		p := at(c.along, c.side)
		d := calc.HaversineNM(end.Threshold.Lat, end.Threshold.Lon, p.Lat, p.Lon)
		if got := onFinalLined(p, c.hdg, d, end); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
