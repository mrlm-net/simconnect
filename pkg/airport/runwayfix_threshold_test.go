package airport

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// A runway fix already where it belongs stays there (#1646), some tens of
// metres off the layout's point as a charted threshold is: an approach's at
// its end's landing threshold, a SID's at the pavement end, where the
// take-off starts. A SID's at the landing threshold of a displaced end
// still goes to the pavement end, and one at the runway's centre to its
// end.
func TestAtRunwayEndsKeepsFixAtLandingThreshold(t *testing.T) {
	for _, displaced := range []float64{0, 300} {
		l := runwayFixLayout(displaced)
		e24 := l.Runways[0].Secondary
		thr := e24.LandingThreshold()
		centre := l.Runways[0].Center
		charted := fixAlong(thr, 240, 20)
		chartedSID := fixAlong(e24.Threshold, 240, 20)
		rw := func(tp types.SIMCONNECT_FACILITY_LEG_TYPE, at LatLon) Leg {
			return Leg{Type: tp, Fix: "RW24", FixKind: "R", Position: at}
		}
		sid := func(name string, at LatLon) Procedure {
			return Procedure{Name: name, RunwayTransitions: []Transition{{Runway: "24", Legs: []Leg{
				rw(types.SIMCONNECT_FACILITY_LEG_TYPE_IF, at), {Type: types.SIMCONNECT_FACILITY_LEG_TYPE_CA, Course: 240, Alt1: 500}}}}}
		}
		p := Procedures{ICAO: "TEST",
			Departures: []Procedure{sid("CHARTED", chartedSID), sid("LANDING", thr)},
			Approaches: []Approach{{Name: "ILS 24", Runway: "24",
				Final:  []Leg{func() Leg { m := rw(types.SIMCONNECT_FACILITY_LEG_TYPE_CF, charted); m.MAP = true; return m }()},
				Missed: []Leg{rw(types.SIMCONNECT_FACILITY_LEG_TYPE_TF, centre)}}},
		}
		got, _ := p.AtRunwayEnds(l)
		for _, c := range []struct {
			what      string
			got, want LatLon
		}{
			{"MAP 20 m from the landing threshold kept", got.Approaches[0].Final[0].Position, charted},
			{"SID fix 20 m from the pavement end kept", got.Departures[0].RunwayTransitions[0].Legs[0].Position, chartedSID},
			{"SID fix at the landing threshold on the pavement end", got.Departures[1].RunwayTransitions[0].Legs[0].Position, e24.Threshold},
			{"centre fix on the landing threshold", got.Approaches[0].Missed[0].Position, thr},
		} {
			if d := fixDistM(c.got, c.want); d > 0.01 {
				t.Errorf("displaced %.0f m: %s: %.0f m off", displaced, c.what, d)
			}
		}
	}
}
