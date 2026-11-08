package traffic

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The aircraft ahead turns off the way and stops just past the turn, its
// fuselage clear of the way, a wing still in it: the one behind stops for
// it, at its normal braking, rather than taxiing past the wing (user, live
// LKPR 2026-11-07: "the plane behind should keep distance and slow down or
// stop when the plane ahead is turning"). Before, only its fuselage was
// looked at: from C22, one for runway 24 stopped 51 m into its turn off the
// way to 06, and the one for 06 went past at 13 kt, 1.6 m from its wing.
func TestFollowerStopsForATurnedOffWing(t *testing.T) {
	const minClear = 3.0 // metres between the two, wings and fuselages
	g := lkprGraph(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	prof := DefaultMotionProfile()
	mk := func(rwy string) *GroundPath {
		route, err := g.RouteToRunwayEntry(c22, rwy, "", withSpan(airport.RouteOptions{}, prof))
		if err != nil {
			t.Fatal(err)
		}
		p, err := NewGroundPath(route.Points, prof)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, pair := range [][2]string{{"24", "06"}, {"06", "24"}} {
		lp, fp := mk(pair[0]), mk(pair[1])
		// Where the leader's way leaves the follower's.
		split := 0.0
		for s := 0.0; s < lp.Length(); s++ {
			if _, off := fp.DistanceTo(lp.PointAt(s)); off > 2 {
				split = s
				break
			}
		}
		if split == 0 {
			t.Fatalf("%s/%s: the ways never part", pair[0], pair[1])
		}
		for past := 45.0; past <= 60; past++ {
			t.Run(fmt.Sprintf("%s-%s/%.0fm", pair[0], pair[1], past), func(t *testing.T) {
				picture := NewGroundPicture()
				now := time.Now()
				mkd := func(obj uint32, path *GroundPath) *groundDrive {
					return &groundDrive{mover: NewGroundMover(path, prof), picture: picture, followTraffic: true, prof: prof, object: obj,
						clock: func() time.Time { return now }}
				}
				lead, foll := mkd(201, lp), mkd(202, fp)
				lead.mover.HoldAt(split + past)
				foll.mover.HoldAt(0) // sent off once the leader stands
				released := false
				least, hardest := math.Inf(1), 0.0
				for frame := 0; frame < 60*240; frame++ {
					now = now.Add(time.Second / 60)
					for _, d := range []*groundDrive{lead, foll} {
						if d == foll && !released {
							continue // not on the taxiway yet
						}
						p := d.mover.Pose()
						picture.Report(d.object, p.Position, p.Heading, prof, now)
					}
					for _, d := range []*groundDrive{lead, foll} {
						d.followAhead(now)
						d.mover.Step(1.0 / 60)
					}
					if !released && lead.mover.v == 0 && lead.mover.s >= split+past-1 {
						foll.mover.ClearHold()
						released = true
					}
					if !released {
						continue
					}
					c := bodyClearance(foll.mover.Pose(), lead.mover.Pose(), prof)
					if c < least {
						least = c
					}
					hardest = math.Min(hardest, foll.mover.a)
					if c < minClear {
						t.Fatalf("t %.1f s: %.1f m from the leader standing %.0f m into its turn off (follower %.1f kt, %.0f m along)",
							float64(frame)/60, c, past, foll.mover.v/ktsToMS, foll.mover.s)
					}
					if foll.mover.a < -1.5*prof.Decel-0.01 {
						t.Fatalf("t %.1f s: braking %.2f m/s² (normal cap %.2f), %.1f m from the leader", float64(frame)/60, foll.mover.a, 1.5*prof.Decel, c)
					}
					if foll.mover.s > split+150 {
						break // well past
					}
				}
				if !released {
					t.Fatal("the leader never stood")
				}
				t.Logf("closest %.1f m, hardest braking %.2f m/s², the follower %.0f m along at %.1f kt", least, hardest, foll.mover.s, foll.mover.v/ktsToMS)
			})
		}
	}
}

// bodyClearance is the least distance between two aircraft of profile p,
// each drawn as its fuselage axis (nose to tail), a straight wing line
// across its reference point and its tailplane.
func bodyClearance(a, b GroundPose, p MotionProfile) float64 {
	pts := func(q GroundPose) []airport.LatLon {
		var out []airport.LatLon
		nose := p.WheelbaseMeters*pushNoseFactor - p.RefAheadMeters
		for d := -(p.TailMeters + p.RefAheadMeters); d <= nose; d++ {
			out = append(out, offsetHeading(q.Position, q.Heading, d))
		}
		gear := offsetHeading(q.Position, q.Heading, -p.RefAheadMeters)
		for w := 1.0; w <= p.SpanMeters/2; w++ {
			out = append(out, offsetHeading(gear, q.Heading+90, w), offsetHeading(gear, q.Heading-90, w))
		}
		// The tailplane, a third of the span across, 3 m in from the tail.
		tp := offsetHeading(q.Position, q.Heading, -(p.TailMeters + p.RefAheadMeters - 3))
		for w := 1.0; w <= p.SpanMeters/6; w++ {
			out = append(out, offsetHeading(tp, q.Heading+90, w), offsetHeading(tp, q.Heading-90, w))
		}
		return out
	}
	best := math.Inf(1)
	pb := pts(b)
	for _, x := range pts(a) {
		for _, y := range pb {
			best = math.Min(best, localDist(x, y))
		}
	}
	return best
}
