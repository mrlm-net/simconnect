package traffic

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// steerClient records the yaw rates written for the nose wheel.
type steerClient struct {
	eventClient
	yaw []float64 // per placement: the yaw written, NaN none
}

func (c *steerClient) SetDataOnSimObject(def, obj uint32, f types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	if def == DefaultInjectDefinitionBase+injDefSteer {
		c.yaw[len(c.yaw)-1] = *(*float64)(p)
		return nil
	}
	return c.eventClient.SetDataOnSimObject(def, obj, f, n, size, p)
}

// steerRun places an A320 at kts along 50 m north, a quarter turn right of
// radius, then 60 m east (pushed tail first when reverse; standing still for
// a second halfway round when stop) and is the yaw written at each
// placement, and the index of the first standing one (-1 none).
func steerRun(t *testing.T, kts, radius float64, reverse, stop bool) ([]float64, int) {
	t.Helper()
	c := &steerClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	inj.Takeover(42)
	inj.SetModel(42, "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery")
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 42, 1200, 12.25))
	v := kts * ktsToMS
	arc := radius * math.Pi / 2
	place := func(s, k float64) {
		var pose GroundPose
		switch {
		case s < 50:
			pose = GroundPose{Position: offset(lkpr, 0, s), Heading: 0}
		case s < 50+arc:
			a := (s - 50) / radius
			pose = GroundPose{Position: offset(lkpr, radius*(1-math.Cos(a)), 50+radius*math.Sin(a)), Heading: a * 180 / math.Pi}
		default:
			pose = GroundPose{Position: offset(lkpr, radius+(s-50-arc), 50+radius), Heading: 90}
		}
		if reverse {
			pose.Heading = normDeg(pose.Heading + 180) // tail first: pushed
		}
		pose.GroundSpeedKts = k
		c.yaw = append(c.yaw, math.NaN())
		if err := inj.Place(42, pose); err != nil {
			t.Fatal(err)
		}
		w = w.Add(time.Second / 60)
	}
	stoppedAt := -1
	for s := 0.0; s < 50+arc+60; s += v / 60 {
		if stop && stoppedAt < 0 && s >= 50+arc/2 {
			stoppedAt = len(c.yaw)
			for range 60 {
				place(s, 0)
			}
		}
		place(s, kts)
	}
	return c.yaw, stoppedAt
}

// The nose wheel steers into a turn (#1446): through a 40 m right turn at
// 10 kt the yaw rate written is the turn's, right positive; straight again
// it goes back to 0 and is no longer written; pushed back round the same
// turn it steers the other way. Not written, the simulator showed the
// nose wheel straight through every turn (live LKPR 2026-11-04).
func TestNoseWheelSteersIntoTheTurn(t *testing.T) {
	const radius, kts = 40.0, 10.0
	fwd, _ := steerRun(t, kts, radius, false, false)
	n := len(fwd)
	mid := fwd[n*50/100] // halfway round the turn
	want := kts * ktsToMS / radius * 0.3048
	if math.Abs(mid-want) > 0.15*want {
		t.Errorf("mid-turn: yaw %.4f written, want %.4f (right, the turn's)", mid, want)
	}
	if !math.IsNaN(fwd[5]) {
		t.Errorf("straight before the turn: yaw %.4f written, want none", fwd[5])
	}
	zero := false
	for _, y := range fwd[n*66/100:] {
		zero = zero || y == 0
	}
	if !zero || !math.IsNaN(fwd[n-1]) {
		t.Errorf("straight after the turn: last yaws %v, want one 0 then none", fwd[n-10:])
	}
	back, _ := steerRun(t, kts, radius, true, false)
	if b := back[n*50/100]; !(b < -0.5*want) {
		t.Errorf("pushed round the turn: yaw %.4f, want the other way (about %.4f)", b, -want)
	}
}

// The yaw rate written is the one the aircraft turns at, whatever its
// speed, none standing, never more than its main wheels roll with: written
// as a nose wheel angle's rate at 10 kt and held standing, the wheels on
// the inside of the turn stood still and the nose came up (live LKPR,
// CSA1822 BCS3).
func TestNoseWheelYawIsTheTurns(t *testing.T) {
	const radius = 40.0
	// At 25 kt: the turn's own rate, not 10 kt's.
	fast, _ := steerRun(t, 25, radius, false, false)
	want := 25 * ktsToMS / radius * 0.3048
	if mid := fast[len(fast)*50/100]; math.Abs(mid-want) > 0.15*want {
		t.Errorf("25 kt mid-turn: yaw %.4f written, want %.4f", mid, want)
	}
	// Standing halfway round: 0 at once, then nothing written.
	stop, at := steerRun(t, 10, radius, false, true)
	written := 0
	for _, y := range stop[at+1 : at+60] { // the first still arrives there
		if math.IsNaN(y) {
			continue
		}
		if y != 0 {
			t.Fatalf("standing mid-turn: yaw %.4f written, want none", y)
		}
		written++
	}
	if written > 1 {
		t.Errorf("standing mid-turn: 0 written %d times, want once", written)
	}
	// A tight turn at walking pace: kept under its speed over the half track.
	slow, _ := steerRun(t, 1, 3, false, false)
	limit := 1 * ktsToMS / steerHalfTrackM * 0.3048
	for i, y := range slow {
		if !math.IsNaN(y) && math.Abs(y) > limit*1.01 {
			t.Fatalf("1 kt, 3 m turn: yaw %.4f written at %d, over %.4f (the inside wheels standing)", y, i, limit)
		}
	}
}

// Through taxi turns and pushes at their speeds the yaw rate written never
// has the main wheels on the inside of the turn stand or roll back (the
// simulator turns each wheel from the yaw rate and the speed), and none is
// written standing (live LKPR: CSA1822 BCS3's right main wheels stood still
// in a right turn while taxiing).
func TestNoseWheelInsideWheelsRoll(t *testing.T) {
	const halfTrackM = 3.5 // about half an A320's or A220's main gear track
	for _, c := range []struct {
		kts, radius float64
		reverse     bool
	}{{25, 60, false}, {15, 40, false}, {10, 25, false}, {5, 20, false}, {3, 15, false}, {2, 12, false}, {2, 20, true}, {1.5, 12, true}} {
		yaws, at := steerRun(t, c.kts, c.radius, c.reverse, true)
		// It steers: halfway round (before the stop), into the turn, the
		// other way pushed. Measured frame to frame, a push at 2 kt moved
		// under 2 cm a frame and never steered.
		if y := yaws[at-5]; math.IsNaN(y) || (y > 0) == c.reverse {
			t.Errorf("%.1f kt, %.0f m turn (push %v): yaw %.4f written halfway round, want it steering", c.kts, c.radius, c.reverse, y)
		}
		for i, y := range yaws {
			if math.IsNaN(y) {
				continue
			}
			v := c.kts * ktsToMS
			if i > at && i < at+60 { // standing
				if y != 0 {
					t.Errorf("%.0f kt, %.0f m turn: standing, yaw %.4f rad/s written", c.kts, c.radius, y/0.3048)
					break
				}
				continue
			}
			if inside := v - math.Abs(y/0.3048)*halfTrackM; inside < 0 {
				t.Errorf("%.0f kt, %.0f m turn (push %v): the inside wheels at %.1f m/s at %d", c.kts, c.radius, c.reverse, inside, i)
				break
			}
		}
	}
}
