package traffic

import (
	"fmt"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestGroundTurnSweep (#1664): taxi routes from a sample of stands to
// every runway end at LKPR, LROP and EDDM (traffic everywhere), driven by the shared ground mover at 60 fps: the
// heading written turns no quicker than the jump trace allows over any
// 200 ms, and the speed changes by no more than groundSweepDvMS a frame
// (a sudden slow-down at a corner is a jump too). Each route's time is
// logged, to compare between trees: no route slower than needed.
func TestGroundTurnSweep(t *testing.T) {
	const fps, window = 60, 0.2
	prof := ProfileFor("A320").Motion
	dt := 1.0 / fps
	for _, icao := range []string{"LKPR", "LROP", "EDDM"} {
		l := smoothLayout(t, icao)
		g, err := airport.BuildGraph(l)
		if err != nil {
			t.Fatal(err)
		}
		var ends []string
		for _, r := range l.Runways {
			ends = append(ends, r.Primary.Name, r.Secondary.Name)
		}
		total, routes := 0.0, 0
		for i, p := range l.Parking {
			if i%9 != 0 || p.Radius < 15 {
				continue
			}
			for _, end := range ends {
				r, err := g.RouteToRunway(i, end, airport.RouteOptions{})
				if err != nil || len(r.Points) < 2 {
					continue
				}
				path, err := NewGroundPath(r.Points, prof)
				if err != nil {
					continue
				}
				m := NewGroundMover(path, prof)
				type frame struct{ at, hdg, v float64 }
				var log []frame
				worstTurn, worstDv, secs := 0.0, 0.0, 0.0
				for n := 0; n < 3600*fps; n++ {
					pose := m.Step(dt)
					secs += dt
					v := pose.GroundSpeedKts * ktsToMS
					if len(log) > 0 {
						worstDv = math.Max(worstDv, math.Abs(v-log[len(log)-1].v))
					}
					log = append(log, frame{secs, pose.Heading, v})
					for j := len(log) - 2; j >= 0; j-- {
						if gap := secs - log[j].at; gap >= window-1e-9 {
							worstTurn = math.Max(worstTurn, math.Abs(math.Remainder(pose.Heading-log[j].hdg, 360))/gap)
							break
						}
					}
					if pose.Arrived {
						break
					}
				}
				routes++
				total += secs
				where := fmt.Sprintf("%s %s to %s", icao, p.Label(), end)
				if worstTurn > jumpTraceDegPerSec {
					t.Errorf("%s: turning %.1f°/s over 200 ms, want %.0f at most", where, worstTurn, jumpTraceDegPerSec)
				}
				if worstDv > groundSweepDvMS {
					t.Errorf("%s: speed changed %.2f m/s in a frame, want %.1f at most", where, worstDv, groundSweepDvMS)
				}
				t.Logf("%s: %.1f s, %.0f m", where, secs, path.Length())
			}
		}
		t.Logf("%s: %d routes, %.0f s in all", icao, routes, total)
		if routes == 0 {
			t.Errorf("%s: no route driven", icao)
		}
	}
}

// groundSweepDvMS: the most a ground mover's speed may change in a frame.
const groundSweepDvMS = 0.5
