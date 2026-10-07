package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

func TestClassifyObserved(t *testing.T) {
	field := airport.LatLon{Lat: 50.1008, Lon: 14.26}
	east := func(nm float64) (float64, float64) { return calc.DisplaceByHeading(field.Lat, field.Lon, 90, nm*1852) }
	lat, lon := east(1)
	flat, flon := east(40)
	cases := []struct {
		name string
		o    Observed
		want string
	}{
		{"parked", Observed{Lat: lat, Lon: lon, OnGround: true}, ObservedParked},
		{"taxiing", Observed{Lat: lat, Lon: lon, OnGround: true, GroundKts: 12}, ObservedDeparture},
		{"elsewhere", Observed{Lat: flat, Lon: flon, OnGround: true}, ""},
		{"inbound", Observed{Lat: flat, Lon: flon, AltFt: 12000, TrackDeg: 270, VSFpm: -1500}, ObservedArrival},
		{"outbound", Observed{Lat: flat, Lon: flon, AltFt: 32000, TrackDeg: 90}, ObservedOverflight},
		{"given", Observed{Lat: flat, Lon: flon, Kind: "Departure"}, ObservedDeparture},
		{"ground station", Observed{Lat: lat, Lon: lon, OnGround: true, Type: "TWR"}, ""},
	}
	for _, c := range cases {
		if got, why := ClassifyObserved(c.o, field); got != c.want {
			t.Errorf("%s: %q (%s), want %q", c.name, got, why, c.want)
		}
	}
}

func TestSightingAt(t *testing.T) {
	seen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	s := Sighting{Position: airport.LatLon{Lat: 50, Lon: 14}, AltFt: 10000, GroundKts: 240, TrackDeg: 0, VSFpm: -1000, SeenAt: seen}
	p, alt := s.At(seen.Add(time.Minute))
	if d := calc.HaversineNM(50, 14, p.Lat, p.Lon); d < 3.9 || d > 4.1 {
		t.Errorf("moved %.2f NM in a minute at 240 kt", d)
	}
	if alt != 9000 {
		t.Errorf("altitude %.0f, want 9000", alt)
	}
	// Projected at most ObservedProjectMax.
	p, _ = s.At(seen.Add(time.Hour))
	if d := calc.HaversineNM(50, 14, p.Lat, p.Lon); d > 41 {
		t.Errorf("projected %.0f NM", d)
	}
}

// A real flight spawns at once, its later sightings refresh it, Drop plays
// out one in progress and removes one waiting.
func TestManagerObserved(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{})
	m.SetAirports("LKPR")
	m.SetEnabled(true)
	s := &Sighting{ID: "abc123", Registration: "OK-TVA", SeenAt: now}
	m.Add([]Flight{
		{Callsign: "TVS1", Type: "B738", Destination: "LKPR", STA: now.Add(40 * time.Minute), Observed: s},
		{Callsign: "OKTVB", Type: "B738", Origin: "LKPR", STD: now.Add(24 * time.Hour), Observed: &Sighting{ID: "def", OnGround: true}},
	})
	m.Tick(now)
	if len(sp.spawned) != 2 {
		t.Fatalf("spawned %d, want both at once", len(sp.spawned))
	}
	for _, f := range sp.spawned {
		if f.Arrival() && f.Stage != "observed" {
			t.Errorf("arrival stage %q", f.Stage)
		}
	}
	m.Update("TVS1", FlightApproaching, now)
	if !m.Observe("arrival", "TVS1", Sighting{ID: "abc123", Registration: "OK-TVX"}, "LOWW", "") {
		t.Fatal("observe refused")
	}
	if f, _ := m.Flight("arrival", "TVS1"); f.Observed.Registration != "OK-TVX" || f.Origin != "LOWW" {
		t.Errorf("not refreshed: %+v", f.Observed)
	}
	m.Update("OKTVB", FlightBoarding, now)
	if !m.Retime("OKTVB", now.Add(5*time.Minute)) {
		t.Error("retime refused on the stand")
	}
	m.Drop("arrival", "TVS1", now)
	if f, _ := m.Flight("arrival", "TVS1"); f.Status != FlightApproaching {
		t.Errorf("dropped off the approach: %s", f.Status)
	}
	m.Update("TVS1", FlightParked, now)
	m.Tick(now.Add(time.Second))
	if f, _ := m.Flight("arrival", "TVS1"); f.Status != FlightDone {
		t.Errorf("dropped and parked, still %s", f.Status)
	}
	m.Drop("departure", "OKTVB", now)
	if f, _ := m.Flight("departure", "OKTVB"); f.Status != FlightDone {
		t.Errorf("dropped on the stand, still %s", f.Status)
	}
}
