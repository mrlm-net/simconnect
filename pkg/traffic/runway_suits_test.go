package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestRunwaySuits (#1561): an airliner never on grass or water, a light
// aircraft on grass too; a surface not known counts as paved.
func TestRunwaySuits(t *testing.T) {
	grass := airport.Runway{Surface: "grass", Length: 1017}
	paved := airport.Runway{Surface: "asphalt", Hard: true, Length: 2300}
	water := airport.Runway{Surface: "water", Length: 3000}
	for _, c := range []struct {
		model string
		r     airport.Runway
		want  bool
	}{
		{"A320", paved, true},
		{"A320", grass, false},
		{"A320", water, false},
		{"A320", airport.Runway{Length: 2000}, true},
		{"DA62", grass, true},
		{"DA62", paved, true},
		{"DA62", water, false},
		{"", grass, true},
	} {
		if got := RunwaySuits(c.model, c.r); got != c.want {
			t.Errorf("%q on %q: %v, want %v", c.model, c.r.Surface, got, c.want)
		}
	}
	if !RunwayLongEnough("A320", paved, 0) || RunwayLongEnough("A320", airport.Runway{Length: 900}, 0) || !RunwayLongEnough("DA62", paved, 0) || !RunwayLongEnough("", airport.Runway{Length: 100}, 0) {
		t.Error("RunwayLongEnough: A320 on 2300 m, not on 900 m; DA62 on 2300 m; no model anywhere")
	}
}
