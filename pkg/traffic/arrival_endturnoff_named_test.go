package traffic

import "testing"

// TestEndTurnoffMidRollPlayerExit (#1674, #1096): C the exit the player
// named (VacateAt) and missed on the roll, the arrival turns round at the
// end and comes back to C: B at the end is not put in its place.
func TestEndTurnoffMidRollPlayerExit(t *testing.T) {
	const kts = 70.0
	ctl, _, nose := midRollPastC(t)
	ctl.exitNamed = true
	turn := ctl.endTurn(kts, nose)
	if p := ctl.Plan(); len(turn) == 0 || !p.EndTurn || p.Exit.Taxiway != "C" {
		t.Errorf("C named, missed at %.0f kt: planned %s, end turn %v (%d points); want the turn round at the end and back to C",
			kts, p.Exit.Taxiway, p.EndTurn, len(turn))
	}
}
