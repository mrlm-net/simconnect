package world

import (
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Stand use (#833): which stands a flight takes, by what the scenery says
// each parking spot is (its TYPE: gate, GA ramp, cargo ramp, military):
// an airline's passenger flight a gate first, then another ramp when the
// gates are full, never a cargo or military stand; GA and business flights
// a GA ramp first; a cargo airline a cargo stand first. Live, LKPR: TVS
// flights stood on the north GA ramps (N51–N58) with gates free, the
// scheduler having asked for no type at all.
const (
	standAirline = ""      // passenger airline: gates first
	standGA      = "ga"    // light aircraft, business jets: GA ramps first
	standCargo   = "cargo" // cargo airline: cargo stands first
)

var (
	gateStands = []types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_SMALL, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_MEDIUM,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_EXTRA,
	}
	cargoStands = []types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_CARGO}
	// civilStands are every civil stand but the cargo ones: gates, GA
	// ramps and the untyped (a scenery that types nothing).
	civilStands = append(append([]types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_NONE,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_DOCK_GA}, gateStands...), gaRamps...)
)

// standPreferences are the stand types a use tries, in order.
func standPreferences(use string) [][]types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE {
	switch use {
	case standGA:
		return [][]types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{gaRamps, civilStands}
	case standCargo:
		return [][]types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{cargoStands, gaRamps, civilStands}
	}
	return [][]types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{gateStands, civilStands}
}

// assignStand gives req a stand of use's types, the first that has one
// free; the last error when none has.
func assignStand(alloc *traffic.StandAllocator, req traffic.StandRequirements, use string) (int, error) {
	err := traffic.ErrNoStand
	for _, kinds := range standPreferences(use) {
		r := req
		r.Types = kinds
		var s int
		if s, err = alloc.Assign(r); err == nil {
			return s, nil
		}
	}
	return -1, err
}
