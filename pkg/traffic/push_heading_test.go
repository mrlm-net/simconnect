package traffic

import (
	"fmt"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestPushTurnsSteadily: pushed round an arc, the aircraft turns at one
// rate per metre pushed, frame to frame, from 2 m into the arc to 2 m
// before its end (within 2% of the mean there), and never turns faster
// than that anywhere. Before, its heading was the path's chord over ±1 m:
// on an arc sampled every metre or so, the chord turned unevenly as its
// ends passed the points at different times, and the turn rate dropped by
// up to half for a frame or two at every point (the tail ticked every
// metre of the push).
func TestPushTurnsSteadily(t *testing.T) {
	for _, model := range []string{"FSLTL_FAIB_A320_SmartWings", "FSLTL_FAIB_B738_TVS-Smartwings_NC", "Asobo PassiveAircraft Cessna 172"} {
		for _, turn := range []float64{-50, 50} {
			for _, kts := range []float64{PushbackSpeedKts, 1} {
				for _, fps := range []float64{60, 10} {
					name := fmt.Sprintf("%s %+.0f at %.0f kt, %.0f fps", model, turn, kts, fps)
					pushTurnsSteadily(t, name, model, turn, kts, 1/fps)
				}
			}
		}
	}
}

func pushTurnsSteadily(t *testing.T, name, model string, turn, kts, dt float64) {
	t.Helper()
	prof := MotionProfileFor(model)
	prof.CruiseKts, prof.MinTurnKts = kts, math.Min(1, kts)
	gear := offset(lkpr, 0, -prof.RefAheadMeters)
	path, err := NewArcPath([]airport.LatLon{gear, offset(gear, 0, -40), offset(gear, turn, -40)}, prof, 25)
	if err != nil {
		t.Fatal(err)
	}
	// The arc: from the first point the path turns at to the last.
	pts := path.Points()
	from, to := -1.0, -1.0
	along := 0.0
	for i := 1; i+1 < len(pts); i++ {
		along += localDist(pts[i-1], pts[i])
		if math.Abs(headingDiff(localBearing(pts[i-1], pts[i]), localBearing(pts[i], pts[i+1]))) > 0.1 {
			if from < 0 {
				from = along
			}
			to = along
		}
	}
	if from < 0 || to-from < 10 {
		t.Fatalf("%s: no arc on the path", name)
	}
	m := NewPushbackMover(path, prof, 0)
	type step struct{ s, rate float64 }
	var steps []step
	last := m.Pose()
	for i := 0; i < int(900/dt); i++ {
		p := m.Step(dt)
		if ds := p.Distance - last.Distance; ds > 1e-4 {
			steps = append(steps, step{(p.Distance + last.Distance) / 2, math.Abs(headingDiff(last.Heading, p.Heading)) / ds})
		}
		last = p
		if p.Arrived {
			break
		}
	}
	mean, n := 0.0, 0
	for _, s := range steps {
		if s.s > from+2 && s.s < to-2 {
			mean += s.rate
			n++
		}
	}
	if n < 20 {
		t.Fatalf("%s: only %d steps in the arc", name, n)
	}
	mean /= float64(n)
	worst, fastest := 0.0, 0.0
	for _, s := range steps {
		fastest = math.Max(fastest, s.rate)
		if s.s > from+2 && s.s < to-2 {
			worst = math.Max(worst, math.Abs(s.rate-mean)/mean)
		}
	}
	if worst > 0.02 {
		t.Errorf("%s: turns %.0f%% off its steady rate (%.2f°/m) in the arc", name, worst*100, mean)
	}
	if fastest > mean*1.02 {
		t.Errorf("%s: turns at %.2f°/m somewhere, the arc %.2f°/m", name, fastest, mean)
	}
}

// TestPathHeadingAt: the path's direction turns from one segment's to the
// next within d of the point between them, steadily, past a point repeated
// (fillet leaves one where two arcs meet), and is the segment's own
// elsewhere.
func TestPathHeadingAt(t *testing.T) {
	o := lkpr
	a, b, c := offset(o, 0, 0), offset(o, 0, 20), offset(o, 20, 20)
	g := newPlainPath([]airport.LatLon{a, b, b, c}, MotionProfileFor("FSLTL_FAIB_A320_SmartWings"))
	const d = 1.0
	last := 0.0
	for s := 0.0; s <= g.Length(); s += 0.01 {
		h, ok := g.headingAt(s, d)
		if !ok {
			t.Fatalf("no heading at %.2f m", s)
		}
		want := -1.0
		switch {
		case s < 20-d:
			want = 0
		case s > 20+d:
			want = 90
		}
		if want >= 0 && math.Abs(headingDiff(h, want)) > 0.01 {
			t.Errorf("%.0f° at %.2f m, want %.0f°", h, s, want)
		}
		if s > 0 && math.Abs(headingDiff(last, h)) > 90/(2*d)*0.01*1.01 {
			t.Errorf("turned %.2f° in 1 cm at %.2f m", headingDiff(last, h), s)
		}
		last = h
	}
}
