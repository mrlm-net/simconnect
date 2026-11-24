package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// cornerWay is straight metres north, a corner, and as far on east.
func cornerWay(straight float64) []airport.LatLon {
	return []airport.LatLon{lkpr, offset(lkpr, 0, straight), offset(lkpr, straight, straight)}
}

// uTurnWay is straight metres north, across east and as far back south.
func uTurnWay(straight, across float64) []airport.LatLon {
	return []airport.LatLon{lkpr, offset(lkpr, 0, straight), offset(lkpr, across, straight), offset(lkpr, across, 0)}
}

// TestSpeedPlanTrueRadius (#1651): in the middle of an arc the planned
// speed is what LateralAccel allows on that arc's radius, and the straight
// before it is still at cruise. Before, it was planned on twice the radius
// or on none.
func TestSpeedPlanTrueRadius(t *testing.T) {
	prof := DefaultMotionProfile()
	for _, r := range []float64{8, 12, 20, 40} {
		path, err := NewArcPath(cornerWay(250), prof, r)
		if err != nil {
			t.Fatal(err)
		}
		start := 250 - r // the tangent of a right angle
		mid := start + r*math.Pi/4
		got, want := path.SpeedLimitKts(mid), math.Sqrt(prof.LateralAccel*r)/ktsToMS
		if math.Abs(got-want) > 0.1*want {
			t.Errorf("radius %.0f m: %.1f kt in the arc, want %.1f", r, got, want)
		}
		if got := path.SpeedLimitKts(start - 120); math.Abs(got-prof.CruiseKts) > 0.1 {
			t.Errorf("radius %.0f m: %.1f kt 120 m before the arc, want cruise %.0f", r, got, prof.CruiseKts)
		}
	}
}

// TestRolloutIgnoresCentrelineBend (#1651): a bend of a degree between a
// runway's centreline nodes is no turn to brake for.
func TestRolloutIgnoresCentrelineBend(t *testing.T) {
	prof := DefaultMotionProfile()
	prof.CruiseKts = 130
	ro := DefaultRolloutProfile()
	p, pts := lkpr, []airport.LatLon{lkpr}
	for i := range 6 {
		p = offsetHeading(p, 90+float64(i%2), 300)
		pts = append(pts, p)
	}
	path, err := newGroundPath(pts, prof, firmZone{meters: math.Inf(1), decel: ro.BrakeDecel, lateral: ro.ExitLateralAccel})
	if err != nil {
		t.Fatal(err)
	}
	for s := 0.0; s <= path.Length(); s++ {
		if v := path.SpeedLimitKts(s); v < prof.CruiseKts-0.1 {
			t.Fatalf("%.1f kt at %.0f m of a straight rollout, want %.0f", v, s, prof.CruiseKts)
		}
	}
}

// turnFrames are the frame lengths driven at: 60 a second, live's 30, and
// the 106 ms of the live trace (jumpTraceDegPerSec).
var turnFrames = []float64{1.0 / 60, 1.0 / 30, 0.106}

// TestVehiclesSlowForArcs (#1651, #1664): driven frame by frame from its
// cruise, each service vehicle takes the arcs of a corner and a U-turn no
// faster than about what LateralAccel allows on them, and never turns
// quicker than the jump trace allows. Before, they held their cruise: a
// follow-me car about 70°/s, a tug about 60°/s.
func TestVehiclesSlowForArcs(t *testing.T) {
	for _, c := range []struct {
		name   string
		prof   MotionProfile
		radius float64
	}{
		{"tug on the road", tugRoadProfile(), tugTurnMeters},
		{"tug driving off", tugProfile(), 6},
		{"fuel", fuelRoadProfile(), 8},
		{"stairs", stairsProfile(), 6},
		{"bus", busProfile(), BusTurnMeters / 2},
		{"follow-me", followMeProfile(), 8},
		{"rescue", rescueRoadProfile(), 8},
	} {
		for _, way := range [][]airport.LatLon{cornerWay(450), uTurnWay(450, 40)} {
			path, err := NewArcPath(way, c.prof, c.radius)
			if err != nil {
				t.Fatal(err)
			}
			first := 450 - c.radius
			mid := first + c.radius*math.Pi/4
			most := 1.25 * math.Max(c.prof.MinTurnKts, math.Sqrt(c.prof.LateralAccel*c.radius)/ktsToMS)
			for _, dt := range turnFrames {
				m := NewGroundMoverFrom(path, c.prof, 0, c.prof.CruiseKts)
				last, checked, worst, worstKts := m.Pose(), false, 0.0, 0.0
				for range int(900 / dt) {
					p := m.Step(dt)
					if rate := math.Abs(headingDiff(last.Heading, p.Heading)) / dt; rate > worst {
						worst, worstKts = rate, p.GroundSpeedKts
					}
					if !checked && p.Distance >= mid {
						checked = true
						if kts := m.v / ktsToMS; kts > most {
							t.Errorf("%s, frame %.3f s: %.1f kt in an arc of %.0f m, want %.1f at most", c.name, dt, kts, c.radius, most)
						}
					}
					if last = p; p.Arrived {
						break
					}
				}
				if !checked || !last.Arrived {
					t.Fatalf("%s, frame %.3f s: through the turn %v, arrived %v", c.name, dt, checked, last.Arrived)
				}
				if worst > jumpTraceDegPerSec {
					t.Errorf("%s, frame %.3f s: turns at %.1f°/s at %.1f kt, want %.0f at most", c.name, dt, worst, worstKts, jumpTraceDegPerSec)
				}
			}
		}
	}
}

// TestEndTurnSlowsForItsRadius (#1651): the short-field end turn round of
// an 18 m runway (8 m radius, TFFJ), as endTurn lays it and the rollout
// limits it. A DHC6 slows below TurnSpeedKts for it and never turns quicker
// than the jump trace allows. Before, it took it at 8 kt, 45-46°/s live.
func TestEndTurnSlowsForItsRadius(t *testing.T) {
	ap := ProfileFor("DHC6")
	ro := withDefaults(ap.Rollout, DefaultRolloutProfile())
	const length, hdg = 642.0, 100.0
	land := airport.RunwayEnd{Name: "10", Heading: hdg, Threshold: lkpr}
	back := airport.RunwayEnd{Name: "28", Heading: hdg + 180, Threshold: offsetHeading(lkpr, hdg, length)}
	rwy := airport.Runway{Width: 18, Length: length, Primary: land, Secondary: back}
	c := &ArrivalController{}
	c.req.Graph = &airport.Graph{}
	c.plan = &ArrivalPlan{Runway: rwy, End: land, EndTurn: true}
	nose := offsetHeading(lkpr, hdg, 300)
	turn := c.endTurn(40, nose)
	if len(turn) < 4 {
		t.Fatal("no turn round: nothing checked")
	}
	pts := append([]airport.LatLon{nose}, turn...)
	from := pathLen(pts[:2])
	to := pathLen(pts)
	pts = append(pts, offsetHeading(back.Threshold, back.Heading, 300))
	prof := ap.Motion
	fast := prof
	fast.CruiseKts = 60
	path, err := newGroundPath(pts, fast, firmZone{meters: math.Inf(1), decel: ro.BrakeDecel, lateral: ro.ExitLateralAccel})
	if err != nil {
		t.Fatal(err)
	}
	path.LimitRange(from, to, TurnSpeedKts, ro.BrakeDecel)
	path.LimitRange(to, path.Length(), BacktrackKts, ro.BrakeDecel)
	gear := moverGear(prof, "DHC6")
	gear.Decel, gear.Jerk = ro.BrakeDecel, RolloutJerk
	for _, dt := range turnFrames {
		m := NewGroundMoverFrom(path, gear, hdg, 40)
		last, halfKts, worst := m.Pose(), -1.0, 0.0
		for range int(400 / dt) {
			p := m.Step(dt)
			worst = math.Max(worst, math.Abs(headingDiff(last.Heading, p.Heading))/dt)
			if halfKts < 0 && math.Abs(headingDiff(hdg, p.Heading)) >= 90 {
				halfKts = m.v / ktsToMS
			}
			if last = p; p.Arrived {
				break
			}
		}
		if halfKts < 0 {
			t.Fatalf("frame %.3f s: never half round: nothing checked", dt)
		}
		if halfKts > TurnSpeedKts-0.7 {
			t.Errorf("frame %.3f s: %.1f kt half round the 8 m turn, want under %.1f", dt, halfKts, TurnSpeedKts-0.7)
		}
		if worst > jumpTraceDegPerSec {
			t.Errorf("frame %.3f s: turns at %.1f°/s, want %.0f at most", dt, worst, jumpTraceDegPerSec)
		}
	}
}
