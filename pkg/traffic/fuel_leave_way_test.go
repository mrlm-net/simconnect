package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestFuelTruckLeavesTowardsItsWayHome (#1630): with the only road 60 m
// behind the tails and the depot beyond it, the fuel truck comes in from
// the tail and leaves the wing back that way, round in a loop off the
// aircraft: never on past the nose to loop back along the whole fuselage.
func TestFuelTruckLeavesTowardsItsWayHome(t *testing.T) {
	at := func(x, y float64) airport.LatLon { return offsetHeading(offsetHeading(lkpr, 90, x), 0, y) }
	l := &airport.Layout{
		TaxiPoints: []airport.TaxiPoint{{Index: 0, Position: at(-150, -60)}, {Index: 1, Position: at(0, -60)}, {Index: 2, Position: at(150, -60)}},
		TaxiPaths: []airport.TaxiPath{
			{Index: 0, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_VEHICLE, Start: 0, End: 1},
			{Index: 1, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_VEHICLE, Start: 1, End: 2},
			{Index: 2, Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING, Start: 1, End: 0},
		},
		Parking: []airport.Parking{{Index: 0, Type: types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE, Radius: 5, Position: at(0, -90)}},
	}
	prof := ProfileFor("A320").Motion
	pose := GroundPose{Position: at(0, 0), Heading: 0}
	c := &tugClient{}
	inj := NewInjector(c)
	f := NewSimObjectFuelTruck(c, inj, "FSDT_FuelTruck_7296", 9002, prof)
	f.Layout = l
	if err := f.Attach(pose); err != nil {
		t.Fatal(err)
	}
	if !f.hasDepot {
		t.Fatal("no way in from the depot: nothing checked")
	}
	f.Handle(assignedMsg(9002, 56))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 56, 1200, 3))
	for k := 0; k < 10*900 && !f.Fuelling(); k++ {
		f.Update(pose, false, 0.1)
	}
	if !f.Fuelling() {
		t.Fatal("never at the wing: nothing checked")
	}
	if headingDiff(f.pose.Heading, pose.Heading) > 90 || headingDiff(f.pose.Heading, pose.Heading) < -90 {
		t.Fatalf("parked facing %.0f°, want the nose's way (in from the tail): nothing checked", f.pose.Heading)
	}
	frame := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
	furthest := -1e9
	for k := 0; k < 10*900 && !f.Done(); k++ {
		f.Update(pose, true, 0.1)
		if _, y := frame.to(f.pose.Position); y > furthest {
			furthest = y
		}
	}
	if !f.Done() {
		t.Fatal("never home")
	}
	// Its loop takes it beside the nose; leaving by the nose takes it
	// FuelLeaveAheadMeters past it before any loop.
	if past := fuelNoseMeters + FuelLeaveAheadMeters; furthest >= past {
		t.Errorf("drove %.0f m ahead of the nose gear (%.0f past the nose): its way home runs from the tail", furthest, furthest-fuelNoseMeters)
	}
}
