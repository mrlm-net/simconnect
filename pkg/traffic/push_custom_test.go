package traffic

import (
	"testing"
)

// TestStandardPushAirports: a stand's standard push reads back as a route
// at every test airport — from the stand, ending at the planned pose with
// its facing said — and a push drawn for a stand is flown as drawn.
func TestStandardPushAirports(t *testing.T) {
	const model = "FSLTL_B738_RYR"
	for _, icao := range testAirports {
		g := airportGraph(t, icao)
		found := 0
		for i := 0; i < len(g.Layout.Parking) && found < 2; i++ {
			p := g.Layout.Parking[i]
			if p.Radius < 15 { // a stand for a narrow-body
				continue
			}
			r, ok := StandardPush(g, i, model)
			if !ok {
				continue
			}
			found++
			if len(r.Points) < 2 || r.Said == "" || localDist(r.Points[0], StandPoint(p, 0)) > 60 {
				t.Errorf("%s %s: %d points, said %q, starts %.0f m from the stand", icao, p.Label(), len(r.Points), r.Said, localDist(r.Points[0], StandPoint(p, 0)))
				continue
			}
			// Drawn as the standard one, turned the other way at its end: the
			// departure flies it and says its facing.
			drawn := r
			SetCustomPush(icao, p.Label(), drawn)
			got, err := PlanPush(TaxiRequest{Graph: g, Parking: i, Model: model, Runway: g.Layout.Runways[0].Primary.Name})
			ClearCustomPush(icao, p.Label())
			if err != nil {
				t.Errorf("%s %s: custom push not planned: %v", icao, p.Label(), err)
				continue
			}
			if len(got.Push) != len(drawn.Points) || got.Push[len(got.Push)-1] != drawn.Points[len(drawn.Points)-1] {
				t.Errorf("%s %s: flew %d points, drawn %d", icao, p.Label(), len(got.Push), len(drawn.Points))
			}
		}
		if found == 0 {
			t.Errorf("%s: no stand with a standard push", icao)
		}
	}
}
