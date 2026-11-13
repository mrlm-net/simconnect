package traffic

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// crossingHolds (#1532): the crossings cleared are released until passed,
// the first one not cleared is where it holds.
func TestCrossingHolds(t *testing.T) {
	d := groundDrive{holdAtCrossings: true, crossZones: []crossZone{
		{from: 100, to: 160, runway: "06/24", nodes: [2]airport.NodeID{1, 2}},
		{from: 400, to: 450, runway: "12/30", nodes: [2]airport.NodeID{3, 4}},
	}}
	for _, c := range []struct {
		s    float64
		next int
		at   airport.NodeID
		past []airport.NodeID
	}{
		{50, 0, 1, nil},                      // held short of the first crossing
		{50, 1, 3, []airport.NodeID{1, 2}},   // cleared over the first, held at the second
		{130, 1, 3, []airport.NodeID{2}},     // on the runway: the far side still released
		{200, 1, 3, nil},                     // over it
		{200, 2, -1, []airport.NodeID{3, 4}}, // both cleared
		{500, 2, -1, nil},                    // all passed
	} {
		d.nextCross = c.next
		at, past := d.crossingHolds(c.s, nil)
		if at != c.at || !slices.Equal(past, c.past) {
			t.Errorf("at %.0f m, %d cleared: held at %d past %v, want %d %v", c.s, c.next, at, past, c.at, c.past)
		}
	}
	d.holdAtCrossings = false
	if at, past := d.crossingHolds(0, nil); at != -1 || len(past) != 4 {
		t.Errorf("not held at crossings: %d %v, want all four released", at, past)
	}
}
