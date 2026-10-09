package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

// fakeFuel records what the departure asks of its fuel truck: it is at the
// wing after arriveAfter updates and gone leaveAfter updates after it was
// told to leave.
type fakeFuel struct {
	attached, removed       int
	updates, leaveAt        int
	arriveAfter, leaveAfter int
}

func (f *fakeFuel) Handle(engine.Message) bool { return false }
func (f *fakeFuel) Attach(GroundPose) error    { f.attached++; return nil }
func (f *fakeFuel) Remove() error              { f.removed++; return nil }
func (f *fakeFuel) Update(_ GroundPose, leave bool, _ float64) error {
	f.updates++
	if leave && f.leaveAt == 0 && f.updates > f.arriveAfter {
		f.leaveAt = f.updates
	}
	return nil
}
func (f *fakeFuel) Fuelling() bool {
	return f.attached > 0 && f.updates > f.arriveAfter && f.leaveAt == 0
}
func (f *fakeFuel) Clear() bool { return f.leaveAt > 0 && f.updates >= f.leaveAt+f.leaveAfter/2 }
func (f *fakeFuel) Done() bool  { return f.leaveAt > 0 && f.updates >= f.leaveAt+f.leaveAfter }

// TestTaxiControllerFuel: with 20 minutes to the STD the fuel truck comes
// FuelStartDelay after the aircraft is waiting, refuels and leaves before
// the tug is due; with only a few minutes it does not come at all.
func TestTaxiControllerFuel(t *testing.T) {
	fuel := &fakeFuel{arriveAfter: 60 * 60, leaveAfter: 60 * 30}
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{Fuel: fuel, PushbackAt: time.Now().Add(20 * time.Minute), HoldForClearances: true})
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiAwaitingPushback, 600) {
		t.Fatalf("state %v", ctl.State())
	}
	start := *now
	run(TaxiPushback, 60*20) // 20 s: not yet
	if fuel.attached != 0 {
		t.Fatalf("fuel truck sent %v after waiting began", now.Sub(start))
	}
	run(TaxiPushback, 60*90)
	if fuel.attached != 1 {
		t.Fatalf("fuel truck attached %d times after %v", fuel.attached, now.Sub(start))
	}
	// Refuelling, then off well before the tug (TugLeadTime before the STD).
	for i := 0; i < 60*60*20 && fuel.leaveAt == 0; i++ {
		run(TaxiPushback, 1)
	}
	left := *now
	if fuel.leaveAt == 0 {
		t.Fatal("never left")
	}
	if by := ctl.gateAt.Add(-TugLeadTime); left.After(by) {
		t.Errorf("left at %v, after the tug is due (%v)", left.Sub(start), by.Sub(start))
	}
	if d := left.Sub(start) - FuelStartDelay - time.Minute; d < FuelServiceTime/2 {
		t.Errorf("refuelled for only %v", d)
	}
	t.Logf("sent after %v, left after %v (STD after %v)", FuelStartDelay, left.Sub(start), ctl.gateAt.Sub(start))

	// Cleared to push while the truck is at the wing: it leaves first.
	fuel2 := &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 20}
	ctl2, _, run2, _ := injectedDeparture(t, TaxiRequest{Fuel: fuel2, PushbackAt: time.Now().Add(20 * time.Minute), HoldForClearances: true})
	go func() {
		for range ctl2.Events() {
		}
	}()
	run2(TaxiAwaitingPushback, 600)
	run2(TaxiPushback, 60*60*3)
	if !fuel2.Fuelling() {
		t.Fatalf("not refuelling (attached %d, updates %d)", fuel2.attached, fuel2.updates)
	}
	ctl2.ClearPushback()
	run2(TaxiPushback, 60*60)
	if fuel2.leaveAt == 0 {
		t.Fatal("still at the wing after the push was cleared")
	}
	if ctl2.State() == TaxiPushback && !fuel2.Clear() {
		t.Error("pushed with the fuel truck at the wing")
	}
	if ctl2.State() != TaxiPushback {
		t.Errorf("state %v, want pushback once the truck is off the wing", ctl2.State())
	}

	// Five minutes to the STD: no time to refuel.
	fuel3 := &fakeFuel{}
	ctl3, _, run3, _ := injectedDeparture(t, TaxiRequest{Fuel: fuel3, PushbackAt: time.Now().Add(5 * time.Minute), HoldForClearances: true})
	go func() {
		for range ctl3.Events() {
		}
	}()
	run3(TaxiAwaitingPushback, 600)
	run3(TaxiPushback, 60*60*5)
	if fuel3.attached != 0 {
		t.Errorf("fuel truck sent with no time to refuel")
	}
	if err := ctl.Cancel(); err != nil || fuel.removed != 1 {
		t.Errorf("cancel: %v, removed %d", err, fuel.removed)
	}
}

// TestSimObjectFuelTruck: at LKPR the truck appears at a depot, drives in
// along the fuselage to the right wing without crossing the aircraft,
// parks there facing the way the aircraft does, and on leave drives on and
// home, where it is removed.
func TestSimObjectFuelTruck(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	for _, label := range []string{"B9", "C22", "A3"} {
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
		spot := FuelSpot(pose, prof, 0)
		at := airport.LatLon{Lat: c.created[0].Latitude, Lon: c.created[0].Longitude}
		if d := localDist(at, spot.Position); d < 50 {
			t.Fatalf("%s: appeared %.0f m from the spot: not at a depot", label, d)
		}
		f.Handle(assignedMsg(9002, 56))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 56, 1200, 3))
		// The fuselage: from the tail to the nose along the axis.
		nose := offsetHeading(pose.Position, pose.Heading, prof.WheelbaseMeters+4)
		tail := offsetHeading(pose.Position, pose.Heading+180, prof.TailMeters)
		closest := math.Inf(1)
		track := func() {
			p := f.pose.Position
			if localDist(p, pose.Position) < 60 {
				closest = math.Min(closest, segmentDist(p, tail, nose))
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
			t.Fatalf("%s: never reached the wing", label)
		}
		spot.Heading = f.pose.Heading
		if d := localDist(f.pose.Position, spot.Position); d > 1.5 {
			t.Errorf("%s: parked %.1f m off the spot", label, d)
		}
		if h := math.Abs(headingDiff(f.pose.Heading, pose.Heading)); h > 10 && h < 170 {
			t.Errorf("%s: parked %.0f° off the aircraft heading", label, h)
		}
		drove := float64(steps) / 60
		// Staying while refuelling.
		for i := 0; i < 600; i++ {
			f.Update(pose, false, 1.0/60)
		}
		if !f.Fuelling() || f.Clear() {
			t.Fatalf("%s: left while refuelling", label)
		}
		for i := 0; i < 60*900 && !f.Done(); i++ {
			if err := f.Update(pose, true, 1.0/60); err != nil {
				t.Fatal(err)
			}
			track()
		}
		if !f.Done() || len(c.removed) != 1 {
			t.Fatalf("%s: done %v, removed %v", label, f.Done(), c.removed)
		}
		if closest < 4 {
			t.Errorf("%s: drove %.1f m from the fuselage axis", label, closest)
		}
		t.Logf("%s: drove in in %.0f s, closest to the fuselage %.1f m", label, drove, closest)
	}
}

// segmentDist is the distance from p to the segment a–b (meters).
func segmentDist(p, a, b airport.LatLon) float64 {
	ab := localDist(a, b)
	if ab < 0.01 {
		return localDist(a, p)
	}
	along := localDist(a, p) * math.Cos((localBearing(a, p)-localBearing(a, b))*math.Pi/180)
	switch {
	case along <= 0:
		return localDist(a, p)
	case along >= ab:
		return localDist(b, p)
	}
	return math.Abs(localDist(a, p) * math.Sin((localBearing(a, p)-localBearing(a, b))*math.Pi/180))
}

// TestFuelSpotLowWing: for a light aircraft the truck parks beyond the
// wingtip, not in the wing; for an airliner under the wing as before.
func TestFuelSpotLowWing(t *testing.T) {
	pose := GroundPose{Position: airport.LatLon{Lat: 50.1, Lon: 14.26}, Heading: 0}
	for _, c := range []struct {
		model  string
		beyond bool
	}{{"DA62", true}, {"PC12", true}, {"A320", false}} {
		prof := MotionProfileFor(c.model)
		spot := FuelSpot(pose, prof, 0)
		side := math.Abs(alongHeading(pose.Position, pose.Heading+90, spot.Position))
		if got := side > prof.SpanMeters/2; got != c.beyond {
			t.Errorf("%s: %.1f m out, span %.1f m: beyond the tip %v, want %v", c.model, side, prof.SpanMeters, got, c.beyond)
		}
	}
}
