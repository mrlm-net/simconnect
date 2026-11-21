package worldtest

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// tncmLargestStandM is the radius of TNCM's largest stand offline: a
// 747 (half span 32 m) parks on none of them.
const tncmLargestStandM = 28

// TestScheduleEnrouteArrivalsFitStands: an hour of the schedule at TNCM,
// whose stands are too small for a 747, with KLM's fleet (a B744 among
// it) installed. No arrival is created in a type with no stand there,
// and none is cancelled for want of a stand at its handover: a cancelled
// aircraft vanishes (follow-up to #1616, a B744 EHAM → TNCM flew en
// route for minutes and was cancelled with "no free suitable stand").
func TestScheduleEnrouteArrivalsFitStands(t *testing.T) {
	h, log := startTNCM(t)
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"TNCM"}, Density: 2.5, MaxAircraft: 8, Seed: 1616}); err != nil {
		t.Fatal(err)
	}
	// KLM's 747, added as a timetable's flight is: refused, not flown.
	sta := h.Clock.Now().Add(60 * time.Minute)
	_ = h.World.AddFlights([]traffic.Flight{
		{Callsign: "KLM744", Airline: "KLM", Type: "B744", Origin: "EHAM", Destination: "TNCM", STD: sta.Add(-9 * time.Hour), STA: sta, DistanceNM: 3780},
	})
	// An aircraft en route holds no stand: its stand is given at its
	// handover (the issue's comment: a cancelled heavy held TNCM's 747
	// stand from its spawn to its cancellation).
	held := map[string]bool{}
	for range 60 {
		h.Advance(time.Minute)
		var stands []struct {
			Owner string `json:"owner"`
		}
		if err := h.World.Get("/api/stands?icao=TNCM", &stands); err != nil {
			t.Fatal(err)
		}
		lines := log.String()
		for _, s := range stands {
			if s.Owner != "" && !held[s.Owner] && stillEnroute(lines, s.Owner) {
				held[s.Owner] = true
				t.Errorf("%s holds a stand en route", s.Owner)
			}
		}
	}
	created := 0
	for _, l := range strings.Split(log.String(), "\n") {
		if strings.Contains(l, "cancelled") || strings.Contains(l, "no free suitable stand") {
			t.Error(l)
		}
		if !strings.Contains(l, "schedule: arrival") || !strings.Contains(l, "→ TNCM") {
			continue
		}
		created++
		for _, f := range strings.Split(l, ", ") {
			if p := traffic.ProfileFor(f); p.Type == f && p.Motion.SpanMeters/2 > tncmLargestStandM {
				t.Errorf("created in a type with no stand at TNCM: %s", l)
			}
		}
	}
	if created == 0 {
		t.Fatalf("no arrival created in an hour")
	}
	t.Logf("%d arrivals created", created)
}

// stillEnroute reports whether the log has cs created en route and not
// handed over yet.
func stillEnroute(log, cs string) bool {
	en := false
	for _, l := range strings.Split(log, "\n") {
		switch {
		case strings.Contains(l, cs+" schedule: at "):
			return false
		case strings.Contains(l, cs+" schedule: arrival") && strings.Contains(l, " en route,"):
			en = true
		}
	}
	return en
}
