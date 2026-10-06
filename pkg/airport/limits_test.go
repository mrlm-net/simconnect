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

// The initial climb of a departure clearance: FL100 unless the airport
// has its own.
func TestInitialClimb(t *testing.T) {
	for _, icao := range []string{"LKPR", "LFPG", "KJFK"} {
		if got := LimitsFor(&Layout{ICAO: icao}, nil).InitialClimbFt; got != 10000 {
			t.Errorf("%s: %v, want 10000", icao, got)
		}
	}
}

// A SID of its own climb overrides the airport's.
func TestInitialClimbBySID(t *testing.T) {
	l := Limits{InitialClimbFt: 7000, InitialClimbs: map[string]float64{"BALT7D": 5000}}
	if got := l.InitialClimbFor("balt7d"); got != 5000 {
		t.Errorf("BALT7D: %v", got)
	}
	if got := l.InitialClimbFor("VOZ5M"); got != 7000 {
		t.Errorf("VOZ5M: %v", got)
	}
	if got := (Limits{}).InitialClimbFor("X"); got != DefaultInitialClimbFt {
		t.Errorf("none: %v", got)
	}
}

// The published values of the airports validated in #376 (EDDM, LOWW,
// EGLL; sources in KnownLimits): transition altitude, the SIDs' initial
// climb, reverse thrust and preferential runways.
func TestLimitsForValidatedAirports(t *testing.T) {
	for _, c := range []struct {
		icao      string
		ta, climb float64
		noReverse bool
		preferred []string
	}{
		{"EDDM", 5000, 7000, true, nil},
		{"LOWW", 10000, 5000, true, nil},
		{"EGLL", 6000, 6000, false, []string{"27R", "27L"}},
	} {
		lim := LimitsFor(&Layout{ICAO: c.icao}, nil)
		if lim.TransitionAltitudeFt != c.ta || lim.InitialClimbFt != c.climb || lim.NoReverseThrust != c.noReverse || !slices.Equal(lim.PreferredRunways, c.preferred) {
			t.Errorf("%s: TA %.0f, initial climb %.0f, no reverse %v, preferred %v; want %.0f %.0f %v %v", c.icao,
				lim.TransitionAltitudeFt, lim.InitialClimbFt, lim.NoReverseThrust, lim.PreferredRunways, c.ta, c.climb, c.noReverse, c.preferred)
		}
	}
}

// The sim's transition altitude (the AIRPORT record, meters) is used where
// no published value is known; a published one wins (read live, MSFS
// 2024: EHAM 914.4 m, KJFK 5486.4 m).
func TestLimitsTransitionFromSim(t *testing.T) {
	cases := []struct {
		icao   string
		meters float64
		want   float64
	}{
		{"EHAM", 914.4, 3000},
		{"KJFK", 5486.4, 18000},
		{"LOWW", 0, 10000},   // published (AIP), nothing from the sim
		{"LKPR", 3048, 5000}, // published wins over the sim
		{"ZZZZ", 0, DefaultTransitionAltitudeFt},
	}
	for _, c := range cases {
		got := LimitsFor(&Layout{ICAO: c.icao, TransitionAltitude: c.meters}, nil).TransitionAltitudeFt
		if got != c.want {
			t.Errorf("%s: %.0f ft, want %.0f", c.icao, got, c.want)
		}
	}
}
