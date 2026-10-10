package nav

import (
	"errors"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestExpandRoute: a filed route over a small network: airways walked
// fix by fix, DCT legs, speed/level groups and unknown tokens passed over,
// a fix ident with two fixes resolved by nearness, a wrong airway refused.
func TestExpandRoute(t *testing.T) {
	p := func(lat, lon float64) airport.LatLon { return airport.LatLon{Lat: lat, Lon: lon} }
	w := func(id, region string, lat, lon float64) Fix {
		return Fix{Ident: id, Region: region, Kind: KindWaypoint, Position: p(lat, lon)}
	}
	ref := func(f Fix) *FixRef { return &FixRef{Key: f.Key(), Position: f.Position} }
	a, b, c, d := w("AAAAA", "XX", 50, 14), w("BBBBB", "XX", 50, 15), w("CCCCC", "XX", 50, 16), w("DDDDD", "XX", 50.05, 15)
	far, twin := w("TWIN", "YY", 30, 0), w("TWIN", "XX", 50.2, 16.5)
	links := map[FixKey][]RouteLink{
		a.Key(): {{Airway: "UA1", Type: AirwayJet, Next: ref(b)}, {Airway: "UB2", Type: AirwayJet, Next: ref(d)}},
		b.Key(): {{Airway: "UA1", Type: AirwayJet, Prev: ref(a), Next: ref(c)}},
		c.Key(): {{Airway: "UA1", Type: AirwayJet, Prev: ref(b)}, {Airway: "UB2", Type: AirwayJet, Prev: ref(d)}},
		d.Key(): {{Airway: "UB2", Type: AirwayJet, Prev: ref(a), Next: ref(c)}},
	}
	g := BuildAirwayGraph([]Fix{a, b, c, d, far, twin}, links)
	steps, skipped, err := g.ExpandRoute("LKPR1A AAAAA/N0450F350 UA1 CCCCC DCT TWIN EDDF", p(50, 13.5))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range steps {
		got = append(got, s.Airway+" "+s.Fix.Ident+" "+s.Fix.Region)
	}
	want := []string{"DCT AAAAA XX", "UA1 BBBBB XX", "UA1 CCCCC XX", "DCT TWIN XX"}
	if !slices.Equal(got, want) {
		t.Errorf("steps %v, want %v", got, want)
	}
	if !slices.Equal(skipped, []string{"LKPR1A", "EDDF"}) {
		t.Errorf("skipped %v", skipped)
	}
	if _, _, err := g.ExpandRoute("AAAAA UA1 DDDDD", p(50, 14)); !errors.Is(err, ErrNoRoute) {
		t.Errorf("UA1 from A to D: %v, want ErrNoRoute", err)
	}
}
