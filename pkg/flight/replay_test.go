//go:build windows

package flight

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type replayClient struct {
	mapped map[uint32]string
	sent   []string
	data   map[string]uint32
	poses  []userPose
	defs   int
}

func (f *replayClient) MapClientEventToSimEvent(id uint32, name string) error {
	f.mapped[id] = name
	return nil
}

func (f *replayClient) TransmitClientEvent(_ uint32, id uint32, data uint32, _ uint32, _ types.SIMCONNECT_EVENT_FLAG) error {
	f.sent = append(f.sent, f.mapped[id])
	f.data[f.mapped[id]] = data
	return nil
}

func (f *replayClient) AddToDataDefinition(uint32, string, string, types.SIMCONNECT_DATATYPE, float32, uint32) error {
	f.defs++
	return nil
}

func (f *replayClient) SetDataOnSimObject(_ uint32, obj uint32, _ types.SIMCONNECT_DATA_SET_FLAG, _ uint32, size uint32, data unsafe.Pointer) error {
	if obj == types.SIMCONNECT_OBJECT_ID_USER && size == uint32(unsafe.Sizeof(userPose{})) {
		f.poses = append(f.poses, *(*userPose)(data))
	}
	return nil
}

// TestUserReplay: frozen on start; placed every frame (SimConnect's pitch
// and bank signs); levers and lights sent at first and then only as they
// change; freed on stop.
func TestUserReplay(t *testing.T) {
	f := &replayClient{mapped: map[uint32]string{}, data: map[string]uint32{}}
	u := NewUserReplay(f, 0)
	if err := u.Start(); err != nil {
		t.Fatal(err)
	}
	if f.data["FREEZE_ATTITUDE_SET"] != 1 || len(f.sent) != 3 {
		t.Fatalf("start: %v", f.sent)
	}
	s := Sample{Lat: 50, Lon: 14, AltFt: 1500, Pitch: 8, Bank: 5, Heading: 240, GearHandle: true, FlapsHandle: 50, Lights: LightLanding,
		EngineCount: 2, Throttle: [Engines]float64{90, 90}}
	if err := u.Apply(s); err != nil {
		t.Fatal(err)
	}
	if len(f.poses) != 1 || f.poses[0].Pitch != -8 || f.poses[0].Bank != -5 || f.poses[0].Alt != 1500 || f.defs != 6 {
		t.Fatalf("placed %+v, %d definitions", f.poses, f.defs)
	}
	if f.data["GEAR_SET"] != 1 || f.data["FLAPS_SET"] != 8192 || f.data["LANDING_LIGHTS_SET"] != 1 || f.data["AXIS_THROTTLE2_SET"] != 14745 {
		t.Errorf("first frame: %v", f.data)
	}
	n := len(f.sent)
	s.Lat += 0.0001
	s.Throttle[0] = 90.2 // under the threshold
	u.Apply(s)
	if len(f.sent) != n || len(f.poses) != 2 {
		t.Errorf("unchanged: sent %v", f.sent[n:])
	}
	s.GearHandle = false
	u.Apply(s)
	if len(f.sent) != n+1 || f.sent[n] != "GEAR_SET" || f.data["GEAR_SET"] != 0 {
		t.Errorf("gear up: sent %v", f.sent[n:])
	}
	u.Stop()
	if f.data["FREEZE_LATITUDE_LONGITUDE_SET"] != 0 {
		t.Error("not freed")
	}
}

type fakeInjector struct {
	poses    []traffic.FlownPose
	gear     []bool
	engines  []bool
	throttle []float64
	lights   []traffic.Lights
}

func (f *fakeInjector) PlaceFlown(_ uint32, p traffic.FlownPose) error {
	f.poses = append(f.poses, p)
	return nil
}
func (f *fakeInjector) SetGear(_ uint32, down bool) error { f.gear = append(f.gear, down); return nil }
func (f *fakeInjector) SetFlaps(uint32, float64) error    { return nil }
func (f *fakeInjector) SetSpoilers(uint32, float64) error { return nil }
func (f *fakeInjector) SetLights(_ uint32, l traffic.Lights) error {
	f.lights = append(f.lights, l)
	return nil
}
func (f *fakeInjector) SetEngines(_ uint32, _ int, on bool) error {
	f.engines = append(f.engines, on)
	return nil
}
func (f *fakeInjector) SetThrottle(_ uint32, _ int, pct float64) error {
	f.throttle = append(f.throttle, pct)
	return nil
}

// TestGhost: placed as flown each frame; engines started once N1 runs,
// gear, lights and throttle as they change.
func TestGhost(t *testing.T) {
	f := &fakeInjector{}
	g := NewGhost(f, 7)
	s := Sample{Lat: 50, Lon: 14, AltFt: 1200, CGFt: 9.5, Pitch: 3, OnGround: true, GS: 12, GearHandle: true, Lights: LightBeacon | LightTaxi,
		EngineCount: 2, N1: [Engines]float64{20, 20}, Throttle: [Engines]float64{30, 30}}
	g.Apply(s)
	g.Apply(s)
	s.Throttle = [Engines]float64{95, 95}
	s.Lights |= LightLanding | LightStrobe
	g.Apply(s)
	if len(f.poses) != 3 || f.poses[0].CGFt != 9.5 || !f.poses[0].OnGround || f.poses[0].PitchDeg != 3 {
		t.Fatalf("poses %+v", f.poses)
	}
	if len(f.engines) != 1 || !f.engines[0] || len(f.gear) != 1 {
		t.Errorf("engines %v gear %v", f.engines, f.gear)
	}
	if len(f.throttle) != 2 || f.throttle[1] != 95 || len(f.lights) != 2 || !f.lights[1].Landing || !f.lights[1].Strobe {
		t.Errorf("throttle %v lights %+v", f.throttle, f.lights)
	}
}

type ghostClient struct {
	created []types.SIMCONNECT_DATA_INITPOSITION
	title   string
	removed int
}

func (g *ghostClient) AICreateNonATCAircraft(title, _ string, init types.SIMCONNECT_DATA_INITPOSITION, _ uint32) error {
	g.title, g.created = title, append(g.created, init)
	return nil
}
func (g *ghostClient) AIRemoveObject(uint32, uint32) error { g.removed++; return nil }

// TestGhostReplayStart: the ghost is created where the Player stands, in
// SimConnect's attitude signs; its Player seeks; stopped before it was
// given an object, nothing is removed.
func TestGhostReplayStart(t *testing.T) {
	tr := &Track{Samples: []Sample{{T: 0, Lat: 50, Lon: 14, AltFt: 1000, Pitch: 5, Bank: 2, Heading: 90, GS: 140},
		{T: 10, Lat: 50.01, Lon: 14, AltFt: 1500, Pitch: 5, Heading: 90, GS: 150}}}
	c := &ghostClient{}
	r := NewGhostReplay(c, nil, tr, "FSLTL A320", "OK-ABC", 42)
	r.Player().Seek(5, time.Now())
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	if len(c.created) != 1 || c.title != "FSLTL A320" {
		t.Fatalf("created %+v", c.created)
	}
	if i := c.created[0]; math.Abs(i.Altitude-1250) > 1 || i.Pitch != -5 || i.Heading != 90 || i.OnGround != 0 {
		t.Errorf("created at %+v, want the sample 5 s in", i)
	}
	if r.ObjectID() != 0 || r.Stop() != nil || c.removed != 0 {
		t.Error("removed before it had an object")
	}
}
