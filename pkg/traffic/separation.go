//go:build windows
// +build windows

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

// SeparationPair is two airborne aircraft and how far apart they are.
type SeparationPair struct {
	A, B       string  // call signs (tail, else title)
	LateralNM  float64 `json:"lateralNM"`
	VerticalFt float64 `json:"verticalFt"`
	// Loss: closer than the minima in both.
	Loss bool `json:"loss"`
}

// AirborneSeparation lists the pairs of airborne aircraft, closest first,
// marking those closer than minNM and minFt at once.
func AirborneSeparation(aircraft []TrackedAircraft, minNM, minFt float64) []SeparationPair {
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
			out = append(out, SeparationPair{A: name(a), B: name(b), LateralNM: l, VerticalFt: v, Loss: l < minNM && v < minFt})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LateralNM < out[j].LateralNM })
	return out
}
