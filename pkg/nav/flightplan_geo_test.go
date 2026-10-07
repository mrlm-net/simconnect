package nav

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestProfilePointLongLeg: a TOD on a transatlantic leg lies on the great
// circle, not thousands of NM off a flat displacement (#41).
func TestProfilePointLongLeg(t *testing.T) {
	lkpr, kjfk := airport.LatLon{Lat: 50.1, Lon: 14.26}, airport.LatLon{Lat: 40.64, Lon: -73.78}
	leg := calc.HaversineNM(lkpr.Lat, lkpr.Lon, kjfk.Lat, kjfk.Lon)
	wps := []Waypoint{{Position: lkpr}, {Position: kjfk, DistanceNM: leg}}
	d := leg - 120
	wps = insertProfilePoint(wps, d, "TOD", 37000)
	if len(wps) != 3 || wps[1].Ident != "TOD" {
		t.Fatalf("%+v", wps)
	}
	p := wps[1].Position
	if off := calc.HaversineNM(p.Lat, p.Lon, kjfk.Lat, kjfk.Lon); math.Abs(off-120) > 0.5 {
		t.Errorf("TOD %.1f NM from KJFK, want 120", off)
	}
	if off := calc.HaversineNM(lkpr.Lat, lkpr.Lon, p.Lat, p.Lon); math.Abs(off-d) > 0.5 {
		t.Errorf("TOD %.1f NM along, want %.1f", off, d)
	}
}

// TestPositionAtAntimeridian: across ±180 the position takes the short way
// on the great circle, and the track is the one there (#42).
func TestPositionAtAntimeridian(t *testing.T) {
	a, b := airport.LatLon{Lat: -17, Lon: 178}, airport.LatLon{Lat: -18, Lon: -178}
	leg := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
	fp := &FlightPlan{Waypoints: []Waypoint{{Position: a, AltFt: 30000}, {Position: b, AltFt: 30000, DistanceNM: leg}}}
	p, _, trk := fp.PositionAt(leg / 2)
	if p.Lon > -179 && p.Lon < 179 {
		t.Fatalf("halfway at lon %.2f: the long way round", p.Lon)
	}
	if da, db := calc.HaversineNM(a.Lat, a.Lon, p.Lat, p.Lon), calc.HaversineNM(p.Lat, p.Lon, b.Lat, b.Lon); math.Abs(da-db) > 0.1 {
		t.Errorf("halfway %.1f / %.1f NM", da, db)
	}
	if want := calc.BearingDegrees(p.Lat, p.Lon, b.Lat, b.Lon); math.Abs(trk-want) > 1e-6 {
		t.Errorf("track %.2f, want %.2f", trk, want)
	}
	// At the end: the track arriving, not 0 or the initial bearing.
	_, _, end := fp.PositionAt(leg)
	if end < 90 || end > 120 {
		t.Errorf("track at the end %.1f", end)
	}
	// Long legs: the track follows the great circle (LKPR → KJFK starts
	// north-west, ends south-west).
	lkpr, kjfk := airport.LatLon{Lat: 50.1, Lon: 14.26}, airport.LatLon{Lat: 40.64, Lon: -73.78}
	n := calc.HaversineNM(lkpr.Lat, lkpr.Lon, kjfk.Lat, kjfk.Lon)
	fp = &FlightPlan{Waypoints: []Waypoint{{Position: lkpr}, {Position: kjfk, DistanceNM: n}}}
	if _, _, trk := fp.PositionAt(n - 50); trk < 220 || trk > 270 {
		t.Errorf("track near KJFK %.0f", trk)
	}
}

// TestCruiseLevelMaxFL: airports so high that the floor is above the
// type's ceiling get the ceiling, never a level the type cannot fly (E21).
func TestCruiseLevelMaxFL(t *testing.T) {
	p := Performance{CruiseTASKts: 150, ClimbTASKts: 100, DescentTASKts: 120, MaxFL: 150, ClimbFPM: 700, DescentFPM: 700}
	if fl := CruiseLevel(p, 90, 300, 14000, 1000); fl != 150 {
		t.Errorf("eastbound FL%d, want FL150", fl)
	}
	if fl := CruiseLevel(p, 270, 300, 14000, 1000); fl != 140 {
		t.Errorf("westbound FL%d, want FL140", fl)
	}
	// Normal airports: as before.
	if fl := CruiseLevel(p, 90, 300, 1000, 1000); fl != 150 {
		t.Errorf("FL%d", fl)
	}
}

// TestATISNoQNH: without a QNH there is no transition level either (#50).
func TestATISNoQNH(t *testing.T) {
	w := StaticWeather(240, 8, 10000, 15, 8, 0)
	if a := NewATIS("Ruzyne", 'A', time.Now(), w, RunwayUse{}, 5000, 0); a.TransitionLevel != 0 {
		t.Errorf("TL %d without QNH", a.TransitionLevel)
	}
	w = StaticWeather(240, 8, 10000, 15, 8, 1013)
	if a := NewATIS("Ruzyne", 'A', time.Now(), w, RunwayUse{}, 5000, 0); a.TransitionLevel != 70 {
		t.Errorf("TL %d at 1013, want 70", a.TransitionLevel)
	}
}
