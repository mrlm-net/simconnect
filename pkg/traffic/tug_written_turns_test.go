package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTugWrittenTurnRate (#1664): the tug's headings as the injector sends
// them (what the jump trace compares, lane ease, mover changes and the
// stick's bar included), at a live frame rate and at the worldtest's
// 200 ms: over any 200 ms no more than the trace allows (9°, 45°/s), on
// the way in, off the nose and home, its speed changing by no more than
// groundSweepDvMS a frame. The traces were 9.5–10.8° in 200 ms
// at 13 kt.
func TestTugWrittenTurnRate(t *testing.T) {
	const window = 200 * time.Millisecond
	prof := ProfileFor("A320").Motion
	for _, fps := range []int{60, 5} {
		dt := 1.0 / float64(fps)
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
				now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
				c := &tugClient{}
				inj := NewInjector(c)
				inj.now = func() time.Time { return now }
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

				type sent struct {
					at       time.Time
					hdg, kts float64
				}
				var log []sent
				worst, worstKts, worstAt := 0.0, 0.0, ""
				worstDv, dvAt := 0.0, "" // a sudden slow-down is a jump too
				// step moves the tug one frame and checks its last placement
				// against the one a window before.
				step := func(phase string, at GroundPose, pushing bool) {
					now = now.Add(time.Duration(dt * float64(time.Second)))
					if err := tug.Update(at, pushing, dt); err != nil {
						t.Fatal(err)
					}
					inj.mu.Lock()
					o := inj.objects[55]
					var s lastSent
					if o != nil {
						s = o.sent
					}
					inj.mu.Unlock()
					if s.at.IsZero() || len(log) > 0 && !s.at.After(log[len(log)-1].at) {
						return
					}
					if len(log) > 0 {
						if dv := math.Abs(s.kts-log[len(log)-1].kts) * ktsToMS; dv > worstDv {
							worstDv, dvAt = dv, phase
						}
					}
					log = append(log, sent{s.at, s.hdg, s.kts})
					for j := len(log) - 2; j >= 0; j-- {
						if gap := s.at.Sub(log[j].at); gap >= window {
							if turn := math.Abs(math.Remainder(s.hdg-log[j].hdg, 360)) / gap.Seconds(); turn > worst {
								worst, worstKts, worstAt = turn, s.kts, phase
							}
							break
						}
					}
				}
				drive := func(phase string, at GroundPose, pushing bool, done func() bool) bool {
					for n := 0; n < 30*60*fps; n++ {
						if done() {
							return true
						}
						step(phase, at, pushing)
					}
					return false
				}
				if !drive("in", pose, true, tug.Connected) {
					t.Errorf("%d fps, %s %s: never at the nose", fps, icao, stand.Label())
					continue
				}
				end := GroundPose{Position: offsetHeading(pose.Position, pose.Heading+180, 45), Heading: pose.Heading}
				if r, ok := StandardPush(g, i, "A320"); ok {
					back := localDist(NoseGear(pose.Position, pose.Heading, prof), pose.Position)
					end = GroundPose{Position: offsetHeading(r.Nose, r.Facing+180, back), Heading: r.Facing}
				}
				pushFrames := 60 * fps
				pushed := pose
				for n := 1; n <= pushFrames; n++ {
					f := float64(n) / float64(pushFrames)
					pushed = GroundPose{
						Position: airport.LatLon{
							Lat: pose.Position.Lat + (end.Position.Lat-pose.Position.Lat)*f,
							Lon: pose.Position.Lon + (end.Position.Lon-pose.Position.Lon)*f,
						},
						Heading: normDeg(pose.Heading + headingDiff(pose.Heading, end.Heading)*f),
					}
					step("push", pushed, true)
				}
				if !drive("home", pushed, false, tug.Done) {
					t.Errorf("%d fps, %s %s: never home", fps, icao, stand.Label())
				}
				if worstDv > groundSweepDvMS {
					t.Errorf("%d fps, %s %s, %s: speed changed %.2f m/s in a frame, want %.1f at most", fps, icao, stand.Label(), dvAt, worstDv, groundSweepDvMS)
				}
				if worst > jumpTraceDegPerSec {
					t.Errorf("%d fps, %s %s, %s: sent turning %.1f°/s over 200 ms at %.1f kt, want %.0f at most", fps, icao, stand.Label(), worstAt, worst, worstKts, jumpTraceDegPerSec)
				}
			}
			if drives == 0 {
				t.Errorf("%d fps, %s: no tug driven: nothing checked", fps, icao)
			}
		}
	}
}
