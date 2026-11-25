package world

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// TestEndInUseNoWeather (#1661): the runway in use without weather. With
// nothing declared it is as before: an end the layout lacks gives none,
// and the end named on a runway closed at its other end stays. Only an
// end not used for this gives another, on a runway open at both ends.
func TestEndInUseNoWeather(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := endInUse(l, "99", nav.RunwayLimits{}, false); got != nil {
		t.Errorf("an end the layout lacks: %v, want none", nav.Names(got))
	}
	half := nav.RunwayLimits{Closed: []string{"06"}}
	for _, arrival := range []bool{false, true} {
		if got := endInUse(l, "24", half, arrival); len(got) != 1 || got[0].Name != "24" {
			t.Errorf("24 with 06 closed (arrival %v): %v, want 24 as before", arrival, nav.Names(got))
		}
	}
	for i := range l.Runways {
		if l.Runways[i].Secondary.Name == "24" {
			l.Runways[i].Secondary.Use = airport.RunwayUseLanding
		}
		if l.Runways[i].Primary.Name == "24" {
			l.Runways[i].Primary.Use = airport.RunwayUseLanding
		}
	}
	got := endInUse(l, "24", nav.RunwayLimits{}, false)
	if len(got) != 1 || got[0].Name == "24" || !got[0].ForTakeoff() {
		t.Errorf("24 landings only, a departure: %v, want another end used for take-offs", nav.Names(got))
	}
	if got := endInUse(l, "24", half, false); len(got) == 1 && (got[0].Name == "24" || got[0].Name == "06") {
		t.Errorf("24 landings only, 06 closed, a departure: %v, not on the half-closed runway", nav.Names(got))
	}
	if got := endInUse(l, "24", nav.RunwayLimits{}, true); len(got) != 1 || got[0].Name != "24" {
		t.Errorf("24 landings only, an arrival: %v, want 24", nav.Names(got))
	}
}
