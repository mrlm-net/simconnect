package nav

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Back to the preferred runway after a spell of calm only once the wind of
// the last RunwayBackMemory has been within its limits: a calm half hour in
// a morning of north-easterlies put LKPR on 24, and the north-easterly back
// put it on 06 again (live, 06 → 24 at 07:46 in calm, → 06 at 10:17 at
// 027°/7 kt, departures re-planned both times).
func TestRunwaySelectorBackToPreferredRemembersTheHour(t *testing.T) {
	l := lkprInfo(t).Layout
	lim := RunwayLimitsFrom(airport.LimitsFor(l, loadLKPRProcedures(t)))
	now := time.Date(2026, 11, 4, 5, 0, 0, 0, time.UTC)
	var s RunwaySelector
	at := func(min int) time.Time { return now.Add(time.Duration(min) * time.Minute) }
	for i := 0; i <= 120; i++ { // a north-easterly: 06
		if u := s.Choose(at(i), l, StaticWeather(25, 7, 9999, 10, 6, 1030), lim); i == 120 && u.Arrival.Name != "06" {
			t.Fatalf("north-easterly: %s, want 06", u.Arrival.Name)
		}
	}
	for i := 121; i <= 160; i++ { // calm for 40 minutes: 06 stays
		if u := s.Choose(at(i), l, StaticWeather(30, 0, 9999, 10, 6, 1030), lim); u.Arrival.Name != "06" {
			t.Fatalf("calm for %d min after an hour of north-easterly: %s, want 06 kept", i-120, u.Arrival.Name)
		}
	}
	back := false
	for i := 161; i <= 200 && !back; i++ { // calm on: back to 24
		back = s.Choose(at(i), l, StaticWeather(30, 0, 9999, 10, 6, 1030), lim).Arrival.Name == "24"
	}
	if !back {
		t.Fatal("never back to the preferred 24 after 80 minutes of calm")
	}
}
