//go:build windows

package traffic

import "testing"

// Every pushback leaves the nose facing the way out, within 60° of its
// first leg (10 m) of taxi, not a pivot on the spot, at every stand, both runways, five types (#491: LKPR
// A6 faced 148° with its taxi heading 282°; #492: at A1 a CS300 was pushed
// straight and pivoted 90° on the spot; the rule is general, so the test
// is too). Short moves (up to
// 50 m: self-manoeuvring and small stands) turn on the apron anyway.
func TestPushbackFacesWayOut(t *testing.T) {
	g := lkprGraph(t)
	bad, total := 0, 0
	for i, st := range g.Layout.Parking {
		for _, m := range []string{"FSLTL_B738_RYR", "FSLTL_E190_LOT", "FSLTL_FAIB_A321_WZZ-Wizz Air", "FSLTL_SBAI_BCS3_CSA-Lines", "FSLTL A320 Air France SL"} {
			for _, rwy := range []string{"24", "06"} {
				ec := &eventClient{}
				ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
				if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: rwy, Model: m, Tail: "T1"}); err != nil {
					continue
				}
				p, err := ctl.pushPath()
				if err != nil {
					continue
				}
				total++
				end := p.PointAt(p.Length())
				nose := localBearing(end, p.PointAt(p.Length()-5))
				r := ctl.route
				far := end
				for k := ctl.pushJunction; k < len(r.Points); k++ {
					if localDist(end, r.Points[k]) > 10 {
						far = r.Points[k]
						break
					}
				}
				if d := headingDiff(nose, localBearing(end, far)); (d > 60 || d < -60) && p.Length() > 50 {
					bad++
					t.Errorf("%s %-10.10s %s: nose %.0f, taxi heads %.0f, push %.0f m", st.Label(), m[6:], rwy, nose, localBearing(end, far), p.Length())
				}
			}
		}
	}
	t.Logf("%d of %d pushes leave the nose over 60° off the way out", bad, total)
	if total < 700 {
		t.Errorf("only %d pushes planned", total)
	}
}
