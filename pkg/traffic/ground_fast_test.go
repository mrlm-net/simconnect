package traffic

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// simGroundClient answers each object's ground request as the simulator
// would: with the definition last asked for.
type simGroundClient struct {
	eventClient
	def        map[uint32]uint32
	full, fast int // answers of each
}

func (c *simGroundClient) RequestDataOnSimObject(req, def, obj uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin, interval, limit uint32) error {
	if def == DefaultInjectDefinitionBase+injDefGround || def == DefaultInjectDefinitionBase+injDefGroundFast {
		if c.def == nil {
			c.def = map[uint32]uint32{}
		}
		c.def[obj] = def
	}
	return c.eventClient.RequestDataOnSimObject(req, def, obj, period, flags, origin, interval, limit)
}

func (c *simGroundClient) asksFast(obj uint32) bool {
	return c.def[obj] == DefaultInjectDefinitionBase+injDefGroundFast
}

// answer hands inj obj's ground sample g as asked for: the ground alone
// when the fast definition is.
func (c *simGroundClient) answer(inj *Injector, req, obj uint32, g injectGround) {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(injectGround{})))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	if c.asksFast(obj) {
		c.fast++
		h.DwDefineID = types.DWORD(DefaultInjectDefinitionBase + injDefGroundFast)
		*(*injectGroundFast)(unsafe.Pointer(&buf[off])) = injectGroundFast{GroundFt: g.GroundFt, Lat: g.Lat, Lon: g.Lon}
	} else {
		c.full++
		h.DwDefineID = types.DWORD(DefaultInjectDefinitionBase + injDefGround)
		*(*injectGround)(unsafe.Pointer(&buf[off])) = g
	}
	inj.Handle(engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))})
}

// A stand arrival: taxiing, only the ground under it is read every frame;
// stopping on its stand brings the full sample back in the same call, and
// its rest is learnt there as before.
func TestGroundFastTaxiThenRestOnStand(t *testing.T) {
	c := &simGroundClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj, req, title = 42, DefaultInjectRequestBase + 1, "FSLTL A320 Air France SL"
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, title)
	if c.asksFast(obj) {
		t.Fatal("taken over: the ground alone asked for before any full sample")
	}
	// Created in the air: no rest yet.
	c.answer(inj, req, obj, injectGround{GroundFt: 1200, CGFt: 12.25, StaticPitch: 0.9, PlaneFt: 1500})
	rolling := injectGround{GroundFt: 1200, CGFt: 12.25, StaticPitch: 0.9, PlaneFt: 1212.25, PlanePitch: 0.5, OnGround: 1, GS: 15}
	pose := GroundPose{Position: lkpr, Heading: 214, GroundSpeedKts: 15}
	for k := 0; k < 600; k++ { // 10 s taxiing in
		w = w.Add(time.Second / 60)
		pose.Position = offsetHeading(pose.Position, 214, 15*0.5144/60)
		if err := inj.Place(obj, pose); err != nil {
			t.Fatal(err)
		}
		c.answer(inj, req, obj, rolling)
	}
	if !c.asksFast(obj) || c.full > 2 {
		t.Fatalf("taxiing: fast %v, %d full samples of %d; want the ground alone", c.asksFast(obj), c.full, c.full+c.fast)
	}
	pose.GroundSpeedKts = 0
	w = w.Add(time.Second / 60)
	if err := inj.Place(obj, pose); err != nil {
		t.Fatal(err)
	}
	if c.asksFast(obj) {
		t.Fatal("stopped: still the ground alone after the stopping placement")
	}
	// On its stand: placed every 30th frame (Detail), the simulator settling
	// it at 11.9 ft, 0.71°.
	settled := injectGround{GroundFt: 1200, CGFt: 12.25, StaticPitch: 0.9, PlaneFt: 1211.9, PlanePitch: 0.71, OnGround: 1}
	for k := 1; k < 5*60; k++ {
		w = w.Add(time.Second / 60)
		if k%30 == 0 {
			if err := inj.Place(obj, pose); err != nil {
				t.Fatal(err)
			}
		}
		if c.asksFast(obj) {
			t.Fatalf("frame %d on its stand: the ground alone", k)
		}
		c.answer(inj, req, obj, settled)
	}
	r, ok := inj.rests[title]
	if !ok || math.Abs(r[0]-11.9) > 1e-6 {
		t.Errorf("rest on its stand %v (%v), want 11.9 ft learnt", r, ok)
	}
	if o := inj.objects[obj]; math.Abs(o.cgFt-12.25) > 1e-9 || math.Abs(o.staticPitch-0.9) > 1e-9 {
		t.Errorf("static CG %.2f pitch %.2f, want 12.25 and 0.9 kept through the fast samples", o.cgFt, o.staticPitch)
	}
}

// A landing: the ground alone on the approach, the full sample from
// AirBlendFt down, so its main gear height is learnt at touchdown.
func TestGroundFastLandingLearnsMains(t *testing.T) {
	c := &simGroundClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { w = w.Add(time.Second / 60); return w }
	const obj, req, title = 42, DefaultInjectRequestBase + 1, "FSLTL_FAIB_B738_TVS-Skytravel_TSOC"
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, title)
	air := injectGround{GroundFt: 1200, CGFt: 8.48, StaticPitch: 0.71, PlaneFt: 1700}
	c.answer(inj, req, obj, air)
	const arm, pitch = 1.2, 4.2
	fastHigh := false
	for k := 600; k >= 0; k-- { // 500 ft down to the runway
		h := float64(k) * 500 / 600
		pos := offsetHeading(lkpr, 244, float64(600-k)*1.2)
		if err := inj.PlaceAir(obj, ApproachPose{Position: pos, Heading: 244, GroundSpeedKts: 140, HeightFt: h, PitchDeg: pitch, Landing: true, MainGearBehindM: arm}); err != nil {
			t.Fatal(err)
		}
		if h > 2*AirBlendFt && c.asksFast(obj) {
			fastHigh = true
		}
		if h < AirBlendFt && c.asksFast(obj) {
			t.Fatalf("%.0f ft: the ground alone below AirBlendFt", h)
		}
		air.PlaneFt = 1200 + h + 8.48
		c.answer(inj, req, obj, air)
	}
	if !fastHigh {
		t.Error("approach: the full sample every frame, want the ground alone")
	}
	down := injectGround{GroundFt: 1200, CGFt: 8.48, StaticPitch: 0.71, PlaneFt: 1209.55, PlanePitch: -pitch, OnGround: 1, GS: 130}
	for k := 0; k < 30; k++ {
		if err := inj.PlaceAir(obj, ApproachPose{Position: offsetHeading(lkpr, 244, 720+float64(k)*1.1), Heading: 244, GroundSpeedKts: 130, OnGround: true, PitchDeg: pitch, Landing: true, MainGearBehindM: arm}); err != nil {
			t.Fatal(err)
		}
		if c.asksFast(obj) {
			t.Fatalf("frame %d of touchdown: the ground alone", k)
		}
		c.answer(inj, req, obj, down)
	}
	if _, ok := inj.mains[title]; !ok {
		t.Error("touchdown: main gear height not learnt")
	}
}

// A take-off roll reads the full sample from strutsLearnMinKts, so the
// struts and their reach are learnt (struts.go), known or not.
func TestGroundFastRollLearnsStruts(t *testing.T) {
	c := &simGroundClient{}
	inj := NewInjector(c)
	w := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { w = w.Add(time.Second / 60); return w }
	const obj, req, title = 42, DefaultInjectRequestBase + 1, "FSLTL A320 Air France SL"
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(obj, title)
	roll := injectGround{GroundFt: 1200, CGFt: 12.25, StaticPitch: 0.9, PlaneFt: 1212.85, PlanePitch: 0.95, OnGround: 1, GS: 60}
	c.answer(inj, req, obj, roll)
	pos := lkpr
	for k := 0; k < 120; k++ {
		pos = offsetHeading(pos, 244, 60*0.5144/60)
		if err := inj.PlaceAir(obj, ApproachPose{Position: pos, Heading: 244, GroundSpeedKts: 60, OnGround: true}); err != nil {
			t.Fatal(err)
		}
		c.answer(inj, req, obj, roll)
	}
	s, ok := inj.struts[strutsKey(12.25)]
	if !ok || math.Abs(s-0.6) > 1e-6 {
		t.Fatalf("struts %.2f (%v), want 0.60 learnt on the roll", s, ok)
	}
	pos = offsetHeading(pos, 244, 1)
	if err := inj.PlaceAir(obj, ApproachPose{Position: pos, Heading: 244, GroundSpeedKts: 61, OnGround: true}); err != nil {
		t.Fatal(err)
	}
	if c.asksFast(obj) {
		t.Error("struts known: the ground alone on the roll, their reach not learnt")
	}
}
