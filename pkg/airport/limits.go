//go:build windows
// +build windows

package airport

import (
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Defaults for Limits (#335).
const (
	// DefaultTransitionAltitudeFt is the transition altitude outside
	// KnownLimits; USTransitionAltitudeFt applies in the US and Canada
	// (K… and C… ICAO codes).
	DefaultTransitionAltitudeFt = 5000.0
	USTransitionAltitudeFt      = 18000.0
	// DefaultClimbHandoverFt is the climb-out hand-over height above the
	// field when no SID gives an initial climb; MinClimbHandoverFt is the
	// lowest one taken from a SID: a SID climbing higher first raises it,
	// one that levels off lower does not bring MSFS AI in earlier than the
	// live-tested hand-over.
	DefaultClimbHandoverFt = 1500.0
	MinClimbHandoverFt     = 1500.0
	// DefaultTaxiMaxKts and DefaultApronMaxKts cap the taxi speed on
	// taxiways and on apron taxilanes.
	DefaultTaxiMaxKts  = 30.0
	DefaultApronMaxKts = 15.0
)

// Limits are the values that belong to an airport rather than to an
// aircraft or a global constant (#335): taken from the facility data where
// it has them (the SIDs' initial climb) and from KnownLimits otherwise.
// Controllers combine them with the aircraft profile.
type Limits struct {
	ICAO string
	// TransitionAltitudeFt is the transition altitude (feet MSL).
	TransitionAltitudeFt float64
	// ClimbHandoverFt is the height above the field at which an injected
	// take-off hands the aircraft to MSFS AI for the climb-out.
	ClimbHandoverFt float64
	// MSAFt is the minimum sector altitude (feet MSL); 0 when unknown.
	MSAFt float64
	// TaxiMaxKts and ApronMaxKts cap the taxi speed on taxiways and on
	// apron taxilanes (the edges at a stand's junction, Graph.Apron).
	TaxiMaxKts, ApronMaxKts float64
	// PreferredRunways are the preferential runway ends in order, e.g.
	// "24", "06" at LKPR; empty when the airport has none.
	PreferredRunways []string
	// NoReverseThrust restricts reverse thrust to idle (noise).
	NoReverseThrust bool
	// DeicingPads are the remote de-icing positions (#323); none: aircraft
	// are de-iced on their stands.
	DeicingPads []DeicingPad
	// InitialClimbFt is the level a departure clearance climbs to ("climb
	// via SID to flight level 100"); 0 takes DefaultInitialClimbFt (MSFS
	// SID data carries no usable altitude for it).
	InitialClimbFt float64
	// InitialClimbs are initial climbs of single SIDs, by name ("BALT7D"),
	// where they differ from InitialClimbFt.
	InitialClimbs map[string]float64
}

// InitialClimbFor is the initial climb of a departure clearance on sid:
// its own (InitialClimbs), else the airport's (InitialClimbFt), else
// DefaultInitialClimbFt.
func (l Limits) InitialClimbFor(sid string) float64 {
	if ft := l.InitialClimbs[strings.ToUpper(sid)]; ft > 0 {
		return ft
	}
	if l.InitialClimbFt > 0 {
		return l.InitialClimbFt
	}
	return DefaultInitialClimbFt
}

// DefaultInitialClimbFt is the initial climb of a departure clearance where
// an airport has none of its own: FL100.
const DefaultInitialClimbFt = 10000.0

// KnownLimits are published values of airports, keyed by ICAO code. Zero
// fields take the defaults (or the facility data); LimitsFor fills the rest.
var KnownLimits = map[string]Limits{
	"LKPR": {TransitionAltitudeFt: 5000, PreferredRunways: []string{"24", "06"}},
	"EDDF": {TransitionAltitudeFt: 5000},
	"EDDM": {TransitionAltitudeFt: 5000},
	"LOWW": {TransitionAltitudeFt: 10000},
	"EGLL": {TransitionAltitudeFt: 6000},
	"LFPG": {TransitionAltitudeFt: 5000},
	"EHAM": {TransitionAltitudeFt: 3000},
	"EPWA": {TransitionAltitudeFt: 6500},
	"LSZH": {TransitionAltitudeFt: 7000},
}

// LimitsFor returns the limits of the airport of l: KnownLimits for its
// ICAO code, the climb-out hand-over from the SIDs' initial climb in p (nil
// without procedures) and defaults for the rest. l may be nil when p is set.
func LimitsFor(l *Layout, p *Procedures) Limits {
	icao, elevFt := "", 0.0
	if l != nil {
		icao, elevFt = l.ICAO, convert.MetersToFeet(l.Altitude)
	}
	if icao == "" && p != nil {
		icao = p.ICAO
	}
	icao = strings.ToUpper(strings.TrimSpace(icao))
	lim := KnownLimits[icao]
	lim.ICAO = icao
	lim.PreferredRunways = slices.Clone(lim.PreferredRunways)
	lim.DeicingPads = slices.Clone(lim.DeicingPads)
	if lim.TransitionAltitudeFt == 0 {
		lim.TransitionAltitudeFt = DefaultTransitionAltitudeFt
		if strings.HasPrefix(icao, "K") || strings.HasPrefix(icao, "C") {
			lim.TransitionAltitudeFt = USTransitionAltitudeFt
		}
	}
	if lim.ClimbHandoverFt == 0 {
		lim.ClimbHandoverFt = DefaultClimbHandoverFt
		if alt := initialClimbFt(p); alt > 0 && l != nil {
			lim.ClimbHandoverFt = max(MinClimbHandoverFt, alt-elevFt)
		}
	}
	if lim.InitialClimbFt == 0 {
		lim.InitialClimbFt = DefaultInitialClimbFt
	}
	if lim.TaxiMaxKts == 0 {
		lim.TaxiMaxKts = DefaultTaxiMaxKts
	}
	if lim.ApronMaxKts == 0 {
		lim.ApronMaxKts = DefaultApronMaxKts
	}
	return lim
}

// initialClimbFt is the highest initial climb (feet MSL) of the SIDs in p:
// the altitude of a course, heading or fix to an altitude leg (CA, VA, FA)
// that starts a runway transition, or the common route of a SID without
// one; 0 when none does.
func initialClimbFt(p *Procedures) float64 {
	if p == nil {
		return 0
	}
	best := 0.0
	first := func(legs []Leg) {
		if len(legs) == 0 {
			return
		}
		switch lg := legs[0]; lg.Type {
		case types.SIMCONNECT_FACILITY_LEG_TYPE_CA, types.SIMCONNECT_FACILITY_LEG_TYPE_VA, types.SIMCONNECT_FACILITY_LEG_TYPE_FA:
			best = max(best, convert.MetersToFeet(lg.Alt1))
		}
	}
	for _, d := range p.Departures {
		if len(d.RunwayTransitions) == 0 {
			first(d.Legs)
		}
		for _, t := range d.RunwayTransitions {
			first(t.Legs)
		}
	}
	return best
}
