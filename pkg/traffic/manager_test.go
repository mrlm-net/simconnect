//go:build windows
// +build windows

package traffic

import (
	"errors"
	"testing"
	"time"
)

type fakeSpawner struct {
	spawned []ManagedFlight
	removed []ManagedFlight
	onSpawn func(f ManagedFlight)
}

func (s *fakeSpawner) Spawn(f ManagedFlight) {
	s.spawned = append(s.spawned, f)
	if s.onSpawn != nil {
		s.onSpawn(f)
	}
}
func (s *fakeSpawner) Remove(f ManagedFlight) { s.removed = append(s.removed, f) }

var t0 = time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)

func flight(cs, from, to string, std time.Time, block time.Duration) Flight {
	return Flight{Callsign: cs, Airline: cs[:3], Type: "A320", Origin: from, Destination: to, STD: std, STA: std.Add(block)}
}

func statusOf(m *TrafficManager, key string) FlightStatus {
	for _, f := range m.Flights() {
		if f.Key() == key {
			return f.Status
		}
	}
	return 255
}

func TestManagerSpawnsOnTime(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1}, "LKPR")
	m.Add([]Flight{
		flight("CSA100", "LKPR", "EDDF", t0.Add(30*time.Minute), time.Hour),
		flight("DLH200", "EDDF", "LKPR", t0.Add(10*time.Minute), time.Hour), // STA 09:10
		flight("BAW300", "EGLL", "EDDF", t0, time.Hour),                     // not ours
	})
	m.Tick(t0.Add(19 * time.Minute))
	if len(sp.spawned) != 0 {
		t.Fatalf("spawned %v before its time", sp.spawned[0].Callsign)
	}
	m.Tick(t0.Add(20 * time.Minute)) // STD 08:30 − 10 min
	if len(sp.spawned) != 1 || sp.spawned[0].Callsign != "CSA100" || !sp.spawned[0].Departure() {
		t.Fatalf("spawned %+v, want the departure CSA100", sp.spawned)
	}
	m.Tick(t0.Add(45 * time.Minute)) // STA 09:10 − 25 min
	if len(sp.spawned) != 2 || sp.spawned[1].Callsign != "DLH200" || sp.spawned[1].Kind != "arrival" {
		t.Fatalf("spawned %+v, want the arrival DLH200", sp.spawned)
	}
	if len(m.Flights()) != 2 {
		t.Fatalf("%d flights managed, want 2", len(m.Flights()))
	}
}

func TestManagerLimitsAndSpacing(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, MaxAircraft: 3, DepartureSpacing: 2 * time.Minute, RemoveDepartedAfter: 5 * time.Minute}, "LKPR")
	sp.onSpawn = func(f ManagedFlight) { m.Update(f.Callsign, FlightBoarding, f.Since) }
	var fs []Flight
	for i := 0; i < 6; i++ {
		fs = append(fs, flight("CSA10"+string(rune('0'+i)), "LKPR", "EDDF", t0.Add(15*time.Minute), time.Hour))
	}
	m.Add(fs)
	for s := 0; s <= 10*60; s += 10 {
		m.Tick(t0.Add(time.Duration(s) * time.Second))
	}
	if len(sp.spawned) != 3 {
		t.Fatalf("%d spawned, want MaxAircraft 3", len(sp.spawned))
	}
	// 2 min apart: 08:05, 08:07, 08:09.
	for i, f := range sp.spawned {
		if want := t0.Add(time.Duration(5+2*i) * time.Minute); !f.Since.Equal(want) {
			t.Errorf("%s spawned at %v, want %v", f.Callsign, f.Since.Format("15:04:05"), want.Format("15:04:05"))
		}
	}
	// One leaves: the next may come.
	m.Update(sp.spawned[0].Callsign, FlightDeparted, t0.Add(11*time.Minute))
	m.Tick(t0.Add(11 * time.Minute))
	if len(sp.spawned) != 3 {
		t.Fatal("a departed aircraft freed its place before it was removed")
	}
	m.Tick(t0.Add(16 * time.Minute)) // removed 5 min after departing
	if len(sp.removed) != 1 || len(sp.spawned) != 4 {
		t.Fatalf("removed %d, spawned %d; want 1 and 4", len(sp.removed), len(sp.spawned))
	}
}

func TestManagerRetriesAndCancels(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1}, "LKPR")
	sp.onSpawn = func(f ManagedFlight) {
		m.Failed(f.Callsign, errors.New("model refused"), f.Since)
	}
	m.Add([]Flight{flight("CSA100", "LKPR", "EDDF", t0.Add(10*time.Minute), time.Hour)})
	for s := 0; s <= 5*60; s += 5 {
		m.Tick(t0.Add(time.Duration(s) * time.Second))
	}
	if len(sp.spawned) != 3 {
		t.Fatalf("%d attempts, want 3", len(sp.spawned))
	}
	for i, f := range sp.spawned {
		if f.Attempts != i+1 {
			t.Errorf("attempt %d reported as %d", i+1, f.Attempts)
		}
	}
	if s := statusOf(m, "departure CSA100"); s != FlightCancelled {
		t.Fatalf("status %v after 3 failures, want cancelled", s)
	}
	// A flight never started in time is cancelled as late.
	m.Add([]Flight{flight("CSA200", "LKPR", "EDDF", t0.Add(-20*time.Minute), time.Hour)})
	m.SetEnabled(false)
	m.Tick(t0)
	if s := statusOf(m, "departure CSA200"); s != FlightCancelled {
		t.Fatalf("status %v of a flight 20 min past its STD", s)
	}
}

func TestManagerTurnaround(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{}, "LKPR")
	arr := flight("CSA101", "EDDF", "LKPR", t0, time.Hour)                       // STA 09:00
	dep := flight("CSA102", "LKPR", "EDDF", t0.Add(2*time.Hour), time.Hour)      // STD 10:00
	other := flight("CSA104", "LKPR", "EDDF", t0.Add(90*time.Minute), time.Hour) // 30 min after: too short
	m.Add([]Flight{arr, dep, other})
	var d ManagedFlight
	for _, f := range m.Flights() {
		if f.Callsign == "CSA102" {
			d = f
		}
	}
	if d.TurnFrom != "CSA101" {
		t.Fatalf("CSA102 turns from %q, want CSA101", d.TurnFrom)
	}
	now := t0.Add(35 * time.Minute)
	m.Tick(now) // the arrival
	m.Update("CSA101", FlightApproaching, now)
	m.Update("CSA101", FlightParked, t0.Add(62*time.Minute))
	m.Tick(t0.Add(80 * time.Minute))  // CSA104 spawns fresh at 09:20
	m.Tick(t0.Add(110 * time.Minute)) // STD 10:00 − 10 min: adopts
	var last ManagedFlight
	for _, f := range sp.spawned {
		last = f
	}
	if len(sp.spawned) != 3 || last.Callsign != "CSA102" || last.TurnFrom != "CSA101" {
		t.Fatalf("spawned %d, last %s from %q", len(sp.spawned), last.Callsign, last.TurnFrom)
	}
	m.Update("CSA102", FlightBoarding, t0.Add(111*time.Minute))
	if s := statusOf(m, "arrival CSA101"); s != FlightDone {
		t.Fatalf("arrival %v once its aircraft boards as CSA102", s)
	}
	if len(sp.removed) != 0 {
		t.Fatal("the turning aircraft was removed")
	}
}

func TestManagerTurnaroundLateArrival(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{Checks: []SituationCheck{}}, "LKPR")
	m.Add([]Flight{flight("CSA101", "EDDF", "LKPR", t0, time.Hour), flight("CSA102", "LKPR", "EDDF", t0.Add(2*time.Hour), time.Hour)})
	m.Tick(t0.Add(35 * time.Minute))
	m.Update("CSA101", FlightApproaching, t0.Add(35*time.Minute))
	// Still approaching past the departure's STD + 15 min: the departure waits.
	m.Tick(t0.Add(140 * time.Minute))
	if s := statusOf(m, "departure CSA102"); s != FlightScheduled {
		t.Fatalf("departure %v while its aircraft is still arriving", s)
	}
	// The arrival fails: the departure goes with a fresh aircraft.
	m.Failed("CSA101", errors.New("go-around failed"), t0.Add(141*time.Minute))
	if len(sp.removed) != 1 {
		t.Fatal("the failed arrival was not removed")
	}
	m.Tick(t0.Add(141 * time.Minute))
	if s := statusOf(m, "departure CSA102"); s != FlightCancelled {
		t.Fatalf("departure %v 21 min late without an aircraft, want cancelled", s)
	}
}

func TestManagerSourceAndBoards(t *testing.T) {
	sp := &fakeSpawner{}
	asked := 0
	m := NewTrafficManager(sp, ManagerOptions{Source: func(from, to time.Time, airports []string) []Flight {
		asked++
		return Schedule(DefaultScheduleConfig(), ScheduleOptions{Focus: airports, Seed: uint64(from.Unix())}, from, to)
	}}, "LKPR")
	m.Tick(t0.Add(30 * time.Minute))
	m.Tick(t0.Add(40 * time.Minute))
	if asked != 3 { // 08:00, 09:00, 10:00 (horizon 2 h from 08:30)
		t.Fatalf("source asked %d times, want 3", asked)
	}
	deps, arrs := m.Board("LKPR")
	if len(deps) == 0 || len(arrs) == 0 {
		t.Fatalf("boards: %d departures, %d arrivals", len(deps), len(arrs))
	}
	for i := 1; i < len(deps); i++ {
		if deps[i].STD.Before(deps[i-1].STD) {
			t.Fatal("departures not by STD")
		}
	}
	if m.Active() == 0 {
		t.Fatal("nothing spawned in a busy morning")
	}
	if m.Active() > m.Options().MaxPerAirport {
		t.Fatalf("%d active at one airport", m.Active())
	}
}

func TestModelsFor(t *testing.T) {
	models := []string{
		"FSLTL_FAIB_B738_TVS-Smartwings_NC", "FSLTL_BCS3_TVS_Smartwings-STUB", "FSLTL_B738F_ZZZZ",
		"FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", "737 Max 8 Passengers :: White",
		"FSLTL_B738_KLM", "A321 :: 01 Aeroflot Airlines", "737 Max 8 BBJ", "FSLTL_E170_LOT_RETROJET",
		"FSLTL_E190_LOT", "Asobo PassiveAircraft B737-800 :: B737_800_UNITED",
	}
	cases := []struct {
		airline, name, typ string
		want               []string
	}{
		{"TVS", "Smartwings", "B738", []string{"FSLTL_FAIB_B738_TVS-Smartwings_NC", "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", "FSLTL_B738_KLM"}},
		{"TVS", "Smartwings", "B38M", []string{"FSLTL_FAIB_B738_TVS-Smartwings_NC", "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", "737 Max 8 Passengers :: White"}},
		{"CSA", "Czech Airlines", "A320", []string{"FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery"}},
		{"LOT", "LOT", "E190", []string{"FSLTL_E190_LOT", "FSLTL_E170_LOT_RETROJET"}},
	}
	for _, c := range cases {
		got := ModelsFor(models, c.airline, c.name, c.typ, 0)
		if len(got) != len(c.want) {
			t.Errorf("%s %s: %q, want %q", c.airline, c.typ, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s %s: %q, want %q", c.airline, c.typ, got, c.want)
				break
			}
		}
	}
}
