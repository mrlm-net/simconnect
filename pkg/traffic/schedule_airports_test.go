package traffic

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// launchAirports are the airports a game is started at in VECTOR.PRO's menu
// (#1610). The list is not kept in this repository: these are the ten the
// issue names.
var launchAirports = []string{"LKPR", "LROP", "EDDM", "KJFK", "EGLL", "LFPG", "LOWI", "TNCM", "TFFJ", "KLAX"}

// scheduleLayout is icao's layout from the simconnect-data clone
// ($SIMCONNECT_DATA, its own folder or fields/<kind>/); nil without it.
func scheduleLayout(t *testing.T, icao string) *airport.Layout {
	t.Helper()
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "simconnect-data")
	}
	paths := []string{filepath.Join(dir, icao, "layout.json")}
	if m, _ := filepath.Glob(filepath.Join(dir, "fields", "*", icao, "layout.json")); len(m) > 0 {
		paths = append(paths, m...)
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var raw airport.RawAirport
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		l, err := airport.BuildLayout(raw)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		return l
	}
	return nil
}

// TestDefaultScheduleServesEveryAirport: every airport the default schedule
// lists, and every airport a game is launched at, gets arrivals and
// departures from DrawFlight, as the game's spawner asks for them, with the
// config alone and with the airport's layout (#1610: TNCM, TFFJ and KLAX got
// "no airline flies to"). Each flight's type fits the runway at both ends.
func TestDefaultScheduleServesEveryAirport(t *testing.T) {
	cfg := DefaultScheduleConfig()
	runway := map[string]float64{}
	asked := append([]string(nil), launchAirports...)
	for _, a := range cfg.Airports {
		runway[a.ICAO] = a.RunwayM
		asked = append(asked, a.ICAO)
	}
	at := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	const seeds = 40
	// A launch airport gets a flight from every draw, as the game asks for
	// one at a time; any other airport of the config from most of them (a
	// draw can fail when every airline it tries flies too far for its
	// fleet, EBBR with its layout 36 of 40 before #1610).
	want := map[string]int{}
	for _, icao := range asked {
		want[icao] = seeds * 3 / 4
	}
	for _, icao := range launchAirports {
		want[icao] = seeds
	}
	seen := map[string]bool{}
	for _, icao := range asked {
		if seen[icao] {
			continue
		}
		seen[icao] = true
		layouts := []map[string]*airport.Layout{nil}
		if l := scheduleLayout(t, icao); l != nil {
			layouts = append(layouts, map[string]*airport.Layout{icao: l})
		}
		for _, lm := range layouts {
			field := runway[icao]
			if l := lm[icao]; l != nil {
				for _, r := range l.Runways {
					field = math.Max(field, r.Length)
				}
			}
			for _, arrival := range []bool{false, true} {
				got := 0
				for seed := uint64(1); seed <= seeds; seed++ {
					f, ok := DrawFlight(cfg, ScheduleOptions{Seed: seed, Layouts: lm}, icao, at, arrival)
					if !ok {
						continue
					}
					got++
					other := f.Destination
					if arrival {
						other = f.Origin
					}
					need := cfg.limits(f.Type).RunwayM
					if field > 0 && need > field {
						t.Errorf("%s: %s %s needs %.0f m, the runway is %.0f m", icao, f.Callsign, f.Type, need, field)
					}
					if r := runway[other]; r > 0 && need > r {
						t.Errorf("%s: %s %s to or from %s needs %.0f m, its runway is %.0f m", icao, f.Callsign, f.Type, other, need, r)
					}
				}
				if got < want[icao] {
					t.Errorf("%s (layout %v), arrival %v: %d of %d draws give a flight, want %d", icao, lm != nil, arrival, got, seeds, want[icao])
				}
			}
		}
	}
}

// TestTFFJFlightsFitItsRunway: St Barthelemy's 642 m runway (#1610) gets
// a type the engine knows, so a model of it can be found, and whose
// take-off distance fits the runway of the layout when there is one.
func TestTFFJFlightsFitItsRunway(t *testing.T) {
	cfg := DefaultScheduleConfig()
	field := 0.0
	for _, a := range cfg.Airports {
		if a.ICAO == "TFFJ" {
			field = a.RunwayM
		}
	}
	var lm map[string]*airport.Layout
	if l := scheduleLayout(t, "TFFJ"); l != nil {
		lm = map[string]*airport.Layout{"TFFJ": l}
		field = 0
		for _, r := range l.Runways {
			field = math.Max(field, r.Length)
		}
		t.Logf("TFFJ layout: longest runway %.0f m", field)
	} else {
		t.Log("no TFFJ layout (SIMCONNECT_DATA): the config's runway")
	}
	if field <= 0 || field > 1000 {
		t.Fatalf("TFFJ runway %.0f m: not the short field", field)
	}
	at := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	types := map[string]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		for _, arrival := range []bool{false, true} {
			f, ok := DrawFlight(cfg, ScheduleOptions{Seed: seed, Layouts: lm}, "TFFJ", at, arrival)
			if !ok {
				t.Fatalf("seed %d arrival %v: no flight", seed, arrival)
			}
			types[f.Type]++
			p := ProfileFor(f.Type)
			if p.Type != f.Type {
				t.Errorf("%s %s: not a type the engine knows (profile %q), no model can be matched", f.Callsign, f.Type, p.Type)
			}
			if p.TakeoffDistanceM > field {
				t.Errorf("%s %s: take-off distance %.0f m, the runway is %.0f m", f.Callsign, f.Type, p.TakeoffDistanceM, field)
			}
			if cfg.limits(f.Type).RunwayM > field {
				t.Errorf("%s %s: schedule runway %.0f m, the runway is %.0f m", f.Callsign, f.Type, cfg.limits(f.Type).RunwayM, field)
			}
		}
	}
	t.Logf("TFFJ types: %v", types)
}
