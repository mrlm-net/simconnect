package traffic

import (
	"testing"
	"time"
)

// Cargo airlines fly freighters, the others never one (#1300): FedEx's
// 777 is its B77LF, Smartwings' 737 never a freighter.
func TestCargoModels(t *testing.T) {
	models := []string{"FSLTL_B77LF_FDX", "FSLTL_B738F_FDX", "FSLTL_FAIB_B738_TVS-Smartwings", "FSLTL_B748F_UPS", "FSLTL_B744F_UPS",
		"FSLTL A320 DLH Lufthansa", "FSLTL_B738F_ZZZZ", "FSLTL_A332F_BCS", "FSLTL_B77LF_QTR_CARGO"}
	for _, c := range []struct{ airline, typ, want string }{
		{"FDX", "B77L", "FSLTL_B77LF_FDX"},
		{"FDX", "B738", "FSLTL_B738F_FDX"},
		{"UPS", "B748", "FSLTL_B748F_UPS"},
		{"BCS", "A332", "FSLTL_A332F_BCS"},
		{"TVS", "B738", "FSLTL_FAIB_B738_TVS-Smartwings"},
	} {
		got := ModelsFor(models, c.airline, "", c.typ, 0)
		if len(got) == 0 || got[0] != c.want {
			t.Errorf("%s %s: %v, want %s first", c.airline, c.typ, got, c.want)
		}
		for _, m := range got {
			if freighter(m) != cargoAirline(c.airline) {
				t.Errorf("%s %s: given %s", c.airline, c.typ, m)
			}
		}
	}
	if got := ModelsFor(models, "QTR", "", "B77L", 0); len(got) > 0 {
		t.Errorf("Qatar's passenger flights: %v, want no freighter", got)
	}
}

// A day at LKPR has cargo flights among the airlines' (#1300): the
// schedule had none, so the cargo stands stayed empty.
func TestCargoInSchedule(t *testing.T) {
	day := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	cargo := map[string]bool{}
	for _, a := range DefaultScheduleConfig().Airlines {
		if a.Cargo {
			cargo[a.ICAO] = true
		}
	}
	var got []string
	for _, f := range Schedule(DefaultScheduleConfig(), ScheduleOptions{Focus: []string{"LKPR"}, Seed: 3}, day, day.Add(24*time.Hour)) {
		if cargo[f.Airline] {
			got = append(got, f.Callsign+" "+f.Type)
		}
	}
	t.Logf("%d cargo flights: %v", len(got), got)
	if len(got) < 2 {
		t.Errorf("%d cargo flights in a day at LKPR", len(got))
	}
}
