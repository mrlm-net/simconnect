package traffic

import (
	"fmt"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// tugTurnStalls counts the stalls in a tug's turn: steps is the heading it
// was placed at, frame to frame (signed degrees). A stall is a gap of 1 to
// 10 frames turning less than a fifth of the frames either side of it,
// which turn the same way at least 0.01° each: the tug stops turning and
// turns on. A turn reversing (the frames either side turning opposite
// ways), a turn starting from straight (a frame either side under 0.01°)
// or a straight stretch (a gap longer than 10 frames) is no stall.
func tugTurnStalls(steps []float64) int {
	const minStep, small, maxGap = 0.01, 0.2, 10
	n := 0
	for a := 0; a < len(steps); a++ {
		sa := math.Abs(steps[a])
		if sa < minStep {
			continue
		}
		b := a + 1
		for b < len(steps) && b <= a+maxGap && math.Abs(steps[b]) < small*sa {
			b++
		}
		if b == a+1 || b >= len(steps) || b > a+maxGap {
			continue
		}
		sb := math.Abs(steps[b])
		if sb < minStep || steps[a]*steps[b] < 0 {
			continue
		}
		stalled := true
		for _, s := range steps[a+1 : b] {
			if math.Abs(s) >= small*math.Min(sa, sb) {
				stalled = false
			}
		}
		if stalled {
			n++
		}
	}
	return n
}

func TestTugTurnStallsCriterion(t *testing.T) {
	for _, c := range []struct {
		name  string
		steps []float64
		want  int
	}{
		{"steady", []float64{0.1, 0.1, 0.1, 0.1}, 0},
		{"every other frame", []float64{0.2, 0, 0.2, 0, 0.2}, 2},
		{"every third frame", []float64{0.3, 0, 0, 0.3, 0, 0, 0.3}, 2},
		{"reversing", []float64{0.1, 0.05, 0, -0.05, -0.1}, 0},
		{"reversing through a still frame", []float64{0.1, 0, -0.1}, 0},
		{"starting from straight", []float64{0, 0, 0, 0, 0.1, 0.1}, 0},
		{"ending into straight", []float64{0.1, 0.1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.1}, 0},
		{"too little to tell", []float64{0.005, 0, 0.005}, 0},
		{"slowing", []float64{0.1, 0.08, 0.06, 0.04, 0.02, 0.01}, 0},
	} {
		if got := tugTurnStalls(c.steps); got != c.want {
			t.Errorf("%s: %d stalls, want %d", c.name, got, c.want)
		}
	}
}

// TestTugTurnsEveryFrame: pushed through a turn, the tug turns on the nose
// every frame, not in steps. Before, the bar turned only once the nose
// wheel had moved 5 cm since its last turn, and then by one frame's ease:
// at 60 frames a second the tug (its heading, its place on the nose and its
// bar) stood still every other frame at 3 kt and two frames in three at
// 1 kt, and swung on in the next.
func TestTugTurnsEveryFrame(t *testing.T) {
	for _, model := range []string{"FSLTL_E190_AUA", "FSLTL_FAIB_A320_SmartWings", "FSLTL_FAIB_B738_TVS-Smartwings_NC", "FSLTL_A332F_MAS", "Asobo PassiveAircraft Cessna 172"} {
		for _, turn := range []float64{-50, 50} {
			for _, kts := range []float64{PushbackSpeedKts, 1} {
				for _, fps := range []float64{60, 30} {
					name := fmt.Sprintf("%s %+.0f at %.0f kt, %.0f fps", model, turn, kts, fps)
					steps := pushedTugSteps(t, model, turn, kts, 1/fps)
					turning := 0
					for _, s := range steps {
						if math.Abs(s) >= 0.01 {
							turning++
						}
					}
					if turning < 30 {
						t.Fatalf("%s: turning in only %d frames", name, turning)
					}
					if n := tugTurnStalls(steps); n > 0 {
						t.Errorf("%s: the tug stops turning and turns on %d times in %d turning frames", name, n, turning)
					}
				}
			}
		}
	}
}

// pushedTugSteps pushes model back 40 m and through a turn at kts, frames
// dt apart, and returns the tug's placed heading frame to frame.
func pushedTugSteps(t *testing.T, model string, turn, kts, dt float64) []float64 {
	t.Helper()
	prof := MotionProfileFor(model)
	prof.CruiseKts, prof.MinTurnKts = kts, math.Min(1, kts)
	gear := offset(lkpr, 0, -prof.RefAheadMeters)
	path, err := NewArcPath([]airport.LatLon{gear, offset(gear, 0, -40), offset(gear, turn, -40)}, prof, 25)
	if err != nil {
		t.Fatal(err)
	}
	m := NewPushbackMover(path, prof, 0)
	span := ProfileFor(model).WingspanM
	if span == 0 {
		span = 11
	}
	title := TugTitleForModel("", model, span)
	c := &stickClient{}
	inj := NewInjector(c)
	tug := NewSimObjectTug(c, inj, title, 9001, prof)
	if a, ok := TugAheadFor(title); ok {
		tug.AheadMeters = a
	}
	if err := tug.Attach(m.Pose()); err != nil {
		t.Fatal(err)
	}
	tug.Handle(assignedMsg(9001, 55))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
	var steps []float64
	last, have := 0.0, false
	for i := 0; i < int(900/dt); i++ {
		pose := m.Step(dt)
		if err := tug.Update(pose, true, dt); err != nil {
			t.Fatal(err)
		}
		h := c.last[55].Heading
		if have {
			steps = append(steps, headingDiff(last, h))
		}
		last, have = h, true
		if pose.Arrived {
			break
		}
	}
	return steps
}
