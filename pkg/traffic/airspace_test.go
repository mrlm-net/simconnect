package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestSeparationRequired: who is separated from whom by class (#570).
func TestSeparationRequired(t *testing.T) {
	for _, c := range []struct {
		cl   AirspaceClass
		a, b string
		want bool
	}{
		{ClassC, "IFR", "IFR", true}, {ClassC, "IFR", "VFR", true}, {ClassC, "VFR", "VFR", false},
		{ClassD, "IFR", "IFR", true}, {ClassD, "IFR", "VFR", false}, {ClassD, "VFR", "VFR", false},
		{ClassE, "IFR", "IFR", true}, {ClassE, "VFR", "IFR", false},
		{ClassG, "IFR", "IFR", false},
	} {
		if got := SeparationRequired(c.cl, c.a, c.b); got != c.want {
			t.Errorf("class %s, %s and %s: separated %v, want %v", c.cl, c.a, c.b, got, c.want)
		}
	}
}

// TestTrafficInformation: an A320 3 NM east of an aircraft tracking north,
// flying west, is at 3 o'clock, crossing right to left; the phrase and its
// acknowledgement.
func TestTrafficInformation(t *testing.T) {
	p := airport.LatLon{Lat: 50.1, Lon: 14.26}
	other := offsetHeading(p, 90, 3*1852)
	clock, nm, dir := TrafficRelative(p, 0, other, 270)
	if clock != 3 || nm != 3 || dir != "crossing right to left" {
		t.Fatalf("%d o'clock, %.0f NM, %s", clock, nm, dir)
	}
	if _, _, d := TrafficRelative(p, 0, offsetHeading(p, 0, 4*1852), 180); d != "opposite direction" {
		t.Errorf("head-on: %s", d)
	}
	tx := TrafficInformation(PosTower, "OKABC", clock, nm, dir, "Airbus A320", "2500 feet")
	if want := "OKABC, traffic, 3 o'clock, 3 miles, crossing right to left, Airbus A320, 2500 feet"; tx.Text != want {
		t.Errorf("%q, want %q", tx.Text, want)
	}
	if rb, ok := Readback(tx); !ok || rb.Text != "Looking out, OKABC" {
		t.Errorf("acknowledged %q %v", rb.Text, ok)
	}
}
