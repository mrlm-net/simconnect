package airport

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestMissedOfClimb: the altitude a missed approach climbs to is its
// highest minimum, a ceiling only where that is the climb level (#1659).
// The legs of TNCM, EBBR and KNPA are the facility data's.
func TestMissedOfClimb(t *testing.T) {
	const ft = 0.3048
	const (
		at    = types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT
		above = types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_ABOVE
		below = types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_BELOW
	)
	onbed, next := LatLon{Lat: 18.04, Lon: -63.25}, LatLon{Lat: 18.10, Lon: -63.40}
	climb := func(feet float64) Leg {
		return Leg{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_CA, AltDesc: above, Alt1: feet * ft, Course: 250}
	}
	vectors := func(d types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR, feet float64) Leg {
		return Leg{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_VM, AltDesc: d, Alt1: feet * ft, Course: 250}
	}
	direct := func(fix string, p LatLon, feet float64) Leg {
		l := Leg{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_DF, Fix: fix, FixKind: "W", Position: p}
		if feet > 0 {
			l.AltDesc, l.Alt1 = above, feet*ft
		}
		return l
	}
	hold := func(fix string, p LatLon, feet float64) Leg {
		return Leg{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_HM, Fix: fix, FixKind: "W", Position: p, AltDesc: below, Alt1: feet * ft}
	}
	cases := []struct {
		name  string
		endFt float64
		legs  []Leg
		want  float64
	}{
		{"TNCM RNAV 10", 64, []Leg{direct("ONBED", onbed, 4000), hold("ONBED", onbed, 14000)}, 4000},
		{"EBBR ILS 25R", 156, []Leg{climb(700), vectors(below, 3000)}, 3000},
		{"EBBR ILS 25L X", 209, []Leg{climb(700), vectors(below, 4000)}, 4000},
		{"EBBR ILS 01", 227, []Leg{climb(1500), vectors(below, 4000)}, 4000},
		{"EBBR RNAV 07L", 171, []Leg{climb(800), vectors(at, 4000)}, 4000},
		{"KNPA ILS 07L Y", 76, []Leg{climb(600), direct("CUPER", onbed, 1500), hold("CUPER", onbed, 11000)}, 1500},
		{"a hold at another fix than the minimum", 64, []Leg{direct("ONBED", onbed, 4000), hold("NEXT", next, 14000)}, 4000},
		{"the lower of two ceilings", 156, []Leg{climb(700), direct("ONBED", onbed, 0), hold("ONBED", onbed, 4000), hold("NEXT", next, 14000)}, 4000},
		{"only a ceiling", 156, []Leg{vectors(below, 4000)}, 4000},
		{"nothing published", 156, []Leg{direct("ONBED", onbed, 0)}, 0},
	}
	for _, c := range cases {
		p := Procedures{Approaches: []Approach{{Name: "RNAV 10", Runway: "10", Missed: c.legs,
			Final: []Leg{{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_TF, Fix: "RW10", FixKind: "R",
				Position: LatLon{Lat: 18.04, Lon: -63.11}, AltDesc: at, Alt1: c.endFt * ft}}}}}
		m, err := p.MissedOf("RNAV 10")
		if err != nil || math.Abs(m.AltitudeFt-c.want) > 1 {
			t.Errorf("%s: climbs to %.0f ft, want %.0f (%v)", c.name, m.AltitudeFt, c.want, err)
		}
	}
}
