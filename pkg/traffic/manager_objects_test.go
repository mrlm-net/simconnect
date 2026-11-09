package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// MaxObjects: no aircraft spawned at an airport while our objects in view
// there (aircraft and vehicles, Objects) and those spawning reach it; one
// gone, the next comes.
func TestManagerMaxObjects(t *testing.T) {
	sp := &fakeSpawner{}
	vehicles := 4 // tugs, stairs, buses already there
	var m *TrafficManager
	m = NewTrafficManager(sp, ManagerOptions{MinTurn: -1, MaxAircraft: 20, MaxPerAirport: 20, DepartureSpacing: time.Minute, MaxObjects: 7,
		Objects: func(icao string) int {
			if icao != "LKPR" {
				t.Errorf("objects asked for %s", icao)
			}
			n := vehicles
			for _, f := range m.flights {
				if f.Status.active() && f.Status != FlightSpawning {
					n++
				}
			}
			return n
		}}, "LKPR")
	sp.onSpawn = func(f ManagedFlight) { m.Update(f.Callsign, FlightBoarding, f.Since) }
	var fs []Flight
	for i := 0; i < 8; i++ {
		fs = append(fs, flight("CSA20"+string(rune('0'+i)), "LKPR", "EDDF", t0.Add(15*time.Minute), time.Hour))
	}
	m.Add(fs)
	for s := 0; s <= 10*60; s += 10 {
		m.Tick(t0.Add(time.Duration(s) * time.Second))
	}
	// 4 vehicles + 3 aircraft = 7.
	if len(sp.spawned) != 3 {
		t.Fatalf("%d spawned, want 3 (7 objects: 4 vehicles and 3 aircraft)", len(sp.spawned))
	}
	for _, f := range sp.spawned {
		m.Update(f.Callsign, FlightBoarding, t0.Add(10*time.Minute))
	}
	m.Tick(t0.Add(11 * time.Minute))
	if len(sp.spawned) != 3 {
		t.Fatalf("%d spawned at the cap, want still 3", len(sp.spawned))
	}
	vehicles = 2 // two vehicles gone
	for s := 11 * 60; s <= 15*60; s += 10 {
		m.Tick(t0.Add(time.Duration(s) * time.Second))
	}
	if len(sp.spawned) != 5 {
		t.Fatalf("%d spawned with two vehicles gone, want 5", len(sp.spawned))
	}
	m.SetMaxObjects(0)
	for s := 15 * 60; s <= 25*60; s += 10 {
		m.Tick(t0.Add(time.Duration(s) * time.Second))
	}
	if len(sp.spawned) != 8 {
		t.Fatalf("%d spawned with no limit, want all 8", len(sp.spawned))
	}
}

// CountNear counts the objects taken over and placed within the radius,
// aircraft and vehicles alike.
func TestInjectorCountNear(t *testing.T) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	at := airport.LatLon{Lat: 50.1008, Lon: 14.26}
	for k, d := range []float64{0, 1500, 5900, 6100, 20000} {
		obj := uint32(50 + k)
		if err := inj.Takeover(obj); err != nil {
			t.Fatal(err)
		}
		inj.Handle(sampleMsg(DefaultInjectRequestBase+2*uint32(inj.objects[obj].slot)+1, obj, injectGround{GroundFt: 1160, CGFt: 8, StaticPitch: 1,
			PlaneFt: 1168, PlanePitch: 1, OnGround: 1, Lat: at.Lat, Lon: at.Lon}))
		if err := inj.Place(obj, GroundPose{Position: offsetHeading(at, 90, d), Heading: 90}); err != nil {
			t.Fatal(err)
		}
	}
	if err := inj.Takeover(60); err != nil { // never placed
		t.Fatal(err)
	}
	if n := inj.CountNear(at, 6000); n != 3 {
		t.Fatalf("%d within 6 km, want 3", n)
	}
}
