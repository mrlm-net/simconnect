package traffic

import (
	"fmt"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestAimShiftKeepsSpeed: an approach whose touchdown is shifted flies at
// the speed it reports: shown frame to frame within 2% of it on the final,
// never a step of more than 2 kt in a frame, and still touches down the
// shift from where it would have. Before, the shift was faded in by moving
// the aircraft along the runway: a short-field circuit at TFFJ (a DHC6
// joining 1132 m out, its touchdown 240 m earlier) flew 54 kt while it
// reported 73, and at the end of the fade went to 73 in one frame.
func TestAimShiftKeepsSpeed(t *testing.T) {
	thr := airport.LatLon{Lat: 17.9044, Lon: -62.8461}
	for _, c := range []struct {
		name              string
		kts, start, shift float64
	}{
		{"DHC6 short field", 73, 1132, -240},
		{"airliner spread late", 140, 9000, 60},
		{"airliner spread early", 140, 3704, -60},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := DefaultApproachProfile()
			p.StartKts, p.ApproachKts = c.kts, c.kts
			plain := NewApproachMover(thr, 99, c.start, p)
			m := NewApproachMover(thr, 99, c.start, p)
			m.SetAimShift(c.shift)
			const dt = 0.1
			last := m.Pose()
			lastV, haveV := 0.0, false
			worstOff, worstStep := 0.0, 0.0
			var at string
			for i := 0; i < 20*600 && last.Phase != ApproachDone; i++ {
				q := m.Step(dt)
				v := localDist(last.Position, q.Position) / dt / ktsToMS
				if q.Phase == ApproachFinal && last.Phase == ApproachFinal {
					if off := math.Abs(v/q.GroundSpeedKts - 1); off > worstOff {
						worstOff, at = off, fmt.Sprintf("%.0f m out: %.1f kt shown, %.1f kt reported", -q.Distance, v, q.GroundSpeedKts)
					}
				}
				if haveV {
					worstStep = math.Max(worstStep, math.Abs(v-lastV))
				}
				last, lastV, haveV = q, v, true
			}
			if last.Phase != ApproachDone {
				t.Fatal("never landed")
			}
			if worstOff > 0.02 {
				t.Errorf("flies %.0f%% off the speed it reports (%s)", worstOff*100, at)
			}
			if worstStep > 2 {
				t.Errorf("its speed stepped %.1f kt in a frame", worstStep)
			}
			b := plain.Pose()
			for i := 0; i < 20*600 && !b.OnGround; i++ {
				b = plain.Step(dt)
			}
			if d := last.Touchdown - b.Touchdown; math.Abs(d-c.shift) > 1 {
				t.Errorf("touchdown moved %.1f m, want %.0f", d, c.shift)
			}
		})
	}
}
