package traffic

import (
	"testing"
	"time"
)

func TestLocalHourCivil(t *testing.T) {
	summer := time.Date(2026, 7, 1, 5, 0, 0, 0, time.UTC)
	winter := time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)
	cfg := DefaultScheduleConfig()
	pos := map[string]ScheduleAirport{}
	for _, a := range cfg.Airports {
		pos[a.ICAO] = a
	}
	for _, c := range []struct {
		icao           string
		summer, winter int
	}{
		{"LKPR", 7, 6}, {"LEMD", 7, 6}, {"EGLL", 6, 5}, {"LPPT", 6, 5},
	} {
		p := pos[c.icao].Position
		if h := LocalHour(c.icao, p, summer); h != c.summer {
			t.Errorf("%s at 05Z in July: %02d local, want %02d", c.icao, h, c.summer)
		}
		if h := LocalHour(c.icao, p, winter); h != c.winter {
			t.Errorf("%s at 05Z in December: %02d local, want %02d", c.icao, h, c.winter)
		}
	}
	// Unknown regions keep solar time.
	if h := LocalHour("XXXX", pos["LKPR"].Position, winter); h != 5 {
		t.Errorf("solar fallback at 14°E, 05:00Z: %02d, want 05", h)
	}
}

// Madrid's morning bank is at 07:00 CEST, 05:00 UTC; on solar time from
// its longitude (3.6°W) it fell two hours late, at the night's end.
func TestScheduleWavesOnCivilTime(t *testing.T) {
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	flights := Schedule(DefaultScheduleConfig(), ScheduleOptions{Focus: []string{"LEMD"}, Seed: 5}, day, day.Add(24*time.Hour))
	byUTC := map[int]int{}
	for _, f := range flights {
		byUTC[focusTime(f, []string{"LEMD"}).UTC().Hour()]++
	}
	// 07–09 local is the bank (waves 1, 0.9), 10–12 the lull after it (0.65, 0.7).
	bank, lull := byUTC[5]+byUTC[6], byUTC[8]+byUTC[9]
	if bank <= lull {
		t.Errorf("LEMD 05–07Z %d movements, 08–10Z %d: the morning bank is not at 07:00 local", bank, lull)
	}
}
