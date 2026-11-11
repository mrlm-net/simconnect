package traffic

import (
	"math"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// wheelsClient records the INITPOSITION Airspeed of every placement and
// the VELOCITY BODY Z written after it (NaN: none).
type wheelsClient struct {
	eventClient
	speeds []uint32
	vz     []float64
}

func (c *wheelsClient) SetDataOnSimObject(def, obj uint32, f types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	switch def {
	case DefaultInjectDefinitionBase + injDefPosition:
		c.speeds = append(c.speeds, uint32((*types.SIMCONNECT_DATA_INITPOSITION)(p).Airspeed))
		c.vz = append(c.vz, math.NaN())
		return nil
	case DefaultInjectDefinitionBase + injDefWheels:
		if len(c.vz) == 0 || !math.IsNaN(c.vz[len(c.vz)-1]) {
			panic("VELOCITY BODY Z written before its placement, or twice")
		}
		c.vz[len(c.vz)-1] = *(*float64)(p)
		return nil
	}
	return c.eventClient.SetDataOnSimObject(def, obj, f, n, size, p)
}

// wheelsRun places an A320 at every frame of m and returns the poses, the
// Airspeed passed with each and the VELOCITY BODY Z written after it.
func wheelsRun(t *testing.T, m *GroundMover) ([]GroundPose, []uint32, []float64) {
	t.Helper()
	c := &wheelsClient{}
	inj := NewInjector(c)
	inj.Takeover(42)
	inj.SetModel(42, "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery")
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 42, 1200, 12.25))
	poses := drive(m, 600)
	for _, p := range poses {
		if err := inj.Place(42, p); err != nil {
			t.Fatal(err)
		}
	}
	return poses, c.speeds, c.vz
}

// checkPushWheels fails unless every pushed frame of poses moving at
// restMaxKts or more, from the first wheelMinMoveM on, passes Airspeed 0
// and writes VELOCITY BODY Z minus its speed: the wheels turn backwards.
func checkPushWheels(t *testing.T, poses []GroundPose, speeds []uint32, vz []float64) {
	t.Helper()
	moving := 0
	for k, p := range poses {
		if p.GroundSpeedKts < restMaxKts || localDist(poses[0].Position, p.Position) < 2*wheelMinMoveM {
			continue
		}
		moving++
		if want := -p.GroundSpeedKts * 0.514444; speeds[k] != 0 || math.IsNaN(vz[k]) || math.Abs(vz[k]-want) > 1e-6 {
			t.Fatalf("pushback frame %d (%.1f kt, heading %.0f): Airspeed %d, VELOCITY BODY Z %.3f, want 0 and %.3f", k, p.GroundSpeedKts, p.Heading, speeds[k], vz[k], want)
		}
	}
	if moving < 60 {
		t.Fatalf("the push barely moved: %d frames at %.1f kt or more", moving, restMaxKts)
	}
}

// Pushed back, the wheels turn backwards: INITPOSITION Airspeed can only be
// zero or positive and spins them forward (user, live LKPR 2026-11-11:
// "wheels are not spinning properly on plane during pushback"), so it is 0
// and VELOCITY BODY Z, which the models turn them from, is written minus
// the ground speed the whole push, through the tug's swing too. Taxiing
// forward, the ground speed is passed and nothing written.
func TestPushbackWheelsSpin(t *testing.T) {
	prof := DefaultMotionProfile()
	push := prof
	push.CruiseKts, push.MinTurnKts, push.Accel, push.Decel = 3, 1, 0.15, 0.25
	// The main gear backwards 40 m south, then swung 90° east (tail west).
	path, err := NewArcPath([]airport.LatLon{lkpr, offset(lkpr, 0, -40), offset(lkpr, 40, -40)}, push, 20)
	if err != nil {
		t.Fatal(err)
	}
	poses, speeds, vz := wheelsRun(t, NewPushbackMover(path, push, 0))
	checkPushWheels(t, poses, speeds, vz)
	if last := len(vz) - 1; poses[last].GroundSpeedKts < restMaxKts && !math.IsNaN(vz[last]) && vz[last] != 0 {
		t.Errorf("stopped after the push: VELOCITY BODY Z %.3f left", vz[last])
	}

	fwd, err := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 300), offset(lkpr, 200, 300)}, prof)
	if err != nil {
		t.Fatal(err)
	}
	poses, speeds, vz = wheelsRun(t, NewGroundMover(fwd, prof))
	for k, p := range poses {
		if want := uint32(math.Round(p.GroundSpeedKts)); speeds[k] != want || !math.IsNaN(vz[k]) {
			t.Fatalf("taxi frame %d (%.1f kt): Airspeed %d, VELOCITY BODY Z %.3f, want %d and none", k, p.GroundSpeedKts, speeds[k], vz[k], want)
		}
	}
}
