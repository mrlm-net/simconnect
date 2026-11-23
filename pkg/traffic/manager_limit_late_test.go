package traffic

import (
	"strings"
	"testing"
	"time"
)

// TestManagerRetimesDepartureHeldByLimit: a departure held by the aircraft
// limit past its start keeps its time on the stand: spawned late, it is
// estimated DepartureLead after its spawn, with the limit as the reason;
// one spawned on time keeps its STD (#1679).
func TestManagerRetimesDepartureHeldByLimit(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, StartStands: -1, MaxAircraft: 1, MaxPerAirport: 1, RemoveDepartedAfter: 5 * time.Minute}, "LKPR")
	sp.onSpawn = func(f ManagedFlight) { m.Update(f.Callsign, FlightBoarding, f.Since) }
	m.Add([]Flight{
		flight("CSA100", "LKPR", "EDDF", t0.Add(15*time.Minute), time.Hour),
		flight("CSA200", "LKPR", "EDDF", t0.Add(20*time.Minute), time.Hour),
	})
	departed := false
	for s := 0; s <= 30*60; s += 10 {
		now := t0.Add(time.Duration(s) * time.Second)
		if !departed && !now.Before(t0.Add(12*time.Minute)) {
			m.Update("CSA100", FlightDeparted, now)
			departed = true
		}
		m.Tick(now)
	}
	if len(sp.spawned) != 2 {
		t.Fatalf("%d spawned, want 2", len(sp.spawned))
	}
	first, late := sp.spawned[0], sp.spawned[1]
	if first.Callsign != "CSA100" || !first.Estimated.IsZero() {
		t.Errorf("%s spawned on time estimated %v, want its STD", first.Callsign, first.Estimated)
	}
	lead := m.Options().DepartureLead
	start := late.STD.Add(-lead)
	if late.Callsign != "CSA200" || !late.Since.After(start) {
		t.Fatalf("%s spawned at %v, not held past its start %v", late.Callsign, late.Since, start)
	}
	if want := late.Since.Add(lead); !late.Estimated.Equal(want) {
		t.Errorf("%s spawned %v late: estimated %v, want %v", late.Callsign, late.Since.Sub(start), late.Estimated, want)
	}
	if !strings.HasPrefix(late.Note, "late: aircraft limit") {
		t.Errorf("note %q, want late: aircraft limit", late.Note)
	}
}
