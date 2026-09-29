//go:build windows
// +build windows

package traffic

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

func TestOverflights(t *testing.T) {
	cfg := DefaultScheduleConfig()
	o := OverflightOptions{Centre: pictureLKPR.Position, RadiusNM: 150, PerHour: 8, Seed: 3, Exclude: []string{"LKPR"}}
	day := time.Date(2026, 9, 29, 6, 0, 0, 0, time.UTC)
	fs := Overflights(cfg, o, day, day.Add(6*time.Hour))
	if len(fs) < 20 {
		t.Fatalf("%d overflights in 6 busy hours", len(fs))
	}
	if !reflect.DeepEqual(fs, Overflights(cfg, o, day, day.Add(6*time.Hour))) {
		t.Fatal("not deterministic")
	}
	pos := map[string]ScheduleAirport{}
	for _, a := range cfg.Airports {
		pos[a.ICAO] = a
	}
	for _, f := range fs {
		a, b := pos[f.Origin], pos[f.Destination]
		if f.Origin == "LKPR" || f.Destination == "LKPR" {
			t.Errorf("%s touches an excluded airport", f.Callsign)
		}
		for _, x := range []ScheduleAirport{a, b} {
			if calc.HaversineNM(o.Centre.Lat, o.Centre.Lon, x.Position.Lat, x.Position.Lon) <= o.RadiusNM {
				t.Errorf("%s: %s is inside the area", f.Callsign, x.ICAO)
			}
		}
		if f.Enter.Before(day) || !f.Enter.Before(day.Add(6*time.Hour)) || !f.Exit.After(f.Enter) {
			t.Errorf("%s: enter %v exit %v", f.Callsign, f.Enter, f.Exit)
		}
		if !f.STD.Before(f.Enter) || !f.Exit.Before(f.STA) {
			t.Errorf("%s: STD %v enter %v exit %v STA %v", f.Callsign, f.STD, f.Enter, f.Exit, f.STA)
		}
		// Through the area: at least half the radius inside.
		if in := f.Exit.Sub(f.Enter).Hours() * cruiseKts(f.Type); in < o.RadiusNM/2-10 {
			t.Errorf("%s crosses only %.0f NM", f.Callsign, in)
		}
	}
}

func TestManagerEnrouteArrival(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, Checks: []SituationCheck{}}, "LKPR")
	m.Add([]Flight{flight("DLH1", "EDDF", "LKPR", t0, 2*time.Hour)}) // STA 10:00
	m.Tick(t0.Add(74 * time.Minute))
	if len(sp.spawned) != 0 {
		t.Fatal("spawned before STA − 25 − 20 min")
	}
	m.Tick(t0.Add(75 * time.Minute)) // 09:15
	if len(sp.spawned) != 1 || sp.spawned[0].Stage != "enroute" {
		t.Fatalf("spawned %+v, want the enroute stage", sp.spawned)
	}
	m.Attach("DLH1", 42)
	m.Update("DLH1", FlightEnroute, t0.Add(76*time.Minute))
	m.Tick(t0.Add(95 * time.Minute)) // STA − 25: no second spawn, it is on its way
	if len(sp.spawned) != 1 {
		t.Fatal("spawned again while enroute")
	}
	m.Update("DLH1", FlightApproaching, t0.Add(96*time.Minute)) // handed over at the STAR entry
	if f := find(m, "arrival DLH1"); f.Stage != "" || f.ObjectID != 42 {
		t.Fatalf("after the handover: stage %q, object %d", f.Stage, f.ObjectID)
	}
}

func TestManagerEnrouteFailsToEntry(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, Checks: []SituationCheck{}}, "LKPR")
	sp.onSpawn = func(f ManagedFlight) {
		if f.Stage == "enroute" {
			m.Failed(f.Callsign, errors.New("no plan"), f.Since)
		}
	}
	m.Add([]Flight{flight("DLH1", "EDDF", "LKPR", t0, 2*time.Hour)})
	for s := 75 * 60; s <= 96*60; s += 10 {
		m.Tick(t0.Add(time.Duration(s) * time.Second))
	}
	if len(sp.spawned) != 2 || sp.spawned[1].Stage != "" || sp.spawned[1].Since.Before(t0.Add(95*time.Minute)) {
		t.Fatalf("spawns %d; the second must be at the STAR entry at STA − 25", len(sp.spawned))
	}
	if sp.spawned[1].Attempts != 1 {
		t.Errorf("attempt %d: the enroute failure counted", sp.spawned[1].Attempts)
	}
}

func TestManagerOverflightLeavesWithArea(t *testing.T) {
	pic := NewTrafficPicture(PictureOptions{Centre: Centre{ICAO: "LKPR"}, RadiusNM: 150})
	pic.SetAirports([]AirportRef{pictureLKPR})
	sp := &fakeSpawner{}
	over := Flight{Callsign: "BAW9", Airline: "BAW", Type: "A320", Origin: "EGLL", Destination: "LHBP",
		STD: t0, STA: t0.Add(2 * time.Hour), Enter: t0.Add(50 * time.Minute), Exit: t0.Add(75 * time.Minute)}
	m := NewTrafficManager(sp, ManagerOptions{Picture: pic, Checks: []SituationCheck{},
		Overflights: func(from, to time.Time) []Flight {
			if !over.Enter.Before(from) && over.Enter.Before(to) {
				return []Flight{over}
			}
			return nil
		}}, "LKPR")
	m.Tick(t0.Add(49 * time.Minute))
	if len(sp.spawned) != 0 {
		t.Fatal("spawned before it enters")
	}
	now := t0.Add(50 * time.Minute)
	m.Tick(now)
	if len(sp.spawned) != 1 || !sp.spawned[0].Overflight() || sp.spawned[0].Stage != "enroute" {
		t.Fatalf("spawned %+v", sp.spawned)
	}
	m.Attach("BAW9", 77)
	m.Update("BAW9", FlightEnroute, now)
	near := offsetHeading(pictureLKPR.Position, 270, 50*1852)
	pic.Observe(now, []Observation{{ObjectID: 77, Tail: "BAW9", Position: near, AGLFt: 35000}})
	m.Tick(now.Add(time.Second))
	// Gone from the picture (out of the area): removed after LeftAfter.
	later := now.Add(PictureStaleAfter + 2*time.Second)
	pic.Observe(later, nil)
	m.Tick(later.Add(30 * time.Second))
	if len(sp.removed) != 0 {
		t.Fatal("removed before LeftAfter")
	}
	m.Tick(later.Add(2 * time.Minute))
	if f := find(m, "overflight BAW9"); len(sp.removed) != 1 || f.Status != FlightDone || f.Note != "left the area" {
		t.Fatalf("removed %d, %v %q", len(sp.removed), f.Status, f.Note)
	}
}

func TestOverflightsOutOfLandingFlow(t *testing.T) {
	sp := &fakeSpawner{}
	over := Flight{Callsign: "BAW9", Airline: "BAW", Type: "A320", Origin: "EGLL", Destination: "LHBP",
		STD: t0, STA: t0.Add(2 * time.Hour), Enter: t0.Add(30 * time.Minute), Exit: t0.Add(55 * time.Minute)}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, Overflights: func(from, to time.Time) []Flight {
		if !over.Enter.Before(from) && over.Enter.Before(to) {
			return []Flight{over}
		}
		return nil
	}}, "LKPR")
	m.Add([]Flight{flight("DLH1", "EDDF", "LKPR", t0.Add(-30*time.Minute), 2*time.Hour)}) // STA 09:30
	m.Tick(t0.Add(30 * time.Minute))
	if f := find(m, "overflight BAW9"); f.Note != "" || !f.Estimated.IsZero() || f.Status != FlightSpawning {
		t.Fatalf("overflight %v, note %q, estimated %v", f.Status, f.Note, f.Estimated)
	}
}

func TestOverflightAirlinesServeBothEnds(t *testing.T) {
	cfg := DefaultScheduleConfig()
	al := map[string]Airline{}
	for _, a := range cfg.Airlines {
		al[a.ICAO] = a
	}
	day := time.Date(2026, 9, 29, 6, 0, 0, 0, time.UTC)
	for seed := uint64(1); seed <= 5; seed++ {
		for _, f := range Overflights(cfg, OverflightOptions{Centre: pictureLKPR.Position, RadiusNM: 100, PerHour: 8, Seed: seed}, day, day.Add(12*time.Hour)) {
			a := al[f.Airline]
			for _, end := range []string{f.Origin, f.Destination} {
				if !contains(a.Bases, end) && !contains(a.Regions, "*") && !contains(a.Regions, end[:2]) {
					t.Errorf("%s %s→%s: %s does not serve %s", f.Callsign, f.Origin, f.Destination, f.Airline, end)
				}
			}
		}
	}
}
