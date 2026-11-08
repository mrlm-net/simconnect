package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A hold-short queue moves up like traffic in a jam: when the leader moves
// up, each follower starts only after the one ahead has moved off, at
// least a second later and never in the same frame, and they stand at
// spacings of their own. Started the moment the leader moved, the queue
// rolled off in step (live LKPR, 2026-11-07). Five aircraft queued for
// both ends of LKPR runway 06/24.
func TestQueueMovesUpLikeAJam(t *testing.T) {
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	for _, rwy := range []string{"24", "06"} {
		t.Run(rwy, func(t *testing.T) {
			route, err := g.RouteToRunwayEntry(c22, rwy, "", withSpan(airport.RouteOptions{}, DefaultMotionProfile()))
			if err != nil {
				t.Fatal(err)
			}
			prof := DefaultMotionProfile()
			path, err := NewGroundPath(route.Points, prof)
			if err != nil {
				t.Fatal(err)
			}
			const n = 5
			moveUp := 60.0 // the leader holds this far short of the end, then moves up
			if path.Length() < moveUp+n*60+100 {
				t.Fatalf("route only %.0f m", path.Length())
			}
			picture := NewGroundPicture()
			now := time.Now()
			planes := make([]*groundDrive, n)
			for k := range planes {
				m := NewGroundMover(path, prof)
				m.HoldAt(0) // sent off one after the other
				planes[k] = &groundDrive{mover: m, picture: picture, followTraffic: true, prof: prof, object: uint32(100 + k),
					clock: func() time.Time { return now }}
			}
			planes[0].mover.hold = path.Length() - moveUp
			frame, active := 0, 0 // the first active ones are on the taxiway
			step := func() {
				now = now.Add(time.Second / 60)
				frame++
				for _, d := range planes[:active] {
					p := d.mover.Pose()
					picture.Report(d.object, p.Position, p.Heading, prof, now)
				}
				for _, d := range planes[:active] {
					d.followAhead(now)
					d.mover.Step(1.0 / 60)
				}
			}
			// Into the queue: one sent every 25 s, until all stand still.
			for k := range planes {
				if k > 0 {
					planes[k].mover.ClearHold()
				}
				active = k + 1
				for range 60 * 25 {
					step()
				}
			}
			for i := 0; i < 60*300; i++ {
				still := true
				for _, d := range planes {
					still = still && d.mover.v == 0
				}
				if still && planes[n-1].mover.Pose().Distance > 1 {
					break
				}
				step()
			}
			spacing := map[float64]bool{}
			for k := 1; k < n; k++ {
				a, b := planes[k-1].mover.Pose(), planes[k].mover.Pose()
				if b.GroundSpeedKts != 0 || a.GroundSpeedKts != 0 {
					t.Fatalf("queue not standing: %d at %.1f kt", k, b.GroundSpeedKts)
				}
				spacing[math.Round((a.Distance-b.Distance)*10)/10] = true
			}
			// The leader moves up; when does each start?
			planes[0].mover.ClearHold()
			started := make([]int, n)
			minGap := math.Inf(1)
			for i := 0; i < 60*120; i++ {
				step()
				for k, d := range planes {
					if started[k] == 0 && d.mover.v > 0 {
						started[k] = frame
					}
				}
				for k := 1; k < n; k++ {
					minGap = math.Min(minGap, localDist(planes[k-1].mover.Pose().Position, planes[k].mover.Pose().Position))
				}
			}
			for k := 1; k < n; k++ {
				if started[k] == 0 {
					t.Fatalf("aircraft %d never moved up (started %v)", k, started)
				}
				if dt := float64(started[k]-started[k-1]) / 60; dt < 1 {
					t.Errorf("aircraft %d started %.2f s after the one ahead (frames %v), want at least 1 s", k, dt, started)
				}
			}
			if len(spacing) < 2 {
				t.Errorf("all stood at the same spacing %v", spacing)
			}
			if minGap < TrafficGapMeters {
				t.Errorf("came within %.1f m", minGap)
			}
			t.Logf("runway %s: starts (frames) %v, spacings %v, closest %.1f m", rwy, started, spacing, minGap)
		})
	}
}
