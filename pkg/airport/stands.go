package airport

import (
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// StandSize is a parking spot's size class, from its TYPE and RADIUS.
type StandSize int

const (
	// StandNone is a spot not meant for parking aircraft: fuel, vehicles.
	StandNone StandSize = iota
	// StandSmall fits light and regional aircraft (radius below 15 m).
	StandSmall
	// StandMedium fits narrow-bodies such as the A320 and B737.
	StandMedium
	// StandHeavy fits wide-bodies.
	StandHeavy
)

func (s StandSize) String() string {
	switch s {
	case StandSmall:
		return "small"
	case StandMedium:
		return "medium"
	case StandHeavy:
		return "heavy"
	}
	return "none"
}

// Size classes a parking spot. The TYPE decides where the scenery names a
// size (GATE_SMALL/MEDIUM/HEAVY, RAMP_GA_SMALL/MEDIUM/LARGE); otherwise the
// RADIUS does: below 15 m small, below 25 m medium, heavy above.
func (p Parking) Size() StandSize {
	switch p.Type {
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_NONE, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_FUEL,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE:
		return StandNone
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_SMALL, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL:
		return StandSmall
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_MEDIUM, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_MEDIUM:
		return StandMedium
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_LARGE,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_EXTRA, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_EXTRA:
		return StandHeavy
	}
	switch {
	case p.Radius < 15:
		return StandSmall
	case p.Radius < 25:
		return StandMedium
	}
	return StandHeavy
}

// ServesAirline reports whether the stand is assigned to the airline code
// (case-insensitive), or to no airline in particular.
func (p Parking) ServesAirline(code string) bool {
	if len(p.Airlines) == 0 || code == "" {
		return true
	}
	return slices.ContainsFunc(p.Airlines, func(a string) bool { return strings.EqualFold(a, code) })
}

// ParkingConflicts returns the parking spots whose RADIUS circles overlap
// spot i: an aircraft filling spot i blocks them (split or alternate
// stands, e.g. LKPR's two S22), in index order. It is nil for an unknown i.
func (l *Layout) ParkingConflicts(i int) []int {
	if i < 0 || i >= len(l.Parking) {
		return nil
	}
	p := l.Parking[i]
	var out []int
	for _, q := range l.Parking {
		if q.Index != i && calc.HaversineMeters(p.Position.Lat, p.Position.Lon, q.Position.Lat, q.Position.Lon) < p.Radius+q.Radius-standOverlapToleranceMeters {
			out = append(out, q.Index)
		}
	}
	return out
}

// standOverlapToleranceMeters ignores circles that only touch: neighbouring
// gates are usually drawn edge to edge.
const standOverlapToleranceMeters = 0.5

// SuitableStands returns the parking spots with at least minRadius meters
// of RADIUS (half the span an aircraft needs, plus margin) and, if any are
// given, one of the TYPEs, in index order. Spots of class StandNone (fuel,
// vehicles) are never suitable.
func (l *Layout) SuitableStands(minRadius float64, kinds ...types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE) []int {
	var out []int
	for _, p := range l.Parking {
		if p.Size() == StandNone || p.Radius < minRadius {
			continue
		}
		if len(kinds) > 0 && !slices.Contains(kinds, p.Type) {
			continue
		}
		out = append(out, p.Index)
	}
	return out
}
