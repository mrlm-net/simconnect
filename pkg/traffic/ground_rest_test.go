package traffic

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// sampleMsg is a ground sample with all its fields.
func sampleMsg(req, obj uint32, g injectGround) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(injectGround{})))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	*(*injectGround)(unsafe.Pointer(&buf[off])) = g
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

// A model seen resting on its stand (a departure) rests the next of its
// kind (an arrival, created in the air) at that height and pitch: it
// touches down there, not on extended struts (MovingPitchDeg); one of a kind
// not seen resting is placed RestAboveStaticShare above its static height.
func TestInjectorRestByModel(t *testing.T) {
	c := &eventClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 10, 8, 22, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return w } // moments apart: each height as it is, not blending (smoothCG)
	const a, b, title = 42, 43, "FSLTL A320 Air France SL"
	for _, id := range []uint32{a, b} {
		if err := inj.Takeover(id); err != nil {
			t.Fatal(err)
		}
		inj.SetModel(id, title)
	}
	inj.Handle(sampleMsg(DefaultInjectRequestBase+3, b, injectGround{GroundFt: 1200, CGFt: 12.25, PlaneFt: 1500, StaticPitch: 0.9})) // b in the air
	placedAlt := func() (float64, float64) {
		var got types.SIMCONNECT_DATA_INITPOSITION
		copy(unsafe.Slice((*byte)(unsafe.Pointer(&got)), unsafe.Sizeof(got)), c.waypoints[len(c.waypoints)-1])
		return got.Altitude, got.Pitch
	}
	if err := inj.PlaceAir(b, ApproachPose{HeightFt: 0}); err != nil {
		t.Fatal(err)
	}
	if got, _ := placedAlt(); math.Abs(got-(1200+12.25*(1+RestAboveStaticShare))) > 1e-6 {
		t.Errorf("unknown kind: touchdown at %.3f ft", got)
	}
	// a stands on its stand: 11.9 ft, 0.71°.
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, a, injectGround{GroundFt: 1200, CGFt: 12.25, StaticPitch: 0.9, PlaneFt: 1211.9, PlanePitch: 0.71, OnGround: 1}))
	w = w.Add(2 * time.Second)
	if err := inj.PlaceAir(b, ApproachPose{HeightFt: 0, OnGround: true}); err != nil {
		t.Fatal(err)
	}
	// Touching down (from the air): its wheels where the simulator puts
	// them rolling with the on-ground flag, its static CG height (no roll
	// height learnt for the title yet), pitched as a's rest.
	if alt, pitch := placedAlt(); math.Abs(alt-1212.25) > 1e-6 || math.Abs(pitch-0.9) > 1e-6 {
		t.Errorf("touchdown at %.3f ft, pitch %.2f, want the static 1212.25 and pitch 0.90, on all its wheels", alt, pitch)
	}
	w = w.Add(2 * time.Second)
	if err := inj.Place(b, GroundPose{Position: airport.LatLon{Lat: 50, Lon: 14}}); err != nil {
		t.Fatal(err)
	}
	if alt, pitch := placedAlt(); math.Abs(alt-1211.9) > 1e-6 || math.Abs(pitch-0.9) > 1e-6 {
		t.Errorf("taxiing at %.3f ft, pitch %.2f, want a's rest 1211.9, its static pitch 0.90", alt, pitch)
	}
}

// TestInjectorLearnsSettledRest: placed standing and then left alone, the
// height the simulator settles an aircraft at becomes its rest; a sample
// right after a placement (our own) does not.
func TestInjectorLearnsSettledRest(t *testing.T) {
	c := &eventClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 10, 8, 23, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj = 42
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	created := injectGround{GroundFt: 1200, CGFt: 8.48, StaticPitch: 0.7, PlaneFt: 1208.48, PlanePitch: 0.7, OnGround: 1}
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, created))
	pose := GroundPose{Position: airport.LatLon{Lat: 50, Lon: 14}}
	placedAlt := func() float64 {
		var got types.SIMCONNECT_DATA_INITPOSITION
		copy(unsafe.Slice((*byte)(unsafe.Pointer(&got)), unsafe.Sizeof(got)), c.waypoints[len(c.waypoints)-1])
		return got.Altitude
	}
	inj.Place(obj, pose)
	settled := created
	settled.PlaneFt = 1208.13
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, settled)) // at once: ours
	inj.Place(obj, pose)
	if got := placedAlt(); math.Abs(got-1208.48) > 1e-6 {
		t.Fatalf("learnt from a sample right after placing: %.2f", got)
	}
	w = w.Add(300 * time.Millisecond)
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, settled)) // the sim settled it
	inj.Place(obj, pose)
	// Blended in at CGBlendFtPerSecond, not snapped: 0.3 s, 0.3 ft of 0.35.
	if got := placedAlt(); math.Abs(got-1208.18) > 1e-6 {
		t.Errorf("0.3 s after settling, placed at %.2f, want 1208.18 on its way", got)
	}
	for k := 0; k < 10; k++ {
		w = w.Add(100 * time.Millisecond)
		inj.Place(obj, pose)
	}
	if got := placedAlt(); math.Abs(got-1208.13) > 1e-6 {
		t.Errorf("placed at %.2f, want the settled 1208.13", got)
	}
}

// TestInjectorNoRestFromRoll: placed by PlaceAir (a take-off roll, a
// rollout) a while, the samples are its own: not learnt as its rest.
func TestInjectorNoRestFromRoll(t *testing.T) {
	c := &eventClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj = 42
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	g := injectGround{GroundFt: 1200, CGFt: 8.48, StaticPitch: 0.7, PlaneFt: 1208.48, PlanePitch: 0.7, OnGround: 1}
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, g))
	inj.Place(obj, GroundPose{Position: airport.LatLon{Lat: 50, Lon: 14}}) // lined up
	for i := 0; i < 30; i++ { // a 3 s roll, placed every 100 ms
		w = w.Add(100 * time.Millisecond)
		inj.PlaceAir(obj, ApproachPose{OnGround: true, GroundSpeedKts: float64(10 + 3*i)})
		rolled := g
		rolled.PlaneFt = 1209.2 // what the sim shows of the roll
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, rolled))
	}
	w = w.Add(2 * time.Second) // past the blend from what the roll showed
	inj.Place(obj, GroundPose{Position: airport.LatLon{Lat: 50, Lon: 14}})
	var got types.SIMCONNECT_DATA_INITPOSITION
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&got)), unsafe.Sizeof(got)), c.waypoints[len(c.waypoints)-1])
	if math.Abs(got.Altitude-1208.48) > 1e-6 {
		t.Errorf("rest learnt from the roll: placed at %.2f, want 1208.48", got.Altitude)
	}
}

// Rolling with the on-ground flag: the static CG height, where the
// simulator puts a rolling aircraft itself; at lift-off the height blends
// up from there. At touchdown the flag waits until the wheels are blended
// down to it.
func TestGroundFlagBlends(t *testing.T) {
	c := &eventClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, title = 42, "FSLTL A220 CSA"
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, title)
	sent := func() types.SIMCONNECT_DATA_INITPOSITION {
		var got types.SIMCONNECT_DATA_INITPOSITION
		copy(unsafe.Slice((*byte)(unsafe.Pointer(&got)), unsafe.Sizeof(got)), c.waypoints[len(c.waypoints)-1])
		return got
	}
	// Rolling with the flag: the simulator shows the wheels at 7.00 ft.
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: 1200, CGFt: 7.05, StaticPitch: 0, PlaneFt: 1207.05, OnGround: 1}))
	for k := 0; k < 3; k++ {
		w = w.Add(time.Second / 60)
		inj.PlaceAir(obj, ApproachPose{OnGround: true, GroundSpeedKts: 120})
		inj.Handle(sampleMsg(DefaultInjectRequestBase+1, obj, injectGround{GroundFt: 1200, CGFt: 7.05, PlaneFt: 1207.00, OnGround: 1, GS: 120}))
	}
	if got := sent().Altitude - 1200; math.Abs(got-7.05) > 1e-6 {
		t.Errorf("rolling at %.2f ft, want the static 7.05", got)
	}
	// Lift-off: on from 7.05, a frame's blend at most (on extended struts
	// all along: no step to put in).
	w = w.Add(time.Second / 60)
	inj.PlaceAir(obj, ApproachPose{HeightFt: 0.1, GroundSpeedKts: 132})
	if got, want := sent().Altitude-1200-0.1, 7.05; math.Abs(got-want) > 2*CGBlendFtPerSecond/60+1e-6 {
		t.Errorf("first frame off the ground at %.2f ft, want %.2f and a frame's blend", got, want)
	}
	// Touchdown with the cg 0.5 ft above the roll height: the flag held off
	// until it is there.
	inj.mu.Lock()
	inj.objects[obj].cgShown = 7.50
	inj.mu.Unlock()
	w = w.Add(time.Second / 60)
	inj.PlaceAir(obj, ApproachPose{OnGround: true, GroundSpeedKts: 130})
	if p := sent(); p.OnGround != 0 {
		t.Fatalf("touchdown 0.5 ft above the roll height: flag set at once")
	}
	for k := 0; k < 60 && sent().OnGround == 0; k++ {
		w = w.Add(time.Second / 60)
		inj.PlaceAir(obj, ApproachPose{OnGround: true, GroundSpeedKts: 128})
	}
	if p := sent(); p.OnGround == 0 || math.Abs(p.Altitude-1207.05) > groundFlagWithinFt+0.02 {
		t.Errorf("after the blend: flag %d at %.2f ft, want set at about 1207.05", p.OnGround, p.Altitude)
	}
}

// TestInjectorPlaceRest: parked, an aircraft is put at its static pitch and
// CG height, where the simulator settles it on all its wheels, not on
// extended struts (a degree off) with the main gear in the air (live,
// B738s at 9.72 ft on the nose wheel alone); and that pose is not learnt
// as its kind's rest.
func TestInjectorPlaceRest(t *testing.T) {
	c := &eventClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 10, 20, 16, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const a, title = 42, "FSLTL_FAIB_B738_TVS-Smartwings_NC"
	if err := inj.Takeover(a); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(a, title)
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, a, injectGround{GroundFt: 1200, CGFt: 8.48, StaticPitch: 0.71, PlaneFt: 1209.72, PlanePitch: 1.71, OnGround: 1}))
	if err := inj.PlaceRest(a, GroundPose{Position: airport.LatLon{Lat: 50, Lon: 14}, Heading: 240}); err != nil {
		t.Fatal(err)
	}
	var got types.SIMCONNECT_DATA_INITPOSITION
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&got)), unsafe.Sizeof(got)), c.waypoints[len(c.waypoints)-1])
	if math.Abs(got.Altitude-1208.48) > 1e-6 || math.Abs(got.Pitch-0.71) > 1e-6 || got.OnGround != 1 || got.Heading != 240 {
		t.Errorf("parked at %.2f ft, pitch %.2f, on ground %d, heading %.0f; want 1208.48, 0.71, 1, 240", got.Altitude, got.Pitch, got.OnGround, got.Heading)
	}
	// The simulator shows it settled there a while later: not its kind's rest.
	before := inj.rests[title]
	w = w.Add(time.Minute)
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, a, injectGround{GroundFt: 1200, CGFt: 8.48, StaticPitch: 0.71, PlaneFt: 1208.40, PlanePitch: 0.71, OnGround: 1}))
	if r := inj.rests[title]; r != before {
		t.Errorf("parked pose learnt as the rest of its kind: %v, was %v", r, before)
	}
}
