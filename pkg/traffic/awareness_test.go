//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestGroundPictureFollowing: two departures from neighbouring LKPR stands
// to the same runway share a ground picture. The first is held at the
// holding point (no line-up clearance); the second queues behind it at a
// safe gap instead of stopping on top of it, and both depart once the first
// is cleared.
func TestGroundPictureFollowing(t *testing.T) {
	queueBehind(t, true)
}

func queueBehind(t *testing.T, shared bool) float64 {
	g := lkprGraph(t)
	picture := NewGroundPicture()
	type plane struct {
		ctl *TaxiController
		inj *Injector
		obj uint32
		p   int
	}
	now := time.Now()
	var planes []plane
	for k, label := range []string{"C22", "C20"} {
		pi, _ := g.Layout.ParkingIndex(label)
		ec := &eventClient{}
		inj := NewInjector(ec)
		opts := []TaxiOption{TaxiWithInjector(inj)}
		if shared {
			opts = append(opts, TaxiWithGroundPicture(picture))
		}
		ctl := NewTaxiController(NewFleet(ec), opts...)
		// The first waits at the holding point for its line-up clearance.
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: pi, Runway: "24", Model: "A320", RollingTakeoffChance: -1, HoldForClearances: true}); err != nil {
			t.Fatal(err)
		}
		ctl.now = func() time.Time { return now }
		obj := uint32(77 + k)
		ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, obj))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, obj, 1200, 12))
		go func() {
			for range ctl.Events() {
			}
		}()
		planes = append(planes, plane{ctl, inj, obj, pi})
	}
	for _, p := range planes {
		p.ctl.ClearPushback()
		p.ctl.ClearToTaxi()
	}
	minDist := 1e9
	for i := 0; i < 60*1800; i++ {
		now = now.Add(time.Second / 60)
		done := true
		for _, p := range planes {
			if !p.ctl.State().Terminal() {
				done = false
				p.ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, p.obj, g.Layout.Parking[p.p].Position, 0, 0, true))
			}
		}
		if done {
			break
		}
		a, b := planes[0].ctl, planes[1].ctl
		// Whichever holds short first is cleared once the other has queued
		// behind it (or stopped on top of it); then the other.
		for _, p := range [][2]*TaxiController{{a, b}, {b, a}} {
			first, second := p[0], p[1]
			if first.State() != TaxiHoldingShort {
				continue
			}
			if second.State().Terminal() || second.State() >= TaxiDeparting {
				first.ClearForTakeoff() // the other has gone
				continue
			}
			if first.mover != nil && second.mover != nil && second.State() >= TaxiTaxiing && second.mover.Pose().GroundSpeedKts < 0.5 &&
				localDist(first.mover.Pose().Position, second.mover.Pose().Position) < 120 {
				first.ClearForTakeoff() // the other has queued behind
			}
		}
		taxiing := func(c *TaxiController) bool {
			return c.mover != nil && c.State() >= TaxiTaxiing && c.State() < TaxiDeparting
		}
		if taxiing(a) && taxiing(b) {
			if d := localDist(a.mover.Pose().Position, b.mover.Pose().Position); d < minDist {
				minDist = d
			}
		}
	}
	if shared {
		for k, p := range planes {
			if p.ctl.State() != TaxiComplete {
				t.Errorf("plane %d ended %v", k, p.ctl.State())
			}
		}
		if minDist < 30 {
			t.Errorf("taxiing aircraft came within %.1f m of each other", minDist)
		}
	}
	t.Logf("shared picture %v: closest while both taxiing %.1f m", shared, minDist)
	return minDist
}

// TestGroundPictureControl: the same without the picture ends with the two
// aircraft on top of each other — the test does exercise the queueing.
func TestGroundPictureControl(t *testing.T) {
	if d := queueBehind(t, false); d >= 30 {
		t.Errorf("without the picture the aircraft stayed %.1f m apart: the scenario does not test queueing", d)
	}
}

// TestGroundPictureGivesWay: two aircraft on routes crossing at right
// angles, reaching the crossing at about the same time: the one further
// away holds back (slows or stops short of it) and goes once the other is
// through; the one closer keeps its speed; they never come closer than
// their spans allow, and both get through (#334).
func TestGroundPictureGivesWay(t *testing.T) {
	origin := airport.LatLon{Lat: 50.1, Lon: 14.26}
	prof := DefaultMotionProfile()
	picture := NewGroundPicture()
	now := time.Now()
	type plane struct {
		d *groundDrive
	}
	mk := func(id uint32, hdg, start float64) *groundDrive {
		a := offsetHeading(origin, hdg+180, start)
		b := offsetHeading(origin, hdg, 300)
		path, err := NewGroundPath([]airport.LatLon{a, b}, prof)
		if err != nil {
			t.Fatal(err)
		}
		return &groundDrive{mover: NewGroundMover(path, prof), picture: picture, followTraffic: true, prof: prof, object: id,
			clock: func() time.Time { return now }}
	}
	planes := []*groundDrive{mk(1, 90, 160), mk(2, 0, 150)}
	half := prof.SpanMeters / 2
	if half <= 0 {
		half = DefaultHalfSpanMeters
	}
	minDist := 1e9
	slowest := map[uint32]float64{1: 1e9, 2: 1e9} // lowest speed once up to speed
	for i := 0; i < 60*300; i++ {
		now = now.Add(time.Second / 60)
		for _, d := range planes {
			p := d.mover.Pose()
			picture.Report(d.object, p.Position, p.Heading, prof, now)
		}
		for _, d := range planes {
			d.followAhead(now)
			p := d.mover.Step(1.0 / 60)
			if p.Distance > 60 && p.Distance < 200 {
				slowest[d.object] = math.Min(slowest[d.object], p.GroundSpeedKts)
			}
		}
		a, b := planes[0].mover.Pose(), planes[1].mover.Pose()
		minDist = math.Min(minDist, localDist(a.Position, b.Position))
		if a.Arrived && b.Arrived {
			break
		}
	}
	if !planes[0].mover.Pose().Arrived || !planes[1].mover.Pose().Arrived {
		t.Fatalf("not through: %+v / %+v", planes[0].mover.Pose(), planes[1].mover.Pose())
	}
	// Aircraft 2 is 10 m closer to the crossing: 1 gives way.
	if slowest[1] > 8 || slowest[2] < 12 {
		t.Errorf("lowest speeds near the crossing %v: want 1 holding back, 2 going on", slowest)
	}
	if minDist < 2*half {
		t.Errorf("came within %.1f m (spans %.1f m)", minDist, 2*half)
	}
	t.Logf("closest %.1f m, lowest speeds %v", minDist, slowest)
}
