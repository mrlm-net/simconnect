package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTugTurnRadius (#1693): a tug's placements as the injector sends
// them, over any 1 s while it moves faster than 0.2 m/s, never turn
// tighter than tugMinTurnMeters (way ÷ turn) as shown in its lane: driving
// in, off the nose and home, for a towbarless, a towbar and a stick tug
// (its body, not its bar). Before, a tug off the nose
// turned home at up to 90°/s at walking pace, a 0.5–1.5 m radius (live
// EGLL, FSDT_TPX_200).
func TestTugTurnRadius(t *testing.T) {
	const (
		fps    = 60
		window = time.Second
		moving = 0.2 // m/s
	)
	dt := 1.0 / float64(fps)
	prof := ProfileFor("A320").Motion
	for _, title := range []string{TugTowbarlessTitle, DefaultTugTitle, "FSDT_Pushback_Trepel_700"} {
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
				now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
				c := &tugClient{}
				inj := NewInjector(c)
				inj.now = func() time.Time { return now }
				tug := NewSimObjectTug(c, inj, title, 9001, prof)
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

				var log []lastSent
				worst, worstTurn, worstMS, worstAt := math.Inf(1), 0.0, 0.0, ""
				step := func(phase string, at GroundPose, pushing bool) {
					now = now.Add(time.Duration(dt * float64(time.Second)))
					if err := tug.Update(at, pushing, dt); err != nil {
						t.Fatal(err)
					}
					inj.mu.Lock()
					var s lastSent
					if o := inj.objects[55]; o != nil {
						s = o.sent
					}
					inj.mu.Unlock()
					if s.at.IsZero() || len(log) > 0 && !s.at.After(log[len(log)-1].at) {
						return
					}
					// The tug's body as written: a stick tug's model origin is
					// its bar's hook, which swings as the bar eases.
					body := tug.axle(GroundPose{Position: s.pos, Heading: s.hdg})
					s.pos, s.hdg = body.Position, body.Heading
					log = append(log, s)
					if phase == "push" {
						return // on the nose: the aircraft's turn, not the tug's
					}
					way := 0.0
					for j := len(log) - 2; j >= 0; j-- {
						way += localDist(log[j].pos, log[j+1].pos)
						gap := s.at.Sub(log[j].at)
						if gap < window {
							continue
						}
						turn := math.Abs(math.Remainder(s.hdg-log[j].hdg, 360))
						if ms := way / gap.Seconds(); ms > moving && turn > 1 {
							if r := way / (turn * math.Pi / 180); r < worst {
								worst, worstTurn, worstMS, worstAt = r, turn/gap.Seconds(), ms, phase
							}
						}
						break
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
					t.Errorf("%s, %s %s: never at the nose", title, icao, stand.Label())
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
					t.Errorf("%s, %s %s: never home", title, icao, stand.Label())
				}
				if worst < tugMinTurnMeters {
					t.Errorf("%s, %s %s, %s: turned on a %.1f m radius (%.0f°/s at %.1f m/s), want %.0f m at least", title, icao, stand.Label(), worstAt, worst, worstTurn, worstMS, tugMinTurnMeters)
				}
			}
			if drives == 0 {
				t.Errorf("%s, %s: no tug driven: nothing checked", title, icao)
			}
		}
	}
}
