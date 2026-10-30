package traffic

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// withSources puts model sources in use for a test, the game's default
// switches, then forgets them.
func withSources(t *testing.T, src map[string]ModelSource) {
	t.Helper()
	gameDefaults(t)
	SetModelSources(src)
	t.Cleanup(func() { SetModelSources(nil) })
}

// A maker's packages: its FSLTL-format traffic and its flyable liveries,
// one provider (the user's WANTED.solutions).
var makerSources = map[string]ModelSource{
	"fsltl_wanted_a320_flame":                           {Package: "wanted-fsltl-traffic", Creator: "WANTED.solutions"},
	"fsltl_wanted_lj25_flame":                           {Package: "wanted-fsltl-traffic", Creator: "WANTED.solutions"},
	"fenixa320 cfm sl :: wanted.solutions flame ok-wsh": {Package: "wanted-fenix-liveries", Creator: "WANTED.solutions", Flyable: true},
	"fsltl_faib_a320_smartwings_czechairlineslivery":    {Package: "fsltl-traffic-base"},
	"airbus a320 neo asobo":                             {Package: "fs-base-aircraft"},
	"fsdt_tug_1":                                        {Package: "fsdreamteam-gsx-pro"},
	"a32nx flybywire":                                   {Package: "flybywire-aircraft-a320-neo", Creator: "FlyByWire Simulations", Flyable: true},
}

// TestProvidersGameDefaults: by default our aircraft are the game's own:
// MSFS on, FSLTL and a maker's traffic off, GSX's vehicles on, a flyable
// add-on off; a maker's traffic package is not taken for a flyable add-on.
func TestProvidersGameDefaults(t *testing.T) {
	withSources(t, makerSources)
	for p, want := range map[string]bool{"MSFS": true, "FSLTL": false, "GSX": true, "WANTED.solutions": false, "FlyByWire Simulations": false} {
		if got := ProviderOn(p); got != want {
			t.Errorf("ProviderOn(%q) = %v, want %v", p, got, want)
		}
	}
	if ProviderFlyable("WANTED.solutions") {
		t.Errorf("WANTED.solutions flyable: its traffic package counted as a flyable add-on's")
	}
	SetProviders(map[string]bool{"WANTED.solutions": true})
	if !TitleOn("FSLTL_WANTED_A320_Flame") || TitleOn("FenixA320 CFM SL :: WANTED.solutions Flame OK-WSH") {
		t.Errorf("switched on: traffic %v, flyable livery %v; want the traffic only", TitleOn("FSLTL_WANTED_A320_Flame"), TitleOn("FenixA320 CFM SL :: WANTED.solutions Flame OK-WSH"))
	}
}

// TestReservedAirlineModels: WTD flies its own traffic models only, its
// provider off or not, never a flyable livery nor another airline's; a
// type it has none of, none.
func TestReservedAirlineModels(t *testing.T) {
	withSources(t, makerSources)
	titles := []string{"FSLTL_WANTED_A320_Flame", "FSLTL_WANTED_LJ25_Flame", "FenixA320 CFM SL :: WANTED.solutions Flame OK-WSH",
		"FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", "Airbus A320 Neo Asobo"}
	if got := ModelsFor(titles, "WTD", "WANTED", "A320", 0); !slices.Equal(got, []string{"FSLTL_WANTED_A320_Flame"}) {
		t.Errorf("WTD A320: %v, want its traffic model only", got)
	}
	if got := ModelsFor(titles, "WTD", "WANTED", "LJ25", 0); !slices.Equal(got, []string{"FSLTL_WANTED_LJ25_Flame"}) {
		t.Errorf("WTD LJ25: %v", got)
	}
	if got := ModelsFor(titles, "MCR", "MyCrew", "A320", 0); len(got) != 0 {
		t.Errorf("MCR without its models: %v, want none", got)
	}
}

// TestReservedAirlinesKept: WTD and MCR are said as ours whatever a table
// fed over the shipped one says, mixed case kept.
func TestReservedAirlinesKept(t *testing.T) {
	over, _ := json.Marshal([]TelephonyItem{{ICAO: "WTD", Telephony: "WOODTOWN", Name: "Other"}})
	if err := dict.Use("traffic.telephony", over); err != nil {
		t.Fatal(err)
	}
	airlines, _ := json.Marshal([]Airline{{ICAO: "MCR", Name: "Other", Telephony: "OTHER", Fleet: map[string]float64{"B738": 1}, Bases: []string{"EDDF"}, Weight: 5}})
	if err := dict.Use("traffic.airlines", airlines); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dict.Reset("traffic.telephony"); _ = dict.Reset("traffic.airlines") })
	if tel, _, ok := Telephony("WTD"); !ok || tel != "WANTED" {
		t.Errorf("Telephony(WTD) = %q %v", tel, ok)
	}
	cfg := DefaultScheduleConfig()
	cfg.Airlines = append(cfg.Airlines, Airline{ICAO: "MCR", Telephony: "OTHER"})
	if s := cfg.SaidCallsign("MCR1234"); s != "MyCrew 1234" {
		t.Errorf("MCR1234 said %q, want MyCrew 1234", s)
	}
	if s := cfg.SaidCallsign("WTD123"); s != "Wanted 123" {
		t.Errorf("WTD123 said %q, want Wanted 123", s)
	}
	n := 0
	for _, a := range DefaultScheduleConfig().Airlines {
		if a.ICAO == "MCR" {
			n++
			if a.Name != "MyCrew" {
				t.Errorf("MCR in the airlines: %+v", a)
			}
		}
	}
	if n != 1 {
		t.Errorf("MCR %d times in the airlines", n)
	}
}

// TestReservedAirlinesScheduled: a day at LKPR has a few of their flights,
// not many.
func TestReservedAirlinesScheduled(t *testing.T) {
	cfg := DefaultScheduleConfig()
	day := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	ours, all := 0, 0
	for seed := uint64(1); seed <= 5; seed++ {
		for _, f := range Schedule(cfg, ScheduleOptions{Focus: []string{"LKPR"}, Density: 3, Seed: seed}, day, day.Add(24*time.Hour)) {
			all++
			if reservedAirline(f.Airline) {
				ours++
				if !strings.HasPrefix(f.Callsign, f.Airline) {
					t.Errorf("%+v", f)
				}
			}
		}
	}
	t.Logf("5 days at LKPR: %d of %d flights ours", ours, all)
	if ours == 0 || ours*50 > all {
		t.Errorf("%d of %d flights ours, want a few", ours, all)
	}
}
