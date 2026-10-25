package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTugBacksOffClearOfTraffic: after the push the tug backs away from
// the nose only once no aircraft taxis through the space it backs into
// (offline LKPR: the tug of CSA311, pushed and waiting to taxi, backed off
// 5 m from TVS698 taxiing past the nose).
func TestTugBacksOffClearOfTraffic(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	i, _ := l.ParkingIndex("B9")
	c := &tugClient{}
	inj := NewInjector(c)
	prof := DefaultMotionProfile()
	tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
	tug.Layout = l
	pic := NewGroundPicture()
	now := time.Now()
	tug.SetTraffic(pic, 77, func() time.Time { return now })
	stand := l.Parking[i]
	pose := GroundPose{Position: StandPoint(stand, prof.RefAheadMeters), Heading: stand.Heading}
	if err := tug.Attach(pose); err != nil {
		t.Fatal(err)
	}
	tug.Handle(assignedMsg(9001, 55))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
	for k := 0; k < 60*600 && !tug.Connected(); k++ {
		now = now.Add(time.Second / 60)
		tug.Update(pose, true, 1.0/60)
	}
	if !tug.Connected() {
		t.Fatal("never reached the nose")
	}
	start := tug.pose.Position
	// An aircraft taxiing across, 10 m in front of the tug on the side it
	// backs into (the tug faces its aircraft).
	back := tug.pose.Heading + 180
	hdg := back + 90
	at := offsetHeading(start, back, 10)
	var ahead []airport.LatLon
	for d := 0.0; d <= 120; d += trafficBodyStep {
		ahead = append(ahead, offsetHeading(at, hdg, d))
	}
	for k := 0; k < 60*30; k++ {
		now = now.Add(time.Second / 60)
		pic.Report(1, at, hdg, prof, now)
		pic.ReportPath(1, ahead, DefaultHalfSpanMeters)
		pic.Report(77, pose.Position, pose.Heading, prof, now)
		if err := tug.Update(pose, false, 1.0/60); err != nil {
			t.Fatal(err)
		}
		if d := localDist(tug.pose.Position, start); d > 1 {
			t.Fatalf("backed off %.1f m into the taxiing aircraft after %.1f s", d, float64(k)/60)
		}
	}
	// It has passed: the tug backs off and leaves.
	pic.Forget(1)
	for k := 0; k < 60*30; k++ {
		now = now.Add(time.Second / 60)
		pic.Report(77, pose.Position, pose.Heading, prof, now)
		tug.Update(pose, false, 1.0/60)
	}
	if d := localDist(tug.pose.Position, start); d < 5 {
		t.Errorf("still %.1f m from the nose 30 s after the aircraft passed", d)
	}
}
