package traffic

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// startGroundRun plays the generated LKPR schedule (density 3) through a
// manager for four hours from 10:00, each aircraft boarding, taxiing and
// landing on time, and returns how many of ours are on the ground every
// minute: departures from their spawn until 8 minutes after STD, arrivals
// from STA until done (turned into their next departure or removed).
func startGroundRun(t *testing.T, startStands int) []int {
	return startGroundRunAt(t, startStands, time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC))
}

// startGroundRunAt is startGroundRun from start.
func startGroundRunAt(t *testing.T, startStands int, start time.Time) []int {
	t.Helper()
	cfg := DefaultScheduleConfig()
	src := func(from, to time.Time, airports []string) []Flight {
		return Schedule(cfg, ScheduleOptions{Focus: airports, Density: 3, Seed: 7 ^ uint64(from.Unix()/3600)}, from, to)
	}
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{Source: src, MaxAircraft: 40, MaxPerAirport: 40, EnrouteLead: -1, StartStands: startStands, Night: lkprNight}, "LKPR")
	var counts []int
	for now := start; now.Before(start.Add(4 * time.Hour)); now = now.Add(10 * time.Second) {
		m.Tick(now)
		ground := 0
		for _, f := range m.Flights() {
			switch {
			case f.Status == FlightSpawning && f.Departure():
				m.Update(f.Callsign, FlightBoarding, now)
			case f.Status == FlightSpawning:
				m.Update(f.Callsign, FlightApproaching, now)
			case f.Status == FlightBoarding && !now.Before(f.STD):
				m.Update(f.Callsign, FlightTaxiing, now)
			case f.Status == FlightTaxiing && !now.Before(f.STD.Add(8*time.Minute)):
				m.Update(f.Callsign, FlightDeparted, now)
			case f.Status == FlightApproaching && !now.Before(f.STA):
				m.Update(f.Callsign, FlightLanded, now)
			case f.Status == FlightLanded && !now.Before(f.STA.Add(5*time.Minute)):
				m.Update(f.Callsign, FlightParked, now)
			}
			if f.Status == FlightBoarding || f.Status == FlightTaxiing || f.Status == FlightLanded || f.Status == FlightParked {
				ground++
			}
		}
		if now.Sub(start)%time.Minute == 0 {
			counts = append(counts, ground)
		}
	}
	return counts
}

// Traffic starting at an airport finds its stands taken at once
// (StartStands): within 5 minutes as many on the ground as through the
// hours after, which stay level as arrivals turn into departures. Without,
// the ground started empty and filled one departure a minute (live LKPR
// 2026-11-11: half an hour to look busy).
func TestManagerStartStandsGroundLevel(t *testing.T) {
	with, without := startGroundRun(t, 0), startGroundRun(t, -1)
	line := func(c []int) string {
		var b strings.Builder
		for i := 0; i < len(c); i += 15 {
			fmt.Fprintf(&b, " %d", c[i])
		}
		return b.String()
	}
	t.Logf("on the ground every 15 min, start stands:%s", line(with))
	t.Logf("on the ground every 15 min, none:        %s", line(without))
	steady := append([]int(nil), with[60:]...)
	sort.Ints(steady)
	median := steady[len(steady)/2]
	if median < 6 {
		t.Fatalf("hardly any traffic on the ground: median %d", median)
	}
	if with[5] < median*6/10 {
		t.Errorf("5 minutes in, %d on the ground, against %d in the hours after", with[5], median)
	}
	if without[5] >= with[5] {
		t.Errorf("start stands put no more on the ground at 5 minutes: %d against %d", with[5], without[5])
	}
	for i := 5; i < len(with); i++ {
		if with[i] < median/2 || with[i] > median*2 {
			t.Errorf("minute %d: %d on the ground, the level %d", i, with[i], median)
			break
		}
	}
}

// Traffic starting in the evening, the last wave gone, still finds an
// apron with aircraft on it: those due next morning stand there overnight
// (NightStop), dark until their crew comes (live LKPR 22:15: no aircraft
// on the remote stands, the next departure at 05:10).
func TestManagerStartStandsEvening(t *testing.T) {
	start := time.Date(2026, 11, 11, 21, 15, 0, 0, time.UTC) // 22:15 at LKPR
	with := startGroundRunAt(t, 0, start)
	t.Logf("on the ground at 5, 30, 60 min: %d %d %d", with[5], with[30], with[60])
	if with[5] < 15 {
		t.Errorf("5 minutes in, %d on the ground at night, want the stands filled", with[5])
	}
	if with[60] < 15 {
		t.Errorf("an hour in, %d on the ground: the overnight ones stay", with[60])
	}
}

// lkprNight: LKPR's night (UTC+1) from 22:00 to its 05:00 morning, as the
// World's nightAt gives it for the generated schedule's waves.
func lkprNight(_ string, t time.Time) (time.Time, bool) {
	l := t.Add(time.Hour)
	if h := l.Hour(); h >= 5 && h < 22 {
		return time.Time{}, false
	}
	m := time.Date(l.Year(), l.Month(), l.Day(), 5, 0, 0, 0, time.UTC)
	if !m.After(l) {
		m = m.Add(24 * time.Hour)
	}
	return m.Add(-time.Hour), true
}
