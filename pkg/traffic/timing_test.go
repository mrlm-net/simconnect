package traffic

import (
	"math/rand/v2"
	"testing"
)

// TestTimingSpread: many aircraft's draws stay within each band and use
// it; the same seed gives the same draws; zero spreads give exactly 1.
func TestTimingSpread(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	bands := []struct {
		name   string
		spread float64
		get    func(timing) float64
	}{
		{"beacon", BeaconLeadSpread, func(x timing) float64 { return x.beacon }},
		{"taxi light", TaxiLightSpread, func(x timing) float64 { return x.taxiLight }},
		{"tug", TugDisconnectSpread, func(x timing) float64 { return x.tug }},
		{"flaps", FlapsSpread, func(x timing) float64 { return x.flaps }},
		{"gear", GearUpSpread, func(x timing) float64 { return x.gearUp }},
		{"taxi speed", TaxiSpeedSpread, func(x timing) float64 { return x.taxiSpeed }},
		{"push speed", PushbackSpeedSpread, func(x timing) float64 { return x.pushSpeed }},
	}
	lo, hi := map[string]float64{}, map[string]float64{}
	for i := 0; i < 2000; i++ {
		d := drawTiming(rng)
		for _, b := range bands {
			v := b.get(d)
			if v < 1-b.spread-1e-9 || v > 1+b.spread+1e-9 {
				t.Fatalf("%s factor %.3f outside ±%.2f", b.name, v, b.spread)
			}
			if i == 0 || v < lo[b.name] {
				lo[b.name] = v
			}
			if i == 0 || v > hi[b.name] {
				hi[b.name] = v
			}
		}
	}
	for _, b := range bands {
		if hi[b.name]-lo[b.name] < 1.8*b.spread { // the band is used, not a constant
			t.Errorf("%s spans only %.3f–%.3f", b.name, lo[b.name], hi[b.name])
		}
	}
	a, c := drawTiming(rand.New(rand.NewPCG(7, 7))), drawTiming(rand.New(rand.NewPCG(7, 7)))
	if a != c {
		t.Error("same seed, different draws")
	}
	saved := BeaconLeadSpread
	BeaconLeadSpread = 0
	defer func() { BeaconLeadSpread = saved }()
	if got := drawTiming(rng).beacon; got != 1 {
		t.Errorf("zero spread gives %.3f, want 1", got)
	}
}

// TestSeededDeparturesRepeat: two departures with the same seed taxi at the
// same speed; different seeds (almost surely) differ.
func TestSeededDeparturesRepeat(t *testing.T) {
	speed := func(seed uint64) float64 {
		g := lkprGraph(t)
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)), TaxiWithSeed(seed))
		c22, _ := g.Layout.ParkingIndex("C22")
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: "A320"}); err != nil {
			t.Fatal(err)
		}
		return ctl.profile().CruiseKts
	}
	if a, b := speed(42), speed(42); a != b {
		t.Errorf("same seed: %.3f and %.3f kt", a, b)
	}
	if a, b := speed(42), speed(43); a == b {
		t.Errorf("different seeds, same taxi speed %.3f kt", a)
	}
}
