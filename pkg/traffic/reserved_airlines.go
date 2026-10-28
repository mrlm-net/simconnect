package traffic

import (
	"slices"
	"strings"
)

// ReservedAirlines are our own airlines (the user): flown in their own
// traffic models only (preferRanked), occasionally from LKPR. Their codes
// are in no list of airline codes we know (Wikipedia's List of airline
// codes, retrieved 2026-10-28) and they are kept over every table: the
// shipped one, the MyCrew API's and a local override.
var ReservedAirlines = []Airline{
	{ICAO: "WTD", Name: "WANTED", Telephony: "WANTED", Fleet: map[string]float64{"A20N": 1, "A319": 1, "A320": 1, "A321": 1, "LJ25": 1},
		Bases: []string{"LKPR"}, Regions: reservedRegions, Weight: 0.03},
	{ICAO: "MCR", Name: "MyCrew", Telephony: "MyCrew", Fleet: map[string]float64{"A20N": 1, "A319": 1, "A320": 1, "A321": 1},
		Bases: []string{"LKPR"}, Regions: reservedRegions, Weight: 0.03},
}

var reservedRegions = []string{"LK", "LO", "LZ", "LH", "EP", "ED", "LJ", "LD", "LI", "LF", "LE", "LS", "EH", "EB", "EG"}

// reservedAirline reports whether code is a reserved airline's.
func reservedAirline(code string) bool {
	return slices.ContainsFunc(ReservedAirlines, func(a Airline) bool { return strings.EqualFold(a.ICAO, code) })
}

// withReserved is airlines with the reserved ones over those of their
// codes.
func withReserved(airlines []Airline) []Airline {
	out := slices.DeleteFunc(slices.Clone(airlines), func(a Airline) bool { return reservedAirline(a.ICAO) })
	return append(out, ReservedAirlines...)
}
