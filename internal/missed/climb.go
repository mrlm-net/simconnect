// Package missed holds the rule for the altitude a missed approach climbs
// to, shared by pkg/airport and pkg/traffic.
package missed

import "math"

// CircuitHeightFt is the height over the field a missed approach's minimum
// has to reach to be the climb level by itself.
const CircuitHeightFt = 3000.0

// Limit is one point's altitude constraint in feet, 0 meaning none.
type Limit struct{ MinFt, MaxFt float64 }

// ClimbFt is the altitude a missed approach climbs to: its highest minimum
// (#1659). A ceiling is the climb level only where it has no minimum beside
// it and no minimum reaches circuitFt (EBBR 25R: 700 ft, then at or below
// 3000), then the lowest such ceiling above the minimums. A point with both
// is a window, its top only a limit (TNCM: hold at ONBED between 4000 and
// 14000). 0 when nothing is published.
func ClimbFt(limits []Limit, circuitFt float64) float64 {
	floor := 0.0
	for _, l := range limits {
		floor = math.Max(floor, l.MinFt)
	}
	if floor >= circuitFt {
		return floor
	}
	ceiling := math.Inf(1)
	for _, l := range limits {
		if l.MinFt == 0 && l.MaxFt > floor {
			ceiling = math.Min(ceiling, l.MaxFt)
		}
	}
	if math.IsInf(ceiling, 1) {
		return floor
	}
	return ceiling
}
