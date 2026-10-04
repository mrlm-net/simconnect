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
// already close enough for the push to stop for it (#452) (at LKPR one drove into a neighbour's push and
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
		{"within the margin, clear of the push", reach - 2, true},
		{"already in it: the push stops for it", half + 18 + PushClearMarginMeters - 1, false},
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

// Facing oncoming traffic, an aircraft keeps the junction it will turn off
// at clear (#444). LKPR, live: CSA273 taxied west along Z towards WZZ1529,
// pushed onto Z facing it just beyond the Z junction (20/16 in local
// meters); CSA273 stopped at the gap behind WZZ1529 with its nose over the
// junction's north branch, where WZZ1529 was to turn, and neither moved
// again. The stop now leaves that branch clear by the half-span and margin.
func TestJunctionStopFacingOncoming(t *testing.T) {
	g := lkprGraph(t)
	tp := g.Layout.TaxiPoints
	var pts []airport.LatLon
	for _, i := range []int{753, 752, 1877, 751, 1878, 750, 749, 1879, 748, 678, 677, 674} {
		pts = append(pts, tp[i].Position)
	}
	prof := MotionProfileFor("BCS3")
	path, err := NewGroundPath(pts, prof)
	if err != nil {
		t.Fatal(err)
	}
	// WZZ1529 stood 35 m beyond the junction, facing it.
	junction := tp[678].Position
	wzz := offsetHeading(tp[677].Position, localBearing(tp[677].Position, tp[674].Position), 12)
	sBody := 0.0
	for s := 0.0; s < path.Length(); s++ {
		if localDist(path.PointAt(s), wzz) < localDist(path.PointAt(sBody), wzz) {
			sBody = s
		}
	}
	sBody -= 20 // its tail
	noseTip := (pushNoseFactor - 1) * prof.WheelbaseMeters
	gap := sBody - noseTip - TrafficGapMeters
	clear := DefaultHalfSpanMeters + GiveWayMarginMeters
	d := &groundDrive{graph: g, prof: prof}
	stop := d.junctionStop(path, 0, gap, noseTip, clear)
	if stop >= gap {
		t.Fatalf("stop %.0f m: not short of the gap stop %.0f m", stop, gap)
	}
	// The north branch (Z towards A1) stays clear of the nose.
	nose := path.PointAt(stop + noseTip)
	for _, i := range []int{746, 745, 744, 743} {
		if dist := localDist(nose, tp[i].Position); dist < clear {
			t.Errorf("nose %.0f m from the branch at node %d, want at least %.0f", dist, i, clear)
		}
	}
	if localDist(path.PointAt(stop), junction) > 80 {
		t.Errorf("stop %.0f m from the junction: further back than needed", localDist(path.PointAt(stop), junction))
	}
	// No junction ahead: the gap stop stands.
	if s := d.junctionStop(path, 0, 10, noseTip, clear); s != 10 && s > 10 {
		t.Errorf("stop %.1f beyond the given 10 m", s)
	}
}

// A pushback waits while a moving aircraft's wing is over its corridor, not
// only its fuselage (#446; LKPR, live: DLH1740 pushed from A3 while TVS1823,
// pushing up A1, was a fuselage and 3 m from the corridor). A parked
// aircraft at the same distance does not hold it.
func TestPushWaitsForMovingWing(t *testing.T) {
	now := time.Unix(0, 0)
	base := airport.LatLon{Lat: 50.1, Lon: 14.26}
	var corridor []airport.LatLon
	for d := 0.0; d <= 60; d += trafficBodyStep {
		corridor = append(corridor, offsetHeading(base, 0, d))
	}
	half, oh := 12.0, 17.0
	// Abeam the corridor, fuselage parallel to it: its wing reaches 5 m in.
	beside := offsetHeading(offsetHeading(base, 0, 30), 90, half+PushClearMarginMeters+oh-5)
	for _, moving := range []bool{true, false} {
		p := NewGroundPicture()
		p.Report(2, beside, 0, DefaultMotionProfile(), now)
		if moving {
			p.ReportPush(2, []airport.LatLon{offsetHeading(beside, 90, 200)}, oh) // its way on, far off
		}
		_, blocked := p.corridorBlocked(1, corridor, half, true, now)
		if blocked != moving {
			t.Errorf("moving %v: blocked %v", moving, blocked)
		}
	}
}

// Pushed and waiting for its taxi clearance, an aircraft's planned way holds
// a neighbour's push that has not started, but gives it no priority over
// moving traffic (#452; LKPR, live: TVS706 pushed onto A1 where TVS795 was
// about to taxi, and TVS795 drove through the push).
func TestPlannedTaxiHoldsPushNotTraffic(t *testing.T) {
	now := time.Unix(0, 0)
	base := airport.LatLon{Lat: 50.1, Lon: 14.26}
	line := func(from airport.LatLon, hdg, length float64) []airport.LatLon {
		var pts []airport.LatLon
		for d := trafficBodyStep; d <= length; d += trafficBodyStep {
			pts = append(pts, offsetHeading(from, hdg, d))
		}
		return pts
	}
	corridor := line(base, 0, 60)
	// The waiting aircraft 100 m east, its way on west across the corridor.
	waiter := offsetHeading(offsetHeading(base, 0, 40), 90, 100)
	p := NewGroundPicture()
	p.Report(2, waiter, 270, DefaultMotionProfile(), now)
	p.ReportPlanned(2, line(waiter, 270, 200), 17)
	if _, blocked := p.corridorBlocked(1, corridor, 12, true, now); !blocked {
		t.Error("a push starts across the planned taxi of a waiting aircraft")
	}
	// A taxiing aircraft crossing the planned way does not give way to it.
	p.Report(3, offsetHeading(waiter, 270, 60), 180, DefaultMotionProfile(), now)
	path, err := NewGroundPath([]airport.LatLon{offsetHeading(offsetHeading(waiter, 270, 60), 0, 80), offsetHeading(offsetHeading(waiter, 270, 60), 180, 80)}, DefaultMotionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if gw := p.giveWay(3, path, 0, GiveWayLookMeters, 17, now); !math.IsInf(gw, 1) {
		t.Errorf("taxiing traffic gives way at %.0f m to an aircraft not cleared to move", gw)
	}
}

// Beside a push under way, an aircraft whose way starts within the margin of
// the corridor but whose wings do not overlap it waits (#452).
func TestWaitBesidePushUnderWay(t *testing.T) {
	now := time.Unix(0, 0)
	base := airport.LatLon{Lat: 50.1, Lon: 14.26}
	var corridor []airport.LatLon
	for d := 0.0; d <= 60; d += trafficBodyStep {
		corridor = append(corridor, offsetHeading(base, 0, d))
	}
	p := NewGroundPicture()
	p.Report(1, base, 180, DefaultMotionProfile(), now)
	p.ReportPush(1, corridor, 17)
	// 35 m abeam: within reach (12+17+10) but beyond where the push stops
	// for it (12+17+3).
	start := offsetHeading(offsetHeading(base, 0, 30), 90, 35)
	p.Report(2, start, 0, DefaultMotionProfile(), now)
	path, err := NewGroundPath([]airport.LatLon{start, offsetHeading(start, 0, 100)}, DefaultMotionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if gw := p.giveWay(2, path, 0, GiveWayLookMeters, 12, now); math.IsInf(gw, 1) {
		t.Error("drives on beside a push under way, not overlapping it")
	}
}

// A push under way does not stop for an aircraft giving way to it (#466;
// LKPR, live: AFR1246, taxiing, stopped for AFR657's push, and the push
// stopped for AFR1246's wing — each waited for the other for good). The
// waiting aircraft stands beside the corridor, its fuselage clear of it, a
// wing within reach, a little path left before its stop.
func TestPushUnderWayNotHeldByWaitingTraffic(t *testing.T) {
	picture := NewGroundPicture()
	ctl, frames, now := pushbackWithPicture(t, picture)
	frames(60)
	ctl.ClearPushback()
	for i := 0; i < 60*60 && ctl.State() != TaxiPushback; i++ {
		frames(1)
	}
	frames(60 * 5) // under way
	rest := pushCorridor(ctl.mover.Path(), ctl.mover.Pose().Distance, ctl.profile())
	end := rest[len(rest)-1]
	dir := localBearing(rest[len(rest)-2], end)
	prof := DefaultMotionProfile()
	oh := 17.0
	// Parallel to the corridor's end, abeam it: beyond the fuselage reach
	// (the pusher's half-span and PushClearMarginMeters), within the wing's.
	off := ctl.halfSpan() + PushClearMarginMeters + oh/2
	at := offsetHeading(end, dir+90, off)
	for i := 0; i < 60*300 && ctl.State() == TaxiPushback; i++ {
		picture.Report(99, at, dir, prof, *now)
		picture.ReportPath(99, []airport.LatLon{offsetHeading(at, dir, 2), offsetHeading(at, dir, 4)}, oh)
		frames(1)
	}
	if ctl.State() != TaxiAwaitingTaxi {
		t.Fatalf("the push never finished (%v): held by the aircraft waiting beside it", ctl.State())
	}
}

// TestGroundPictureFollowsSameWay: an aircraft behind another on the same
// taxiway, going the same way, follows it at a gap and never gives way to
// it (live, OKOPA stopped behind a B737 "giving way to the Boeing 737
// ahead").
func TestGroundPictureFollowsSameWay(t *testing.T) {
	origin := airport.LatLon{Lat: 50.1, Lon: 14.26}
	prof := DefaultMotionProfile()
	picture := NewGroundPicture()
	now := time.Now()
	mk := func(id uint32, start float64) *groundDrive {
		a := offsetHeading(origin, 270, start)
		b := offsetHeading(origin, 90, 600)
		path, err := NewGroundPath([]airport.LatLon{a, b}, prof)
		if err != nil {
			t.Fatal(err)
		}
		return &groundDrive{mover: NewGroundMover(path, prof), picture: picture, followTraffic: true, prof: prof, object: id,
			clock: func() time.Time { return now }}
	}
	lead, behind := mk(1, 100), mk(2, 200)
	gaveWay, closest := false, 1e9
	for i := 0; i < 60*300; i++ {
		now = now.Add(time.Second / 60)
		for _, d := range []*groundDrive{lead, behind} {
			p := d.mover.Pose()
			picture.Report(d.object, p.Position, p.Heading, prof, now)
		}
		for _, d := range []*groundDrive{lead, behind} {
			d.followAhead(now)
			d.mover.Step(1.0 / 60)
		}
		if behind.givingWay == lead.object {
			gaveWay = true
		}
		closest = math.Min(closest, localDist(lead.mover.Pose().Position, behind.mover.Pose().Position))
		if lead.mover.Pose().Arrived && behind.mover.Pose().Arrived {
			break
		}
	}
	if gaveWay {
		t.Error("gave way to the aircraft ahead going the same way")
	}
	if closest < prof.WheelbaseMeters+prof.TailMeters {
		t.Errorf("closed to %.0f m behind it", closest)
	}
}
