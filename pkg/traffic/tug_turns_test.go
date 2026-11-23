package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// tugTurnStands is how many stands of an airport TestTugTurnRate drives at.
const tugTurnStands = 8

// TestTugTurnRate (#1664): replayed at 60 frames a second, a tug driving in
// from its depot, off the nose and home never turns quicker than the jump
// trace allows a steady turn (45°/s), and still gets there. Before, it
// took its 6 m arcs at 13 kt: 47-54°/s, unseen a frame at a time under the
// trace's 3° floor.
func TestTugTurnRate(t *testing.T) {
	const dt = 1.0 / 60
	prof := ProfileFor("A320").Motion
	for _, icao := range []string{"LKPR", "LROP"} {
		l := smoothLayout(t, icao)
		g, err := airport.BuildGraph(l)
		if err != nil {
			t.Fatal(err)
		}
		stands, drives := smoothStands(l, prof), 0
		every := max(1, len(stands)/tugTurnStands)
		for k := 0; k < len(stands); k += every {
			i := stands[k]
			stand := l.Parking[i]
			pose := GroundPose{Position: StandPoint(stand, 0), Heading: stand.Heading}
			c := &tugClient{}
			inj := NewInjector(c)
			tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
			tug.Layout = l
			if err := tug.Attach(pose); err != nil {
				t.Fatal(err)
			}
			if tug.arrive == nil {
				continue
			}
			tug.Handle(assignedMsg(9001, 55))
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
			drives++

			worst, worstKts, worstAt := 0.0, 0.0, ""
			// drive steps the tug until done, noting how fast it is shown
			// turning while it drives on a mover of its own.
			drive := func(phase string, at GroundPose, pushing bool, done func() bool) bool {
				var last *GroundMover
				var hdg float64
				for n := 0; n < 30*60*60; n++ {
					if done() {
						return true
					}
					if err := tug.Update(at, pushing, dt); err != nil {
						t.Fatal(err)
					}
					m := tug.arrive
					if m == nil {
						m = tug.away
					}
					if m == nil {
						last = nil
						continue
					}
					var road *GroundMover
					if !m.reverse {
						road = m
					}
					h := lane(tug.pose, road).Heading
					if m == last {
						if rate := math.Abs(headingDiff(hdg, h)) / dt; rate > worst {
							worst, worstKts, worstAt = rate, tug.pose.GroundSpeedKts, phase
						}
					}
					last, hdg = m, h
				}
				return false
			}
			if !drive("in", pose, true, tug.Connected) {
				t.Errorf("%s %s: never at the nose", icao, stand.Label())
				continue
			}

			end := GroundPose{Position: offsetHeading(pose.Position, pose.Heading+180, 45), Heading: pose.Heading}
			if r, ok := StandardPush(g, i, "A320"); ok {
				back := localDist(NoseGear(pose.Position, pose.Heading, prof), pose.Position)
				end = GroundPose{Position: offsetHeading(r.Nose, r.Facing+180, back), Heading: r.Facing}
			}
			const pushFrames = 60 * 60
			pushed := pose
			for n := 1; n <= pushFrames; n++ {
				f := float64(n) / pushFrames
				pushed = GroundPose{
					Position: airport.LatLon{
						Lat: pose.Position.Lat + (end.Position.Lat-pose.Position.Lat)*f,
						Lon: pose.Position.Lon + (end.Position.Lon-pose.Position.Lon)*f,
					},
					Heading: normDeg(pose.Heading + headingDiff(pose.Heading, end.Heading)*f),
				}
				if err := tug.Update(pushed, true, dt); err != nil {
					t.Fatal(err)
				}
			}
			if !drive("home", pushed, false, tug.Done) {
				t.Errorf("%s %s: never home", icao, stand.Label())
			}
			if worst > jumpTraceDegPerSec {
				t.Errorf("%s %s, %s: turns at %.1f°/s at %.1f kt, want %.0f at most", icao, stand.Label(), worstAt, worst, worstKts, jumpTraceDegPerSec)
			}
		}
		if drives == 0 {
			t.Errorf("%s: no tug driven: nothing checked", icao)
		}
	}
}
