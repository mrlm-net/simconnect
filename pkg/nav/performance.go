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

// performances are the known types, rounded public figures.
var performances = map[string]Performance{
	"A20N": {CruiseTASKts: 450, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 390, ClimbFPM: 2200, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2200, TaxiKg: 150},
	"A320": {CruiseTASKts: 450, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 390, ClimbFPM: 2000, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2500, TaxiKg: 200},
	"A321": {CruiseTASKts: 450, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 390, ClimbFPM: 1800, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2800, TaxiKg: 200},
	"B738": {CruiseTASKts: 453, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 410, ClimbFPM: 2000, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 280, BurnKgH: 2500, TaxiKg: 200},
	"B38M": {CruiseTASKts: 453, ClimbTASKts: 300, DescentTASKts: 290, MaxFL: 410, ClimbFPM: 2100, DescentFPM: 1800, DescentMach: 0.78, DescentIASKts: 290, BurnKgH: 2200, TaxiKg: 180},
	"B77W": {CruiseTASKts: 490, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 410, ClimbFPM: 1800, DescentFPM: 1800, DescentMach: 0.84, DescentIASKts: 300, BurnKgH: 7500, TaxiKg: 600},
	"B789": {CruiseTASKts: 488, ClimbTASKts: 310, DescentTASKts: 300, MaxFL: 410, ClimbFPM: 2000, DescentFPM: 1800, DescentMach: 0.85, DescentIASKts: 300, BurnKgH: 5500, TaxiKg: 400},
	"E190": {CruiseTASKts: 440, ClimbTASKts: 290, DescentTASKts: 280, MaxFL: 410, ClimbFPM: 2200, DescentFPM: 1800, DescentMach: 0.75, DescentIASKts: 250, BurnKgH: 1800, TaxiKg: 120},
	"CRJ9": {CruiseTASKts: 450, ClimbTASKts: 290, DescentTASKts: 280, MaxFL: 410, ClimbFPM: 2200, DescentFPM: 1800, DescentMach: 0.72, DescentIASKts: 290, BurnKgH: 1600, TaxiKg: 100},
	"AT76": {CruiseTASKts: 275, ClimbTASKts: 200, DescentTASKts: 240, MaxFL: 250, ClimbFPM: 1300, DescentFPM: 1500, BurnKgH: 650, TaxiKg: 50},
	"DH8D": {CruiseTASKts: 330, ClimbTASKts: 220, DescentTASKts: 260, MaxFL: 250, ClimbFPM: 1500, DescentFPM: 1600, DescentIASKts: 270, BurnKgH: 1000, TaxiKg: 60},
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
