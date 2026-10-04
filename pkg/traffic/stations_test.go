package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The stations as the airport map says them: at LKPR the AIP's unit call
// signs, as the scenery names its frequencies only "RUZYNE"; elsewhere the
// scenery's name for what the frequency is.
func TestStationFor(t *testing.T) {
	lkpr := &airport.Layout{ICAO: "LKPR", Frequencies: []airport.Frequency{
		{Kind: airport.FreqClearance, MHz: 120.06, Name: "RUZYNE"},
		{Kind: airport.FreqGround, MHz: 121.91, Name: "RUZYNE"},
		{Kind: airport.FreqTower, MHz: 134.56, Name: "RUZYNE"},
		{Kind: airport.FreqApproach, MHz: 118.31, Name: "RUZYNE"},
	}}
	for _, c := range []struct {
		pos        Position
		name, freq string
	}{
		{PosDelivery, "Ruzyne Delivery", "120.06"},
		{PosGround, "Ruzyne Ground", "121.91"},
		{PosTower, "Ruzyne Tower", "134.56"},
		{PosApproach, "Ruzyne Radar", "118.31"},
		{PosDeparture, "Ruzyne Radar", "118.31"}, // no departure frequency: approach's
	} {
		if name, freq := StationFor(lkpr, c.pos); name != c.name || freq != c.freq {
			t.Errorf("LKPR %s: %q %q, want %q %q", c.pos, name, freq, c.name, c.freq)
		}
	}
	other := &airport.Layout{ICAO: "XXXX", Frequencies: []airport.Frequency{{Kind: airport.FreqTower, MHz: 118.1, Name: "SOMEWHERE TOWER"}}}
	if name, freq := StationFor(other, PosTower); name != "Somewhere Tower" || freq != "118.10" {
		t.Errorf("XXXX tower: %q %q", name, freq)
	}
	if name, freq := StationFor(nil, PosTower); name != PositionName(PosTower) || freq != "" {
		t.Errorf("no layout: %q %q", name, freq)
	}
}
