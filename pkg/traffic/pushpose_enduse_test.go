package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestStandardPushSkipsLandingOnlyEnds (#1661): a stand's standard push is
// voted by the ends that take off. At an LKPR stand whose standard push is
// the one most ends take while another end pushes another way, those ends
// made landing only leave that push no standard: a departure to the other
// end pushes its own way, not the one voted for by ends it never uses.
func TestStandardPushSkipsLandingOnlyEnds(t *testing.T) {
	t.Cleanup(standardPushes.Clear) // cached for the stands other tests push from
	const model = "FSLTL_B738_RYR"
	g := lkprGraph(t)
	for i := range g.Layout.Parking {
		req := TaxiRequest{Graph: g, Parking: i, Model: model}
		req.resolveAircraft()
		req.Options = withSpan(req.Options, req.Profile)
		req.Options.OwnStands = []int{i}
		std := planStandardPush(req)
		if std == nil {
			continue
		}
		var voters []string
		other := ""
		for _, e := range standardPushEnds(g) {
			p := endPush(req, e.Name)
			switch {
			case p == nil:
			case samePose(*p, *std):
				voters = append(voters, e.Name)
			case other == "":
				other = e.Name
			}
		}
		if other == "" {
			continue
		}

		g2 := lkprGraph(t)
		for k := range g2.Layout.Runways {
			r := &g2.Layout.Runways[k]
			for _, e := range []*airport.RunwayEnd{&r.Primary, &r.Secondary} {
				for _, v := range voters {
					if e.Name == v {
						e.Use = airport.RunwayUseLanding
					}
				}
			}
		}
		PlanStandardPushes(g2, model, []int{i})
		v, ok := standardPushes.Load(stdKey(g2, i))
		if !ok {
			t.Fatalf("stand %s: no standard push planned", g.Layout.Parking[i].Label())
		}
		if p := v.(*pushPose); p != nil && samePose(*p, *std) {
			t.Errorf("stand %s: standard push facing %.0f°, voted for by %v, landing only", g.Layout.Parking[i].Label(), std.heading, voters)
		}

		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g2, Parking: i, Runway: other, Model: model, Tail: "T1", RollingTakeoffChance: -1, StandOccupied: func(int) bool { return false }}); err != nil {
			t.Fatal(err)
		}
		if ctl.pushPose == nil {
			t.Fatalf("stand %s to %s: no push", g.Layout.Parking[i].Label(), other)
		}
		if samePose(*ctl.pushPose, *std) {
			t.Errorf("stand %s to %s: pushed facing %.0f°, the way of %v, landing only", g.Layout.Parking[i].Label(), other, ctl.pushPose.heading, voters)
		}
		t.Logf("stand %s: standard facing %.0f° by %v; to %s facing %.0f°", g.Layout.Parking[i].Label(), std.heading, voters, other, ctl.pushPose.heading)
		return
	}
	t.Fatal("no LKPR stand whose standard push one end does not take")
}
