//go:build windows
// +build windows

package airport

import (
	"math"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestLimitsForLKPR: LKPR's transition altitude and preferential runways
// from KnownLimits, the climb-out hand-over from the SIDs' initial climb
// (CA to 1700 ft MSL; the field is at about 1200 ft, so the floor applies).
func TestLimitsForLKPR(t *testing.T) {
	l := loadLKPR(t)
	p := loadLKPRProcedures(t)
	lim := LimitsFor(l, &p)
	if lim.ICAO != "LKPR" || lim.TransitionAltitudeFt != 5000 {
		t.Errorf("ICAO %q TA %.0f, want LKPR 5000", lim.ICAO, lim.TransitionAltitudeFt)
	}
	if !slices.Equal(lim.PreferredRunways, []string{"24", "06"}) {
		t.Errorf("preferred %v, want [24 06]", lim.PreferredRunways)
	}
	if ic := initialClimbFt(&p); math.Abs(ic-1700) > 1 {
		t.Errorf("initial climb %.0f ft, want 1700", ic)
	}
	want := max(MinClimbHandoverFt, 1700-convert.MetersToFeet(l.Altitude))
	if math.Abs(lim.ClimbHandoverFt-want) > 1 {
		t.Errorf("hand-over %.0f ft, want %.0f", lim.ClimbHandoverFt, want)
	}
	if lim.TaxiMaxKts != DefaultTaxiMaxKts || lim.ApronMaxKts != DefaultApronMaxKts {
		t.Errorf("taxi %.0f / apron %.0f kt", lim.TaxiMaxKts, lim.ApronMaxKts)
	}
	lim.PreferredRunways[0] = "30"
	if KnownLimits["LKPR"].PreferredRunways[0] != "24" {
		t.Error("LimitsFor shares PreferredRunways with KnownLimits")
	}
}

// TestLimitsForDefaults: without procedures the hand-over is the default;
// the transition altitude comes from the table or the region.
func TestLimitsForDefaults(t *testing.T) {
	cases := []struct {
		icao string
		ta   float64
	}{{"LOWW", 10000}, {"EHAM", 3000}, {"KJFK", 18000}, {"CYYZ", 18000}, {"LZIB", 5000}}
	for _, c := range cases {
		lim := LimitsFor(&Layout{ICAO: c.icao}, nil)
		if lim.TransitionAltitudeFt != c.ta || lim.ClimbHandoverFt != DefaultClimbHandoverFt {
			t.Errorf("%s: TA %.0f hand-over %.0f, want %.0f %.0f", c.icao, lim.TransitionAltitudeFt, lim.ClimbHandoverFt, c.ta, DefaultClimbHandoverFt)
		}
	}
	// A high initial climb hands over at its height above the field.
	p := &Procedures{Departures: []Procedure{{Name: "TEST1A", RunwayTransitions: []Transition{{Runway: "09", Legs: []Leg{
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_CA, Alt1: convert.FeetToMeters(3000)}}}}}}}
	lim := LimitsFor(&Layout{ICAO: "ZZZZ", Altitude: convert.FeetToMeters(500)}, p)
	if math.Abs(lim.ClimbHandoverFt-2500) > 1 {
		t.Errorf("hand-over %.0f ft, want 2500", lim.ClimbHandoverFt)
	}
}
