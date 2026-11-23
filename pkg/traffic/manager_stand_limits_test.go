package traffic

import (
	"testing"
	"time"
)

// TestManagerSessionStandLimits: a session's StartStands and NightStops
// (#1677) size the departures put on their stands at the start and through
// the night; 0 changes nothing, negative puts none, and the session's
// NightStops wins over a timing set after it.
func TestManagerSessionStandLimits(t *testing.T) {
	type setup func(m *TrafficManager)
	count := func(start time.Time, due []time.Duration, opts ManagerOptions, set setup) (startStands, nightStops int) {
		opts.MinTurn, opts.MaxAircraft, opts.MaxPerAirport, opts.EnrouteLead, opts.Night = -1, 40, 40, -1, lkprNight
		m := NewTrafficManager(&fakeSpawner{}, opts, "LKPR")
		set(m)
		var fs []Flight
		for i, d := range due {
			fs = append(fs, flight("CSA"+string(rune('A'+i))+"00", "LKPR", "EDDF", start.Add(d), time.Hour))
		}
		m.Add(fs)
		m.Tick(start)
		for _, f := range m.Flights() {
			if f.StartStand {
				startStands++
			}
			if f.NightStop {
				nightStops++
			}
		}
		return startStands, nightStops
	}
	none := func(*TrafficManager) {}

	// In the day, six due within StartWindow.
	day := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	dayDue := []time.Duration{20 * time.Minute, 30 * time.Minute, 40 * time.Minute, 50 * time.Minute, 60 * time.Minute, 70 * time.Minute}
	base, _ := count(day, dayDue, ManagerOptions{}, none)
	if zero, _ := count(day, dayDue, ManagerOptions{}, func(m *TrafficManager) { m.SetStartStands(0); m.SetNightStops(0) }); zero != base {
		t.Errorf("SetStartStands(0): %d start stands, want %d as without", zero, base)
	}
	if base != len(dayDue) {
		t.Fatalf("fixture: %d start stands of %d due", base, len(dayDue))
	}
	if got, _ := count(day, dayDue, ManagerOptions{}, func(m *TrafficManager) { m.SetStartStands(2) }); got != 2 {
		t.Errorf("SetStartStands(2): %d start stands", got)
	}
	if got, _ := count(day, dayDue, ManagerOptions{StartStands: 3}, func(m *TrafficManager) { m.SetStartStands(-1) }); got != 0 {
		t.Errorf("SetStartStands(-1): %d start stands", got)
	}
	if got, _ := count(day, dayDue, ManagerOptions{StartStands: 3}, func(m *TrafficManager) { m.SetStartStands(4); m.SetStartStands(0) }); got != 3 {
		t.Errorf("SetStartStands back to 0: %d start stands, want the options' 3", got)
	}

	// In the night (22:15 local), eight of the morning wave; start stands
	// off so only the night stops count.
	night := time.Date(2026, 11, 22, 21, 15, 0, 0, time.UTC)
	var morning []time.Duration
	for i := range 8 {
		morning = append(morning, 7*time.Hour+time.Duration(i)*10*time.Minute) // 05:15 local on
	}
	noStart := func(m *TrafficManager) { m.SetStartStands(-1) }
	timing := func(n int) setup {
		return func(m *TrafficManager) {
			ti := m.Options().Timing()
			ti.NightStops = n
			m.SetTiming(ti)
		}
	}
	both := func(a ...setup) setup {
		return func(m *TrafficManager) {
			for _, f := range a {
				f(m)
			}
		}
	}
	if _, got := count(night, morning, ManagerOptions{NightStops: 5}, noStart); got != 5 {
		t.Fatalf("fixture: %d night stops, want the options' 5", got)
	}
	if _, got := count(night, morning, ManagerOptions{NightStops: 5}, both(noStart, func(m *TrafficManager) { m.SetNightStops(0) })); got != 5 {
		t.Errorf("SetNightStops(0): %d night stops, want 5 as without", got)
	}
	if _, got := count(night, morning, ManagerOptions{NightStops: 5}, both(noStart, func(m *TrafficManager) { m.SetNightStops(2) }, timing(7))); got != 2 {
		t.Errorf("SetNightStops(2), then a timing of 7: %d night stops, want 2", got)
	}
	if _, got := count(night, morning, ManagerOptions{NightStops: 5}, both(noStart, func(m *TrafficManager) { m.SetNightStops(-1) })); got != 0 {
		t.Errorf("SetNightStops(-1): %d night stops", got)
	}
	if s, n := (&TrafficManager{}).StandLimits(); s != 0 || n != 0 {
		t.Errorf("a new manager's session limits %d %d, want 0 0", s, n)
	}
}
