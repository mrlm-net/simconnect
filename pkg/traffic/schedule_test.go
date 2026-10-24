package traffic

import (
	"reflect"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

var scheduleDay = time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)

func daySchedule(opts ScheduleOptions) []Flight {
	return Schedule(DefaultScheduleConfig(), opts, scheduleDay, scheduleDay.Add(24*time.Hour))
}

func TestScheduleDeterministic(t *testing.T) {
	opts := ScheduleOptions{Focus: []string{"LKPR"}, Seed: 7}
	a, b := daySchedule(opts), daySchedule(opts)
	if len(a) == 0 || !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, different schedules (%d vs %d flights)", len(a), len(b))
	}
	opts.Seed = 8
	if reflect.DeepEqual(a, daySchedule(opts)) {
		t.Fatal("different seeds, same schedule")
	}
}

func TestScheduleWavesAndDensity(t *testing.T) {
	flights := daySchedule(ScheduleOptions{Focus: []string{"LKPR"}, Seed: 1})
	byHour := map[int]int{}
	for _, f := range flights {
		// LKPR is at 14°E: local solar time is about UTC+1.
		byHour[focusTime(f, []string{"LKPR"}).Add(time.Hour).Hour()]++
	}
	night := byHour[1] + byHour[2] + byHour[3]
	peak := byHour[7] + byHour[18]
	if night >= peak/4 {
		t.Errorf("night %d movements, peak %d: waves not followed", night, peak)
	}
	// LKPR is a major (size 2): 12 movements in the peak hour; the day sums
	// the waves (about 12.5 peak hours).
	if n := len(flights); n < 120 || n > 180 {
		t.Errorf("%d flights a day at LKPR, want about 150", n)
	}
	double := daySchedule(ScheduleOptions{Focus: []string{"LKPR"}, Seed: 1, Density: 2})
	if r := float64(len(double)) / float64(len(flights)); r < 1.7 || r > 2.3 {
		t.Errorf("density 2 gives %.2f× the flights", r)
	}
}

func TestScheduleFlightsFit(t *testing.T) {
	cfg := DefaultScheduleConfig()
	airports := map[string]ScheduleAirport{}
	for _, a := range cfg.Airports {
		airports[a.ICAO] = a
	}
	flights := daySchedule(ScheduleOptions{Focus: []string{"LKPR", "LKTB"}, Seed: 3})
	seen := map[string]bool{}
	deps, arrs := 0, 0
	for _, f := range flights {
		if seen[f.Callsign] {
			t.Errorf("call sign %s twice", f.Callsign)
		}
		seen[f.Callsign] = true
		o, d := airports[f.Origin], airports[f.Destination]
		if o.ICAO == "" || d.ICAO == "" || o.ICAO == d.ICAO {
			t.Fatalf("%s %s→%s: bad ends", f.Callsign, f.Origin, f.Destination)
		}
		switch "LKPR" {
		case f.Origin:
			deps++
		case f.Destination:
			arrs++
		}
		lim := cfg.Types[f.Type]
		dist := calc.HaversineNM(o.Position.Lat, o.Position.Lon, d.Position.Lat, d.Position.Lon)
		if dist < lim.MinNM || dist > lim.MaxNM {
			t.Errorf("%s %s %s→%s: %.0f NM outside %v", f.Callsign, f.Type, f.Origin, f.Destination, dist, lim)
		}
		if o.RunwayM < lim.RunwayM || d.RunwayM < lim.RunwayM {
			t.Errorf("%s %s %s→%s: runway too short", f.Callsign, f.Type, f.Origin, f.Destination)
		}
		if !f.STA.After(f.STD) || f.STA.Sub(f.STD) > 16*time.Hour {
			t.Errorf("%s: STD %v STA %v", f.Callsign, f.STD, f.STA)
		}
	}
	if deps == 0 || arrs == 0 || deps > 2*arrs || arrs > 2*deps {
		t.Errorf("LKPR: %d departures, %d arrivals", deps, arrs)
	}
}

func TestScheduleHomeCarriers(t *testing.T) {
	share := func(flights []Flight, airline string) float64 {
		n := 0
		for _, f := range flights {
			if f.Airline == airline {
				n++
			}
		}
		return float64(n) / float64(len(flights))
	}
	lkpr := daySchedule(ScheduleOptions{Focus: []string{"LKPR"}, Seed: 5})
	if s := share(lkpr, "TVS") + share(lkpr, "CSA"); s < 0.3 {
		t.Errorf("home carriers %.0f%% of LKPR", s*100)
	}
	// A stand naming an airline makes it a home carrier there.
	//
	// A REAL STAND: type and radius are what make one. This fixture was a bare
	// `Parking{}` — type 0, NONE — which `standShares` skips along with fuel pumps
	// and vehicle bays, because an airline named on one of those does not fly from
	// it. The old flag counted every parking row, so the fixture passed while
	// saying something the engine no longer means.
	layout := &airport.Layout{ICAO: "LKPR", Latitude: 50.1008, Longitude: 14.26,
		Runways: []airport.Runway{{Length: 3715}},
		Parking: []airport.Parking{{Type: 9, Radius: 26, Airlines: []string{"KLM"}}}}
	withStands := daySchedule(ScheduleOptions{Focus: []string{"LKPR"}, Seed: 5, Layouts: map[string]*airport.Layout{"LKPR": layout}})
	if share(withStands, "KLM") < 2*share(lkpr, "KLM") {
		t.Errorf("KLM %.0f%% with its stand, %.0f%% without", share(withStands, "KLM")*100, share(lkpr, "KLM")*100)
	}
}

func TestScheduleUnknownFocus(t *testing.T) {
	if f := daySchedule(ScheduleOptions{Focus: []string{"ZZZZ"}}); len(f) != 0 {
		t.Fatalf("%d flights at an unknown airport", len(f))
	}
	// A loaded layout makes it known: a regional airport.
	layout := &airport.Layout{ICAO: "LKHK", Latitude: 50.25, Longitude: 15.84, Runways: []airport.Runway{{Length: 2500}}}
	f := daySchedule(ScheduleOptions{Focus: []string{"LKHK"}, Layouts: map[string]*airport.Layout{"LKHK": layout}})
	if len(f) == 0 || len(f) > 90 {
		t.Fatalf("%d flights at a regional airport", len(f))
	}
}

// Overflights cross the area along their great circle (#468): Dublin–Seoul
// (over Norway) does not cross 100 NM around LKPR — a straight line in
// latitude and longitude did; Paris–Warsaw (76 NM from Prague at 51.3N
// 13.8E) and Frankfurt–Warsaw do.
func TestCrossingGreatCircle(t *testing.T) {
	lkpr := airport.LatLon{Lat: 50.1008, Lon: 14.2600}
	pos := map[string]airport.LatLon{}
	for _, a := range DefaultScheduleConfig().Airports {
		pos[a.ICAO] = a.Position
	}
	for _, c := range []struct {
		from, to string
		want     bool
	}{{"EIDW", "RKSI", false}, {"LFPG", "EPWA", true}, {"EDDF", "EPWA", true}} {
		a, b := pos[c.from], pos[c.to]
		dist := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
		if _, _, ok := crossing(a, b, dist, lkpr, 100); ok != c.want {
			t.Errorf("%s–%s crosses 100 NM around LKPR: %v, want %v", c.from, c.to, ok, c.want)
		}
	}
}

// Asked an hour at a time, as the manager does, overflights are other
// flights each hour, not the first hour's again (dropped as repeats: no
// overflight for hours, live at LKPR).
func TestOverflightsDifferEachHour(t *testing.T) {
	cfg := DefaultScheduleConfig()
	o := OverflightOptions{Centre: airport.LatLon{Lat: 50.1, Lon: 14.26}, RadiusNM: 80, Density: 3, Seed: 7, Exclude: []string{"LKPR"}}
	h0 := time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC)
	seen := map[string]bool{}
	repeats, total := 0, 0
	for h := 0; h < 4; h++ {
		for _, f := range Overflights(cfg, o, h0.Add(time.Duration(h)*time.Hour), h0.Add(time.Duration(h+1)*time.Hour)) {
			total++
			if seen[f.Callsign] {
				repeats++
			}
			seen[f.Callsign] = true
		}
	}
	t.Logf("%d overflights in 4 hours, %d repeated call signs", total, repeats)
	if total == 0 || repeats > total/4 {
		t.Errorf("%d of %d overflights repeat an earlier hour's call sign", repeats, total)
	}
}
