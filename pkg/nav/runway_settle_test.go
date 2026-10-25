package nav

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// In the settle after a start, a runway chosen stays while it is still the
// choice without the margin: no swaps at one wind (live LKPR, 170°/24 kt:
// 24's crosswind at the margined limit, 24 and 12 swapped four times in a
// minute).
func TestRunwaySelectorSettleNoFlip(t *testing.T) {
	l := lkprInfo(t).Layout
	lim := RunwayLimitsFrom(airport.LimitsFor(l, loadLKPRProcedures(t)))
	now := time.Date(2026, 10, 25, 6, 48, 23, 0, time.UTC)
	var s RunwaySelector
	s.Resettle()
	if u := s.Choose(now, l, StaticWeather(170, 20, 9999, 7, 3, 1005), lim); u.Arrival.Name != "24" {
		t.Fatalf("first choice %s, want 24", u.Arrival.Name)
	}
	for i := 1; i <= 120; i++ {
		dir := 169.0 + float64(i%3) // 169–171°, 24's crosswind 22.8–23.2 kt
		w := StaticWeather(dir, 24, 9999, 7, 3, 1005)
		if u := s.Choose(now.Add(time.Duration(i)*time.Second), l, w, lim); u.Arrival.Name != "24" {
			t.Fatalf("%d s in, at %.0f°/24 kt: %s, want 24 kept within its limits", i, dir, u.Arrival.Name)
		}
	}
}
