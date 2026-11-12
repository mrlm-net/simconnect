package traffic

import (
	"testing"
	"time"
)

// startArrivals plays the generated LKPR schedule (density 3) through a
// manager for 30 minutes from 10:00, every spawn taken at once, and
// returns the arrivals spawned.
func startArrivals(t *testing.T, min time.Duration) []ManagedFlight {
	t.Helper()
	cfg := DefaultScheduleConfig()
	src := func(from, to time.Time, airports []string) []Flight {
		return Schedule(cfg, ScheduleOptions{Focus: airports, Density: 3, Seed: 7 ^ uint64(from.Unix()/3600)}, from, to)
	}
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{Source: src, MaxAircraft: 40, MaxPerAirport: 40, EnrouteLead: -1, StartStands: -1, StartAirborneMin: min}, "LKPR")
	start := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	for now := start; now.Before(start.Add(30 * time.Minute)); now = now.Add(10 * time.Second) {
		m.Tick(now)
		for _, f := range m.Flights() {
			if f.Status == FlightSpawning {
				m.Update(f.Callsign, FlightApproaching, now)
			}
		}
	}
	var out []ManagedFlight
	for _, f := range sp.spawned {
		if f.Arrival() {
			out = append(out, f)
		}
	}
	return out
}

// Arrivals due sooner than ArrivalLead when traffic begins appear on their
// STAR where they would be by now (stage "midway"), as the landing flow
// spaces them: the first landing a few minutes after the start, not
// ArrivalLead after
// (the user, live LKPR 2026-11-11: "not have long slot at start"). Those
// due within StartAirborneMin are left out; later ones keep their STAR
// entry.
func TestManagerStartAirborneArrivals(t *testing.T) {
	start := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	o := ManagerOptions{}
	o.defaults()
	with := startArrivals(t, 0)
	midway, early, firstSTA := 0, 0, time.Time{}
	for _, f := range with {
		if f.Stage == "midway" {
			midway++
			if !f.StartAirborne {
				t.Errorf("%s midway, not StartAirborne", f.Callsign)
			}
			if f.STA.Before(start.Add(o.StartAirborneMin)) {
				t.Errorf("%s on its way in, due %s: within %s of the start", f.Callsign, f.STA.Format("15:04"), o.StartAirborneMin)
			}
			if f.STA.After(start.Add(o.ArrivalLead)) {
				t.Errorf("%s on its way in, due %s: it had time for its STAR entry", f.Callsign, f.STA.Format("15:04"))
			}
			if !f.Since.After(start.Add(3 * time.Minute)) {
				early++
			}
		} else if f.StartAirborne {
			t.Errorf("%s StartAirborne, stage %q", f.Callsign, f.Stage)
		}
		if firstSTA.IsZero() || f.STA.Before(firstSTA) {
			firstSTA = f.STA
		}
	}
	t.Logf("%d arrivals on their way in at the start, the first due %s", midway, firstSTA.Format("15:04"))
	if midway < 2 || early < 2 {
		t.Fatalf("%d arrivals on their way in at the start, %d in its first 3 minutes, want a few", midway, early)
	}
	if firstSTA.After(start.Add(12 * time.Minute)) {
		t.Errorf("first arrival due %s, want within minutes of the start", firstSTA.Format("15:04"))
	}
	for _, f := range startArrivals(t, -1) {
		if f.Stage == "midway" || f.STA.Before(start.Add(o.ArrivalLead-o.ArrivalLate)) {
			t.Errorf("StartAirborneMin negative: %s spawned due %s, stage %q", f.Callsign, f.STA.Format("15:04"), f.Stage)
		}
	}
}
