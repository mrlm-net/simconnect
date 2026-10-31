package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTugJoinsTheNoseFacingIt: at every stand and for several types, the tug
// reaches the nose already facing it and on the tow point: no turn and no hop
// in the frame it connects (live LKMT: 180° and 6 m), and the turn before it stays off
// the aircraft (farther back the road itself can pass under it, #1364).
func TestTugJoinsTheNoseFacingIt(t *testing.T) {
	for _, icao := range []string{"LKPR", "LOWW", "EGLL"} {
		l := airportGraph(t, icao).Layout
		for _, typ := range []string{"A320", "B738", "E190"} {
			prof := MotionProfileFor(typ)
			tried, bad := 0, 0
			for _, p := range l.Parking {
				if p.Size() == airport.StandNone || p.Radius*2 < prof.SpanMeters {
					continue
				}
				c := &tugClient{}
				inj := NewInjector(c)
				tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
				tug.Layout = l
				pose := GroundPose{Position: StandPoint(p, prof.RefAheadMeters), Heading: p.Heading}
				if err := tug.Attach(pose); err != nil || tug.arrive == nil {
					continue
				}
				tried++
				tug.Handle(assignedMsg(9001, 55))
				inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
				f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
				path := tug.inPath
				for i, q := range path.pts {
					if x, _ := f.to(q); path.cum[i] > path.Length()-30 && math.Abs(x) > 1 && f.inside(q) {
						t.Errorf("%s %s %s: the turn to the nose goes through the aircraft", icao, typ, p.Label())
						break
					}
				}
				last, lastH := tug.pose.Position, tug.pose.Heading
				for s := 0; s < 60*900 && !tug.Connected(); s++ {
					last, lastH = tug.pose.Position, tug.pose.Heading
					if err := tug.Update(pose, true, 1.0/60); err != nil {
						t.Fatal(err)
					}
				}
				d, turn := localDist(last, tug.pose.Position), math.Abs(headingDiff(lastH, tug.pose.Heading))
				if d > 0.5 || turn > 5 {
					if bad++; bad <= 5 {
						t.Errorf("%s %s %s: joined with %.1f m, %.1f°", icao, typ, p.Label(), d, turn)
					}
				}
			}
			t.Logf("%s %s: %d stands, %d bad", icao, typ, tried, bad)
			if tried == 0 {
				t.Errorf("%s %s: no stand tried", icao, typ)
			}
		}
	}
}
