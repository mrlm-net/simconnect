//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Lights is the exterior light state of an injected aircraft.
type Lights struct {
	Nav, Beacon, Strobe, Taxi, Landing, Logo, Wing bool
}

// Light states by phase, following common airline procedures.
var (
	LightsParked   = Lights{Nav: true, Logo: true}
	LightsPushback = Lights{Nav: true, Beacon: true, Logo: true}
	LightsTaxi     = Lights{Nav: true, Beacon: true, Taxi: true, Logo: true}
	LightsRunway   = Lights{Nav: true, Beacon: true, Strobe: true, Taxi: true, Landing: true, Logo: true}
)

// Injector drives aircraft by position injection (#309). While an aircraft
// is driven by MSFS AI, the AI overrides its lights within about a second;
// once it is taken over here, nothing does, so lights set with SetLights
// stay as set.
//
// Takeover releases AI control and freezes position, altitude and attitude,
// so the aircraft moves only when Place writes it (at InjectHz, from a
// GroundMover). The ground height under each aircraft is requested every
// sim frame; feed messages to Handle. Release unfreezes the aircraft.
//
// Like the other types in this package, Injector never reads the client's
// stream.
type Injector struct {
	client           engine.Client
	defBase, reqBase uint32
	evtBase          uint32
	registered       bool
	mu               sync.Mutex
	objects          map[uint32]*injected // by object ID
	byRequest        map[uint32]uint32    // ground request ID → object ID
	slots            [injectMaxAircraft]bool
	sent             map[uint32]string
}

type injected struct {
	slot           int
	groundFt, cgFt float64
	staticPitch    float64 // degrees, the simulator's convention
	haveGround     bool
	// restFt and restPitch: how the aircraft itself rested on its gear
	// before we placed it (CG above the ground, pitch), measured while
	// stopped. Its gear compresses under its weight: live, a Fenix A319
	// sat 1.1 ft below STATIC CG TO GROUND, so placed at the static
	// values the nose wheel floated as the tug connected (2026-10-03).
	restFt, restPitch float64
	haveRest          bool
	placed            bool // placed by us: the samples since are our own
	lights            Lights
	lightsSent        bool
	taken             bool // taken over (released and frozen), not just watched
}

// Injector definition, request and event offsets.
const (
	injDefPosition = iota
	injDefGround
	injDefGear
	injDefFlaps
	injDefSpoilers
	injDefEngine1 // GENERAL ENG COMBUSTION:1, one definition per engine up to injMaxEngines
)

// injMaxEngines is how many engines SetEngines reaches.
const injMaxEngines = 4

const (
	injEvtFreezeLatLon = iota
	injEvtFreezeAlt
	injEvtFreezeAtt
	injEvtNav
	injEvtBeacon
	injEvtStrobe
	injEvtTaxi
	injEvtLanding
	injEvtLogo
	injEvtWing
	injectEventCount
)

var injectEventNames = [injectEventCount]string{
	"FREEZE_LATITUDE_LONGITUDE_SET", "FREEZE_ALTITUDE_SET", "FREEZE_ATTITUDE_SET",
	"NAV_LIGHTS_SET", "BEACON_LIGHTS_SET", "STROBES_SET", "TAXI_LIGHTS_SET",
	"LANDING_LIGHTS_SET", "LOGO_LIGHTS_SET", "WING_LIGHTS_SET",
}

// injectGround is the ground under an aircraft and how it rests on its
// gear: CG height and STATIC PITCH, the attitude it sits at (the
// simulator's pitch convention, as PLANE PITCH DEGREES: an A320 rests at
// about +0.8°). Placed level, a model resting nose-up dug its nose wheel
// into the ground (live, 2026-10-02).
type injectGround struct {
	GroundFt, CGFt, StaticPitch       float64
	PlaneFt, PlanePitch, OnGround, GS float64
}

// Rest limits: a measured rest further than this from the static values is
// not trusted, and only an aircraft slower than restMaxKts is at rest
// (braking or turning, it pitches).
const (
	restMaxOffFt    = 3.0
	restMaxOffPitch = 3.0
	restMaxKts      = 1.0
)

// InjectorOption configures an Injector.
type InjectorOption func(*Injector)

// InjectorWithIDs sets the SimConnect ID bases: 5 definition IDs, 2 request
// IDs per aircraft (up to 96 aircraft and tugs) and 10 event IDs are used.
func InjectorWithIDs(definitionBase, requestBase, eventBase uint32) InjectorOption {
	return func(i *Injector) { i.defBase, i.reqBase, i.evtBase = definitionBase, requestBase, eventBase }
}

// NewInjector returns an Injector using client.
func NewInjector(client engine.Client, opts ...InjectorOption) *Injector {
	i := &Injector{
		client:    client,
		defBase:   DefaultInjectDefinitionBase,
		reqBase:   DefaultInjectRequestBase,
		evtBase:   DefaultInjectEventBase,
		objects:   map[uint32]*injected{},
		byRequest: map[uint32]uint32{},
		sent:      map[uint32]string{},
	}
	for _, o := range opts {
		o(i)
	}
	return i
}

// track remembers which call a send ID belongs to, for exception reports.
func (i *Injector) track(call string, err error) error {
	if id, e := i.client.GetLastSentPacketID(); e == nil {
		i.sent[id] = call
	}
	return err
}

func (i *Injector) register() error {
	if i.registered {
		return nil
	}
	c := i.client
	if err := i.track("define Initial Position", c.AddToDataDefinition(i.defBase+injDefPosition, "Initial Position", "", types.SIMCONNECT_DATATYPE_INITPOSITION, 0, 0)); err != nil {
		return err
	}
	for k, v := range []struct{ name, unit string }{{"GROUND ALTITUDE", "feet"}, {"STATIC CG TO GROUND", "feet"}, {"STATIC PITCH", "degrees"},
		{"PLANE ALTITUDE", "feet"}, {"PLANE PITCH DEGREES", "degrees"}, {"SIM ON GROUND", "bool"}, {"GROUND VELOCITY", "knots"}} {
		if err := i.track("define "+v.name, c.AddToDataDefinition(i.defBase+injDefGround, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(k))); err != nil {
			return err
		}
	}
	if err := i.track("define GEAR HANDLE POSITION", c.AddToDataDefinition(i.defBase+injDefGear, "GEAR HANDLE POSITION", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)); err != nil {
		return err
	}
	// Flap surfaces directly: MSFS AI objects ignore the flaps handle and
	// FLAPS_* events (#318).
	for k, v := range []string{"TRAILING EDGE FLAPS LEFT PERCENT", "TRAILING EDGE FLAPS RIGHT PERCENT", "LEADING EDGE FLAPS LEFT PERCENT", "LEADING EDGE FLAPS RIGHT PERCENT"} {
		if err := i.track("define "+v, c.AddToDataDefinition(i.defBase+injDefFlaps, v, "percent", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(k))); err != nil {
			return err
		}
	}
	for k, v := range []string{"SPOILERS HANDLE POSITION", "SPOILERS LEFT POSITION", "SPOILERS RIGHT POSITION"} {
		if err := i.track("define "+v, c.AddToDataDefinition(i.defBase+injDefSpoilers, v, "percent", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(k))); err != nil {
			return err
		}
	}
	// Engines one by one: an index the aircraft does not have is never set.
	for k := 1; k <= injMaxEngines; k++ {
		v := fmt.Sprintf("GENERAL ENG COMBUSTION:%d", k)
		if err := i.track("define "+v, c.AddToDataDefinition(i.defBase+injDefEngine1+uint32(k-1), v, "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)); err != nil {
			return err
		}
	}
	for k, name := range injectEventNames {
		if err := i.track("map "+name, c.MapClientEventToSimEvent(i.evtBase+uint32(k), name)); err != nil {
			return err
		}
	}
	i.registered = true
	return nil
}

func (i *Injector) event(obj uint32, evt int, on bool) error {
	data := uint32(0)
	if on {
		data = 1
	}
	return i.track(fmt.Sprintf("%s %d on object %d", injectEventNames[evt], data, obj),
		i.client.TransmitClientEvent(obj, i.evtBase+uint32(evt), data, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY))
}

// Takeover takes objectID away from MSFS AI: AI control is released (any
// waypoints stop), the aircraft is frozen where it is, and the ground height
// under it is requested every sim frame. Taking over a driven aircraft again
// does nothing.
func (i *Injector) Takeover(objectID uint32) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	o, err := i.watch(objectID)
	if err != nil || o.taken {
		return err
	}
	req := i.reqBase + 2*uint32(o.slot)
	if err := i.track(fmt.Sprintf("AIReleaseControl object %d", objectID), i.client.AIReleaseControl(objectID, req)); err != nil {
		return err
	}
	for _, e := range []int{injEvtFreezeLatLon, injEvtFreezeAlt, injEvtFreezeAtt} {
		if err := i.event(objectID, e, true); err != nil {
			return err
		}
	}
	o.taken = true
	return nil
}

// Watch starts requesting the ground height under objectID without taking
// it over, so a later Takeover can place the aircraft on its first frame
// (a moving aircraft taken over before the ground height arrives would stand
// still for a frame or two). Takeover watches by itself.
func (i *Injector) Watch(objectID uint32) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, err := i.watch(objectID)
	return err
}

func (i *Injector) watch(objectID uint32) (*injected, error) {
	if o, ok := i.objects[objectID]; ok {
		return o, nil
	}
	if err := i.register(); err != nil {
		return nil, err
	}
	slot := -1
	for k, used := range i.slots {
		if !used {
			slot = k
			break
		}
	}
	if slot < 0 {
		return nil, ErrInjectorFull
	}
	req := i.reqBase + 2*uint32(slot)
	if err := i.track(fmt.Sprintf("request ground height object %d", objectID),
		i.client.RequestDataOnSimObject(req+1, i.defBase+injDefGround, objectID, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)); err != nil {
		return nil, err
	}
	o := &injected{slot: slot}
	i.slots[slot] = true
	i.objects[objectID] = o
	i.byRequest[req+1] = objectID
	return o, nil
}

// Driven reports whether objectID has been taken over and is driven by the
// injector.
func (i *Injector) Driven(objectID uint32) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	o, ok := i.objects[objectID]
	return ok && o.taken
}

// Place puts objectID at pose, on the ground. Call it at InjectHz. It
// returns ErrGroundUnknown until the first ground height has arrived.
func (i *Injector) Place(objectID uint32, pose GroundPose) error {
	return i.place(objectID, pose, false)
}

// PlaceMoving is Place with the ground speed passed on as the object's
// speed (whole knots), so vehicles animate their wheels while placed.
func (i *Injector) PlaceMoving(objectID uint32, pose GroundPose) error {
	return i.place(objectID, pose, true)
}

func (i *Injector) place(objectID uint32, pose GroundPose, moving bool) error {
	i.mu.Lock()
	o, ok := i.objects[objectID]
	if !ok || !o.taken {
		i.mu.Unlock()
		return ErrNotInjected
	}
	if !o.haveGround {
		i.mu.Unlock()
		return ErrGroundUnknown
	}
	cg, pitch := o.cgFt, o.staticPitch // resting on its gear, not level
	if o.haveRest {
		cg, pitch = o.restFt, o.restPitch
	}
	o.placed = true
	p := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  pose.Position.Lat,
		Longitude: pose.Position.Lon,
		Altitude:  o.groundFt + cg,
		Pitch:     pitch,
		Heading:   pose.Heading,
		OnGround:  1,
	}
	if moving {
		p.Airspeed = types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(math.Max(0, pose.GroundSpeedKts)))
	}
	i.mu.Unlock()
	// Not tracked: at 60 Hz the send-ID map would grow without bound.
	return i.client.SetDataOnSimObject(i.defBase+injDefPosition, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
}

// SetLights switches the lights of objectID to l, sending only the lights
// that change (all of them the first time).
func (i *Injector) SetLights(objectID uint32, l Lights) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	o, ok := i.objects[objectID]
	if !ok || !o.taken {
		return ErrNotInjected
	}
	cur, first := o.lights, !o.lightsSent
	for _, x := range []struct {
		evt       int
		want, has bool
	}{
		{injEvtNav, l.Nav, cur.Nav}, {injEvtBeacon, l.Beacon, cur.Beacon}, {injEvtStrobe, l.Strobe, cur.Strobe},
		{injEvtTaxi, l.Taxi, cur.Taxi}, {injEvtLanding, l.Landing, cur.Landing}, {injEvtLogo, l.Logo, cur.Logo},
		{injEvtWing, l.Wing, cur.Wing},
	} {
		if first || x.want != x.has {
			if err := i.event(objectID, x.evt, x.want); err != nil {
				return err
			}
		}
	}
	o.lights, o.lightsSent = l, true
	return nil
}

// Release unfreezes objectID and stops driving (or watching) it. The aircraft stays where
// it is; give it waypoints to hand it back to MSFS AI.
func (i *Injector) Release(objectID uint32) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	o, ok := i.objects[objectID]
	if !ok {
		return ErrNotInjected
	}
	req := i.reqBase + 2*uint32(o.slot)
	delete(i.objects, objectID)
	delete(i.byRequest, req+1)
	i.slots[o.slot] = false
	var first error
	keep := func(err error) {
		if first == nil {
			first = err
		}
	}
	keep(i.track(fmt.Sprintf("stop ground height object %d", objectID),
		i.client.RequestDataOnSimObject(req+1, i.defBase+injDefGround, objectID, types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)))
	for _, e := range []int{injEvtFreezeLatLon, injEvtFreezeAlt, injEvtFreezeAtt} {
		if !o.taken {
			break // only watched: nothing to unfreeze
		}
		keep(i.event(objectID, e, false))
	}
	return first
}

// Forget drops objectID without sending anything, for an aircraft that was
// removed from the sim.
func (i *Injector) Forget(objectID uint32) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; ok {
		delete(i.byRequest, i.reqBase+2*uint32(o.slot)+1)
		i.slots[o.slot] = false
		delete(i.objects, objectID)
	}
}

// Handle consumes the injector's ground height data and reports exceptions
// caused by its calls as errors. It returns whether msg was the injector's.
func (i *Injector) Handle(msg engine.Message) (bool, error) {
	if msg.SIMCONNECT_RECV == nil {
		return false, nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
	case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
		d := msg.AsSimObjectData()
		obj, ok := i.byRequest[uint32(d.DwRequestID)]
		if !ok {
			return false, nil
		}
		if o := i.objects[obj]; o != nil {
			g := engine.CastDataAs[injectGround](&d.DwData)
			o.groundFt, o.cgFt, o.staticPitch, o.haveGround = g.GroundFt, g.CGFt, g.StaticPitch, true
			rest := g.PlaneFt - g.GroundFt
			if !o.placed && g.OnGround != 0 && g.GS < restMaxKts &&
				math.Abs(rest-g.CGFt) <= restMaxOffFt && math.Abs(g.PlanePitch-g.StaticPitch) <= restMaxOffPitch {
				o.restFt, o.restPitch, o.haveRest = rest, g.PlanePitch, true
			}
		}
		return true, nil
	case types.SIMCONNECT_RECV_ID_EXCEPTION:
		e := msg.AsException()
		call, ok := i.sent[uint32(e.DwSendID)]
		if !ok {
			return false, nil
		}
		return true, fmt.Errorf("traffic: SimConnect exception %d on %s (parameter %d)", e.DwException, call, e.DwIndex)
	}
	return false, nil
}

// String shows the lights as NBSTLOW, a dot for each light that is off:
// nav, beacon, strobe, taxi, landing, logo, wing.
func (l Lights) String() string {
	b := []byte(".......")
	for i, on := range []bool{l.Nav, l.Beacon, l.Strobe, l.Taxi, l.Landing, l.Logo, l.Wing} {
		if on {
			b[i] = "NBSTLOW"[i]
		}
	}
	return string(b)
}

// PlaceAir puts objectID at an approach pose: in the air with the main
// wheels pose.HeightFt above the runway (pose.RunwayFt; unknown, above the
// ground under it), pitched nose up pose.PitchDeg, or on the ground from
// touchdown. Below AirBlendFt the height eases onto the ground under the
// aircraft, so the wheels meet the surface there. Call it at InjectHz.
func (i *Injector) PlaceAir(objectID uint32, pose ApproachPose) error {
	i.mu.Lock()
	o, ok := i.objects[objectID]
	if !ok || !o.taken {
		i.mu.Unlock()
		return ErrNotInjected
	}
	if !o.haveGround {
		i.mu.Unlock()
		return ErrGroundUnknown
	}
	onGround := types.DWORD(0)
	if pose.OnGround {
		onGround = 1
	}
	p := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  pose.Position.Lat,
		Longitude: pose.Position.Lon,
		Altitude:  airAltitude(pose, o.groundFt) + o.cgFt,
		Pitch:     -pose.PitchDeg, // SimConnect: negative is nose up
		Bank:      -pose.BankDeg,  // assumed like the pitch (negative right wing down); check live
		Heading:   pose.Heading,
		OnGround:  onGround,
		Airspeed:  types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(pose.GroundSpeedKts),
	}
	o.placed = true
	i.mu.Unlock()
	return i.client.SetDataOnSimObject(i.defBase+injDefPosition, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
}

// AirBlendFt: below this height an injected aircraft in the air eases from
// the runway's elevation onto the ground under it.
const AirBlendFt = 100.0

// airAltitude is the main wheels' altitude MSL for a pose over ground at
// groundFt: the runway's elevation plus the height (a steady glide path,
// whatever the terrain below), easing onto the ground below AirBlendFt.
func airAltitude(pose ApproachPose, groundFt float64) float64 {
	h := math.Max(pose.HeightFt, 0)
	if pose.RunwayFt == 0 || pose.OnGround {
		return groundFt + h
	}
	w := math.Min(1, h/AirBlendFt)
	return w*(pose.RunwayFt+h) + (1-w)*(groundFt+h)
}

// GroundFt is the ground elevation under an injected object (feet MSL).
func (i *Injector) GroundFt(objectID uint32) (float64, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; ok && o.haveGround {
		return o.groundFt, true
	}
	return 0, false
}

// SetGear moves the gear handle of objectID; the sim animates the gear
// (about 4 s on an A320) even while the aircraft is frozen.
func (i *Injector) SetGear(objectID uint32, down bool) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; !ok || !o.taken {
		return ErrNotInjected
	}
	g := [1]float64{0}
	if down {
		g[0] = 1
	}
	return i.track(fmt.Sprintf("gear handle object %d", objectID),
		i.client.SetDataOnSimObject(i.defBase+injDefGear, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(g)), unsafe.Pointer(&g)))
}

// SetEngines starts or stops engines 1 to n of objectID (at most
// injMaxEngines): combustion on or off (GENERAL ENG COMBUSTION, settable;
// off also sets the RPM to 0).
func (i *Injector) SetEngines(objectID uint32, n int, on bool) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; !ok || !o.taken {
		return ErrNotInjected
	}
	v := [1]float64{0}
	if on {
		v[0] = 1
	}
	for k := 0; k < min(n, injMaxEngines); k++ {
		if err := i.track(fmt.Sprintf("engine %d object %d", k+1, objectID),
			i.client.SetDataOnSimObject(i.defBase+injDefEngine1+uint32(k), objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(v)), unsafe.Pointer(&v))); err != nil {
			return err
		}
	}
	return nil
}

// SetFlaps sets the flap surfaces of objectID to percent (0 up, 100 full);
// the surfaces move at once, so ramp percent over time for a visible
// extension or retraction.
func (i *Injector) SetFlaps(objectID uint32, percent float64) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; !ok || !o.taken {
		return ErrNotInjected
	}
	s := [4]float64{percent, percent, percent, percent}
	// Not tracked: callers ramp it every frame.
	return i.client.SetDataOnSimObject(i.defBase+injDefFlaps, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(s)), unsafe.Pointer(&s))
}

// SetSpoilers sets the spoiler handle and surfaces of objectID to percent
// (100 ground spoilers fully out); ramp it for a visible movement.
func (i *Injector) SetSpoilers(objectID uint32, percent float64) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; !ok || !o.taken {
		return ErrNotInjected
	}
	s := [3]float64{percent, percent, percent}
	return i.client.SetDataOnSimObject(i.defBase+injDefSpoilers, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(s)), unsafe.Pointer(&s))
}
