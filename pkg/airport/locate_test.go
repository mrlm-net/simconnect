//go:build windows
// +build windows

package airport

import (
	"math"
	"testing"
)

// TestLocate: at LKPR with a point-only neighbour (LKJXT, 3.4 km away, no
// geometry in the simulator) and an alias sharing its runways.
func TestLocate(t *testing.T) {
	l, err := BuildLayout(lkprRaw(t))
	if err != nil {
		t.Fatal(err)
	}
	jxt := &Layout{ICAO: "LKJXT", Latitude: 50.08525975, Longitude: 14.22006994}
	alias := &Layout{ICAO: "LK0001", Latitude: l.Latitude, Longitude: l.Longitude, Runways: l.Runways[:1]}
	all := []*Layout{jxt, alias, l}
	stand := l.ParkingByLabel("C22")[0].Position
	if loc, ok := Locate(LocateQuery{Position: stand, OnGround: true}, all); !ok || loc.ICAO != "LKPR" || loc.Feature != AtParking || loc.Meters != 0 {
		t.Errorf("at C22: %+v", loc)
	}
	// On the runway the alias shares: LKPR, the larger, with the alias as
	// an alternative.
	r := l.Runways[0]
	if loc, _ := Locate(LocateQuery{Position: r.Center, OnGround: true}, all); loc.ICAO != "LKPR" || loc.Feature != OnRunway || len(loc.Alternatives) != 1 || loc.Alternatives[0] != "LK0001" {
		t.Errorf("on %s: %+v", r.Name(), loc)
	}
	// Next to the point-only airport, off LKPR's surfaces: that one.
	near := LatLon{Lat: jxt.Latitude + 0.0002, Lon: jxt.Longitude}
	if loc, _ := Locate(LocateQuery{Position: near, OnGround: true}, all); loc.ICAO != "LKJXT" || loc.Feature != NearAirport {
		t.Errorf("by LKJXT: %+v", loc)
	}
	// Far from everything: nowhere.
	if loc, ok := Locate(LocateQuery{Position: LatLon{Lat: 50.5, Lon: 14.26}, OnGround: true}, all); ok {
		t.Errorf("40 km away: %+v", loc)
	}
	// On final for 24, 4 NM out on the glide path, descending: approach to 24.
	_, end, _ := l.RunwayEnd("24")
	f := newLocalFrame(end.Threshold.Lat, end.Threshold.Lon)
	h := (end.Heading + 180) * math.Pi / 180
	d := 4 * metersPerNM
	p := LatLon{Lat: end.Threshold.Lat + d*math.Cos(h)/f.mPerLat, Lon: end.Threshold.Lon + d*math.Sin(h)/f.mPerLon}
	q := LocateQuery{Position: p, HeightFt: 1270, Track: end.Heading, HasTrack: true, VerticalFpm: -700}
	if loc, ok := Locate(q, all); !ok || loc.ICAO != "LKPR" || loc.Feature != OnApproach || loc.Name != "24" {
		t.Errorf("4 NM final 24: %+v", loc)
	}
	// The same place climbing the other way: a departure from 06.
	q.Track, q.VerticalFpm, q.HeightFt = end.Heading+180, 1800, 2500
	if loc, _ := Locate(q, all); loc.Feature != OnDeparture || loc.Name != "06" {
		t.Errorf("climbing out of 06: %+v", loc)
	}
}

// TestTracker: climbing out it stays with where it took off, though
// another airport's approach corridor lies the same way; approaching, its
// destination wins.
func TestTracker(t *testing.T) {
	l, err := BuildLayout(lkprRaw(t))
	if err != nil {
		t.Fatal(err)
	}
	_, e24, _ := l.RunwayEnd("24")
	_, e06, _ := l.RunwayEnd("06")
	f := newLocalFrame(e24.Threshold.Lat, e24.Threshold.Lon)
	at := func(from LatLon, hdg, m float64) LatLon {
		h := hdg * math.Pi / 180
		return LatLon{Lat: from.Lat + m*math.Cos(h)/f.mPerLat, Lon: from.Lon + m*math.Sin(h)/f.mPerLon}
	}
	// A strip 3 NM beyond 24's far end, its runway on 24's line: its
	// approach corridor covers LKPR's climb-out.
	strip := at(e06.Threshold, e24.Heading, 3*metersPerNM)
	other := &Layout{ICAO: "LKXX", Latitude: strip.Lat, Longitude: strip.Lon, Runways: []Runway{{
		Center: strip, Length: 600, Width: 20,
		Primary:   RunwayEnd{Name: "24", Threshold: at(strip, e24.Heading+180, 300)},
		Secondary: RunwayEnd{Name: "06", Threshold: at(strip, e24.Heading, 300)},
	}}}
	all := []*Layout{other, l}
	climb := LocateQuery{Position: at(e06.Threshold, e24.Heading, 1.5*metersPerNM), HeightFt: 900}
	if loc, _ := Locate(climb, all); loc.ICAO != "LKXX" {
		t.Logf("alone, 1.5 NM out of 24: %+v (the strip's approach)", loc)
	}
	tr := NewTracker()
	tr.Update(LocateQuery{Position: e24.Threshold, OnGround: true}, all)
	if tr.From() != "LKPR" {
		t.Fatalf("from %q, want LKPR", tr.From())
	}
	if loc, _ := tr.Update(climb, all); loc.ICAO != "LKPR" || loc.Feature != OnDeparture {
		t.Errorf("climbing out of 24: %+v", loc)
	}
	// Coming in to the strip over LKPR, with the strip as destination.
	tr = NewTracker()
	tr.SetDestination("LKXX")
	if loc, _ := tr.Update(climb, all); loc.ICAO != "LKXX" || loc.Feature != OnApproach {
		t.Errorf("to the strip: %+v", loc)
	}
}
