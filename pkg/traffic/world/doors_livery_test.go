package world

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/addons"
)

// An MSFS 2024 livery spawns as "<preset title> :: <livery name>" and has
// its preset's door: the jetway of a Fenix A320 (L1 29.0 ft ahead) met the
// A320 helper's door (37.65 ft) 2.6 m ahead, at the nose (live, LKPR 23:
// MyCrew OK-MYV), its offset unknown.
func TestJetwayDoorOffsetLiveryTitle(t *testing.T) {
	setDoors(map[string]addons.ModelDoor{
		"FenixA320 CFM SL":           {Type: "A320", First: addons.InteractivePoint{Long: 29, Lat: -6.05}, Door: addons.InteractivePoint{Long: 29, Lat: -6.05}},
		"MyCrew Jetway Helper A320": {Type: "A320", First: addons.InteractivePoint{Long: 37.654, Lat: -6.137}, Door: addons.InteractivePoint{Long: 37.654, Lat: -6.137}},
	}, nil)
	defer setDoors(nil, nil)
	ahead, _, ok := jetwayDoorOffset("FenixA320 CFM SL :: MyCrew Heritage OK-MYV", "MyCrew Jetway Helper A320")
	if !ok {
		t.Fatal("no door offset for the livery title")
	}
	if want := (29 - 37.654) * 0.3048; math.Abs(ahead-want) > 0.01 {
		t.Errorf("ahead %.2f m, want %.2f m", ahead, want)
	}
}
