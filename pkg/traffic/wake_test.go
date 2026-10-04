package traffic

import (
	"testing"
	"time"
)

func TestWakeFor(t *testing.T) {
	for _, c := range []struct {
		typ  string
		want Wake
	}{
		{"A388", Wake{WakeSuper, RecatA}}, {"B77W", Wake{WakeHeavy, RecatB}}, {"B789", Wake{WakeHeavy, RecatC}},
		{"A320", Wake{WakeMedium, RecatD}}, {"B38M", Wake{WakeMedium, RecatD}}, {"E190", Wake{WakeMedium, RecatE}},
		{"AT76", Wake{WakeMedium, RecatE}}, {"DH8D", Wake{WakeMedium, RecatE}}, {"C172", Wake{WakeLight, RecatF}},
		// Model titles through ProfileFor.
		{"FSLTL_FAIB_B738_TVS-Smartwings_NC", Wake{WakeMedium, RecatD}}, {"Asobo PassiveAircraft B787-09", Wake{WakeHeavy, RecatC}},
		// Nothing known: medium.
		{"ZZZZ", Wake{WakeMedium, RecatD}},
	} {
		if got := WakeFor(c.typ); got != c.want {
			t.Errorf("%s: %v%v, want %v%v", c.typ, got.ICAO, got.Recat, c.want.ICAO, c.want.Recat)
		}
	}
}

func TestArrivalSeparation(t *testing.T) {
	j, h, m, l := Wake{WakeSuper, RecatA}, Wake{WakeHeavy, RecatB}, Wake{WakeMedium, RecatD}, Wake{WakeLight, RecatF}
	e := Wake{WakeMedium, RecatE}
	c := Wake{WakeHeavy, RecatC}
	for _, x := range []struct {
		lead, follow Wake
		scheme       SeparationScheme
		want         float64
	}{
		// ICAO Doc 4444.
		{j, h, SchemeICAO, 6}, {j, m, SchemeICAO, 7}, {j, l, SchemeICAO, 8},
		{h, h, SchemeICAO, 4}, {h, m, SchemeICAO, 5}, {h, l, SchemeICAO, 6},
		{m, l, SchemeICAO, 5}, {m, m, SchemeICAO, 3}, {m, h, SchemeICAO, 3}, {l, l, SchemeICAO, 3},
		// RECAT-EU.
		{j, h, SchemeRecat, 4}, {j, e, SchemeRecat, 6}, {h, c, SchemeRecat, 4}, {h, e, SchemeRecat, 5},
		{c, m, SchemeRecat, 3}, {m, l, SchemeRecat, 5}, {e, l, SchemeRecat, 4}, {m, m, SchemeRecat, 3}, {m, j, SchemeRecat, 3},
	} {
		if got := ArrivalSeparationNM(x.lead, x.follow, x.scheme); got != x.want {
			t.Errorf("%v%v → %v%v (scheme %d): %.0f NM, want %.0f", x.lead.ICAO, x.lead.Recat, x.follow.ICAO, x.follow.Recat, x.scheme, got, x.want)
		}
	}
	// Every pair is at least the minimum radar separation.
	for _, a := range []Wake{j, h, c, m, e, l} {
		for _, b := range []Wake{j, h, c, m, e, l} {
			for _, s := range []SeparationScheme{SchemeICAO, SchemeRecat} {
				if ArrivalSeparationNM(a, b, s) < MinRadarSeparationNM {
					t.Errorf("%v → %v below the minimum", a, b)
				}
			}
		}
	}
	if d := SeparationTime(5, 150); d != 2*time.Minute {
		t.Errorf("5 NM at 150 kt: %v", d)
	}
}

func TestDepartureInterval(t *testing.T) {
	j, h, m, l := Wake{WakeSuper, RecatA}, Wake{WakeHeavy, RecatB}, Wake{WakeMedium, RecatD}, Wake{WakeLight, RecatF}
	for _, c := range []struct {
		lead, follow Wake
		same         bool
		want         time.Duration
	}{
		{m, m, false, time.Minute}, {m, m, true, 2 * time.Minute},
		{h, m, false, 2 * time.Minute}, {h, l, false, 2 * time.Minute}, {h, h, false, time.Minute},
		{j, h, false, 2 * time.Minute}, {j, m, false, 3 * time.Minute}, {j, l, true, 3 * time.Minute},
		{l, h, false, time.Minute},
	} {
		if got := DepartureInterval(c.lead, c.follow, c.same); got != c.want {
			t.Errorf("%v → %v same %v: %v, want %v", c.lead.ICAO, c.follow.ICAO, c.same, got, c.want)
		}
	}
	if RunwayOccupancy(h, true) <= RunwayOccupancy(m, true) || RunwayOccupancy(m, false) >= RunwayOccupancy(m, true) {
		t.Error("runway occupancy: heavies longer, take-offs shorter than landings")
	}
}
