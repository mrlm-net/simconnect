//go:build windows
// +build windows

package systems

// The aircraft's class (State.Class): an airliner (business jets
// included), a light aircraft, a helicopter; "" not known.
const (
	ClassAirliner   = "airliner"
	ClassGA         = "ga"
	ClassHelicopter = "helicopter"
)

// Where the take-off data is entered (State.TakeoffComputer): the
// Airbus MCDU, the Boeing FMC; "" none (light aircraft).
const (
	TakeoffMCDU = "MCDU"
	TakeoffFMC  = "FMC"
)

// The reduced take-off thrust (State.ReducedThrust): Airbus FLEX, the
// Boeing assumed temperature; "" none.
const (
	ThrustFlex    = "FLEX"
	ThrustSelTemp = "SEL TEMP"
)

// EngineType is the sim's ENGINE TYPE (0 piston, 1 jet, 2 none, 3 helo
// turbine, 4 unsupported, 5 turboprop), for the class by default.
const EngineType = "engineType"

// gaMaxKg: a turboprop up to this maximum weight (12,500 lb) is a light
// aircraft, heavier an airliner (ATR, Dash 8).
const gaMaxKg = 5700

// classOf is the class the sim's data gives: the engine type, the
// maximum weight; "" when it gives none.
func classOf(s State) string {
	if _, ok := s.Values[EngineType]; !ok {
		return ""
	}
	switch int(s.Values[EngineType]) {
	case 1:
		return ClassAirliner
	case 3:
		return ClassHelicopter
	case 0:
		return ClassGA
	case 5:
		if s.Values[MaxWeight] > gaMaxKg {
			return ClassAirliner
		}
		return ClassGA
	}
	return ""
}

// aircraftKind fills s's class, take-off computer, reduced thrust and
// checklist family: the profile's where it gives them, the class from
// the sim's data otherwise.
func aircraftKind(p Profile, s *State) {
	s.Class, s.TakeoffComputer, s.ReducedThrust, s.ChecklistFamily = p.Class, p.TakeoffComputer, p.ReducedThrust, p.ChecklistFamily
	if s.Class == "" {
		s.Class = classOf(*s)
	}
}
