//go:build windows
// +build windows

package traffic

import (
	"testing"
	"time"
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
