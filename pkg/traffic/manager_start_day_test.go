package traffic

import (
	"testing"
	"time"
)

// TestManagerStartStandsNoOvernightInTheDay: traffic beginning in the day
// with few departures due within StartWindow puts none of the next ones,
// due 2-3 h out, on their stands as night stops; in the evening those due
// from the next morning still stand there overnight, those of the evening
// not (#1677).
func TestManagerStartStandsNoOvernightInTheDay(t *testing.T) {
	run := func(start time.Time, due ...time.Duration) map[string]ManagedFlight {
		sp := &fakeSpawner{}
		m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, MaxAircraft: 40, MaxPerAirport: 40, EnrouteLead: -1, Night: lkprNight}, "LKPR")
		var fs []Flight
		for i, d := range due {
			fs = append(fs, flight("CSA"+string(rune('A'+i))+"00", "LKPR", "EDDF", start.Add(d), time.Hour))
		}
		m.Add(fs)
		m.Tick(start)
		out := map[string]ManagedFlight{}
		for _, f := range m.Flights() {
			out[f.Callsign] = f
		}
		return out
	}
	// 10:03 local (09:03Z): one due within the hour, five 2.5-3 h out.
	day := time.Date(2026, 11, 22, 9, 3, 0, 0, time.UTC)
	got := run(day, 40*time.Minute, 150*time.Minute, 160*time.Minute, 170*time.Minute, 175*time.Minute, 180*time.Minute)
	if !got["CSAA00"].StartStand {
		t.Error("CSAA00, due within StartWindow, is no start stand")
	}
	for cs, f := range got {
		if f.NightStop {
			t.Errorf("%s due %s, at 10:03 local: a night stop", cs, f.STD.Sub(day))
		}
	}
	// 20:15 local (19:15Z): the night begins at 22:00, its morning 05:00.
	eve := time.Date(2026, 11, 22, 19, 15, 0, 0, time.UTC)
	got = run(eve, 150*time.Minute, 9*time.Hour+30*time.Minute)
	if got["CSAA00"].NightStop {
		t.Error("CSAA00, due 22:45 local, stands there from 20:15 as a night stop")
	}
	if !got["CSAB00"].NightStop {
		t.Error("CSAB00, due 05:45 local next morning, is no night stop")
	}
}
