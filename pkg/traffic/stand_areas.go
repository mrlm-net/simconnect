package traffic

import (
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Stand areas: which flights take which stands, where the scenery does not
// say (its airline codes are empty at most airports: LKPR has none). An
// area is the stands whose label starts with a prefix ("LKPR C": C17,
// C18…), for the airlines named and/or the flights of one side of the
// Schengen border (an arrival's origin, a departure's destination). The
// allocator takes the stands of the flight's areas first, then stands in
// no area, and another area's last (StandAllocator.Assign). A table like
// the others, replaceable at runtime (traffic.standAreas, pkg/dict): the
// shipped areas, and a host's local file over them by area.

// StandAreaItem is one area, "ICAO PREFIX" ("LKPR C"). Airlines are ICAO
// codes ("CSA"); Flights is FlightsSchengen, FlightsNonSchengen or "" any.
// Reason says why (shown, not used).
type StandAreaItem struct {
	Area     string   `json:"area"`
	Airlines []string `json:"airlines,omitempty"`
	Flights  string   `json:"flights,omitempty"`
	Reason   string   `json:"reason,omitempty"`
}

// The Flights of a stand area.
const (
	FlightsSchengen    = "schengen"
	FlightsNonSchengen = "non-schengen"
)

const lkprTerminals = "prg.aero: Terminal 1 (piers A and B) for flights outside the Schengen area, Terminal 2 (pier C, gates D) within it"

var shippedStandAreas = []StandAreaItem{
	{Area: "LKPR A", Flights: FlightsNonSchengen, Reason: lkprTerminals},
	{Area: "LKPR B", Flights: FlightsNonSchengen, Reason: lkprTerminals},
	{Area: "LKPR C", Flights: FlightsSchengen, Reason: lkprTerminals},
}

var standAreasNow dict.Value[map[string]StandAreaItem]

func init() {
	dict.Register(dict.Keyed("traffic.standAreas", "area", "", "",
		func() []StandAreaItem { return slices.Clone(shippedStandAreas) },
		func(i StandAreaItem) string { return standKey(i.Area) },
		func(items []StandAreaItem) {
			m := map[string]StandAreaItem{}
			for _, i := range items {
				m[standKey(i.Area)] = i
			}
			standAreasNow.Store(m)
		}))
	_ = dict.Reset("traffic.standAreas") // the shipped copy in use
}

// StandAreaFor is the area of stand label at icao, the longest prefix of
// the label, if it lies in one.
func StandAreaFor(icao, label string) (StandAreaItem, bool) {
	var best StandAreaItem
	n := -1
	for k, a := range standAreasNow.Load() {
		at, prefix, ok := strings.Cut(k, " ")
		if !ok || !strings.EqualFold(at, icao) || !strings.HasPrefix(strings.ToUpper(label), prefix) || len(prefix) <= n {
			continue
		}
		best, n = a, len(prefix)
	}
	return best, n >= 0
}

// StandAreas are the areas in use, by area.
func StandAreas() []StandAreaItem {
	m := standAreasNow.Load()
	out := make([]StandAreaItem, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	slices.SortFunc(out, func(a, b StandAreaItem) int { return strings.Compare(a.Area, b.Area) })
	return out
}

// Area fits for a flight of airline from or to other (its other airport,
// "" unknown): areaFor the flight's (its airline, or its side of the
// Schengen border), areaNone no area or nothing known against it,
// areaOther another flight's.
const (
	areaFor = iota
	areaNone
	areaOther
)

func standAreaFit(icao, label, airline, other string) int {
	a, ok := StandAreaFor(icao, label)
	if !ok {
		return areaNone
	}
	fit := areaNone
	if len(a.Airlines) > 0 && airline != "" {
		if !slices.ContainsFunc(a.Airlines, func(c string) bool { return strings.EqualFold(c, airline) }) {
			return areaOther
		}
		fit = areaFor
	}
	if a.Flights != "" && other != "" && Schengen(icao) {
		side := FlightsNonSchengen
		if Schengen(other) {
			side = FlightsSchengen
		}
		if side != a.Flights {
			return areaOther
		}
		fit = areaFor
	}
	return fit
}

// schengenPrefixes are the ICAO prefixes of the Schengen states' airports
// (their European territory: the Canaries, Madeira and the Azores in it;
// the French overseas departments, Greenland and the Faroes not).
var schengenPrefixes = []string{
	"LO", "EB", "LB", "LD", "LK", "EK", "EE", "EF", "LF", "ED", "ET", "LG", "LH", "BI", "LI",
	"EV", "EY", "EL", "LM", "EH", "EN", "EP", "LP", "LR", "LZ", "LJ", "LE", "GC", "ES", "LS",
}

// Schengen reports whether the airport icao lies in the Schengen area
// (by its ICAO prefix; EKVG, the Faroes, and ENSB, Svalbard, outside).
func Schengen(icao string) bool {
	icao = strings.ToUpper(icao)
	if len(icao) != 4 || icao == "EKVG" || icao == "ENSB" {
		return false
	}
	return slices.Contains(schengenPrefixes, icao[:2])
}
