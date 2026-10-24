package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// stands builds a layout whose parking carries the airline assignments given.
func stands(assigned map[string][]string, radius float64) *airport.Layout {
	l := &airport.Layout{}
	for name, airlines := range assigned {
		_ = name
		l.Parking = append(l.Parking, airport.Parking{
			Type: 9, Radius: radius, Airlines: airlines,
		})
	}
	return l
}

// TestStandCarriers: the airlines an airport's stands name that the config does
// not know become carriers of their own (#1172).
func TestStandCarriers(t *testing.T) {
	known := map[string]bool{"DLH": true}
	l := &airport.Layout{Parking: []airport.Parking{
		{Type: 9, Radius: 26, Airlines: []string{"CLH", "DLH"}},
		{Type: 9, Radius: 20, Airlines: []string{"CLH"}},
		{Type: 9, Radius: 34, Airlines: []string{"BTI"}},
		// Not a stand: a fuel pump names nobody worth scheduling.
		{Type: 12, Radius: 40, Airlines: []string{"XXX"}},
	}}
	cfg := DefaultScheduleConfig()
	got := standCarriers(cfg, l, known, "EDDM")
	by := map[string]Airline{}
	for _, a := range got {
		by[a.ICAO] = a
	}
	if _, ok := by["DLH"]; ok {
		t.Error("an airline the config knows was invented again")
	}
	if _, ok := by["XXX"]; ok {
		t.Error("a fuel pump's airline became a carrier")
	}
	if len(got) != 2 {
		t.Fatalf("%d carriers, want 2 (CLH, BTI)", len(got))
	}
	// MORE STANDS, MORE FLIGHTS — but the weight alone no longer says so, and
	// asserting on it was how a scale mismatch hid: a stand carrier's Weight
	// carried its share while the config's airlines took a flat eightfold, so the
	// two lists were never comparable. Presence is applied once now, by
	// homeWeight, to both. What matters is the product, so that is what is checked.
	shares := standShares(l)
	eff := func(code string) float64 { return by[code].Weight * homeWeight(shares[code]) }
	if eff("CLH") <= eff("BTI") {
		t.Errorf("CLH %.2f not above BTI %.2f (shares %.2f / %.2f)",
			eff("CLH"), eff("BTI"), shares["CLH"], shares["BTI"])
	}
	// NO BASE. A base earns the home carrier's eightfold, and every airline with a
	// stand having it hands it to all of them at once — Munich then had eighty-five
	// airlines sharing it equally and Lufthansa sixth in its own hub.
	if len(by["CLH"].Bases) != 0 {
		t.Errorf("CLH was given a base: %v", by["CLH"].Bases)
	}
	if len(by["CLH"].Regions) != 1 || by["CLH"].Regions[0] != "ED" {
		t.Errorf("CLH regions %v, want [ED]", by["CLH"].Regions)
	}
	// THE WIDEST STAND IT HOLDS is the largest thing it can park, so it is the
	// largest thing it flies in — and the test is the engine's own wing spans, not
	// a list: nothing in BTI's fleet may span more than its 34 m stand allows.
	for who, radius := range map[string]float64{"BTI": 34, "CLH": 26} {
		if len(by[who].Fleet) == 0 {
			t.Errorf("%s was given no fleet at all", who)
		}
		for typ := range by[who].Fleet {
			if half := MotionProfileFor(typ).SpanMeters / 2; half > radius {
				t.Errorf("%s on a %.0f m stand flies %s, half-span %.1f m", who, radius, typ, half)
			}
		}
	}
	// And the bigger stand gets the bigger aeroplane.
	bigOf := func(f map[string]float64) float64 {
		m := 0.0
		for t := range f {
			m = math.Max(m, MotionProfileFor(t).SpanMeters)
		}
		return m
	}
	if bigOf(by["BTI"].Fleet) <= bigOf(by["CLH"].Fleet) {
		t.Errorf("BTI's 34 m stand flies nothing bigger than CLH's 26 m one")
	}
}

// TestStandCarriersNoLayout: no layout, no carriers — and no panic.
func TestStandCarriersNoLayout(t *testing.T) {
	cfg := DefaultScheduleConfig()
	if got := standCarriers(cfg, nil, map[string]bool{}, "EDDM"); len(got) != 0 {
		t.Errorf("%d carriers from no layout", len(got))
	}
	if got := standCarriers(cfg, stands(nil, 25), map[string]bool{}, "EDDM"); len(got) != 0 {
		t.Errorf("%d carriers from a layout with no assignments", len(got))
	}
}
