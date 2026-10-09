package world

import (
	"math"
	"testing"
)

// TestTowerLookTurnWraps (#69): a turn of more than a whole circle back
// still leaves the yaw in [0, 360).
func TestTowerLookTurnWraps(t *testing.T) {
	k := &towerLook{yaw: 10}
	k.turn(-730, 0, 0, nil)
	if k.yaw < 0 || k.yaw >= 360 || math.Abs(k.yaw-0) > 1e-9 {
		t.Errorf("yaw %v, want 0", k.yaw)
	}
	k.turn(725, 0, 0, nil)
	if math.Abs(k.yaw-5) > 1e-9 {
		t.Errorf("yaw %v, want 5", k.yaw)
	}
}
