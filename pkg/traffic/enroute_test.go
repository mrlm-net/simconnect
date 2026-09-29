//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

func TestEnrouteStart(t *testing.T) {
	a := airport.LatLon{Lat: 50, Lon: 12}
	b := airport.LatLon{Lat: 50, Lon: 13}
	c := airport.LatLon{Lat: 50.1, Lon: 14}
	spawn, wps, err := EnrouteStart([]RoutePoint{{a, 36000, 450}, {b, 36000, 450}, {c, 12000, 280}})
	if err != nil {
		t.Fatal(err)
	}
	if spawn.Latitude != a.Lat || spawn.Longitude != a.Lon || spawn.Altitude != 36000 || spawn.Airspeed != 450 || math.Abs(spawn.Heading-90) > 1 {
		t.Fatalf("spawn %+v", spawn)
	}
	if len(wps) != 3 || wps[0].Latitude != b.Lat || wps[1].Altitude != 12000 || wps[1].KtsSpeed != 280 {
		t.Fatalf("waypoints %+v", wps)
	}
	// On along the last track.
	d := calc.HaversineMeters(c.Lat, c.Lon, wps[2].Latitude, wps[2].Longitude)
	if math.Abs(d-EnrouteContinueMeters) > EnrouteContinueMeters/100 {
		t.Errorf("continues %.0f m", d)
	}
	if _, _, err := EnrouteStart([]RoutePoint{{a, 1, 1}}); err == nil {
		t.Error("one point: no error")
	}
	if EnrouteSpeedKts(8000, 450) != 250 || EnrouteSpeedKts(30000, 450) != 450 || EnrouteSpeedKts(30000, 0) != 420 {
		t.Error("speeds")
	}
}
