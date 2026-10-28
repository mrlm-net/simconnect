package airport

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestWaterSurfaces: every water surface is "water", MSFS 2024's water,
// pond, lake, river and waste water too; they came out "", seaplane
// runways looking like land ones.
func TestWaterSurfaces(t *testing.T) {
	for _, s := range []types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE{
		types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_WATER_FSX, types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_OCEAN,
		types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_WATER, types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_POND,
		types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_LAKE, types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_RIVER,
		types.SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_WASTE_WATER,
	} {
		v := int32(s)
		if got := surfaceName(&v); got != "water" {
			t.Errorf("surface %d: %q, want water", s, got)
		}
		if hardSurface(&v) {
			t.Errorf("surface %d: hard", s)
		}
	}
}
