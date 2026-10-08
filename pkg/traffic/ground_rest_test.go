package traffic

import (
	"math"
	"testing"
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
	if err := inj.PlaceAir(b, ApproachPose{HeightFt: 0, OnGround: true}); err != nil {
		t.Fatal(err)
	}
	if alt, pitch := placedAlt(); math.Abs(alt-1211.9) > 1e-6 || math.Abs(pitch-0.71) > 1e-6 {
		t.Errorf("touchdown at %.3f ft, pitch %.2f, want a's rest 1211.9, 0.71", alt, pitch)
	}
	if err := inj.Place(b, GroundPose{Position: airport.LatLon{Lat: 50, Lon: 14}}); err != nil {
		t.Fatal(err)
	}
	if alt, pitch := placedAlt(); math.Abs(alt-1211.9) > 1e-6 || math.Abs(pitch-0.71) > 1e-6 {
		t.Errorf("taxiing at %.3f ft, pitch %.2f, want a's rest 1211.9, 0.71", alt, pitch)
	}
}
