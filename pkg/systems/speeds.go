//go:build windows
// +build windows

package systems

import (
	_ "embed"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
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

// Speeds are V1, VR and V2 for flaps (the handle position; the "*" row for
// a position without its own) and kg; false with no row for the flaps, no
// weight, or a row not as long as WeightsKg.
func (t SpeedTable) Speeds(flaps int, kg float64) (v1, vr, v2 float64, ok bool) {
	row, found := t.Flaps[strconv.Itoa(flaps)]
	if !found {
		row, found = t.Flaps["*"]
	}
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
	if s.SpeedCheckKt == 0 {
		s.SpeedCheckKt = DefaultSpeedCheckKt
	}
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

// DefaultSpeedCheckKt is the take-off roll's speed check of a type that
// gives none: 80 kt, Boeing-style procedures.
const DefaultSpeedCheckKt = 80

// SpeedsItem is a type's take-off speeds (dict "systems.speeds", fed from
// the MyCrew API's "aircraft-speeds" set): its ICAO type and other codes it
// goes by, the speed check and the table.
type SpeedsItem struct {
	Type         string   `json:"type"`
	Aliases      []string `json:"aliases,omitempty"`
	SpeedCheckKt int      `json:"speedCheckKt,omitempty"`
	SpeedTable
}

//go:embed speeds.json
var speedsJSON []byte

func shippedSpeeds() []SpeedsItem {
	var out []SpeedsItem
	_ = json.Unmarshal(speedsJSON, &out)
	return out
}

var speedsNow dict.Value[[]SpeedsItem]

func init() {
	dict.Register(dict.Keyed("systems.speeds", "type", "Approximate typical figures, not from manufacturers' documents (see each item's measured)", "",
		shippedSpeeds, func(i SpeedsItem) string { return strings.ToUpper(i.Type) },
		func(items []SpeedsItem) { speedsNow.Store(items) }).ForSet("aircraft-speeds", nil))
	_ = dict.Reset("systems.speeds")
}

// SpeedsFor is the take-off speeds of a's type: by its ATC type (or model),
// else by a type code in its title ("FenixA319": the A319's); false for
// none.
func SpeedsFor(a Aircraft) (SpeedsItem, bool) {
	items := speedsNow.Load()
	codes := func(i SpeedsItem) []string { return append([]string{i.Type}, i.Aliases...) }
	for _, i := range items {
		for _, c := range codes(i) {
			if a.ATCType != "" && strings.EqualFold(a.ATCType, c) {
				return i, true
			}
		}
	}
	title := strings.ToUpper(a.Title)
	for _, i := range items {
		for _, c := range codes(i) {
			if c != "" && strings.Contains(title, strings.ToUpper(c)) {
				return i, true
			}
		}
	}
	return SpeedsItem{}, false
}

// withSpeeds gives p its type's take-off speeds and speed check where p
// itself gives none.
func withSpeeds(p Profile, a Aircraft) Profile {
	i, ok := SpeedsFor(a)
	if !ok {
		return p
	}
	if p.TakeoffSpeeds == nil {
		t := i.SpeedTable
		p.TakeoffSpeeds = &t
	}
	if p.SpeedCheckKt == 0 {
		p.SpeedCheckKt = i.SpeedCheckKt
	}
	return p
}

// UnmarshalJSON reads an item; read onto a shipped one (a set's or a local
// correction), flaps given replace its rows whole, not row by row: rows of
// the old weights would not fit new ones.
func (i *SpeedsItem) UnmarshalJSON(b []byte) error {
	type plain SpeedsItem
	var given struct {
		Flaps json.RawMessage `json:"flaps"`
	}
	if err := json.Unmarshal(b, &given); err != nil {
		return err
	}
	if given.Flaps != nil {
		i.Flaps = nil
	}
	return json.Unmarshal(b, (*plain)(i))
}
