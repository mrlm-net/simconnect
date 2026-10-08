package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Passenger buses (#887) board with the stairs: not before the boarding
// window (BusBoardingTime before they must leave), only after the stairs,
// each taken from the fleet under its own owner, gone before the stairs
// leave and before the push.
func TestTaxiControllerBuses(t *testing.T) {
	fleet := NewVehicleFleet(map[VehicleKind]int{VehicleStairs: 1, VehicleBus: 2})
	stairs := &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}
	b1, b2 := &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}, &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{Stairs: stairs, Buses: []FuelService{b1, b2}, PushbackAt: time.Now().Add(40 * time.Minute), HoldForClearances: true}, TaxiWithServices(fleet))
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiAwaitingPushback, 600) {
		t.Fatalf("state %v", ctl.State())
	}
	leaveBy := ctl.gateAt.Add(-TugLeadTime - StandServiceClearMargin - BusLeaveBeforeStairs)
	opens := leaveBy.Add(-BusBoardingTime)
	for now.Before(opens.Add(-time.Minute)) {
		run(TaxiPushback, 60)
	}
	if stairs.attached != 1 {
		t.Fatalf("stairs attached %d", stairs.attached)
	}
	if b1.attached != 0 || b2.attached != 0 {
		t.Fatalf("buses sent %v before boarding", opens.Sub(*now))
	}
	run(TaxiPushback, 60*120)
	if b1.attached != 1 || b2.attached != 1 {
		t.Fatalf("buses attached %d, %d at boarding", b1.attached, b2.attached)
	}
	if out, _ := fleet.Out(VehicleBus); out != 2 {
		t.Errorf("%d buses out of the fleet, want 2", out)
	}
	for i := 0; i < 60*60*20 && b1.leaveAt == 0; i++ {
		run(TaxiPushback, 1)
	}
	if b1.leaveAt == 0 || b2.leaveAt == 0 {
		t.Fatal("buses never left")
	}
	if stairs.leaveAt != 0 {
		t.Error("stairs left before the buses")
	}
	ctl.ClearPushback()
	if !run(TaxiPushback, 60*240) {
		t.Fatalf("no push: %v", ctl.State())
	}
	run(TaxiLinedUp, 60*30) // they drive home meanwhile
	if out, _ := fleet.Out(VehicleBus); out != 0 {
		t.Errorf("%d buses still out once home", out)
	}
}

// Without stairs no bus comes.
func TestTaxiControllerBusesNeedStairs(t *testing.T) {
	b := &fakeFuel{arriveAfter: 60, leaveAfter: 60}
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{Buses: []FuelService{b}, PushbackAt: time.Now().Add(20 * time.Minute), HoldForClearances: true})
	go func() {
		for range ctl.Events() {
		}
	}()
	run(TaxiPushback, 60*60*10)
	if b.attached != 0 {
		t.Error("a bus came without stairs")
	}
}

// BusSpot: left of the axis beyond the stairs, abeam the front door, the
// second one further forward, both facing the tail; one bus for a small airliner, two from an A320.
func TestBusSpot(t *testing.T) {
	pose := GroundPose{Heading: 90} // facing east: left is north
	prof := DefaultMotionProfile()
	s0, s1 := BusSpot(0)(pose, prof), BusSpot(1)(pose, prof)
	stairs := StairsSpot(pose, prof, 0, 0)
	if s0.Position.Lat <= stairs.Position.Lat {
		t.Error("bus not outside the stairs (north, heading east)")
	}
	if d := localDist(s0.Position, s1.Position); d < BusGapMeters-0.5 || d > BusGapMeters+0.5 {
		t.Errorf("buses %.1f m apart, want %.0f", d, BusGapMeters)
	}
	if s1.Position.Lon <= s0.Position.Lon || s0.Heading != 270 {
		t.Error("second bus not ahead of the first, or not facing the tail")
	}
	small, large := prof, prof
	small.SpanMeters, large.SpanMeters = 27, 35.8
	if BusesFor(small) != 1 || BusesFor(large) != BusesLarge {
		t.Errorf("buses: %d small, %d large", BusesFor(small), BusesFor(large))
	}
	if busOwner("CSA1", 0) != "CSA1" || busOwner("CSA1", 1) != "CSA1/2" {
		t.Error("bus owners")
	}
}

// A bus at LKPR's remote stands drives in along the vehicle roads to its
// spot beside the stairs, waits and drives home, never closer to the
// fuselage axis than the stairs' far end.
func TestSimObjectBus(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	prof := ProfileFor("A320").Motion
	var stands []int
	for i, p := range l.Parking {
		if !p.IsGate() && p.Size() != airport.StandNone && p.Radius*2 >= prof.SpanMeters {
			stands = append(stands, i)
		}
	}
	if len(stands) == 0 {
		t.Fatal("no remote stand at LKPR")
	}
	for _, i := range stands {
		stand := l.Parking[i]
		label := stand.Label()
		for n := range BusesLarge {
			c := &tugClient{}
			inj := NewInjector(c)
			f := NewSimObjectBus(c, inj, "FSDT_Cobus_3000", 9002, n, prof)
			f.Layout = l
			pose := GroundPose{Position: StandPoint(stand, 0), Heading: stand.Heading}
			if err := f.Attach(pose); err != nil {
				t.Fatal(err)
			}
			spot := BusSpot(n)(pose, prof)
			f.Handle(assignedMsg(9002, 56))
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, 56, 1200, 3))
			nose := offsetHeading(pose.Position, pose.Heading, prof.WheelbaseMeters+4)
			tail := offsetHeading(pose.Position, pose.Heading+180, prof.TailMeters)
			closest := math.Inf(1)
			// Under the wing: within the span, from a few meters ahead of the
			// main gear to the trailing edge.
			main := offsetHeading(pose.Position, pose.Heading+180, prof.RefAheadMeters)
			underWing := false
			track := func() {
				p := f.pose.Position
				if localDist(p, pose.Position) < 60 {
					closest = math.Min(closest, segmentDist(p, tail, nose))
				}
				rel := (localBearing(main, p) - pose.Heading) * math.Pi / 180
				along, side := localDist(main, p)*math.Cos(rel), localDist(main, p)*math.Sin(rel)
				if math.Abs(side) < prof.SpanMeters/2 && along > -6 && along < 3 {
					underWing = true
				}
			}
			steps := 0
			for ; steps < 60*900 && !f.Fuelling(); steps++ {
				if err := f.Update(pose, false, 1.0/60); err != nil {
					t.Fatal(err)
				}
				track()
			}
			if !f.Fuelling() {
				t.Fatalf("%s bus %d: never reached its spot", label, n+1)
			}
			if d := localDist(f.pose.Position, spot.Position); d > 1.5 {
				t.Errorf("%s bus %d: parked %.1f m off the spot", label, n+1, d)
			}
			for i := 0; i < 60*900 && !f.Done(); i++ {
				if err := f.Update(pose, true, 1.0/60); err != nil {
					t.Fatal(err)
				}
				track()
			}
			if !f.Done() {
				t.Fatalf("%s bus %d: never home", label, n+1)
			}
			if underWing {
				t.Errorf("%s bus %d: drove under the wing", label, n+1)
			}
			if closest < StairsSideMeters+2 {
				t.Errorf("%s bus %d: drove %.1f m from the fuselage axis", label, n+1, closest)
			}
			t.Logf("%s bus %d: in after %.0f s, closest to the fuselage %.1f m", label, n+1, float64(steps)/60, closest)
		}
	}
}

// On a turnaround (#887) the deboarding bus comes after the stairs, stays
// BusDeboardTime at the aircraft and drives home; the boarding bus on its
// spot (and request ID) comes only after that, in the boarding window.
func TestTaxiControllerDeboard(t *testing.T) {
	fleet := NewVehicleFleet(map[VehicleKind]int{VehicleStairs: 1, VehicleBus: 1})
	stairs := &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}
	off, on := &fakeFuel{arriveAfter: 60 * 30, leaveAfter: 60 * 60}, &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{Stairs: stairs, Deboard: []FuelService{off}, Buses: []FuelService{on}, PushbackAt: time.Now().Add(40 * time.Minute), HoldForClearances: true}, TaxiWithServices(fleet))
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiAwaitingPushback, 600) {
		t.Fatalf("state %v", ctl.State())
	}
	start := *now
	run(TaxiPushback, 60*30)
	if off.attached != 0 {
		t.Fatalf("deboarding bus sent %v after waiting began, before the stairs were there a minute", now.Sub(start))
	}
	run(TaxiPushback, 60*60)
	if off.attached != 1 {
		t.Fatalf("deboarding bus attached %d", off.attached)
	}
	for i := 0; i < 60*60*10 && off.leaveAt == 0; i++ {
		run(TaxiPushback, 1)
	}
	if off.leaveAt == 0 {
		t.Fatal("deboarding bus never left")
	}
	if d := now.Sub(start); d < BusDeboardTime || d > BusDeboardTime+4*time.Minute {
		t.Errorf("deboarding bus left %v after waiting began", d)
	}
	if on.attached != 0 {
		t.Error("boarding bus sent while the deboarding bus was there")
	}
	leaveBy := ctl.gateAt.Add(-TugLeadTime - StandServiceClearMargin - BusLeaveBeforeStairs)
	for now.Before(leaveBy.Add(-BusBoardingTime + 2*time.Minute)) {
		run(TaxiPushback, 60)
	}
	if !off.Done() || on.attached != 1 {
		t.Fatalf("boarding: deboarding bus done %v, boarding bus attached %d", off.Done(), on.attached)
	}
	ctl.ClearPushback()
	if !run(TaxiPushback, 60*240) {
		t.Fatalf("no push: %v", ctl.State())
	}
}

// A bus takes the creation reply only for itself: not before it was sent
// for, and not once it is home (the next bus on its request ID).
func TestSimObjectBusHandle(t *testing.T) {
	c := &tugClient{}
	inj := NewInjector(c)
	a, b := NewSimObjectBus(c, inj, "FSDT_Cobus_3000", 9002, 0, ProfileFor("A320").Motion), NewSimObjectBus(c, inj, "FSDT_Cobus_3000", 9002, 0, ProfileFor("A320").Motion)
	if b.Handle(assignedMsg(9002, 56)) {
		t.Fatal("a bus not sent for took a reply")
	}
	pose := GroundPose{Heading: 90}
	a.Attach(pose)
	if !a.Handle(assignedMsg(9002, 56)) || a.ObjectID() != 56 {
		t.Fatal("the bus sent for did not take its reply")
	}
	a.Remove()
	b.Attach(pose)
	if a.Handle(assignedMsg(9002, 57)) {
		t.Error("a bus home took the next one's reply")
	}
	if !b.Handle(assignedMsg(9002, 57)) || b.ObjectID() != 57 {
		t.Error("the next bus did not take its reply")
	}
	if len(c.removed) != 1 {
		t.Errorf("removed %v, want only the first", c.removed)
	}
}
