package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A vehicle already in a taxiing aircraft's way drives on out of it rather
// than stop there (aircraft do not see vehicles: it waited in the path and
// the aircraft taxied into it — offline LKPR, the tug of WZZ1639 driving in,
// 9 m from CSA1955); heading toward the aircraft it still stops.
func TestVehicleInPathDrivesOut(t *testing.T) {
	now := time.Now()
	prof := DefaultMotionProfile()
	pic := NewGroundPicture()
	at := airport.LatLon{Lat: 50.1, Lon: 14.26}
	var path []airport.LatLon
	for d := 0.0; d <= 120; d += trafficBodyStep {
		path = append(path, offsetHeading(at, 0, d))
	}
	pic.Report(1, at, 0, prof, now)
	pic.ReportPath(1, path, DefaultHalfSpanMeters)
	in := offsetHeading(at, 0, 45) // in its way, 45 m ahead
	way := func(hdg float64) []airport.LatLon {
		var w []airport.LatLon
		for d := 0.0; d <= 40; d += vehicleLookStep {
			w = append(w, offsetHeading(in, hdg, d))
		}
		return w
	}
	if at, _, ok := pic.VehicleConflict(77, 9001, way(90), vehicleHalfMeters, false, now); ok {
		t.Errorf("crossing out of its way: stops %.0f m on, in the aircraft's path", at)
	}
	if _, _, ok := pic.VehicleConflict(77, 9001, way(180), vehicleHalfMeters, false, now); !ok {
		t.Error("driving toward the aircraft: no stop")
	}
}
