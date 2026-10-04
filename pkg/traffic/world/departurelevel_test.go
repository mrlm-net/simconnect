package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// TestDepartureLevel: FL240, or the filed cruise level when lower.
func TestDepartureLevel(t *testing.T) {
	lim := airport.Limits{TransitionAltitudeFt: 5000}
	for _, c := range []struct {
		p    *planned
		said string
		ft   float64
	}{
		{nil, "flight level 240", 24000},
		{&planned{plan: &nav.FlightPlan{CruiseFL: 370}}, "flight level 240", 24000},
		{&planned{plan: &nav.FlightPlan{CruiseFL: 150}}, "flight level 150", 15000},
	} {
		if said, ft := departureLevel(c.p, lim); said != c.said || ft != c.ft {
			t.Errorf("%v: %q %.0f, want %q %.0f", c.p, said, ft, c.said, c.ft)
		}
	}
}
