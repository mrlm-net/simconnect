package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestEnteredAfter: a give-way stop short of where the route turns onto
// another taxiway names it; a stop with the route going on along the same
// taxiway names none.
func TestEnteredAfter(t *testing.T) {
	start := airport.LatLon{Lat: 50.1, Lon: 14.26}
	pts := []airport.LatLon{start}
	for _, m := range []float64{100, 200, 300} {
		lat, lon := calc.DisplaceByHeading(start.Lat, start.Lon, 90, m)
		pts = append(pts, airport.LatLon{Lat: lat, Lon: lon})
	}
	r := &airport.Route{Points: pts, Edges: []airport.Edge{{Name: "A", Length: 100}, {Name: "A", Length: 100}, {Name: "C", Length: 100}}}
	tr := newRouteTracker(r)
	at := func(m float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(start.Lat, start.Lon, 90, m)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	if got := tr.enteredAfter(at(170)); got != "C" {
		t.Errorf("30 m short of C: %q, want C", got)
	}
	if got := tr.enteredAfter(at(50)); got != "" {
		t.Errorf("150 m short of C, on along A: %q, want none", got)
	}
}

// TestHoldShort: "hold short of C", read back "Holding short of C" (Doc
// 4444 12.3.4.8), released with "continue taxi".
func TestHoldShort(t *testing.T) {
	tx := HoldShort("CSA1", "C")
	if tx.Text != "CSA1, hold short of C" {
		t.Errorf("said %q", tx.Text)
	}
	rb, ok := Readback(tx)
	if !ok || rb.Text != "Holding short of C, CSA1" {
		t.Errorf("read back %q (%v)", rb.Text, ok)
	}
}
