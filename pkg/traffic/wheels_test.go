package traffic

import (
	"math"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// wheelsClient records the INITPOSITION Airspeed of every placement.
type wheelsClient struct {
	eventClient
	speeds []uint32
}

func (c *wheelsClient) SetDataOnSimObject(def, obj uint32, f types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	if def == DefaultInjectDefinitionBase+injDefPosition {
		c.speeds = append(c.speeds, uint32((*types.SIMCONNECT_DATA_INITPOSITION)(p).Airspeed))
		return nil
	}
	return c.eventClient.SetDataOnSimObject(def, obj, f, n, size, p)
}

// wheelsRun places an A320 at every frame of m and returns the poses and the
// Airspeed passed with each.
func wheelsRun(t *testing.T, m *GroundMover) ([]GroundPose, []uint32) {
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
	return poses, c.speeds
}

// Pushed back, the wheels must not spin forward: INITPOSITION Airspeed can
// only be zero or positive, and any speed spins them forward (user report
// 2026-11-07), so the speed passed is 0 the whole push, through the tug's
// swing too. Taxiing forward, the speed is passed as before.
func TestPushbackWheelsDoNotSpinForward(t *testing.T) {
	prof := DefaultMotionProfile()
	push := prof
	push.CruiseKts, push.MinTurnKts, push.Accel, push.Decel = 3, 1, 0.15, 0.25
	// The main gear backwards 40 m south, then swung 90° east (tail west).
	path, err := NewArcPath([]airport.LatLon{lkpr, offset(lkpr, 0, -40), offset(lkpr, 40, -40)}, push, 20)
	if err != nil {
		t.Fatal(err)
	}
	poses, speeds := wheelsRun(t, NewPushbackMover(path, push, 0))
	moving := 0
	for k, p := range poses {
		if p.GroundSpeedKts >= 0.5 {
			moving++
		}
		if speeds[k] != 0 {
			t.Fatalf("pushback frame %d (%.1f kt, heading %.0f): Airspeed %d, want 0", k, p.GroundSpeedKts, p.Heading, speeds[k])
		}
	}
	if moving < 60 {
		t.Fatalf("the push barely moved: %d frames at 0.5 kt or more", moving)
	}

	fwd, err := NewGroundPath([]airport.LatLon{lkpr, offset(lkpr, 0, 300), offset(lkpr, 200, 300)}, prof)
	if err != nil {
		t.Fatal(err)
	}
	poses, speeds = wheelsRun(t, NewGroundMover(fwd, prof))
	for k, p := range poses {
		if want := uint32(math.Round(p.GroundSpeedKts)); speeds[k] != want {
			t.Fatalf("taxi frame %d (%.1f kt): Airspeed %d, want %d", k, p.GroundSpeedKts, speeds[k], want)
		}
	}
}
