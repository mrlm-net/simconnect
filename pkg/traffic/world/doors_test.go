package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/addons"
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
