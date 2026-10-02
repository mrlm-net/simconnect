//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestVehicleGivesWay: a tug driving in from its depot at LKPR stops short
// of an aircraft taxiing across its road, waits while it is there, and
// drives on once it has passed; one already in the aircraft's path clears
// it; parked aircraft and its own do not stop it.
func TestVehicleGivesWay(t *testing.T) {
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
	path := tug.arrive.Path()
	// An aircraft taxiing across the road 150 m in, its path ahead crossing.
	cross := path.PointAt(150)
	hdg := localBearing(path.PointAt(145), path.PointAt(155)) + 90
	at := offsetHeading(cross, hdg+180, 60)
	var ahead []airport.LatLon
	for d := 0.0; d <= 120; d += trafficBodyStep {
		ahead = append(ahead, offsetHeading(at, hdg, d))
	}
	report := func() {
		pic.Report(1, at, hdg, prof, now)
		pic.ReportPath(1, ahead, DefaultHalfSpanMeters)
	}
	// Its own aircraft (77) and a parked one by the road do not count.
	pic.Report(77, pose.Position, pose.Heading, prof, now)
	pic.Report(2, path.PointAt(60), hdg, prof, now)
	report()
	var stopped float64
	for k := 0; k < 60*120; k++ {
		now = now.Add(time.Second / 60)
		report()
		if err := tug.Update(pose, true, 1.0/60); err != nil {
			t.Fatal(err)
		}
		if p := tug.pose; p.Stopped && p.Distance > 50 {
			stopped = p.Distance
		}
	}
	if stopped == 0 {
		t.Fatalf("never stopped for the crossing aircraft (at %.0f m)", tug.pose.Distance)
	}
	reach := vehicleHalfMeters + DefaultHalfSpanMeters + VehicleClearMeters
	if stopped > 150-reach+1 {
		t.Errorf("stopped at %.0f m, inside the aircraft's reach (crossing at 150 m, reach %.0f m)", stopped, reach)
	}
	if !tug.waiting {
		t.Error("not waiting")
	}
	// The aircraft has passed: on to the nose.
	pic.Forget(1)
	for k := 0; k < 60*600 && !tug.Connected(); k++ {
		now = now.Add(time.Second / 60)
		tug.Update(pose, true, 1.0/60)
	}
	if !tug.Connected() {
		t.Fatalf("never reached the nose after the aircraft passed (at %.0f m)", tug.pose.Distance)
	}
	t.Logf("waited %.0f m along its way, %.0f m short of the crossing", stopped, 150-stopped)

	// Already in the path: it does not stop.
	y := &vehicleYield{}
	y.SetTraffic(pic, 77, func() time.Time { return now })
	pic.Report(3, offsetHeading(cross, hdg+180, 80), hdg, prof, now)
	pic.ReportPath(3, ahead, DefaultHalfSpanMeters)
	if _, _, ok := pic.VehicleConflict(77, 0, []airport.LatLon{cross, path.PointAt(152)}, vehicleHalfMeters, false, now); ok {
		t.Error("stops in the middle of the aircraft's path")
	}
}

// TestVehiclesRespectEachOther: two tugs leaving the same depot together
// for neighbouring LKPR stands do not drive through each other: one waits
// and follows (live, two tugs looked like one on the map); an oncoming
// vehicle passes in its own lane.
func TestVehiclesRespectEachOther(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	pic := NewGroundPicture()
	now := time.Now()
	clock := func() time.Time { return now }
	c := &tugClient{}
	inj := NewInjector(c)
	prof := DefaultMotionProfile()
	var tugs []*SimObjectTug
	var poses []GroundPose
	for k, label := range []string{"B9", "B10"} {
		i, _ := l.ParkingIndex(label)
		tug := NewSimObjectTug(c, inj, DefaultTugTitle, uint32(9101+k), prof)
		tug.Layout = l
		tug.SetTraffic(pic, uint32(500+k), clock)
		stand := l.Parking[i]
		pose := GroundPose{Position: StandPoint(stand, prof.RefAheadMeters), Heading: stand.Heading}
		if err := tug.Attach(pose); err != nil {
			t.Fatal(err)
		}
		tug.Handle(assignedMsg(uint32(9101+k), uint32(60+k)))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, uint32(60+k), 1200, 3))
		tugs, poses = append(tugs, tug), append(poses, pose)
	}
	if localDist(tugs[0].pose.Position, tugs[1].pose.Position) > 1 {
		t.Skip("the two stands have different depots: nothing to share")
	}
	closest := math.Inf(1)
	for step := 0; step < 60*600 && !(tugs[0].Connected() && tugs[1].Connected()); step++ {
		now = now.Add(time.Second / 60)
		for k, tug := range tugs {
			if err := tug.Update(poses[k], true, 1.0/60); err != nil {
				t.Fatal(err)
			}
		}
		if step > 60*5 && !tugs[0].Connected() && !tugs[1].Connected() {
			closest = math.Min(closest, localDist(tugs[0].pose.Position, tugs[1].pose.Position))
		}
	}
	if !tugs[0].Connected() || !tugs[1].Connected() {
		t.Fatalf("not both at their noses: %v %v", tugs[0].Connected(), tugs[1].Connected())
	}
	if closest < VehicleGapMeters {
		t.Errorf("the tugs came %.1f m apart on the way", closest)
	}
	t.Logf("closest on the way %.1f m", closest)

	// Oncoming: no conflict; following: one.
	p := l.Parking[0].Position
	pic.ReportVehicle(900, offsetHeading(p, 90, 20), 270, 5, now)
	ahead := []airport.LatLon{p, offsetHeading(p, 90, 2), offsetHeading(p, 90, 4), offsetHeading(p, 90, 20)}
	for i := 4; i < 15; i++ {
		ahead = append(ahead, offsetHeading(p, 90, float64(i)*vehicleLookStep))
	}
	if _, _, ok := pic.VehicleConflict(0, 901, ahead, vehicleHalfMeters, false, now); ok {
		t.Error("stopped for an oncoming vehicle")
	}
	pic.ReportVehicle(900, offsetHeading(p, 90, 20), 90, 5, now)
	if _, v, ok := pic.VehicleConflict(0, 901, ahead, vehicleHalfMeters, false, now); !ok || !v {
		t.Error("drove into the vehicle ahead")
	}
}
