package world

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/addons"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The stairs of a model go to its own door when its type's door is known
// (#1026), else to the estimate.
func TestModelDoors(t *testing.T) {
	setDoors(map[string]addons.ModelDoor{"FSLTL B738 RYR Ryanair": {Type: "B738", Door: addons.InteractivePoint{Long: 38.4}}},
		map[string]addons.ModelDoor{"A320": {Type: "A320", Door: addons.InteractivePoint{Long: 37.654, Lat: -6.137}}})
	defer setDoors(nil, nil)
	a, ok := doorAhead("FSLTL_FAIB_A320_DLH-Lufthansa")
	if !ok || a < 11.4 || a > 11.6 {
		t.Errorf("A320 door %.2f m ahead, %v", a, ok)
	}
	if _, ok := doorAhead("FSLTL_FAIB_B738_TVS-Smartwings"); ok {
		t.Error("a B738 door known though only one title has it")
	}
	if a, ok := doorAhead("FSLTL B738 RYR Ryanair"); !ok || a < 11.6 || a > 11.8 {
		t.Errorf("by title %.2f, %v", a, ok)
	}
	pose := traffic.GroundPose{Heading: 90}
	spot := traffic.StairsSpotAt(pose, a, 0)
	if spot.Heading != 180 {
		t.Errorf("stairs face %.0f°, want the fuselage (180°)", spot.Heading)
	}
}

// A helper goes where the jetway meets our model's door: our model's
// jetway door minus the helper's (the FSLTL A320neo's 37.654 ft, its
// helper's 27.93 ft: 2.96 m ahead).
func TestJetwayDoorOffset(t *testing.T) {
	setDoors(map[string]addons.ModelDoor{
		"FSLTL A20N WZZ Wizzair":    {Type: "A20N", First: addons.InteractivePoint{Raw: "x", Long: 37.654, Lat: -6.137}},
		"MyCrew Jetway Helper A20N": {Type: "A20N", First: addons.InteractivePoint{Raw: "x", Long: 27.93, Lat: -6.05}},
	}, nil)
	defer setDoors(nil, nil)
	a, r, ok := jetwayDoorOffset("FSLTL A20N WZZ Wizzair", "MyCrew Jetway Helper A20N")
	if !ok || a < 2.95 || a > 2.97 || r > -0.02 || r < -0.03 {
		t.Errorf("offset %.3f ahead, %.3f right, %v", a, r, ok)
	}
	if _, _, ok := jetwayDoorOffset("unknown", "MyCrew Jetway Helper A20N"); ok {
		t.Error("an unknown model has no offset")
	}
}

// At a gate our aircraft stops as the simulator parks its own: nose gear
// SimNoseGearInsetM inside the circle (the FSLTL 737's nose gear 42.7 ft
// ahead: 13.02 + 5.35 m).
func TestGateNoseOffset(t *testing.T) {
	setDoors(map[string]addons.ModelDoor{"FSLTL_B738_RYR": {Type: "B738", NoseGearFt: 42.7056, HasNoseGear: true}}, nil)
	defer setDoors(nil, nil)
	if n := gateNoseOffset("FSLTL_B738_RYR", "", SimNoseGearInsetM); n < 18.36 || n > 18.38 {
		t.Errorf("gate offset %.2f m, want 18.37", n)
	}
	if gateNoseOffset("unknown model", "", SimNoseGearInsetM) != 0 {
		t.Error("an unknown model has a gate offset")
	}
}

// A gate's placeholder parked as the simulator parks there (#572): its
// nose gear inset and its side off the parking line become our stop's.
func TestGatePlaceFromPlaceholder(t *testing.T) {
	setDoors(map[string]addons.ModelDoor{"FSLTL_A320_HELPER": {HasNoseGear: true, NoseGearFt: 37.319}}, nil)
	g := &airport.Graph{Layout: &airport.Layout{ICAO: "TEST", Parking: []airport.Parking{{Position: airport.LatLon{Lat: 50, Lon: 14}, Heading: 90, Radius: 20}}}}
	cc := &controlCenter{}
	if in, side := cc.gatePlace(g, 0); in != SimNoseGearInsetM || side != 0 {
		t.Fatalf("unmeasured: %v %v", in, side)
	}
	// Placed 3 m east (ahead) and 0.4 m south (right) of the parking point.
	east, south := 3/(111_320*math.Cos(50*math.Pi/180)), 0.4/111_320
	cc.placeholderParked("TEST", 0, "FSLTL_A320_HELPER", 50-south, 14+east, 90)
	in, side := cc.gatePlace(g, 0)
	if want := 20 - 3 - 37.319*0.3048; math.Abs(in-want) > 0.01 || math.Abs(side-0.4) > 0.01 {
		t.Errorf("gatePlace = %.2f, %.2f; want %.2f, 0.40", in, side, want)
	}
	at := traffic.StandPointSide(g.Layout.Parking[0], 20, side)
	if d := (50 - at.Lat) * 111_320; math.Abs(d-0.4) > 0.01 {
		t.Errorf("StandPointSide: %.2f m right, want 0.40", d)
	}
}
