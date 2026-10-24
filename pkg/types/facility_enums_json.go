package types

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// The facility enums are written to JSON by name ("GATE_HEAVY",
// "HOLD_SHORT", "TF", "AT_OR_ABOVE"): the constant's name without its type
// prefix (a leg type is its ARINC 424 code, as String). A value without a
// name is written as its number. Either is read: the name, or the number
// files written before the names had (airport caches, layout and
// procedures dumps).

var parkingTypeNames = [...]string{"NONE", "RAMP_GA", "RAMP_GA_SMALL", "RAMP_GA_MEDIUM", "RAMP_GA_LARGE", "RAMP_CARGO",
	"RAMP_MIL_CARGO", "RAMP_MIL_COMBAT", "GATE_SMALL", "GATE_MEDIUM", "GATE_HEAVY", "DOCK_GA", "FUEL", "VEHICLE",
	"RAMP_GA_EXTRA", "GATE_EXTRA"}

var parkingNameNames = [...]string{"NONE", "PARKING", "N_PARKING", "NE_PARKING", "E_PARKING", "SE_PARKING", "S_PARKING",
	"SW_PARKING", "W_PARKING", "NW_PARKING", "GATE", "DOCK", "GATE_A", "GATE_B", "GATE_C", "GATE_D", "GATE_E", "GATE_F",
	"GATE_G", "GATE_H", "GATE_I", "GATE_J", "GATE_K", "GATE_L", "GATE_M", "GATE_N", "GATE_O", "GATE_P", "GATE_Q",
	"GATE_R", "GATE_S", "GATE_T", "GATE_U", "GATE_V", "GATE_W", "GATE_X", "GATE_Y", "GATE_Z"}

var surfaceNames = [...]string{0: "CONCRETE", 1: "GRASS", 2: "WATER_FSX", 3: "GRASS_BUMPY", 4: "ASPHALT", 5: "SHORT_GRASS",
	6: "LONG_GRASS", 7: "HARD_TURF", 8: "SNOW", 9: "ICE", 10: "URBAN", 11: "FOREST", 12: "DIRT", 13: "CORAL", 14: "GRAVEL",
	15: "OIL_TREATED", 16: "STEEL_MATS", 17: "BITUMINUS", 18: "BRICK", 19: "MACADAM", 20: "PLANKS", 21: "SAND", 22: "SHALE",
	23: "TARMAC", 24: "WRIGHT_FLYER_TRACK", 26: "OCEAN", 27: "WATER", 28: "POND", 29: "LAKE", 30: "RIVER", 31: "WASTE_WATER",
	32: "PAINT", 254: "UNKNOWN", 255: "UNDEFINED"}

var designatorNames = [...]string{"NONE", "LEFT", "RIGHT", "CENTER", "WATER", "A", "B", "LAST"}

var frequencyNames = [...]string{"NONE", "ATIS", "MULTICOM", "UNICOM", "CTAF", "GROUND", "TOWER", "CLEARANCE", "APPROACH",
	"DEPARTURE", "CENTER", "FSS", "AWOS", "ASOS", "CPT", "GCO"}

var approachNames = [...]string{"UNDEFINED", "GPS", "VOR", "NDB", "ILS", "LOCALIZER", "SDF", "LDA", "VORDME", "NDBDME", "RNAV",
	"LOCALIZER_BACK_COURSE"}

var helipadNames = [...]string{"NONE", "H", "SQUARE", "CIRCLE", "MEDICAL"}

var taxiPointNames = [...]string{0: "NONE", 1: "NORMAL", 2: "HOLD_SHORT", 4: "ILS_HOLD_SHORT", 5: "HOLD_SHORT_NO_DRAW",
	6: "ILS_HOLD_SHORT_NO_DRAW"}

var taxiPathNames = [...]string{"NONE", "TAXI", "RUNWAY", "PARKING", "PATH", "CLOSED", "VEHICLE", "ROAD", "PAINTED_LINE"}

var legJSONNames = func() []string {
	n := append([]string(nil), legTypeNames[:]...)
	n[0] = "NONE"
	return n
}()

var altDescNames = [...]string{"NONE", "AT", "AT_OR_ABOVE", "AT_OR_BELOW", "BETWEEN"}

// MarshalEnumJSON writes v as its name in names (indexed by value), or as
// its number when it has none.
func MarshalEnumJSON(v uint32, names []string) ([]byte, error) {
	if int(v) < len(names) && names[v] != "" {
		return []byte(strconv.Quote(names[v])), nil
	}
	return strconv.AppendUint(nil, uint64(v), 10), nil
}

// UnmarshalEnumJSON reads a name in names or a number (what tells the type
// in errors); null leaves *v as it is.
func UnmarshalEnumJSON(b []byte, v *uint32, names []string, what string) error {
	if string(b) == "null" {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		for i, n := range names {
			if n != "" && n == s {
				*v = uint32(i)
				return nil
			}
		}
		if n, err := strconv.ParseUint(s, 10, 32); err == nil {
			*v = uint32(n)
			return nil
		}
		return fmt.Errorf("types: unknown %s %q", what, s)
	}
	n, err := strconv.ParseUint(string(b), 10, 32)
	if err != nil {
		return fmt.Errorf("types: bad %s %s", what, b)
	}
	*v = uint32(n)
	return nil
}

func unmarshalEnum[T ~uint32](b []byte, t *T, names []string, what string) error {
	v := uint32(*t)
	if err := UnmarshalEnumJSON(b, &v, names, what); err != nil {
		return err
	}
	*t = T(v)
	return nil
}

func (t SIMCONNECT_FACILITY_TAXI_PARKING_TYPE) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), parkingTypeNames[:])
}
func (t *SIMCONNECT_FACILITY_TAXI_PARKING_TYPE) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, parkingTypeNames[:], "parking type")
}

func (t SIMCONNECT_FACILITY_TAXI_PARKING_NAME) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), parkingNameNames[:])
}
func (t *SIMCONNECT_FACILITY_TAXI_PARKING_NAME) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, parkingNameNames[:], "parking name")
}

func (t SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), surfaceNames[:])
}
func (t *SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, surfaceNames[:], "runway surface")
}

func (t SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), designatorNames[:])
}
func (t *SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, designatorNames[:], "runway designator")
}

func (t SIMCONNECT_FACILITY_TAXI_DESIGNATOR) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), designatorNames[:])
}
func (t *SIMCONNECT_FACILITY_TAXI_DESIGNATOR) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, designatorNames[:], "taxi designator")
}

func (t SIMCONNECT_FACILITY_FREQUENCY_TYPE) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), frequencyNames[:])
}
func (t *SIMCONNECT_FACILITY_FREQUENCY_TYPE) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, frequencyNames[:], "frequency type")
}

func (t SIMCONNECT_FACILITY_APPROACH_TYPE) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), approachNames[:])
}
func (t *SIMCONNECT_FACILITY_APPROACH_TYPE) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, approachNames[:], "approach type")
}

func (t SIMCONNECT_FACILITY_HELIPAD_TYPE) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), helipadNames[:])
}
func (t *SIMCONNECT_FACILITY_HELIPAD_TYPE) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, helipadNames[:], "helipad type")
}

func (t SIMCONNECT_FACILITY_TAXI_POINT_TYPE) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), taxiPointNames[:])
}
func (t *SIMCONNECT_FACILITY_TAXI_POINT_TYPE) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, taxiPointNames[:], "taxi point type")
}

func (t SIMCONNECT_FACILITY_TAXI_PATH_TYPE) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), taxiPathNames[:])
}
func (t *SIMCONNECT_FACILITY_TAXI_PATH_TYPE) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, taxiPathNames[:], "taxi path type")
}

func (t SIMCONNECT_FACILITY_LEG_TYPE) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), legJSONNames)
}
func (t *SIMCONNECT_FACILITY_LEG_TYPE) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, legJSONNames, "leg type")
}

func (t SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR) MarshalJSON() ([]byte, error) {
	return MarshalEnumJSON(uint32(t), altDescNames[:])
}
func (t *SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(b, t, altDescNames[:], "altitude descriptor")
}
