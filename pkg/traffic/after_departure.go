package traffic

import "fmt"

// AfterDepartureClimb is tower's amendment before a take-off restricting
// the climb: "BIG JET 345, after departure climb to altitude 4000 feet"
// (ICAO Phraseology Reference Guide, ALL CLEAR: "hold position, after
// departure climb to altitude 6000 feet"), read back "After departure
// climb to 4000 feet, BIG JET 345". ft is feet MSL, an altitude.
func AfterDepartureClimb(cs string, ft float64) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentAfterDeparture,
		Params: map[string]string{ParamAltitude: fmt.Sprintf("%.0f", ft)}})
}
