package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestVehicleConflictClosing (#1629): a service vehicle stops for a moving
// aircraft only where its way closes on it: not for one it drives away
// from (behind its tail, abeam the tail, beyond the nose of one pushing
// away), still for one taxiing across its way ahead, and for one it
// drives towards. The aircraft (an A320) stands at the origin facing
// north; at each step of its way (every 2 m) the vehicle looks 40 m ahead.
func TestVehicleConflictClosing(t *testing.T) {
	base := airport.LatLon{Lat: 50.1, Lon: 14.26}
	at := func(x, y float64) airport.LatLon {
		return offsetHeading(offsetHeading(base, 0, y), 90, x)
	}
	prof := DefaultMotionProfile()
	cases := []struct {
		name string
		// The vehicle: where it stands and which way it drives.
		vx, vy, vh float64
		// The aircraft: pushing (south, tail first) or taxiing north, how far.
		push    bool
		run     float64
		want    bool
		wantMin float64
	}{
		{name: "away from the tail", vx: 0, vy: -45, vh: 180, run: 120},
		{name: "abeam the tail, away", vx: 20, vy: -25, vh: 90, run: 120},
		{name: "beyond the nose of a push, away", vx: 0, vy: 30, vh: 0, push: true, run: 40},
		{name: "crossing 60 m ahead", vx: -40, vy: 60, vh: 90, run: 120, want: true, wantMin: 10},
		{name: "towards the fuselage", vx: 40, vy: 0, vh: 270, run: 120, want: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pic := NewGroundPicture()
			now := time.Now()
			dir := 0.0
			if c.push {
				dir = 180
			}
			var ahead []airport.LatLon
			for d := 0.0; d <= c.run; d += trafficBodyStep {
				ahead = append(ahead, offsetHeading(at(0, 0), dir, d))
			}
			pic.Report(1, at(0, 0), 0, prof, now)
			if c.push {
				pic.ReportPush(1, ahead, prof.SpanMeters/2)
			} else {
				pic.ReportPath(1, ahead, prof.SpanMeters/2)
			}
			p := at(c.vx, c.vy)
			var mine []airport.LatLon
			for d := 0.0; d <= VehicleLookMeters; d += vehicleLookStep {
				mine = append(mine, offsetHeading(p, c.vh, d))
			}
			d, vehicle, ok := pic.VehicleConflict(0, 9001, mine, vehicleHalfMeters, false, now)
			if vehicle {
				t.Fatal("a vehicle conflict without vehicles")
			}
			if ok != c.want {
				t.Fatalf("conflict %v at %.0f m, want %v", ok, d, c.want)
			}
			if ok && d < c.wantMin {
				t.Errorf("conflict at %.0f m, want at %.0f m or more", d, c.wantMin)
			}
		})
	}
}
