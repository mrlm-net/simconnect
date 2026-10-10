package airport

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// TestStraightenLOWW: from C36 to runway 16 the route stays on L (live:
// "L, EX9, M, EX6, L"), and no route from any stand leaves a taxiway to
// come back to it later.
func TestStraightenLOWW(t *testing.T) {
	b, err := os.ReadFile("testdata/LOWW.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	c36, err := l.ParkingIndex("C36")
	if err != nil {
		t.Fatal(err)
	}
	r, err := g.RouteToRunway(c36, "16", RouteOptions{HalfSpan: 17.9})
	if err != nil {
		t.Fatal(err)
	}
	if via := r.SpokenTaxiways(-1); slices.Contains(via, "M") {
		t.Errorf("C36 to 16 via %v, want along L", via)
	}
	for i := range l.Parking {
		for _, end := range []string{"16", "29", "11", "34"} {
			r, err := g.RouteToRunway(i, end, RouteOptions{HalfSpan: 17.9})
			if err != nil {
				continue
			}
			via := r.SpokenTaxiways(-1)
			for k, n := range via {
				if j := slices.Index(via[k+1:], n); j > 0 {
					t.Errorf("stand %d to %s via %v: leaves %s and comes back", i, end, via, n)
					break
				}
			}
		}
	}
}
