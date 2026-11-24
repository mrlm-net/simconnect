package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestRescueEnterShortOfWay (#1643): sent in driving with less of its way
// left than it stops in braking at rescueAccel, it takes the way in
// rolling: no step in its speed, and it gets to the aircraft. Before, it
// braked at v²/(2·what was left): a stop in a frame or two.
func TestRescueEnterShortOfWay(t *testing.T) {
	l := smoothLayout(t, "LKPR")
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	sent := 0
	homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
		spots := RescueStandby(l, end.Name, 1)
		if len(spots) == 0 {
			return
		}
		at := offsetHeading(end.Threshold, end.Heading, rwy.Length/2)
		way, ok := RescueAccess(g, end.Name, at, false)
		if !ok {
			return
		}
		r := convoyDriving(t, l, spots[0])
		if r == nil {
			return
		}
		// On until 20 m of its way are left, then too fast to stop there.
		// Its way out is up to 2.4 km at its 12 kt on the roads: 400 s on
		// LKPR 06, so 20 minutes at most (in two minutes most of it was
		// left, and the speed set from it 137 m/s).
		for i := 0; i < 20*60*10 && r.mover != nil && r.mover.path.Length()-r.mover.s > 20; i++ {
			if err := r.Update(homeStep); err != nil {
				t.Fatal(err)
			}
		}
		m := r.mover
		if m == nil {
			return
		}
		left := m.path.Length() - m.s
		if left > 21 {
			t.Fatalf("%s: %.0f m of its way left after 20 minutes: nothing checked", end.Name, left)
		}
		m.v = math.Max(m.v, 1.5*math.Sqrt(2*rescueAccel*left))
		m.place()
		r.pose = m.Pose()
		sent++
		if err := r.Enter(way); err != nil {
			t.Fatal(err)
		}
		place := GroundPose{Position: at, Heading: end.Heading}
		if bad := convoySteps(t, r, 300, func(float64) error { return r.Follow(place, 0, homeStep) }); bad != "" {
			t.Errorf("%s, %.0f m left at %.1f m/s: %s", end.Name, left, m.v, bad)
		}
		if d := localDist(r.pose.Position, at); d > RescueGapMeters+RescueJoinMeters+20 {
			t.Errorf("%s: %.0f m from the aircraft after 5 minutes, want it there", end.Name, d)
		}
	})
	if sent == 0 {
		t.Error("none sent in short of its way: nothing checked")
	}
}
