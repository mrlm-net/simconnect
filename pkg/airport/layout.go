//go:build windows
// +build windows

package airport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// LatLon is a WGS84 position in decimal degrees.
type LatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Layout is the ground layout of one airport.
//
// Slices are indexed by SimConnect list index: TaxiPoints[i].Index == i, and
// likewise for Runways, Parking and TaxiPaths. TaxiPath.NameIndex indexes
// TaxiNames.
type Layout struct {
	ICAO       string      `json:"icao"`
	Name       string      `json:"name"`
	Latitude   float64     `json:"lat"` // airport reference point
	Longitude  float64     `json:"lon"`
	Altitude   float64     `json:"alt"` // meters MSL
	Runways    []Runway    `json:"runways"`
	Parking    []Parking   `json:"parking"`
	TaxiPoints []TaxiPoint `json:"taxiPoints"`
	TaxiPaths  []TaxiPath  `json:"taxiPaths"`
	TaxiNames  []string    `json:"taxiNames"`
	// Frequencies are the airport's radio frequencies (#416).
	Frequencies []Frequency `json:"frequencies,omitempty"`
}

// Runway is a runway with both of its ends.
type Runway struct {
	Index     int       `json:"index"`
	Center    LatLon    `json:"center"`
	Altitude  float64   `json:"alt"`     // meters MSL
	Heading   float64   `json:"heading"` // degrees true, in the direction of the primary end
	Length    float64   `json:"length"`  // meters
	Width     float64   `json:"width"`   // meters
	Primary   RunwayEnd `json:"primary"`
	Secondary RunwayEnd `json:"secondary"`
}

// Name returns the runway name, e.g. "06/24" or "09L/27R".
func (r Runway) Name() string { return r.Primary.Name + "/" + r.Secondary.Name }

// RunwayEnd is one direction of a runway.
type RunwayEnd struct {
	Number     int32                                       `json:"number"` // 1–36, or 37–44 for N, NE, E, SE, S, SW, W, NW
	Designator types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR `json:"designator"`
	Name       string                                      `json:"name"`    // e.g. "06", "27R", "N"
	Heading    float64                                     `json:"heading"` // degrees true, take-off/landing direction
	// Threshold is the start of the runway surface in this direction. Displaced
	// thresholds are not taken into account.
	Threshold LatLon `json:"threshold"`
}

// Parking is a parking spot (gate, ramp or dock).
type Parking struct {
	Index    int                                         `json:"index"`
	Name     types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME `json:"name"`
	Suffix   types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME `json:"suffix"`
	Number   uint32                                      `json:"number"`
	Type     types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE `json:"type"`
	Heading  float64                                     `json:"heading"` // degrees true
	Radius   float64                                     `json:"radius"`  // meters
	BiasX    float64                                     `json:"biasX"`   // meters east of the airport reference point
	BiasZ    float64                                     `json:"biasZ"`   // meters north of the airport reference point
	Position LatLon                                      `json:"position"`
	// Airlines are the codes of the airlines the stand is assigned to (ICAO
	// or IATA, as the scenery defines them); empty for any airline.
	Airlines []string `json:"airlines,omitempty"`
}

// Label returns the parking spot's short name as shown to pilots: a gate
// letter or compass prefix, the number and a suffix letter, e.g. "C22", "S22",
// "22". Labels are not guaranteed to be unique within an airport.
func (p Parking) Label() string {
	return parkingPrefix(p.Name) + strconv.FormatUint(uint64(p.Number), 10) + gateLetter(p.Suffix)
}

// IsGate reports whether the spot is a gate (GATE_SMALL, GATE_MEDIUM,
// GATE_HEAVY or GATE_EXTRA).
func (p Parking) IsGate() bool {
	switch p.Type {
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_SMALL,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_MEDIUM,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_EXTRA:
		return true
	}
	return false
}

// TaxiPoint is a node of the taxi network.
type TaxiPoint struct {
	Index       int                                       `json:"index"`
	Type        types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE `json:"type"`
	Orientation int32                                     `json:"orientation"`
	BiasX       float64                                   `json:"biasX"`
	BiasZ       float64                                   `json:"biasZ"`
	Position    LatLon                                    `json:"position"`
}

// IsHoldShort reports whether the point is a runway hold-short position,
// including ILS hold-shorts and the NO_DRAW variants without painted markings.
func (t TaxiPoint) IsHoldShort() bool {
	switch t.Type {
	case types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE_HOLD_SHORT,
		types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE_ILS_HOLD_SHORT,
		types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE_HOLD_SHORT_NO_DRAW,
		types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE_ILS_HOLD_SHORT_NO_DRAW:
		return true
	}
	return false
}

// IsILSHoldShort reports whether the point is an ILS critical area hold-short.
func (t TaxiPoint) IsILSHoldShort() bool {
	return t.Type == types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE_ILS_HOLD_SHORT ||
		t.Type == types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE_ILS_HOLD_SHORT_NO_DRAW
}

// TaxiPath is an edge of the taxi network between two taxi points, or between
// a taxi point and a parking spot for PARKING paths.
type TaxiPath struct {
	Index            int                                         `json:"index"`
	Type             types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE    `json:"type"`
	Width            float64                                     `json:"width"` // meters
	RunwayNumber     int32                                       `json:"runwayNumber"`
	RunwayDesignator types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR `json:"runwayDesignator"`
	// Start is a taxi point index.
	Start int32 `json:"start"`
	// End is a taxi point index, or a parking index when Type is PARKING.
	End       int32  `json:"end"`
	NameIndex uint32 `json:"nameIndex"`
}

// EndsAtParking reports whether End is a parking index rather than a taxi
// point index.
func (p TaxiPath) EndsAtParking() bool {
	return p.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING
}

// PathName returns the taxiway name of a path, or "" when it has none.
func (l *Layout) PathName(p TaxiPath) string {
	if int(p.NameIndex) < len(l.TaxiNames) {
		return l.TaxiNames[p.NameIndex]
	}
	return ""
}

// PathEndpoints resolves a path's START and END to positions. ok is false when
// an index is out of range.
func (l *Layout) PathEndpoints(p TaxiPath) (start, end LatLon, ok bool) {
	if p.Start < 0 || int(p.Start) >= len(l.TaxiPoints) || p.End < 0 {
		return start, end, false
	}
	start = l.TaxiPoints[p.Start].Position
	if p.EndsAtParking() {
		if int(p.End) >= len(l.Parking) {
			return start, end, false
		}
		return start, l.Parking[p.End].Position, true
	}
	if int(p.End) >= len(l.TaxiPoints) {
		return start, end, false
	}
	return start, l.TaxiPoints[p.End].Position, true
}

// ParkingByLabel returns every parking spot whose Label matches label,
// ignoring case.
func (l *Layout) ParkingByLabel(label string) []Parking {
	var out []Parking
	for _, p := range l.Parking {
		if strings.EqualFold(p.Label(), label) {
			out = append(out, p)
		}
	}
	return out
}

// RunwayEnd finds a runway end by name, e.g. "24", "06", "6" or "RW27R".
func (l *Layout) RunwayEnd(name string) (Runway, RunwayEnd, bool) {
	want := normalizeRunwayEnd(name)
	for _, r := range l.Runways {
		if r.Primary.Name == want {
			return r, r.Primary, true
		}
		if r.Secondary.Name == want {
			return r, r.Secondary, true
		}
	}
	return Runway{}, RunwayEnd{}, false
}

// HoldShortPoints returns every hold-short taxi point.
func (l *Layout) HoldShortPoints() []TaxiPoint {
	var out []TaxiPoint
	for _, t := range l.TaxiPoints {
		if t.IsHoldShort() {
			out = append(out, t)
		}
	}
	return out
}

// runwayEndName formats a RUNWAY_NUMBER and RUNWAY_DESIGNATOR, e.g. "06",
// "27R", "NE".
func runwayEndName(number int32, designator types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR) string {
	var n string
	switch {
	case number >= 1 && number <= 36:
		n = fmt.Sprintf("%02d", number)
	case number >= 37 && number <= 44:
		n = [...]string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}[number-37]
	default:
		n = "?"
	}
	switch designator {
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_LEFT:
		n += "L"
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_RIGHT:
		n += "R"
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_CENTER:
		n += "C"
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_WATER:
		n += "W"
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_A:
		n += "A"
	case types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_B:
		n += "B"
	}
	return n
}

// normalizeRunwayEnd turns user input such as "rw6l" or "6" into "06L".
func normalizeRunwayEnd(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "RWY"), "RW")
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 1 {
		s = "0" + s
	}
	return s
}

// parkingPrefix maps a parking NAME to the label prefix pilots see.
func parkingPrefix(n types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME) string {
	switch n {
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_N_PARKING:
		return "N"
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_NE_PARKING:
		return "NE"
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_E_PARKING:
		return "E"
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_SE_PARKING:
		return "SE"
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_S_PARKING:
		return "S"
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_SW_PARKING:
		return "SW"
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_W_PARKING:
		return "W"
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_NW_PARKING:
		return "NW"
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_DOCK:
		return "D"
	}
	return gateLetter(n)
}

// gateLetter returns "A"–"Z" for GATE_A–GATE_Z and "" otherwise.
func gateLetter(n types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME) string {
	if n >= types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_A && n <= types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_A+25 {
		return string(rune('A' + int(n-types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_A)))
	}
	return ""
}
