//go:build windows
// +build windows

package systems

import (
	"math"
	"strconv"
)

// Take-off speeds (for a copilot's calls): read from the aircraft where a model
// gives them (the V-speeds its crew entered in the FMS), else from the
// profile's table by the flaps handle and the weight; none, 0.
const (
	V1         = "v1Kt"       // V1, knots, from the FMS (a model's profile)
	VR         = "vrKt"       // VR, knots
	V2         = "v2Kt"       // V2, knots
	DA         = "daFt"       // decision altitude, feet (0 none)
	MDA        = "mdaFt"      // minimum descent altitude, feet (0 none)
	Weight     = "weightKg"   // total weight, kilograms
	FlapsIndex = "flapsIndex" // flaps handle position (FLAPS HANDLE INDEX)
)

// Where State's take-off speeds come from.
const (
	SpeedsFMS   = "fms"   // read from the aircraft
	SpeedsTable = "table" // the profile's table
)

// SpeedTable is a type's take-off speeds by flaps handle position (its
// FLAPS HANDLE INDEX, "1") and total weight: V1, VR and V2 in knots at each
// of WeightsKg, interpolated between them and held at the ends.
type SpeedTable struct {
	Measured  string              `json:"measured,omitempty"` // where the figures come from
	WeightsKg []float64           `json:"weightsKg"`
	Flaps     map[string]SpeedRow `json:"flaps"`
}

// SpeedRow is V1, VR and V2 (knots) at each weight of the table.
type SpeedRow struct {
	V1 []float64 `json:"v1"`
	VR []float64 `json:"vr"`
	V2 []float64 `json:"v2"`
}

// Speeds are V1, VR and V2 for flaps (the handle position) and kg; false
// with no row for the flaps, no weight, or a row not as long as WeightsKg.
func (t SpeedTable) Speeds(flaps int, kg float64) (v1, vr, v2 float64, ok bool) {
	row, found := t.Flaps[strconv.Itoa(flaps)]
	n := len(t.WeightsKg)
	if !found || kg <= 0 || n == 0 || len(row.V1) != n || len(row.VR) != n || len(row.V2) != n {
		return 0, 0, 0, false
	}
	at := func(vs []float64) float64 {
		if kg <= t.WeightsKg[0] {
			return vs[0]
		}
		for i := 1; i < n; i++ {
			if kg <= t.WeightsKg[i] {
				f := (kg - t.WeightsKg[i-1]) / (t.WeightsKg[i] - t.WeightsKg[i-1])
				return math.Round(vs[i-1] + f*(vs[i]-vs[i-1]))
			}
		}
		return vs[n-1]
	}
	return at(row.V1), at(row.VR), at(row.V2), true
}

// takeoffSpeeds fills s's take-off speeds: the aircraft's own when p reads
// them and V1 is set, else p's table.
func takeoffSpeeds(p Profile, s *State) {
	s.SpeedCheckKt = p.SpeedCheckKt
	s.DAFt, s.MDAFt = s.Values[DA], s.Values[MDA]
	if _, ok := p.Values[V1]; ok && s.Values[V1] > 0 {
		s.V1Kt, s.VRKt, s.V2Kt, s.SpeedsFrom = s.Values[V1], s.Values[VR], s.Values[V2], SpeedsFMS
		return
	}
	if p.TakeoffSpeeds == nil {
		return
	}
	if v1, vr, v2, ok := p.TakeoffSpeeds.Speeds(int(math.Round(s.Values[FlapsIndex])), s.Values[Weight]); ok {
		s.V1Kt, s.VRKt, s.V2Kt, s.SpeedsFrom = v1, vr, v2, SpeedsTable
	}
}
