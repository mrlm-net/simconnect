package airport

import (
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/dict"
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
	// Tower is where the control tower stands, when the facility data puts
	// it elsewhere (its TOWER_* fields); nil: the facility's.
	Tower *TowerSite
	// Tugs, FuelTrucks, Stairs and GPUs are the airport's pushback tugs,
	// fuel trucks, boarding stairs and ground power units (#830–#832); 0:
	// sized by its stands (traffic.DefaultFleetSize).
	Tugs, FuelTrucks, Stairs, GPUs int
}

// TowerSite is a control tower: its position and the height of its cab
// above the ground (0: unknown).
type TowerSite struct {
	Position LatLon
	CabM     float64
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
	// LKPR tower: the position the user gave (2026-10-01); the facility's
	// TOWER_LATITUDE/LONGITUDE (50.10183, 14.25732) is not the tower.
	"LKPR": {TransitionAltitudeFt: 5000, PreferredRunways: []string{"24", "06"},
		Tower: &TowerSite{Position: LatLon{Lat: 50.1065908, Lon: 14.2694592}}},
	"EDDF": {TransitionAltitudeFt: 5000},
	// EDDM (#376), from AIP Germany (DFS AIP IFR, effective 01 OCT 2026,
	// read October 2026). No preferential runways: AD 2.20 3.1.4 and 3.2.1
	// assign runways by the arrival fix and the departure direction; the
	// de-icing areas are only on charts AD 2 EDDM 2-5 and 2-7 (AD 2.20 8).
	"EDDM": {
		TransitionAltitudeFt: 5000, // EDDM AD 2.17 item 5 (AD 2 EDDM 1-11): 5000 ft MSL
		InitialClimbFt:       7000, // AD 2 EDDM 5-7-1 to 5-7-48, every SID: "Climb to FL 70"
		NoReverseThrust:      true, // EDDM AD 2.20 2.3: reverse thrust only as needed for safety, idle reverse allowed
	},
	// LOWW (#376), from AIP Austria (Austro Control eAIP, effective 01 OCT
	// 2026, read October 2026). No preferential runways here: AD 2.20 6.2
	// gives arrival and departure runways by time of day and wind (by day,
	// westerly: arrivals 34, departures 29), which one list cannot hold.
	"LOWW": {
		TransitionAltitudeFt: 10000, // LOWW AD 2.17 item 5: 3050 m (10000 ft) AMSL
		InitialClimbFt:       5000,  // LOWW AD 2 MAP 9-1-1 to 9-4-2, every SID: "Climb to ..initially 5000 FT MSL"
		NoReverseThrust:      true,  // LOWW AD 2.21 2.6: no more than idle reverse except for safety/operational reasons
	},
	// EGLL (#376), from UK AIP (NATS eAIP, AIRAC 01 OCT 2026, read October
	// 2026). Reverse thrust is only to be avoided at night (AD 2.21 note 5,
	// 2330–0600), so NoReverseThrust stays off; the remote de-icing areas
	// are only on chart AD 2-EGLL-2-8.
	"EGLL": {
		TransitionAltitudeFt: 6000,                   // EGLL AD 2.17 item 5: 6000 ft
		PreferredRunways:     []string{"27R", "27L"}, // EGLL AD 2.20 6: 27R and 27L preferred to 09R/09L (tailwind up to 5 kt, dry)
		InitialClimbFt:       6000,                   // AD 2-EGLL-6-1 to 6-6, every SID: "do not climb above 6000 until cleared by ATC"
	},
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
	lim := knownLimitsNow.Load()[icao]
	lim.ICAO = icao
	lim.PreferredRunways = slices.Clone(lim.PreferredRunways)
	lim.DeicingPads = slices.Clone(lim.DeicingPads)
	// Deep: a caller changing its copy must not change the table (E19).
	lim.InitialClimbs = maps.Clone(lim.InitialClimbs)
	if lim.Tower != nil {
		t := *lim.Tower
		lim.Tower = &t
	}
	if lim.TransitionAltitudeFt == 0 && l != nil && l.TransitionAltitude > 0 {
		// The sim's own (the AIRPORT record): every airport, not a table.
		lim.TransitionAltitudeFt = math.Round(convert.MetersToFeet(l.TransitionAltitude)/100) * 100
	}
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

// The airports' limits, replaceable at runtime (pkg/dict, #768): items are
// Limits by ICAO, with the field names as they are (no JSON tags).
var knownLimitsNow dict.Value[map[string]Limits]

func init() {
	dict.Register(dict.Keyed("airport.limits", "ICAO", "", "",
		func() []Limits {
			var out []Limits
			for _, k := range slices.Sorted(maps.Keys(KnownLimits)) {
				l := KnownLimits[k]
				l.ICAO = k
				out = append(out, l)
			}
			return out
		}, func(l Limits) string { return strings.ToUpper(l.ICAO) },
		func(items []Limits) {
			m := map[string]Limits{}
			for _, l := range items {
				m[strings.ToUpper(l.ICAO)] = l
			}
			knownLimitsNow.Store(m)
		}))
	_ = dict.Reset("airport.limits")
}
