package traffic

import (
	"fmt"
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

// TestVehicleConflictSeesTheTips (#1630): a vehicle's reach is measured
// from an aircraft's true nose and wing tip, whatever its length and span.
// Before, both were sampled every 2 m from the tail and the root, and up
// to 2 m of each were not there: the vehicle saw it a look later.
func TestVehicleConflictSeesTheTips(t *testing.T) {
	at := airport.LatLon{Lat: 50.1, Lon: 14.26}
	const sees = 10.0
	// check drives at the point tip along out and back, so that sees metres
	// on it is just inside reach of it.
	check := func(what string, prof MotionProfile, tip airport.LatLon, out, reach float64) {
		t.Helper()
		pic := NewGroundPicture()
		now := time.Now()
		pic.Report(1, at, 0, prof, now)
		// Waiting to taxi: its body counts, not a way ahead.
		pic.ReportPlanned(1, []airport.LatLon{at}, prof.SpanMeters/2)
		from := offsetHeading(tip, out, reach-0.05+sees)
		var mine []airport.LatLon
		for d := 0.0; d <= VehicleLookMeters; d += vehicleLookStep {
			mine = append(mine, offsetHeading(from, out+180, d))
		}
		if d, _, ok := pic.VehicleConflict(0, 9001, mine, vehicleHalfMeters, false, now); !ok || d != sees {
			t.Errorf("%s: conflict %v at %.0f m, want at %.0f m", what, ok, d, sees)
		}
	}
	reach := vehicleHalfMeters + VehicleClearMeters
	for tail := 4.0; tail <= 40; tail += 0.7 {
		prof := DefaultMotionProfile()
		prof.TailMeters = tail
		nose := offsetHeading(at, 0, prof.WheelbaseMeters*pushNoseFactor-prof.RefAheadMeters)
		check(fmt.Sprintf("the nose, tail %.1f m", tail), prof, nose, 0, reach+vehicleFuselageHalfMeters)
	}
	for span := 20.0; span <= 64; span += 1.3 {
		prof := DefaultMotionProfile()
		prof.SpanMeters = span
		oh := span / 2
		tip := offsetHeading(offsetHeading(at, 0, -wingSweep*oh), 90, oh)
		check(fmt.Sprintf("the wing tip, span %.1f m", span), prof, tip, localBearing(at, tip), reach)
	}
}
