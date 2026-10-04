package traffic

import (
	"math"
	"sort"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Airborne separation (#395): how far apart airborne aircraft are,
// laterally and vertically, and which pairs are closer than a minimum in
// both at once (separated by either is separated).

// Separation minima: radar separation in a terminal area (3 NM, ICAO Doc
// 4444 §8.7.3), the 5 NM many units keep before the final and en route,
// and 1000 ft vertically.
const (
	TerminalSeparationNM = 3.0
	EnrouteSeparationNM  = 5.0
	VerticalSeparationFt = 1000.0
)

// TowerBelowFt: below this height above the ground at their airport,
// departures and arrivals are separated by the tower — on the runway, by
// the departure interval and the arrival's distance (RunwayController) —
// not by the radar minima.
const TowerBelowFt = 2500.0

// TowerPair reports two aircraft the tower separates: at the same airport,
// one of them below TowerBelowFt (a departure just airborne ahead of an
// arrival on final is mixed-mode runway use, not a loss of separation).
func TowerPair(a, b TrackedAircraft) bool {
	return a.Airport != "" && a.Airport == b.Airport && (a.AGLFt < TowerBelowFt || b.AGLFt < TowerBelowFt)
}

// SeparationPair is two airborne aircraft and how far apart they are.
type SeparationPair struct {
	A, B       string  // call signs (tail, else title)
	LateralNM  float64 `json:"lateralNM"`
	VerticalFt float64 `json:"verticalFt"`
	// Loss: closer than the minima in both, and not a TowerPair.
	Loss bool `json:"loss"`
	// Tower: the tower separates them (TowerPair).
	Tower bool `json:"tower,omitempty"`
	// MinNM: the lateral minimum that applied to the pair.
	MinNM float64 `json:"minNM"`
}

// AirborneSeparation lists the pairs of airborne aircraft, closest first,
// marking those closer than minNM and minFt at once.
func AirborneSeparation(aircraft []TrackedAircraft, minNM, minFt float64) []SeparationPair {
	return airborneSeparation(aircraft, func(TrackedAircraft, TrackedAircraft) float64 { return minNM }, minFt)
}

// AirborneSeparationFor is AirborneSeparation with the minima of o, as
// PredictConflicts applies them: TerminalNM (3 NM) where both are in a
// terminal area, MinNM (5 NM) elsewhere, MinFt vertically.
func AirborneSeparationFor(aircraft []TrackedAircraft, o ConflictOptions) []SeparationPair {
	o = o.withDefaults()
	return airborneSeparation(aircraft, o.minFor, o.MinFt)
}

func airborneSeparation(aircraft []TrackedAircraft, minFor func(a, b TrackedAircraft) float64, minFt float64) []SeparationPair {
	var air []TrackedAircraft
	for _, a := range aircraft {
		if !a.OnGround {
			air = append(air, a)
		}
	}
	name := func(a TrackedAircraft) string {
		if a.Tail != "" {
			return a.Tail
		}
		return a.Title
	}
	var out []SeparationPair
	for i := 0; i < len(air); i++ {
		for j := i + 1; j < len(air); j++ {
			a, b := air[i], air[j]
			l := calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon)
			v := math.Abs(a.AltFt - b.AltFt)
			minNM := minFor(a, b)
			loss := l < minNM && v < minFt && !TowerPair(a, b)
			out = append(out, SeparationPair{A: name(a), B: name(b), LateralNM: l, VerticalFt: v, Loss: loss, Tower: TowerPair(a, b), MinNM: minNM})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LateralNM < out[j].LateralNM })
	return out
}
