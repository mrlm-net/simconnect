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

// pushbackWithPicture is an injected departure from LKPR C22 sharing
// picture, held for its clearances.
func pushbackWithPicture(t *testing.T, picture *GroundPicture) (*TaxiController, func(int), *time.Time) {
	t.Helper()
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj), TaxiWithGroundPicture(picture))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: "FSLTL A320 Air France SL", Tail: "CSA1", HoldForClearances: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	go func() {
		for range ctl.Events() {
		}
	}()
	stand := g.Layout.Parking[c22]
	frames := func(n int) {
		for i := 0; i < n; i++ {
			now = now.Add(time.Second / 60)
			ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, stand.Position, 0, 0, true))
		}
	}
	return ctl, frames, &now
}

// TestPushbackWaitsForTrafficBehind: cleared to push while another
// aircraft taxis along the lane behind the stand, the pushback does not
// start until that aircraft's path is clear of the corridor (#334).
func TestPushbackWaitsForTrafficBehind(t *testing.T) {
	picture := NewGroundPicture()
	ctl, frames, now := pushbackWithPicture(t, picture)
	frames(60)
	path, err := ctl.pushPath()
	if err != nil {
		t.Fatal(err)
	}
	corridor := pushCorridor(path, 0, ctl.profile())
	// Traffic 150 m away whose taxi path runs through the corridor.
	far := offsetHeading(corridor[len(corridor)-1], 90, 150)
	report := func(ahead []airport.LatLon) {
		picture.Report(99, far, 270, DefaultMotionProfile(), *now)
		picture.ReportPath(99, ahead, 17.9)
	}
	report(corridor[len(corridor)/2:])
	ctl.ClearPushback()
	for i := 0; i < 60*20; i++ {
		report(corridor[len(corridor)/2:])
		frames(1)
	}
	if ctl.State() != TaxiAwaitingPushback || !ctl.last.PushbackHeld {
		t.Fatalf("state %v, held %v: pushed into traffic", ctl.State(), ctl.last.PushbackHeld)
	}
	// The traffic has passed: nothing ahead of it crosses the corridor.
	for i := 0; i < 60*10 && ctl.State() == TaxiAwaitingPushback; i++ {
		report(nil)
		frames(1)
	}
	if ctl.State() != TaxiPushback || ctl.last.PushbackHeld {
		t.Fatalf("state %v, held %v: want pushing once clear", ctl.State(), ctl.last.PushbackHeld)
	}
}

// TestPushbackStopsForTraffic: an aircraft moving into what the push still
// sweeps stops it; the push goes on once it is gone.
func TestPushbackStopsForTraffic(t *testing.T) {
	picture := NewGroundPicture()
	ctl, frames, now := pushbackWithPicture(t, picture)
	frames(60)
	ctl.ClearPushback()
	for i := 0; i < 60*60 && ctl.State() != TaxiPushback; i++ {
		frames(1)
	}
	frames(60 * 8) // under way
	if ctl.mover == nil || ctl.mover.Pose().GroundSpeedKts < 0.5 {
		t.Fatal("not pushing")
	}
	rest := pushCorridor(ctl.mover.Path(), ctl.mover.Pose().Distance, ctl.profile())
	intruder := rest[len(rest)-1]
	for i := 0; i < 60*20; i++ {
		picture.Report(99, intruder, 0, DefaultMotionProfile(), *now)
		frames(1)
	}
	held := ctl.mover.Pose()
	if !ctl.last.PushbackHeld || held.GroundSpeedKts > 0.1 {
		t.Fatalf("held %v at %.1f kt", ctl.last.PushbackHeld, held.GroundSpeedKts)
	}
	picture.Forget(99)
	frames(60 * 10)
	if ctl.last.PushbackHeld || ctl.mover == nil || ctl.mover.Pose().Distance <= held.Distance+0.5 {
		t.Fatal("did not go on once clear")
	}
	for i := 0; i < 60*120 && ctl.State() == TaxiPushback; i++ {
		frames(1)
	}
	if ctl.State() != TaxiAwaitingTaxi || ctl.last.PushbackHeld {
		t.Fatalf("after the push: state %v, still held %v", ctl.State(), ctl.last.PushbackHeld)
	}
}

// TestTaxiGivesWayToPushback: a taxiing aircraft whose path crosses the
// corridor of a pushback under way waits for it; the push goes on without
// stopping (#334, live: the push stopped for the taxiing aircraft instead).
func TestTaxiGivesWayToPushback(t *testing.T) {
	picture := NewGroundPicture()
	ctl, frames, now := pushbackWithPicture(t, picture)
	frames(60)
	ctl.ClearPushback()
	for i := 0; i < 60*60 && ctl.State() != TaxiPushback; i++ {
		frames(1)
	}
	frames(60 * 5) // under way
	rest := pushCorridor(ctl.mover.Path(), ctl.mover.Pose().Distance, ctl.profile())
	cross := rest[len(rest)-1]
	// A taxiing aircraft 120 m away, heading across the end of the push
	// (square to where the push corridor ends, whichever way the push goes).
	prof := DefaultMotionProfile()
	dir := localBearing(rest[len(rest)-2], cross)
	a := offsetHeading(cross, dir+90, 120)
	b := offsetHeading(cross, dir-90, 200)
	path, err := NewGroundPath([]airport.LatLon{a, b}, prof)
	if err != nil {
		t.Fatal(err)
	}
	taxi := &groundDrive{mover: NewGroundMover(path, prof), picture: picture, followTraffic: true, prof: prof, object: 99,
		clock: func() time.Time { return *now }}
	minGap, pushStops := 1e9, 0
	wasMoving := false
	for i := 0; i < 60*240 && ctl.State() == TaxiPushback; i++ {
		p := taxi.mover.Pose()
		picture.Report(99, p.Position, p.Heading, prof, *now)
		taxi.followAhead(*now)
		taxi.mover.Step(1.0 / 60)
		frames(1)
		pp := ctl.mover.Pose()
		minGap = math.Min(minGap, localDist(p.Position, pp.Position))
		if wasMoving && pp.GroundSpeedKts < 0.05 && !pp.Arrived {
			pushStops++
		}
		wasMoving = pp.GroundSpeedKts > 0.5
	}
	if ctl.State() != TaxiAwaitingTaxi {
		t.Fatalf("push did not finish: %v", ctl.State())
	}
	if pushStops > 0 || ctl.last.PushbackHeld {
		t.Errorf("the push stopped %d times for the taxiing aircraft", pushStops)
	}
	if minGap < 2*17.9 {
		t.Errorf("came within %.1f m of the pushing aircraft", minGap)
	}
	t.Logf("closest %.1f m", minGap)
}

// TestHoldingAircraftTakesNoPriority: an aircraft holding at its limit 40 m
// beside another's path (a side taxiway at a junction) reports no path
// ahead, so the moving aircraft keeps going: it used to brake to a near stop
// for it (theirs 0 m to the conflict) and only crept on once inside it.
func TestHoldingAircraftTakesNoPriority(t *testing.T) {
	origin := airport.LatLon{Lat: 50.1, Lon: 14.26}
	prof := DefaultMotionProfile()
	picture := NewGroundPicture()
	now := time.Now()
	mk := func(id uint32, a, b airport.LatLon, hold bool) *groundDrive {
		path, err := NewGroundPath([]airport.LatLon{a, b}, prof)
		if err != nil {
			t.Fatal(err)
		}
		d := &groundDrive{mover: NewGroundMover(path, prof), picture: picture, followTraffic: true, prof: prof, object: id,
			clock: func() time.Time { return now }}
		if hold {
			d.mover.HoldAt(0) // holding at its clearance limit, here
		}
		return d
	}
	mover := mk(1, offsetHeading(origin, 270, 200), offsetHeading(origin, 90, 200), false)
	// On a side taxiway facing the junction, its nose gear 40 m from the
	// path: its nose clear of the wing, within the give-way reach.
	side := offsetHeading(origin, 0, 40)
	holder := mk(2, side, origin, true)
	slowest := 1e9
	for i := 0; i < 60*120 && !mover.mover.Pose().Arrived; i++ {
		now = now.Add(time.Second / 60)
		for _, d := range []*groundDrive{mover, holder} {
			p := d.mover.Pose()
			picture.Report(d.object, p.Position, p.Heading, prof, now)
			d.followAhead(now)
			d.mover.Step(1.0 / 60)
		}
		if p := mover.mover.Pose(); p.Distance > 80 && p.Distance < 300 {
			slowest = math.Min(slowest, p.GroundSpeedKts)
		}
	}
	if !mover.mover.Pose().Arrived || slowest < 10 {
		t.Fatalf("slowed to %.1f kt for an aircraft holding beside its path", slowest)
	}
}

// A pushback not started waits for a neighbour's push under way through
// the same corridor (at LKPR A1 and A3 pushed at once and each stopped for
// the other's body for good); the push under way does not wait for it.
func TestPushWaitsForNeighbourPush(t *testing.T) {
	p := NewGroundPicture()
	now := time.Now()
	base := airport.LatLon{Lat: 50.1, Lon: 14.26}
	at := func(east, north float64) airport.LatLon { return offsetHeading(offsetHeading(base, 90, east), 0, north) }
	line := func(e0, n0, e1, n1 float64) []airport.LatLon {
		var out []airport.LatLon
		for i := 0; i <= 20; i++ {
			f := float64(i) / 20
			out = append(out, at(e0+(e1-e0)*f, n0+(n1-n0)*f))
		}
		return out
	}
	// Stand 1 at x=0, stand 2 at x=80 (bodies far apart), both pushing south
	// onto the same taxiway lane at y=-60: 1 already pushing east along it.
	p.Report(1, at(0, -60), 0, MotionProfile{}, now)
	p.ReportPush(1, line(0, -60, 90, -60), 18)
	p.Report(2, at(80, 0), 180, MotionProfile{}, now)
	mine := line(80, 0, 80, -60)
	if _, blocked := p.corridorBlocked(2, mine, 18, true, now); !blocked {
		t.Error("a push starts into a neighbour's push under way")
	}
	if _, blocked := p.corridorBlocked(1, line(0, -60, 90, -60), 18, false, now); blocked {
		t.Error("the push under way stops for a neighbour still on its stand, clear of its corridor")
	}
}

// Beside a push under way, an aircraft whose path meets the push corridor
// within half a span ahead waits where it is; it goes on only when it is
// already in the corridor (at LKPR one drove into a neighbour's push and
// both waited for each other for minutes).
func TestGiveWayToPushUnderWay(t *testing.T) {
	p := NewGroundPicture()
	now := time.Now()
	base := airport.LatLon{Lat: 50.1, Lon: 14.26}
	at := func(east, north float64) airport.LatLon { return offsetHeading(offsetHeading(base, 90, east), 0, north) }
	var corridor []airport.LatLon
	for x := 0.0; x <= 60; x += trafficBodyStep {
		corridor = append(corridor, at(x, 0)) // the push sweeps east along y=0
	}
	p.Report(1, at(0, 0), 270, MotionProfile{}, now)
	p.ReportPush(1, corridor, 18)
	prof := DefaultMotionProfile()
	// Me: north of the corridor, taxiing south across it; the first point
	// within reach (both half-spans and the margin) is a few metres ahead.
	half := 17.0
	reach := half + 18 + GiveWayMarginMeters
	for _, c := range []struct {
		name   string
		startN float64
		wait   bool
	}{
		{"at the corridor's edge", reach + 5, true},
		{"already in it", reach - 2, false},
	} {
		me := at(30, c.startN)
		path, err := NewGroundPath([]airport.LatLon{me, at(30, -80)}, prof)
		if err != nil {
			t.Fatal(err)
		}
		p.Report(2, me, 180, MotionProfile{}, now)
		gw := p.giveWay(2, path, 0, GiveWayLookMeters, half, now)
		if waits := !math.IsInf(gw, 1); waits != c.wait {
			t.Errorf("%s: gives way %v (at %.1f m), want %v", c.name, waits, gw, c.wait)
		}
	}
}
