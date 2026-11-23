package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestExitReachCapsExitSpeed (#1671): judging whether the planned exit is in
// reach, the exit speed holds from rolloutExitLeadM before the exit's point,
// as the rollout plans it. An aircraft 100 m short of a gentle exit may be
// no faster than braking at BrakeDecel to ExitKts by then allows. Before,
// the cap was put on a range of no length, held no point of the plan, and
// the gentle exit's turn alone was the limit.
func TestExitReachCapsExitSpeed(t *testing.T) {
	const hdg, short = 90.0, 100.0
	exit := offsetHeading(lkpr, hdg, 1000)
	p1 := offsetHeading(exit, hdg+15, 150)
	p2 := offsetHeading(p1, hdg+30, 150)
	p3 := offsetHeading(p2, hdg+45, 150)
	c := &ArrivalController{}
	c.plan = &ArrivalPlan{Route: &airport.Route{Points: []airport.LatLon{exit, p1, p2, p3}}}
	c.plan.End = airport.RunwayEnd{Name: "09", Heading: hdg, Threshold: lkpr}
	c.plan.Exit.Path = []airport.NodeID{0, 1, 2}
	nose := offsetHeading(exit, hdg+180, short)

	ro := DefaultRolloutProfile()
	ve := ro.ExitKts * ktsToMS
	most := math.Sqrt(ve*ve + 2*ro.BrakeDecel*(short-rolloutExitLeadM))
	if got := c.exitTurnReach(nose); got <= 0 || got > most+0.01 {
		t.Errorf("in reach at %.1f kt %.0f m short of the exit, want %.1f kt at most (%.0f kt from %.0f m before it)", got/ktsToMS, short, most/ktsToMS, ro.ExitKts, rolloutExitLeadM)
	}
}
