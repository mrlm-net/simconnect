package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// underAheadM: a road crossing this far ahead of the nose gear is in front of
// the aircraft, not under it (EGLL 567: at the edge of the 6 m kept ahead).
const underAheadM = 5.0

// TestTugKeepsOutFromUnderTheAircraft: at every stand and for several types,
// the tug's way in stays out from under the aircraft, down to its last straight
// to the nose, where a vehicle road runs under the aircraft too, and it doesn't
// come back under it on the way home.
func TestTugKeepsOutFromUnderTheAircraft(t *testing.T) {
	for _, icao := range []string{"LKPR", "LOWW", "EGLL"} {
		l := airportGraph(t, icao).Layout
		for _, typ := range []string{"A320", "B738", "E190"} {
			prof := MotionProfileFor(typ)
			tried, in, out := 0, 0, 0
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
				// The wingtip's 3 m margin may be grazed by a tug on its turn, a
				// few frames at 2.9 m (EGLL 27, LOWW F12): 1 m less here.
				near := prof
				near.SpanMeters -= 2
				f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: near}
				under := false
				for s := 0; s < 60*900 && !tug.Connected(); s++ {
					if err := tug.Update(pose, true, 1.0/60); err != nil {
						t.Fatal(err)
					}
					if x, y := f.to(tug.pose.Position); !tug.Connected() && math.Abs(x) > 1.5 && y < underAheadM && f.inside(tug.pose.Position) {
						under = true
					}
				}
				if under {
					if in++; in <= 5 {
						t.Errorf("%s %s %s: under the aircraft on the way in", icao, typ, p.Label())
					}
				}
				left, back := false, false
				for s := 0; s < 60*900 && !tug.Done(); s++ {
					if err := tug.Update(pose, false, 1.0/60); err != nil {
						t.Fatal(err)
					}
					_, y := f.to(tug.pose.Position)
					if i := f.inside(tug.pose.Position) && y < underAheadM; !i {
						left = true
					} else if left {
						back = true
					}
				}
				if back {
					if out++; out <= 5 {
						t.Errorf("%s %s %s: under the aircraft on the way home", icao, typ, p.Label())
					}
				}
			}
			t.Logf("%s %s: %d stands, %d under it on the way in, %d on the way home", icao, typ, tried, in, out)
		}
	}
}
