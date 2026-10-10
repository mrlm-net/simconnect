package traffic

import (
	"hash/fnv"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Intersection departures (#1030): which departures ground offers an
// intersection to, from which, and how often a crew declines it. Two
// tables, replaceable at runtime like the others (pkg/dict): the rules by
// class of aircraft (traffic.intersectionClasses) and the airports' own
// entries (traffic.intersectionAirports). The shipped figures are the
// project's estimates (the user's, live at EDDM), not from a source.

// Intersection classes of aircraft (IntersectionClass).
const (
	ClassLight     = "light"     // light aircraft (wake L): pistons, small turboprops
	ClassTurboprop = "turboprop" // airliner turboprops: ATR, Dash 8
	ClassRegional  = "regional"  // regional jets: CRJ, E-jets (span under RegionalSpanM)
	ClassJet       = "jet"       // narrowbody jets: A320, 737 family
	ClassHeavy     = "heavy"     // widebodies (wake H, J): the full length always
)

// RegionalSpanM: a jet of a smaller span is a regional jet.
const RegionalSpanM = 30.0

// IntersectionClassItem is a class's rule (traffic.intersectionClasses):
// the runway an intersection must leave ahead of it, or the full length
// only; the share of its crews that decline one offered.
type IntersectionClassItem struct {
	Class          string  `json:"class"`
	MinRemainingM  float64 `json:"minRemainingM"`
	FullLengthOnly bool    `json:"fullLengthOnly,omitempty"`
	DeclineShare   float64 `json:"declineShare"`
}

// IntersectionAirportItem is an airport's own intersection departures
// (traffic.intersectionAirports): per runway end and class the entries
// ground may give (none listed for a class: the generic rules); SayRemaining
// has the taxi clearance say the runway left from the entry.
type IntersectionAirportItem struct {
	ICAO         string                         `json:"icao"`
	Runways      map[string]map[string][]string `json:"runways,omitempty"`
	SayRemaining bool                           `json:"sayRemaining,omitempty"`
}

var shippedIntersectionClasses = []IntersectionClassItem{
	{Class: ClassLight, MinRemainingM: 800, DeclineShare: 0.02},
	{Class: ClassTurboprop, MinRemainingM: 1800, DeclineShare: 0.05},
	{Class: ClassRegional, MinRemainingM: 1800, DeclineShare: 0.08},
	{Class: ClassJet, MinRemainingM: 2500, DeclineShare: 0.12},
	{Class: ClassHeavy, FullLengthOnly: true},
}

// shippedIntersectionAirports: none; the generic rules hold everywhere
// (EDDM's 26L and 26R give a jet only B12 and A12, 2,808 m left).
var shippedIntersectionAirports []IntersectionAirportItem

var (
	intersectionClassesNow  dict.Value[map[string]IntersectionClassItem]
	intersectionAirportsNow dict.Value[map[string]IntersectionAirportItem]
)

func init() {
	dict.Register(dict.Keyed("traffic.intersectionClasses", "class", "", "",
		func() []IntersectionClassItem { return slices.Clone(shippedIntersectionClasses) },
		func(i IntersectionClassItem) string { return strings.ToLower(i.Class) },
		func(items []IntersectionClassItem) {
			m := map[string]IntersectionClassItem{}
			for _, i := range items {
				m[strings.ToLower(i.Class)] = i
			}
			intersectionClassesNow.Store(m)
		}))
	dict.Register(dict.Keyed("traffic.intersectionAirports", "icao", "", "",
		func() []IntersectionAirportItem { return slices.Clone(shippedIntersectionAirports) },
		func(i IntersectionAirportItem) string { return strings.ToUpper(i.ICAO) },
		func(items []IntersectionAirportItem) {
			m := map[string]IntersectionAirportItem{}
			for _, i := range items {
				m[strings.ToUpper(i.ICAO)] = i
			}
			intersectionAirportsNow.Store(m)
		}))
	for _, n := range []string{"traffic.intersectionClasses", "traffic.intersectionAirports"} {
		_ = dict.Reset(n) // the shipped copies in use
	}
}

// IntersectionClass is the intersection class of a model: heavy by wake
// (H, J), light by wake L or a piston, a turboprop, a regional jet below
// RegionalSpanM, else a jet.
func IntersectionClass(model string) string {
	switch WakeFor(model).ICAO {
	case WakeHeavy, WakeSuper:
		return ClassHeavy
	case WakeLight:
		return ClassLight
	}
	p := ProfileFor(model)
	switch {
	case p.Category == CategoryPiston:
		return ClassLight
	case p.Category == CategoryTurboprop:
		return ClassTurboprop
	case p.WingspanM > 0 && p.WingspanM < RegionalSpanM:
		return ClassRegional
	}
	return ClassJet
}

// IntersectionRule is the rule of model's class: the runway an
// intersection must leave ahead of it (meters), fullLength when it takes
// the full length only.
func IntersectionRule(model string) (minRemainingM float64, fullLength bool) {
	r, ok := intersectionClassesNow.Load()[IntersectionClass(model)]
	if !ok {
		return 0, true
	}
	return r.MinRemainingM, r.FullLengthOnly
}

// IntersectionEntries are the entries airport icao lists for runway end
// rwy and model's class, ok false when it lists none (the generic rules).
func IntersectionEntries(icao, rwy, model string) (entries []string, ok bool) {
	a, ok := intersectionAirportsNow.Load()[strings.ToUpper(icao)]
	if !ok {
		return nil, false
	}
	byClass, ok := a.Runways[strings.ToUpper(rwy)]
	if !ok {
		return nil, false
	}
	entries, ok = byClass[IntersectionClass(model)]
	return entries, ok && len(entries) > 0
}

// SayRemaining reports that airport icao's taxi clearances to an
// intersection say the runway left from it.
func SayRemaining(icao string) bool {
	return intersectionAirportsNow.Load()[strings.ToUpper(icao)].SayRemaining
}

// DeclineChance is the chance that the crew of callsign in model declines
// an intersection: its class's DeclineShare, times its airline's way (from
// half to one and a half, the same every time for an airline) and its
// weight today (from half to twice, by flight: a full aircraft wants the
// whole runway). Neither the airline's policy nor the weight is known: both
// are drawn from the callsign, so a flight decides the same each time.
func DeclineChance(model, callsign string) float64 {
	r, ok := intersectionClassesNow.Load()[IntersectionClass(model)]
	if !ok || r.FullLengthOnly {
		return 0
	}
	airline := callsign
	if len(airline) > 3 {
		airline = airline[:3]
	}
	return r.DeclineShare * (0.5 + unit("airline "+airline)) * (0.5 + 1.5*unit("weight "+callsign))
}

// unit is s hashed to [0, 1).
func unit(s string) float64 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return float64(h.Sum32()) / (1 << 32)
}
