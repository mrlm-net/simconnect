package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The LKPR evening peak at density 3, aircraft flying their lifecycle as
// the World reports it (Progress on every mile closer): the arrivals come
// as the timetable has them. Live, an airborne arrival's predicted landing
// slid along with each Progress and pushed every arrival behind it on: one
// landing in 15 minutes at 18:00.
func TestManagerPeakArrivalsWithProgress(t *testing.T) {
	g := lkprGraph(t)
	layouts := map[string]*airport.Layout{"LKPR": g.Layout}
	sp := &fakeSpawner{}
	type life struct {
		f    ManagedFlight
		at   time.Time
		step int
	}
	lives := map[string]*life{}
	var now time.Time
	m := NewTrafficManager(sp, ManagerOptions{MaxAircraft: 40, MaxPerAirport: 40,
		Source: func(from, to time.Time, airports []string) []Flight {
			return Schedule(DefaultScheduleConfig(), ScheduleOptions{Focus: airports, Density: 3, Layouts: layouts, Seed: 7 ^ uint64(from.Unix()/3600)}, from, to)
		}}, "LKPR")
	sp.onSpawn = func(f ManagedFlight) { lives[f.Key()] = &life{f: f, at: now} }
	start := time.Date(2026, 10, 24, 16, 38, 0, 0, time.UTC)
	o := m.Options()
	arrivals := 0
	seen := 0
	for s := 0; s <= 90*60; s += 5 {
		now = start.Add(time.Duration(s) * time.Second)
		m.Tick(now)
		for ; seen < len(sp.spawned); seen++ {
			if sp.spawned[seen].Arrival() {
				arrivals++
			}
		}
		for _, l := range lives {
			el := now.Sub(l.at)
			cs := l.f.Callsign
			if l.f.Arrival() {
				switch {
				case l.step == 0 && l.f.Stage == "enroute":
					m.Update(cs, FlightEnroute, now)
					l.step = -1
				case l.step == -1 && el > o.EnrouteLead:
					m.Update(cs, FlightApproaching, now)
					l.step, l.at = 1, now
				case l.step == 0:
					m.Update(cs, FlightApproaching, now)
					l.step = 1
				case l.step == 1 && s%60 == 0 && el < o.ArrivalLead-LandingBeforeSTA:
					m.Progress(cs, now)
				case l.step == 1 && el > o.ArrivalLead-LandingBeforeSTA:
					m.Update(cs, FlightLanded, now)
					l.step = 2
				case l.step == 2 && el > o.ArrivalLead:
					m.Update(cs, FlightParked, now)
					l.step = 3
				}
			} else if l.f.Departure() {
				switch {
				case l.step == 0:
					m.Update(cs, FlightBoarding, now)
					l.step = 1
				case l.step == 1 && now.After(l.f.STD):
					m.Update(cs, FlightTaxiing, now)
					l.step = 2
				case l.step == 2 && now.After(l.f.STD.Add(12*time.Minute)):
					m.Update(cs, FlightDeparted, now)
					l.step = 3
				}
			}
		}
	}
	due := 0
	for _, f := range m.Flights() {
		if f.Arrival() && !f.STA.Before(start) && f.STA.Before(start.Add(90*time.Minute)) {
			due++
		}
	}
	t.Logf("%d arrivals spawned in 90 min, %d due", arrivals, due)
	if arrivals*10 < due*8 {
		t.Fatalf("%d arrivals spawned in 90 minutes of the peak, %d due: want at least 80%%", arrivals, due)
	}
}
