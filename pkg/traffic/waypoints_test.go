package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The flags a waypoint helper sets are what the sim flies by: a push is in
// reverse on the ground, a climb point is off it, above the ground, at a
// throttle.
func TestWaypointFlags(t *testing.T) {
	has := func(w types.SIMCONNECT_DATA_WAYPOINT, f types.SIMCONNECT_WAYPOINT_FLAGS) bool {
		return w.Flags&uint32(f) != 0
	}
	push := PushbackWaypoint(50, 14, 1200, 3)
	if !has(push, types.SIMCONNECT_WAYPOINT_ON_GROUND) || !has(push, types.SIMCONNECT_WAYPOINT_REVERSE) || !has(push, types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED) || push.KtsSpeed != 3 {
		t.Errorf("push: %+v", push)
	}
	taxi := TaxiWaypoint(50, 14, 1200, 15)
	if !has(taxi, types.SIMCONNECT_WAYPOINT_ON_GROUND) || has(taxi, types.SIMCONNECT_WAYPOINT_REVERSE) || !has(taxi, types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED) || taxi.KtsSpeed != 15 {
		t.Errorf("taxi: %+v", taxi)
	}
	if line := LineupWaypoint(50, 14, 1200); !has(line, types.SIMCONNECT_WAYPOINT_ON_GROUND) || has(line, types.SIMCONNECT_WAYPOINT_REVERSE) {
		t.Errorf("line-up: %+v", line)
	}
	climb := ClimbWaypoint(50, 14, 1500, 200, 90)
	if has(climb, types.SIMCONNECT_WAYPOINT_ON_GROUND) || !has(climb, types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL) || !has(climb, types.SIMCONNECT_WAYPOINT_THROTTLE_REQUESTED) ||
		climb.Altitude != 1500 || climb.KtsSpeed != 200 || climb.PercentThrottle != 90 {
		t.Errorf("climb: %+v", climb)
	}
}

// TakeoffClimb: three points out along the runway's heading, each farther,
// higher and faster than the last.
func TestTakeoffClimbAlongTheRunway(t *testing.T) {
	const lat, lon, hdg = 50.1, 14.26, 244.0
	wps := TakeoffClimb(lat, lon, hdg)
	if len(wps) != 3 {
		t.Fatalf("%d points", len(wps))
	}
	want := []float64{2778, 9260, 22224}
	for i, w := range wps {
		if d := calc.HaversineMeters(lat, lon, w.Latitude, w.Longitude); math.Abs(d-want[i]) > 50 {
			t.Errorf("point %d %.0f m out, want %.0f", i, d, want[i])
		}
		brg := calc.BearingDegrees(lat, lon, w.Latitude, w.Longitude)
		if off := math.Abs(math.Mod(brg-hdg+540, 360) - 180); off > 1 {
			t.Errorf("point %d on bearing %.1f, want %.0f", i, brg, hdg)
		}
		if i > 0 && (w.Altitude <= wps[i-1].Altitude || w.KtsSpeed <= wps[i-1].KtsSpeed) {
			t.Errorf("point %d not above and faster than the last: %+v", i, w)
		}
	}
}
