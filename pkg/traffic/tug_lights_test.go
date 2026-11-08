package traffic

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// tugEngineClient records engine writes (GENERAL ENG COMBUSTION:1) per object.
type tugEngineClient struct {
	tugClient
	engineOn []float64 // seconds into the test of each "on" write to object 55
	now      float64
}

func (c *tugEngineClient) SetDataOnSimObject(def, obj uint32, f types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	if def == DefaultInjectDefinitionBase+injDefEngine1 && obj == 55 && *(*float64)(p) == 1 {
		c.engineOn = append(c.engineOn, c.now)
	}
	return c.tugClient.SetDataOnSimObject(def, obj, f, n, size, p)
}

// TestTugEngineRunsForItsLights: ground vehicle lights (Asobo
// GroundVehicles.xml, GSX gyro lights) burn only while the vehicle's engine
// runs, so the tug's engine is started once it is ours and re-sent while it
// waits on the nose and drives off (live LKPR 2026-11-07: tugs sometimes
// dark, never an engine write).
func TestTugEngineRunsForItsLights(t *testing.T) {
	c := &tugEngineClient{}
	inj := NewInjector(c)
	prof := DefaultMotionProfile()
	tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
	pose := GroundPose{Position: lkpr, Heading: 90}
	if err := tug.Attach(pose); err != nil {
		t.Fatal(err)
	}
	if !tug.Handle(assignedMsg(9001, 55)) || tug.ObjectID() != 55 {
		t.Fatal("assignment not handled")
	}
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
	const dt = 1.0 / 60
	if err := tug.Update(pose, true, dt); err != nil {
		t.Fatal(err)
	}
	if len(c.engineOn) == 0 {
		t.Fatal("tug engine not started once assigned: its lights stay off")
	}
	// A minute on the nose: re-sent, not every frame.
	for i := 0; i < 60*60; i++ {
		c.now += dt
		if err := tug.Update(pose, true, dt); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(c.engineOn); n < 4 || n > 10 {
		t.Errorf("%d engine writes in a minute (at %v), want a re-send about every 10 s", n, c.engineOn)
	}
}
