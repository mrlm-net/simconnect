package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestNightAt: the night and its morning by local time: both sides of its
// edges, the clocks going back in it, another time zone, one whose hours do
// not begin on the UTC hour, and a day with no morning (#1303).
func TestNightAt(t *testing.T) {
	waves := traffic.DefaultScheduleConfig().Waves
	at := func(icao string, p airport.LatLon) func(time.Time) int {
		return func(u time.Time) int { return traffic.LocalHour(icao, p, u) }
	}
	lkpr := at("LKPR", airport.LatLon{Lat: 50.1, Lon: 14.26})
	kjfk := at("KJFK", airport.LatLon{Lat: 40.64, Lon: -73.78})
	ist := time.FixedZone("+05:30", 5*3600+1800)
	india := func(u time.Time) int { return u.In(ist).Hour() }
	utc := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, time.UTC) }
	for _, c := range []struct {
		name    string
		hour    func(time.Time) int
		t, want time.Time // want zero: not night
	}{
		{"LKPR 22:59 CEST: not yet", lkpr, utc(2026, 10, 20, 20, 59), time.Time{}},
		{"LKPR 23:00 CEST: the night begins", lkpr, utc(2026, 10, 20, 21, 0), utc(2026, 10, 21, 3, 0)},
		{"LKPR 03:30 CEST", lkpr, utc(2026, 10, 20, 1, 30), utc(2026, 10, 20, 3, 0)},
		{"LKPR 04:59 CEST: the last of it", lkpr, utc(2026, 10, 20, 2, 59), utc(2026, 10, 20, 3, 0)},
		{"LKPR 05:00 CEST: morning", lkpr, utc(2026, 10, 20, 3, 0), time.Time{}},
		{"LKPR the clocks go back: an hour more", lkpr, utc(2026, 10, 25, 0, 30), utc(2026, 10, 25, 4, 0)},
		{"LKPR winter: 05:00 CET", lkpr, utc(2026, 12, 1, 1, 0), utc(2026, 12, 1, 4, 0)},
		{"KJFK 02:00 EST", kjfk, utc(2026, 12, 1, 7, 0), utc(2026, 12, 1, 10, 0)},
		{"+05:30, 01:30: morning at 05:00, not 05:30", india, utc(2026, 10, 20, 20, 0), utc(2026, 10, 20, 23, 30)},
	} {
		got, ok := nightAt(c.t, defaultNightWave, waves, c.hour)
		if ok != !c.want.IsZero() || !got.Equal(c.want) {
			t.Errorf("%s: %v %v, want %v", c.name, got, ok, c.want)
		}
	}
	if _, ok := nightAt(utc(2026, 10, 20, 1, 0), 2, waves, lkpr); ok {
		t.Error("a night all day has no morning")
	}
	var quiet [24]float64
	if _, ok := nightAt(utc(2026, 10, 20, 1, 0), 0.1, quiet, lkpr); ok {
		t.Error("no movements at all: no morning")
	}
	if _, ok := nightAt(utc(2026, 10, 20, 1, 30), 0, waves, lkpr); ok {
		t.Error("threshold 0: no hour of the default waves is night")
	}
}

func f64(v float64) *float64 { return &v }

// TestTuningValidate: values the triggers cannot work with are refused,
// each said; the combinations on the defaults of those left out.
func TestTuningValidate(t *testing.T) {
	base := baseTiming(ScheduleTiming{})
	over := [24]float64{5: 2}
	no := -5.0
	for _, c := range []struct {
		name string
		t    ScheduleTuning
		want string // "" fine
	}{
		{"nothing", ScheduleTuning{}, ""},
		{"a shorter night", ScheduleTuning{NightWave: f64(0.03), MorningWaveMin: f64(60)}, ""},
		{"no turnarounds", ScheduleTuning{MinTurnMin: &no}, ""},
		{"wave over the peak", ScheduleTuning{Waves: &over}, "waves"},
		{"threshold", ScheduleTuning{NightWave: f64(1.5)}, "nightWave"},
		{"no morning wave", ScheduleTuning{MorningWaveMin: f64(0)}, "morningWaveMin"},
		{"cargo", ScheduleTuning{CargoShare: f64(-1)}, "cargoShare"},
		{"turn window, MaxTurn's default", ScheduleTuning{MinTurnMin: f64(200)}, "minTurn"},
		{"lead past the horizon's default", ScheduleTuning{ArrivalLeadMin: f64(180)}, "arrivalLead"},
		{"horizon under the lead's default", ScheduleTuning{HorizonMin: f64(20), MorningWaveMin: f64(0)}, "morningWaveMin"},
	} {
		err := c.t.validate(base)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: %v, want it about %s", c.name, err, c.want)
		}
	}
	err := ScheduleTuning{HorizonMin: f64(20), MorningWaveMin: f64(0)}.validate(base)
	if err == nil || !strings.Contains(err.Error(), "arrivalLead") {
		t.Errorf("every problem at once: %v", err)
	}
}

// TestTuningStore: the local override saved and read back, only what was
// set; a file the triggers cannot work with is left out whole.
func TestTuningStore(t *testing.T) {
	base := baseTiming(ScheduleTiming{})
	file := filepath.Join(t.TempDir(), "sub", "schedule-tuning.json")
	s := loadTuning(file, base)
	if got := s.get(); got.NightWave != nil {
		t.Errorf("no file: %+v", got)
	}
	if err := s.set(ScheduleTuning{NightWave: f64(0.03)}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil || len(raw) != 1 {
		t.Errorf("saved %s: only what was set (%v)", b, err)
	}
	if got := loadTuning(file, base).get(); got.NightWave == nil || *got.NightWave != 0.03 {
		t.Errorf("read back %+v", got)
	}
	if err := os.WriteFile(file, []byte(`{"nightWave":0.03,"cargoShare":-1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadTuning(file, base).get(); got.NightWave != nil {
		t.Errorf("a bad file: %+v, want it left out", got)
	}
	var nilStore *tuningStore
	if nilStore.get().NightWave != nil {
		t.Error("a nil store")
	}
}

// TestTuned: a value left out stays the default, local wins per value.
func TestTuned(t *testing.T) {
	base := traffic.DefaultScheduleConfig()
	cargo := func(c traffic.ScheduleConfig) (n int, w float64) {
		for _, a := range c.Airlines {
			if a.Cargo {
				n, w = n+1, w+a.Weight
			}
		}
		return n, w
	}
	nb, wb := cargo(base)
	if nb == 0 {
		t.Fatal("no cargo airline in the defaults")
	}
	d := ScheduleTuning{}.tuned(base)
	if d.nightWave != defaultNightWave || !d.overnightRemote || d.cfg.Waves != base.Waves {
		t.Errorf("defaults %+v", d)
	}
	if n, _ := cargo(ScheduleTuning{CargoShare: f64(0)}.tuned(base).cfg); n != 0 {
		t.Errorf("share 0: %d cargo airlines", n)
	}
	if n, w := cargo(ScheduleTuning{CargoShare: f64(2)}.tuned(base).cfg); n != nb || w != 2*wb {
		t.Errorf("share 2: %d airlines weight %v, base %d %v", n, w, nb, wb)
	}
	if n, w := cargo(base); n != nb || w != wb {
		t.Error("the base changed")
	}
	off := false
	if u := (ScheduleTuning{OvernightRemote: &off}).tuned(base); u.overnightRemote || u.nightWave != defaultNightWave {
		t.Errorf("one value: %+v", u)
	}
	e := effectiveTuning(ScheduleTuning{NightWave: f64(0.03)}, baseTiming(ScheduleTiming{}))
	if *e.NightWave != 0.03 || *e.MorningWaveMin != 120 || *e.MinTurnMin != 40 || *e.MaxTurnMin != 180 || *e.DepartureLeadMin != 10 || !*e.SelfAnnounce || *e.CargoShare != 1 {
		t.Errorf("effective %+v", e)
	}
}
