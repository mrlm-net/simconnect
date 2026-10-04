package traffic

import "testing"

// TestWakeForGATitles: light GA models by their FSLTL titles (no logo
// light for them; live, OKFHP).
func TestWakeForGATitles(t *testing.T) {
	for _, m := range []string{"FSLTL_GA_B350_ZZZZ", "Asobo PassiveAircraft SR22 :: SR22_0"} {
		if w := WakeFor(m).ICAO; w != WakeLight {
			t.Errorf("%s: %c", m, w)
		}
	}
	if w := WakeFor("FSLTL_FAIB_B738_TVS-Skytravel_TSOC").ICAO; w != WakeMedium {
		t.Errorf("B738: %c", w)
	}
}
