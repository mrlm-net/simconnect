package airport

import "github.com/mrlm-net/simconnect/pkg/types"

// surfaceNames are the runway surfaces by SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE.
var surfaceNames = map[types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE]string{
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_CONCRETE:           "concrete",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_GRASS:              "grass",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_WATER_FSX:          "water",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_GRASS_BUMPY:        "grass",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_ASPHALT:            "asphalt",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_SHORT_GRASS:        "grass",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_LONG_GRASS:         "grass",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_HARD_TURF:          "turf",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_SNOW:               "snow",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_ICE:                "ice",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_URBAN:              "urban",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_FOREST:             "forest",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_DIRT:               "dirt",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_CORAL:              "coral",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_GRAVEL:             "gravel",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_OIL_TREATED:        "oil treated",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_STEEL_MATS:         "steel mats",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_BITUMINUS:          "bitumen",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_BRICK:              "brick",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_MACADAM:            "macadam",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_PLANKS:             "planks",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_SAND:               "sand",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_SHALE:              "shale",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_TARMAC:             "tarmac",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_WRIGHT_FLYER_TRACK: "track",
	types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_OCEAN:              "water",
}

// surfaceName is a raw SURFACE's name; "" when not read or not known.
func surfaceName(s *int32) string {
	if s == nil {
		return ""
	}
	return surfaceNames[types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE(*s)]
}

// hardSurface reports a paved runway: concrete, asphalt, bitumen,
// macadam, tarmac or brick.
func hardSurface(s *int32) bool {
	switch surfaceName(s) {
	case "concrete", "asphalt", "bitumen", "macadam", "tarmac", "brick":
		return true
	}
	return false
}
