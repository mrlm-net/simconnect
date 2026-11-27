package traffic

import (
	"reflect"
	"testing"
	"time"
)

// OverflightOptions.Used keeps call signs out (#1708); nil draws as
// before, and the map given is left as it was.
func TestOverflightsUsedCallsigns(t *testing.T) {
	cfg := DefaultScheduleConfig()
	o := OverflightOptions{Centre: pictureLKPR.Position, RadiusNM: 150, PerHour: 30, Seed: 11, Exclude: []string{"LKPR"}}
	from := time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	base := Overflights(cfg, o, from, to)
	if len(base) < 10 {
		t.Fatalf("%d overflights in a busy hour", len(base))
	}
	o.Used = map[string]bool{}
	if !reflect.DeepEqual(base, Overflights(cfg, o, from, to)) {
		t.Fatal("an empty Used changed the draw")
	}
	// Every other call sign drawn is taken already.
	for i, f := range base {
		if i%2 == 0 {
			o.Used[f.Callsign] = true
		}
	}
	n := len(o.Used)
	got := Overflights(cfg, o, from, to)
	seen := map[string]bool{}
	for _, f := range got {
		if o.Used[f.Callsign] {
			t.Errorf("%s given though used", f.Callsign)
		}
		if seen[f.Callsign] {
			t.Errorf("%s given twice", f.Callsign)
		}
		seen[f.Callsign] = true
	}
	if len(o.Used) != n {
		t.Errorf("Used changed: %d call signs, given %d", len(o.Used), n)
	}
}
