package nav

import (
	"maps"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Performance is the planning data of an aircraft type: typical, not a
// particular airframe's. Speeds are true airspeeds in knots, rates feet per
// minute, fuel kilograms.
type Performance struct {
	Type string `json:"type"` // ICAO type designator; "" for the generic jet
	// CruiseTASKts is the cruise speed; ClimbTASKts and DescentTASKts the
	// mean speeds in the climb and the descent.
	CruiseTASKts  float64 `json:"cruiseTASKts"`
	ClimbTASKts   float64 `json:"climbTASKts"`
	DescentTASKts float64 `json:"descentTASKts"`
	// MaxFL is the highest flight level planned.
	MaxFL int `json:"maxFL"`
	// ClimbFPM and DescentFPM are mean rates over the whole climb and
	// descent.
	ClimbFPM   float64 `json:"climbFPM"`
	DescentFPM float64 `json:"descentFPM"`
	// DescentMach and DescentIASKts are the descent speeds: Mach from
	// cruise, then IAS down to FL100 (250 kt below it); 0 not given
	// (a turboprop has no Mach phase). Rounded, checked against the
	// EUROCONTROL Aircraft Performance Database (indicative figures, #693).
	DescentMach   float64 `json:"descentMach,omitempty"`
	DescentIASKts float64 `json:"descentIASKts,omitempty"`
	// BurnKgH is the cruise fuel flow; a climb burns about half as much
	// again (see Plan).
	BurnKgH float64 `json:"burnKgH"`
	TaxiKg  float64 `json:"taxiKg"`
}

// performances are the known types, rounded typical figures. MaxFL is the
// highest level a flight of the type is normally planned at, below its
// certified ceiling: an airliner seldom files above FL370–FL390 in
// normal operations (a 737 certified to FL410 planned there was too
// high), a business jet flies higher, a turboprop far lower.
var performances = map[string]Performance{
	// Airliners.
	"A319": {CruiseTASKts: 450, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 370, ClimbFPM: 2200, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2300, TaxiKg: 150},
	"A20N": {CruiseTASKts: 450, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 380, ClimbFPM: 2200, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2200, TaxiKg: 150},
	"A320": {CruiseTASKts: 450, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 370, ClimbFPM: 2000, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2500, TaxiKg: 200},
	"A321": {CruiseTASKts: 450, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 360, ClimbFPM: 1800, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2800, TaxiKg: 200},
	"A21N": {CruiseTASKts: 450, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 370, ClimbFPM: 1900, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2400, TaxiKg: 180},
	"B738": {CruiseTASKts: 453, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 380, ClimbFPM: 2000, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 280, BurnKgH: 2500, TaxiKg: 200},
	"B739": {CruiseTASKts: 453, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 370, ClimbFPM: 1900, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 280, BurnKgH: 2600, TaxiKg: 200},
	"B38M": {CruiseTASKts: 453, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 380, ClimbFPM: 2100, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2200, TaxiKg: 180},
	"BCS3": {CruiseTASKts: 445, ClimbTASKts: 290, DescentTASKts: 280, MaxFL: 370, ClimbFPM: 2300, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 280, BurnKgH: 1700, TaxiKg: 120},
	"E190": {CruiseTASKts: 440, ClimbTASKts: 290, DescentTASKts: 280, MaxFL: 370, ClimbFPM: 2200, DescentFPM: 1800, DescentMach: 0.75, DescentIASKts: 250, BurnKgH: 1800, TaxiKg: 120},
	"E195": {CruiseTASKts: 440, ClimbTASKts: 290, DescentTASKts: 280, MaxFL: 370, ClimbFPM: 2100, DescentFPM: 1800, DescentMach: 0.75, DescentIASKts: 250, BurnKgH: 1900, TaxiKg: 120},
	"CRJ9": {CruiseTASKts: 450, ClimbTASKts: 290, DescentTASKts: 280, MaxFL: 370, ClimbFPM: 2200, DescentFPM: 1800, DescentMach: 0.72, DescentIASKts: 290, BurnKgH: 1600, TaxiKg: 100},
	"A332": {CruiseTASKts: 470, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 370, ClimbFPM: 1800, DescentFPM: 1800, DescentMach: 0.82, DescentIASKts: 300, BurnKgH: 5800, TaxiKg: 400},
	"A333": {CruiseTASKts: 470, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 370, ClimbFPM: 1700, DescentFPM: 1800, DescentMach: 0.82, DescentIASKts: 300, BurnKgH: 6000, TaxiKg: 400},
	"A359": {CruiseTASKts: 488, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 390, ClimbFPM: 2000, DescentFPM: 1800, DescentMach: 0.85, DescentIASKts: 300, BurnKgH: 5800, TaxiKg: 400},
	"B763": {CruiseTASKts: 470, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 370, ClimbFPM: 1800, DescentFPM: 1800, DescentMach: 0.80, DescentIASKts: 290, BurnKgH: 4800, TaxiKg: 300},
	"B744": {CruiseTASKts: 490, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 350, ClimbFPM: 1600, DescentFPM: 1800, DescentMach: 0.85, DescentIASKts: 300, BurnKgH: 10500, TaxiKg: 700},
	"B77W": {CruiseTASKts: 490, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 370, ClimbFPM: 1800, DescentFPM: 1800, DescentMach: 0.84, DescentIASKts: 300, BurnKgH: 7500, TaxiKg: 600},
	"B788": {CruiseTASKts: 488, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 390, ClimbFPM: 2100, DescentFPM: 1800, DescentMach: 0.85, DescentIASKts: 300, BurnKgH: 5000, TaxiKg: 350},
	"B789": {CruiseTASKts: 488, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 390, ClimbFPM: 2000, DescentFPM: 1800, DescentMach: 0.85, DescentIASKts: 300, BurnKgH: 5500, TaxiKg: 400},
	// Regional turboprops.
	"AT72": {CruiseTASKts: 270, ClimbTASKts: 200, DescentTASKts: 240, MaxFL: 230, ClimbFPM: 1200, DescentFPM: 1500, BurnKgH: 700, TaxiKg: 50},
	"AT76": {CruiseTASKts: 275, ClimbTASKts: 200, DescentTASKts: 240, MaxFL: 230, ClimbFPM: 1300, DescentFPM: 1500, BurnKgH: 650, TaxiKg: 50},
	"DH8D": {CruiseTASKts: 330, ClimbTASKts: 220, DescentTASKts: 260, MaxFL: 250, ClimbFPM: 1500, DescentFPM: 1600, DescentIASKts: 270, BurnKgH: 1000, TaxiKg: 60},
	// Business jets: above the airliners, from FL410 the levels 4,000 ft
	// apart (LevelRule).
	"E50P": {CruiseTASKts: 390, ClimbTASKts: 250, DescentTASKts: 260, MaxFL: 370, ClimbFPM: 2500, DescentFPM: 2000, DescentMach: 0.65, DescentIASKts: 240, BurnKgH: 350, TaxiKg: 20},
	"E55P": {CruiseTASKts: 440, ClimbTASKts: 270, DescentTASKts: 280, MaxFL: 410, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.74, DescentIASKts: 250, BurnKgH: 550, TaxiKg: 30},
	"E545": {CruiseTASKts: 460, ClimbTASKts: 280, DescentTASKts: 290, MaxFL: 430, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.78, DescentIASKts: 260, BurnKgH: 650, TaxiKg: 40},
	"E550": {CruiseTASKts: 460, ClimbTASKts: 280, DescentTASKts: 290, MaxFL: 430, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.78, DescentIASKts: 260, BurnKgH: 750, TaxiKg: 40},
	// Learjet 25D: cruise up to 464 kt, ceiling FL450 (Wikipedia, "Learjet
	// 25"); climb, descent and burn estimates.
	"LJ25": {CruiseTASKts: 440, ClimbTASKts: 280, DescentTASKts: 290, MaxFL: 450, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.75, DescentIASKts: 260, BurnKgH: 900, TaxiKg: 40},
	"C25B": {CruiseTASKts: 410, ClimbTASKts: 260, DescentTASKts: 270, MaxFL: 410, ClimbFPM: 2800, DescentFPM: 2000, DescentMach: 0.70, DescentIASKts: 250, BurnKgH: 450, TaxiKg: 30},
	"C25C": {CruiseTASKts: 430, ClimbTASKts: 270, DescentTASKts: 280, MaxFL: 410, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.72, DescentIASKts: 250, BurnKgH: 500, TaxiKg: 30},
	"C56X": {CruiseTASKts: 430, ClimbTASKts: 270, DescentTASKts: 280, MaxFL: 410, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.72, DescentIASKts: 250, BurnKgH: 650, TaxiKg: 40},
	"C680": {CruiseTASKts: 450, ClimbTASKts: 280, DescentTASKts: 290, MaxFL: 430, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.75, DescentIASKts: 260, BurnKgH: 700, TaxiKg: 40},
	"C68A": {CruiseTASKts: 440, ClimbTASKts: 280, DescentTASKts: 290, MaxFL: 430, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.75, DescentIASKts: 260, BurnKgH: 650, TaxiKg: 40},
	"C700": {CruiseTASKts: 470, ClimbTASKts: 290, DescentTASKts: 290, MaxFL: 430, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.80, DescentIASKts: 270, BurnKgH: 800, TaxiKg: 50},
	"C750": {CruiseTASKts: 500, ClimbTASKts: 300, DescentTASKts: 300, MaxFL: 450, ClimbFPM: 3500, DescentFPM: 2500, DescentMach: 0.85, DescentIASKts: 270, BurnKgH: 1100, TaxiKg: 60},
	"PC24": {CruiseTASKts: 425, ClimbTASKts: 270, DescentTASKts: 280, MaxFL: 410, ClimbFPM: 3000, DescentFPM: 2000, DescentMach: 0.72, DescentIASKts: 250, BurnKgH: 550, TaxiKg: 30},
	// Business turboprops: below the airliners.
	"PC12": {CruiseTASKts: 270, ClimbTASKts: 180, DescentTASKts: 230, MaxFL: 260, ClimbFPM: 1500, DescentFPM: 1500, DescentIASKts: 220, BurnKgH: 230, TaxiKg: 15},
	"TBM9": {CruiseTASKts: 320, ClimbTASKts: 200, DescentTASKts: 260, MaxFL: 280, ClimbFPM: 1800, DescentFPM: 1800, DescentIASKts: 230, BurnKgH: 220, TaxiKg: 15},
	"B350": {CruiseTASKts: 300, ClimbTASKts: 210, DescentTASKts: 260, MaxFL: 270, ClimbFPM: 2000, DescentFPM: 1800, DescentIASKts: 240, BurnKgH: 350, TaxiKg: 20},
	"BE20": {CruiseTASKts: 280, ClimbTASKts: 200, DescentTASKts: 250, MaxFL: 250, ClimbFPM: 1800, DescentFPM: 1800, DescentIASKts: 230, BurnKgH: 300, TaxiKg: 20},
}

// genericPerformance is a medium twin jet, for unknown types.
var genericPerformance = Performance{CruiseTASKts: 440, ClimbTASKts: 290, DescentTASKts: 280, MaxFL: 370,
	ClimbFPM: 2000, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2500, TaxiKg: 200}

// PerformanceFor returns the planning data of an ICAO type designator
// ("A20N", "b738"); unknown types get a generic medium jet with Type "".
func PerformanceFor(icaoType string) Performance {
	t := strings.ToUpper(strings.TrimSpace(icaoType))
	p, ok := performancesNow.Load()[t]
	if !ok {
		return genericPerformance
	}
	p.Type = t
	return p
}

// The performance table, replaceable at runtime (pkg/dict, #768).
var performancesNow dict.Value[map[string]Performance]

func init() {
	dict.Register(dict.Keyed("nav.performance", "type", "", "",
		func() []Performance {
			var out []Performance
			for _, t := range slices.Sorted(maps.Keys(performances)) {
				p := performances[t]
				p.Type = t
				out = append(out, p)
			}
			return out
		}, func(p Performance) string { return strings.ToUpper(p.Type) },
		func(items []Performance) {
			m := map[string]Performance{}
			for _, p := range items {
				m[strings.ToUpper(p.Type)] = p
			}
			performancesNow.Store(m)
		}))
	_ = dict.Reset("nav.performance")
}
