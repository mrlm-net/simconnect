//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// A push ends on a taxiway facing the way out, whatever the airport: at
// every test airport (every third stand, both ends of the longest runway)
// the push planned to a pose is one a tug can make — at most
// pushPoseMaxMeters, no turn tighter than PushbackMinArcMeters, no loop —
// and the taxi-out starts along the nose (no turn from a standstill).
// Nearly every stand gets a pose; the rest keep the older plans.
func TestPushToPoseEverywhere(t *testing.T) {
	for _, icao := range testAirports {
		g := airportGraph(t, icao)
		l := g.Layout
		var rw *airport.Runway
		for i := range l.Runways {
			if rw == nil || l.Runways[i].Length > rw.Length {
				rw = &l.Runways[i]
			}
		}
		planned, posed, towed := 0, 0, 0
		for i, st := range l.Parking {
			if i%3 != 0 {
				continue
			}
			switch st.Type {
			case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL,
				types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_MEDIUM, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_LARGE,
				types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_EXTRA:
				continue
			}
			for _, rwy := range []string{rw.Primary.Name, rw.Secondary.Name} {
				ec := &eventClient{}
				ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
				if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: rwy, Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1}); err != nil {
					continue
				}
				if !slices.Contains(l.SuitableStands(ctl.profile().SpanMeters/2), i) || ctl.facesOut() {
					continue
				}
				planned++
				p := ctl.pushPose
				if p == nil {
					continue
				}
				posed++
				name := icao + " " + st.Label() + " for " + rwy
				path, err := ctl.pushPath()
				if err != nil {
					t.Errorf("%s: %v", name, err)
					continue
				}
				pts := ctl.pushPts
				if l := path.Length(); l > pushPoseMaxMeters+1 {
					t.Errorf("%s: push %.0f m", name, l)
				}
				if r := tightestTurn(pts); r < PushbackMinArcMeters-3 {
					t.Errorf("%s: push turns on %.1f m", name, r)
				}
				n := len(pts)
				net := math.Abs(headingDiff(localBearing(pts[0], pts[2]), localBearing(pts[n-3], pts[n-1])))
				if turn := totalTurn(pts); turn > net+pushMaxSwerveDeg+1 {
					t.Errorf("%s: push turns %.0f° to turn the aircraft %.0f°", name, turn, net)
				}
				// The push (or the tow after it) ends with the aircraft along
				// the pose, the nose on it.
				end := localBearing(pts[n-3], pts[n-1]) + 180
				if tow := ctl.towPts; tow != nil {
					towed++
					m := len(tow)
					end = localBearing(tow[m-3], tow[m-1])
					if l := pathLen(tow); l > towMaxMeters+1 {
						t.Errorf("%s: tow %.0f m", name, l)
					}
					if r := tightestTurn(tow); r < PushbackMinArcMeters-3 {
						t.Errorf("%s: tow turns on %.1f m", name, r)
					}
					if localDist(tow[m-1], p.nose) > 0.5 {
						t.Errorf("%s: tow ends %.1f m off the pose", name, localDist(tow[m-1], p.nose))
					}
				}
				if d := math.Abs(headingDiff(end, p.heading)); d > 5 {
					t.Errorf("%s: push ends %.0f° off the pose", name, d)
				}
				if !p.within(pushTaxiStartMeters, pushTaxiStartDeg) {
					t.Errorf("%s: taxi-out turns off the nose at the start", name)
				}
				if ctl.route.Nodes[0] != p.from || ctl.route.Nodes[1] != p.to {
					t.Errorf("%s: route starts %v, not on the pose's edge %d→%d", name, ctl.route.Nodes[:2], p.from, p.to)
				}
			}
		}
		t.Logf("%s: %d of %d pushes to a pose, %d with a tow", icao, posed, planned, towed)
		if posed < planned*85/100 {
			t.Errorf("%s: only %d of %d pushes to a pose", icao, posed, planned)
		}
	}
}

// A stand faces out only if its junction lies ahead of the nose gear: the
// junction of EDDF B10 or KJFK A15 is ahead of the stand's reference point
// but under the aircraft, and they taxied off with a turn of 115°–163° from
// a standstill; they are pushed. LKPR's GA stands N52–N54 still face out.
// A push to a pose, whose route starts on a taxiway ahead of the stand,
// does not make a stand face out.
func TestFacesOutFromTheNose(t *testing.T) {
	for _, c := range []struct {
		icao, stand, rwy string
		out              bool
	}{
		{"EDDF", "B10", "25C", false}, {"KJFK", "A15", "13R", false}, {"KJFK", "B1", "13R", false}, {"KJFK", "E10", "13R", false},
		{"LKPR", "N52", "24", true}, {"LKPR", "N53", "24", true}, {"LKPR", "N54", "24", true},
	} {
		g := airportGraph(t, c.icao)
		i, err := g.Layout.ParkingIndex(c.stand)
		if err != nil {
			t.Fatal(err)
		}
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: c.rwy, Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1}); err != nil {
			t.Fatal(err)
		}
		if ctl.facesOut() != c.out {
			t.Errorf("%s %s: faces out %v, want %v", c.icao, c.stand, ctl.facesOut(), c.out)
		}
		if ctl.pushPose != nil && ctl.facesOut() {
			t.Errorf("%s %s: pushed to a pose, yet faces out", c.icao, c.stand)
		}
	}
}

// Push and pull: where no push alone ends cleanly (KJFK D70 for 04L), the
// tug pushes the aircraft back and then tows it forward onto the taxiway;
// the taxi starts from the pose, the nose on it, along its heading.
func TestPushThenTow(t *testing.T) {
	g := airportGraph(t, "KJFK")
	i, err := g.Layout.ParkingIndex("D70")
	if err != nil {
		t.Fatal(err)
	}
	st := g.Layout.Parking[i]
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: "04L", Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1}); err != nil {
		t.Fatal(err)
	}
	p := ctl.pushPose
	if p == nil || ctl.towPts == nil {
		t.Fatalf("no push and tow planned (pose %v)", p != nil)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	towed := false
	for f := 0; f < 60*900 && ctl.State() != TaxiTaxiing && !ctl.State().Terminal(); f++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(positionMsg(mon, 77, st.Position, 0, 0, true))
		towed = towed || ctl.towing
	}
	if !towed || ctl.State() != TaxiTaxiing {
		t.Fatalf("towed %v, state %v", towed, ctl.State())
	}
	pose := ctl.mover.Pose()
	nose := NoseGear(pose.Position, pose.Heading, ctl.profile())
	if d := localDist(nose, p.nose); d > 2 {
		t.Errorf("taxi starts %.1f m from the pose", d)
	}
	if d := math.Abs(headingDiff(pose.Heading, p.heading)); d > 5 {
		t.Errorf("taxi starts %.0f° off the pose's heading", d)
	}
}

// The stands around as they are: the push swings through an empty
// neighbouring stand, never through a taken one. EHAM U26 for 09: with the
// small stands in front of it empty, straight back onto C facing south;
// with them taken, a shorter push clear of them.
func TestPushThroughEmptyStands(t *testing.T) {
	g := airportGraph(t, "EHAM")
	i, err := g.Layout.ParkingIndex("U26")
	if err != nil {
		t.Fatal(err)
	}
	plan := func(occupied func(int) bool) *TaxiController {
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: "09", Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1, StandOccupied: occupied}); err != nil {
			t.Fatal(err)
		}
		if ctl.pushPose == nil {
			t.Fatal("no pose")
		}
		return ctl
	}
	free := plan(func(int) bool { return false })
	if p := free.pushPose; p.lane != "C" || math.Abs(headingDiff(p.heading, 183)) > 15 || free.towPts != nil {
		t.Errorf("neighbours empty: pose on %q facing %.0f° (tow %v), want C facing about 183°", p.lane, p.heading, free.towPts != nil)
	}
	taken := plan(func(int) bool { return true })
	stand := g.Layout.Parking[i]
	gear := offsetHeading(StandPoint(stand, taken.req.NoseOffset), stand.Heading, -taken.profile().RefAheadMeters)
	pv := newFlatPave(pavementAround(g, gear, pushPoseReachMeters+50), gear)
	pv.withStands(g, i, nil)
	base := math.Max(0, pv.intrusion([]airport.LatLon{offsetHeading(gear, stand.Heading, 2), offsetHeading(gear, stand.Heading, 1), gear}, taken.profile()))
	if in := pv.intrusion(taken.pushPts, taken.profile()); in > base+pushClearanceSlackMeters {
		t.Errorf("neighbours taken: the push reaches %.1f m into one", in)
	}
}

// A pushback cleared to face a compass direction ends facing it wherever a
// push can, and keeps its plan where none can: at LKPR, asked for the
// opposite of the planned facing.
func TestPushbackFacing(t *testing.T) {
	g := airportGraph(t, "LKPR")
	turned, kept := 0, 0
	for i, st := range g.Layout.Parking {
		if i%2 != 0 {
			continue
		}
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: "24", Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1}); err != nil {
			continue
		}
		go func() {
			for range ctl.Events() {
			}
		}()
		ctl.state = TaxiAwaitingPushback
		was := ctl.PushFacing()
		if was == "" {
			continue
		}
		h, _ := CompassHeading(was)
		want := CompassName(h + 180)
		if err := ctl.ClearPushbackFacing(want); err != nil {
			t.Fatalf("%s: %v", st.Label(), err)
		}
		if !ctl.pushCleared {
			t.Errorf("%s: not cleared", st.Label())
		}
		switch ctl.PushFacing() {
		case want:
			turned++
		case was:
			kept++
		}
		if _, err := ctl.pushPath(); err != nil {
			t.Errorf("%s facing %s: %v", st.Label(), want, err)
		}
	}
	t.Logf("LKPR: %d pushes turned round, %d kept", turned, kept)
	if turned == 0 {
		t.Error("no push turned to the facing asked for")
	}
	ctl := NewTaxiController(NewFleet(&eventClient{}))
	ctl.state = TaxiPushback
	if err := ctl.ClearPushbackFacing("east"); !errors.Is(err, ErrTooLate) {
		t.Errorf("pushing: %v, want ErrTooLate", err)
	}
	if err := ctl.ClearPushbackFacing("up"); err == nil {
		t.Error("facing up accepted")
	}
	if CompassName(-10) != "north" || CompassName(100) != "east" || CompassName(225+1) != "west" {
		t.Error("CompassName")
	}
}
