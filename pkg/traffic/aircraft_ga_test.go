//go:build windows
// +build windows

package traffic

import "testing"

// Light aircraft (#565): the simulator's AI models resolve to their types,
// single-engined, light wake, approaching slowly; an unknown small aircraft
// is taken for a light single.
func TestLightAircraftProfiles(t *testing.T) {
	for title, want := range map[string]string{
		"Asobo PassiveAircraft C152":         "C152",
		"Asobo PassiveAircraft C172 :: C172_0": "C172",
		"Asobo PassiveAircraft DA40 NG":      "DA40",
		"Asobo PassiveAircraft SR22T":        "SR22",
		"Piper PA-28-181 Archer III":         "P28A",
		"C172SP G1000 Passengers":            "C172",
	} {
		p := ProfileFor(title)
		if p.Type != want {
			t.Errorf("%s: type %q, want %s", title, p.Type, want)
			continue
		}
		if p.Category != CategoryPiston || p.EngineCount() != 1 || WakeFor(title).ICAO != WakeLight {
			t.Errorf("%s: %s, %d engines, wake %v", title, p.Category, p.EngineCount(), WakeFor(title))
		}
		if p.Approach.ApproachKts > 80 || p.Approach.ApproachKts < 50 {
			t.Errorf("%s: approach %.0f kt", title, p.Approach.ApproachKts)
		}
	}
	if g := GenericProfile(10, ""); g.Category != CategoryPiston {
		t.Errorf("unknown 10 m aircraft: %s", g.Category)
	}
}
