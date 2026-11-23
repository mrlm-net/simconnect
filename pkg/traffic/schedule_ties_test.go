package traffic

import (
	"reflect"
	"testing"
	"time"
)

// TestScheduleTieServesABuiltAirport: an airport built at St Maarten with
// a code of its own (XNCM) is served by the "anywhere" airlines alone; tied
// to TNCM (AirportTie.Like) it schedules the same airlines as TNCM, WIA's
// base among them, and never flies to TNCM; tied to region TN its region's
// airlines fly there. A tie left zero changes nothing (#1678).
func TestScheduleTieServesABuiltAirport(t *testing.T) {
	cfg := DefaultScheduleConfig()
	var tncm ScheduleAirport
	for _, a := range cfg.Airports {
		if a.ICAO == "TNCM" {
			tncm = a
		}
	}
	if tncm.ICAO == "" {
		t.Fatal("TNCM is not in the default schedule")
	}
	x := tncm
	x.ICAO = "XNCM"
	cfgX := cfg
	cfgX.Airports = append(append([]ScheduleAirport(nil), cfg.Airports...), x)
	from := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	airlines := func(fs []Flight) map[string]int {
		out := map[string]int{}
		for _, f := range fs {
			out[f.Airline]++
		}
		return out
	}
	real := Schedule(cfg, ScheduleOptions{Focus: []string{"TNCM"}, Seed: 1}, from, to)
	if len(real) == 0 {
		t.Fatal("no flights at TNCM")
	}

	bare := Schedule(cfgX, ScheduleOptions{Focus: []string{"XNCM"}, Seed: 1}, from, to)
	if reflect.DeepEqual(airlines(bare), airlines(real)) {
		t.Fatalf("fixture: XNCM untied already flies TNCM's airlines %v", airlines(real))
	}

	tied := Schedule(cfgX, ScheduleOptions{Focus: []string{"XNCM"}, Seed: 1, Ties: map[string]AirportTie{"XNCM": {Like: "TNCM"}}}, from, to)
	if got, want := airlines(tied), airlines(real); !reflect.DeepEqual(got, want) {
		t.Errorf("XNCM like TNCM: %v, want TNCM's %v", got, want)
	}
	if airlines(tied)["WIA"] == 0 {
		t.Error("XNCM like TNCM: no flight of WIA, based at TNCM")
	}
	for _, f := range tied {
		if f.Origin == "TNCM" || f.Destination == "TNCM" {
			t.Errorf("%s flies %s-%s between XNCM and the airport it is like", f.Callsign, f.Origin, f.Destination)
		}
	}

	region := Schedule(cfgX, ScheduleOptions{Focus: []string{"XNCM"}, Seed: 1, Ties: map[string]AirportTie{"XNCM": {Region: "TN"}}}, from, to)
	regional := 0
	anywhere := map[string]bool{}
	for _, a := range cfg.Airlines {
		anywhere[a.ICAO] = contains(a.Regions, "*")
	}
	for _, f := range region {
		if !anywhere[f.Airline] {
			regional++
		}
		if f.Airline == "WIA" {
			t.Errorf("%s of WIA at XNCM in region TN: WIA's base is TNCM, not XNCM", f.Callsign)
		}
	}
	if regional == 0 {
		t.Errorf("XNCM in region TN: only the anywhere airlines %v", airlines(region))
	}

	for _, ties := range []map[string]AirportTie{{}, {"TNCM": {}}, {"LKPR": {Like: "EDDF"}}} {
		if got := Schedule(cfg, ScheduleOptions{Focus: []string{"TNCM"}, Seed: 1, Ties: ties}, from, to); !reflect.DeepEqual(got, real) {
			t.Errorf("ties %v change TNCM's timetable", ties)
		}
	}

	at := from.Add(5 * time.Hour)
	want, ok1 := DrawFlight(cfg, ScheduleOptions{Seed: 3}, "TNCM", at, false)
	got, ok2 := DrawFlight(cfgX, ScheduleOptions{Seed: 3, Ties: map[string]AirportTie{"XNCM": {Like: "TNCM"}}}, "XNCM", at, false)
	if !ok1 || !ok2 || got.Airline != want.Airline || got.Destination != want.Destination {
		t.Errorf("DrawFlight at XNCM like TNCM: %+v, want as TNCM %+v", got, want)
	}
}
