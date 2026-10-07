package traffic

import (
	"math"
	"testing"
	"unsafe"

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

// The simulator rests a rolling A320 0.6 ft above its static CG height
// whatever we send: learned on one aircraft's roll, the next A320 comes down
// to that height in the air, so its wheels meet the runway there (#854).
func TestInjectorLearnsGroundRest(t *testing.T) {
	c := &eventClient{}
	inj := NewInjector(c)
	const a, b, title = 42, 43, "FSLTL A320 Air France SL"
	for _, id := range []uint32{a, b} {
		if err := inj.Takeover(id); err != nil {
			t.Fatal(err)
		}
		inj.SetModel(id, title)
	}
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, a, injectGround{GroundFt: 1200, CGFt: 12.25}))
	inj.Handle(sampleMsg(DefaultInjectRequestBase+3, b, injectGround{GroundFt: 1200, CGFt: 12.25}))
	placedAlt := func() float64 {
		var got types.SIMCONNECT_DATA_INITPOSITION
		copy(unsafe.Slice((*byte)(unsafe.Pointer(&got)), unsafe.Sizeof(got)), c.waypoints[len(c.waypoints)-1])
		return got.Altitude
	}
	// Before anything is learned: the default share above the static height.
	if err := inj.PlaceAir(b, ApproachPose{HeightFt: 0}); err != nil {
		t.Fatal(err)
	}
	if got, want := placedAlt(), 1200+12.25*(1+RestAboveStaticShare); math.Abs(got-want) > 1e-6 {
		t.Errorf("unlearned touchdown at %.3f ft, want %.3f", got, want)
	}
	// a rolls on the runway; the simulator shows it 0.6 ft higher.
	for i := 0; i < groundRestRun; i++ {
		if err := inj.PlaceAir(a, ApproachPose{OnGround: true, GroundSpeedKts: 120}); err != nil {
			t.Fatal(err)
		}
	}
	inj.Handle(sampleMsg(DefaultInjectRequestBase+1, a, injectGround{GroundFt: 1200, CGFt: 12.25, PlaneFt: 1212.85, OnGround: 1, GS: 120}))
	if err := inj.PlaceAir(b, ApproachPose{HeightFt: 0}); err != nil {
		t.Fatal(err)
	}
	if got := placedAlt(); math.Abs(got-1212.85) > 1e-6 {
		t.Errorf("touchdown at %.3f ft, want the learned 1212.85", got)
	}
}
