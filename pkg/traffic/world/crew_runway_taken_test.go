package world

import "testing"

// Round on its crew's own last look at its runway, another aircraft on
// it (#1680): as crewUnstable's crewRound, its old landing clearance is
// never kept (crewRoundDropsLandingClearance).
func TestCrewRunwayTakenDropsLandingClearance(t *testing.T) {
	crewRoundDropsLandingClearance(t, func(cc *controlCenter, it *controlled) { cc.crewRunwayTaken(it) })
}
