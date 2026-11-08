package world

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/addons"
)

// The helper's door is from its own datum (live, LKPR C22, 2026-11-07:
// CSA1894's FSLTL A320 and the "B748 D-A320" helper, datum 83.5 ft, put
// 16.49 m behind it). Door to door the helper stands 25.45 m behind, so
// its jetway docked 8.96 m ahead of the A320's door; logged before as
// "the door wants 0.00 m ahead".
func TestJetwayDoorOffsetDatum(t *testing.T) {
	setDoors(map[string]addons.ModelDoor{
		"FSLTL_A320_TVS_CSA":               {Type: "A320", First: addons.InteractivePoint{Long: 37.654, Lat: -6.137}},
		"MyCrew Jetway Helper B748 D-A320": {Type: "B748", First: addons.InteractivePoint{Long: 37.654, Lat: -6.137}, DatumFt: 83.5},
	}, nil)
	defer setDoors(nil, nil)
	ahead, right, ok := jetwayDoorOffset("FSLTL_A320_TVS_CSA", "MyCrew Jetway Helper B748 D-A320")
	if !ok || math.Abs(ahead+25.45) > 0.01 || right != 0 {
		t.Fatalf("door offset %.2f m ahead, %.2f m right (%v), want -25.45, 0", ahead, right, ok)
	}
	if gap := -16.49 - ahead; math.Abs(gap-8.96) > 0.01 {
		t.Errorf("CSA1894's jetway %.2f m ahead of its door, want 8.96", gap)
	}
}
