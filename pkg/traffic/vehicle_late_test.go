package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestFuelTruckTurnsBack: told to leave while still driving in, the truck
// turns round and goes home without a jump, is clear of the aircraft at
// once and never reaches the wing (live, OKKQT's truck drove on in and the
// push waited for it).
func TestFuelTruckTurnsBack(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	for _, label := range []string{"S21", "C22"} {
		i, err := l.ParkingIndex(label)
		if err != nil {
			t.Fatal(err)
		}
		c := &tugClient{}
		inj := NewInjector(c)
		prof := ProfileFor("A320").Motion
		f := NewSimObjectFuelTruck(c, inj, "FSDT_FuelTruck_7296", 9002, prof)
		f.Layout = l
		stand := l.Parking[i]
		pose := GroundPose{Position: StandPoint(stand, 0), Heading: stand.Heading}
		if err := f.Attach(pose); err != nil {
			t.Fatal(err)
		}
		f.Handle(assignedMsg(9002, 56))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 56, 1200, 3))
		spot := FuelSpot(pose, prof, 0)
		way := f.arrive.Path().Length()
		// Half way in.
		for f.pose.Distance < way/2 {
			if err := f.Update(pose, false, 1.0/60); err != nil {
				t.Fatal(err)
			}
		}
		shown := func() GroundPose {
			if f.away != nil {
				return lane(f.pose, f.away)
			}
			return lane(f.pose, f.arrive)
		}
		last := shown()
		closest := math.Inf(1)
		for k := 0; k < 60*1200 && !f.Done(); k++ {
			if err := f.Update(pose, true, 1.0/60); err != nil {
				t.Fatal(err)
			}
			if k == 0 && !f.Clear() {
				t.Errorf("%s: turned back %.0f m from the wing, not clear", label, localDist(f.pose.Position, spot.Position))
			}
			if f.Done() {
				break
			}
			p := shown()
			if d, turn := localDist(last.Position, p.Position), math.Abs(headingDiff(last.Heading, p.Heading)); d > 0.5 || turn > 3 {
				t.Fatalf("%s: jumped %.1f m, %.1f° in a frame turning back", label, d, turn)
			}
			last = p
			closest = math.Min(closest, localDist(f.pose.Position, spot.Position))
		}
		if !f.Done() || len(c.removed) != 1 {
			t.Fatalf("%s: not home: done %v, removed %v", label, f.Done(), c.removed)
		}
		if f.Fuelling() || closest < 30 {
			t.Errorf("%s: came %.0f m from the wing after it was told to leave", label, closest)
		}
		t.Logf("%s: way in %.0f m, turned back half way, closest after %.0f m", label, way, closest)
	}
}

// slowFuel is a fakeFuel with a long way in.
type slowFuel struct {
	fakeFuel
	drive time.Duration
}

func (f *slowFuel) DriveIn(GroundPose) time.Duration { return f.drive }

// TestFuelDispatchCountsTheDrive: a truck whose way in takes 6 minutes is
// not sent with 7 minutes to go before it must be off the wing (it would
// arrive with one to refuel); with 20 to the STD it still comes.
func TestFuelDispatchCountsTheDrive(t *testing.T) {
	for _, c := range []struct {
		std  time.Duration
		sent bool
	}{{11 * time.Minute, false}, {20 * time.Minute, true}} {
		fuel := &slowFuel{fakeFuel: fakeFuel{arriveAfter: 60 * 60 * 6}, drive: 6 * time.Minute}
		ctl, _, run, _ := injectedDeparture(t, TaxiRequest{Fuel: fuel, PushbackAt: time.Now().Add(c.std), HoldForClearances: true})
		go func() {
			for range ctl.Events() {
			}
		}()
		run(TaxiAwaitingPushback, 600)
		run(TaxiPushback, 60*120)
		if got := fuel.attached > 0; got != c.sent {
			t.Errorf("STD in %v, 6 minutes' drive: sent %v, want %v", c.std, got, c.sent)
		}
	}
}

// TestTugArriveWithinAddsTheWait: the time a tug waits for an aircraft
// crossing its way is added to how long it may take to arrive (live,
// KLM999's tug was given up after the departures queuing behind its stand
// had held it).
func TestTugArriveWithinAddsTheWait(t *testing.T) {
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
	before := tug.ArriveWithin()
	path := tug.arrive.Path()
	cross := path.PointAt(150)
	hdg := localBearing(path.PointAt(145), path.PointAt(155)) + 90
	at := offsetHeading(cross, hdg+180, 60)
	var ahead []airport.LatLon
	for d := 0.0; d <= 120; d += trafficBodyStep {
		ahead = append(ahead, offsetHeading(at, hdg, d))
	}
	// Held for three minutes by the crossing aircraft.
	for k := 0; k < 60*240; k++ {
		now = now.Add(time.Second / 60)
		pic.Report(1, at, hdg, prof, now)
		pic.ReportPath(1, ahead, DefaultHalfSpanMeters)
		if err := tug.Update(pose, true, 1.0/60); err != nil {
			t.Fatal(err)
		}
	}
	if !tug.waiting {
		t.Fatal("not waiting for the crossing aircraft")
	}
	if got := tug.ArriveWithin() - before; got < 2*time.Minute {
		t.Errorf("ArriveWithin grew by %v after a long wait for the crossing aircraft", got)
	}
}
