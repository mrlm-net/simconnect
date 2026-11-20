package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// sweepingTurnOff is a runway heading 090 and a high-speed exit leaving it
// that sweeps round through a right angle on a wide radius (LKPR 06 onto
// its cross taxiway, about 120 m): 300 m of runway, the arc, 150 m on.
func sweepingTurnOff() []airport.LatLon {
	p := airport.LatLon{Lat: 50.1, Lon: 14.26}
	pts := []airport.LatLon{p}
	for range 30 {
		p = offsetHeading(p, 90, 10)
		pts = append(pts, p)
	}
	const r = 120.0
	for i := 1; i <= 30; i++ {
		p = offsetHeading(p, 90+3*(float64(i)-0.5), r*3*math.Pi/180)
		pts = append(pts, p)
	}
	for range 15 {
		p = offsetHeading(p, 180, 10)
		pts = append(pts, p)
	}
	return pts
}

// TestTurnOffSpeed (#1586): through a right-angle turn off the runway the
// speed is the exit speed, a wide sweeping exit included, whatever the
// taxi speed and at any frame length; the slow-down to it stays gentle.
// Without limitTurnOff a heavy took the arc at its taxi speed, the lateral
// limit not binding (LKPR 06 B77W 15.3 kt in the nightly race run).
func TestTurnOffSpeed(t *testing.T) {
	ap := ProfileFor("B77W")
	ro := ap.Rollout
	for _, taxi := range []float64{14, 15.5, 17} {
		prof := ap.Motion
		prof.CruiseKts = taxi
		for _, dt := range []float64{1.0 / 60, 1.0 / 20, 0.25, 1} {
			path, err := newGroundPath(sweepingTurnOff(), prof, firmZone{meters: 320, decel: ro.BrakeDecel, lateral: ro.ExitLateralAccel})
			if err != nil {
				t.Fatal(err)
			}
			path.limitTurnOff(90, 0, path.Length(), ro.ExitKts)
			mp := prof
			mp.Decel, mp.Jerk = ro.BrakeDecel, RolloutJerk
			m := NewGroundMoverFrom(path, mp, 90, 30)
			entered, worstA, prevV := -1.0, 0.0, m.Pose().GroundSpeedKts
			for range int(120 / dt) {
				pose := m.Step(dt)
				if pose.Distance > 320 {
					m.SetProfile(prof)
				}
				if a := (pose.GroundSpeedKts - prevV) * ktsToMS / dt; pose.Distance > 320 && a < worstA {
					worstA = a
				}
				prevV = pose.GroundSpeedKts
				if entered < 0 && math.Abs(headingDiff(90, pose.Heading)) >= 60 {
					entered = pose.GroundSpeedKts
				}
				if pose.Distance >= path.Length()-1 {
					break
				}
			}
			t.Logf("taxi %.1f kt, frame %.3f s: right angle entered at %.1f kt, firmest braking off the runway %.2f m/s²", taxi, dt, entered, worstA)
			if entered < 0 || entered > ro.ExitKts+1 {
				t.Errorf("taxi %.1f kt, frame %.3f s: entered the right angle at %.1f kt, want ≤ %.0f", taxi, dt, entered, ro.ExitKts+1)
			}
			if worstA < -1.5*prof.Decel {
				t.Errorf("taxi %.1f kt, frame %.3f s: braked at %.2f m/s² off the runway, want gentler than %.2f", taxi, dt, worstA, -1.5*prof.Decel)
			}
		}
	}
}
