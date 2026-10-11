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
