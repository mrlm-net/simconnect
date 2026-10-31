package traffic

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
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
	now              func() time.Time // the wall clock (tests set it)
	client           engine.Client
	defBase, reqBase uint32
	evtBase          uint32
	registered       bool
	mu               sync.Mutex
	objects          map[uint32]*injected // by object ID
	byRequest        map[uint32]uint32    // ground request ID → object ID
	slots            [injectMaxAircraft]bool
	sent             map[uint32]string
	// rests: by aircraft title, how a model rests standing (CG height,
	// pitch) as last seen before we placed one: for one not seen standing
	// (an arrival, created in the air), so it is placed at its rest pitch
	// too, not on extended struts (MovingPitchDeg).
	rests map[string][2]float64
	// gearPartial: by aircraft title, models that refuse the gear's own
	// positions (HoldGearDown): the handle and the nose gear only.
	gearPartial map[string]bool
	// ground: the ground elevation seen under our aircraft, by position.
	ground groundMap
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
	// title: the aircraft title (SetModel); onGroundRun: our placements on
	// the ground in a row.
	title       string
	placedAt    time.Time // when it was last placed on the ground (place)
	onGroundRun int
	// cgShown: the height above the ground last sent (its rest or static
	// CG term), at cgShownAt: a new value blends in (smoothCG).
	cgShown   float64
	cgShownAt time.Time
	// sentGround: the on-ground flag last sent (-1 none yet). With the flag
	// set the simulator puts the wheels on the ground at its own height,
	// whatever we send: a change of the flag blends (smoothCG).
	sentGround int8
	sent       lastSent // the last placement sent (traceJump)
	simStep    float64  // the simulator's step behind the coming placement, seconds (noteSimStep)
	// groundShownFt: the ground height it was last placed on, at
	// groundShownPos and groundShownAt (groundShown).
	groundShownFt  float64
	groundShownPos airport.LatLon
	groundShownAt  time.Time
	// gearSends: HoldGearDown writes tracked for an exception (the first
	// few); gearPartial: the model refused the gear's positions.
	gearSends   int
	gearPartial bool
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

// injDefGearDown: the gear handle and the gear's positions, written
// together by HoldGearDown (after the engines' definitions).
const injDefGearDown = injDefEngine1 + injMaxEngines

// injDefThrottle1: GENERAL ENG THROTTLE LEVER POSITION:1, one definition
// per engine up to injMaxEngines (SetThrottle).
const injDefThrottle1 = injDefGearDown + 1

// injDefGearDownPartial: the gear handle and the nose gear's position, for
// a model that refuses the main gears' (HoldGearDown).
const injDefGearDownPartial = injDefThrottle1 + injMaxEngines

// gearTrackSends: the first HoldGearDown writes of an aircraft watched for
// an exception (it is written every frame on the take-off roll).
const gearTrackSends = 3

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
	Lat, Lon                          float64 // where the ground was sampled
}

// Rest limits: a measured rest further than this from the static values is
// not trusted, and only an aircraft slower than restMaxKts is at rest
// (braking or turning, it pitches).
const (
	restMaxOffFt    = 3.0
	restMaxOffPitch = 3.0
	restMaxKts      = 1.0
)

// With its gear down the simulator draws an aircraft one of two ways: at
// exactly its rest pitch on compressed struts, at its rest height; at any
// other pitch, however little, on extended struts, a share higher and the
// nose up, never shown lower (measured live: a B738 at +0.05° 8.48 → 9.05
// ft, at +1° 9.75; an A320 12.25 → 12.85 with the gear held down and
// placed lower, 2026-10-14). The switch is in one frame: an aircraft
// popped up as the nose lifted at rotation and dropped as the nose wheel
// came down after landing (+0.5 to 0.65 ft, -0.5 to 0.8 ft), whatever
// height it was sent at.
//
// On extended struts, though, a frozen aircraft rests on its nose wheel
// alone, the main gear in the air (live 2026-10-20, GEAR IS ON GROUND:
// B738 at +1° 9.72 ft nose only; A320 at +0.05°, +0.1°, +0.2° and -0.1°
// 12.9 to 13.0 ft nose only; at the static pitch 12.25 ft on all three,
// whatever height was sent). So on the ground our aircraft are placed at
// exactly their static pitch, on compressed struts, never switching while
// they stand or taxi; the struts extend as the nose lifts at rotation and
// compress as it comes down after landing. MovingPitchDeg (once 1°, #676)
// is no longer added.
const MovingPitchDeg = 0.0

// movingPitch is the nose-down pitch (SimConnect: positive) an aircraft on
// the ground is given on top of its rest pitch: the same standing and
// moving, so it never switches struts.
func movingPitch(float64) float64 { return MovingPitchDeg }

// offRest keeps a pitch sent (SimConnect's convention) at least
// offRestDeg from the aircraft's rest pitches: at one it is drawn on
// compressed struts for that frame (a pitch passing through it in a
// rotation or a flare).
func offRest(pitch float64, rests ...float64) float64 {
	for _, r := range rests {
		if d := pitch - r; math.Abs(d) < offRestDeg {
			if d < 0 {
				return r - offRestDeg
			}
			return r + offRestDeg
		}
	}
	return pitch
}

// offRestDeg: a pitch this close to the rest pitch is moved off it (0.05°
// off was enough for the extended struts, live).
const offRestDeg = 0.05

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
		client:      client,
		now:         Now,
		defBase:     DefaultInjectDefinitionBase,
		reqBase:     DefaultInjectRequestBase,
		evtBase:     DefaultInjectEventBase,
		objects:     map[uint32]*injected{},
		byRequest:   map[uint32]uint32{},
		sent:        map[uint32]string{},
		rests:       map[string][2]float64{},
		gearPartial: map[string]bool{},
	}
	for _, o := range opts {
		o(i)
	}
	return i
}

// track remembers which call a send ID belongs to, for exception reports.
func (i *Injector) track(call string, err error) error {
	if id, e := i.client.GetLastSentPacketID(); e == nil {
		noteSent(i.sent, id, call)
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
		{"PLANE ALTITUDE", "feet"}, {"PLANE PITCH DEGREES", "degrees"}, {"SIM ON GROUND", "bool"}, {"GROUND VELOCITY", "knots"},
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}} {
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
	for k := 1; k <= injMaxEngines; k++ {
		v := fmt.Sprintf("GENERAL ENG THROTTLE LEVER POSITION:%d", k)
		if err := i.track("define "+v, c.AddToDataDefinition(i.defBase+injDefThrottle1+uint32(k-1), v, "percent", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)); err != nil {
			return err
		}
	}
	for k, v := range []string{"GEAR HANDLE POSITION", "GEAR CENTER POSITION", "GEAR LEFT POSITION", "GEAR RIGHT POSITION"} {
		unit := "percent over 100"
		if k == 0 {
			unit = "bool"
		}
		if err := i.track("define "+v, c.AddToDataDefinition(i.defBase+injDefGearDown, v, unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(k))); err != nil {
			return err
		}
		if k <= 1 { // the handle and the nose gear
			if err := i.track("define "+v, c.AddToDataDefinition(i.defBase+injDefGearDownPartial, v, unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(k))); err != nil {
				return err
			}
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
	o := &injected{slot: slot, sentGround: -1}
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

// PlaceRest puts objectID at pose standing on its gear for good (an
// arrival parked on its stand, not moved again): its static pitch and CG
// height, where the simulator settles it on all its wheels. Left on
// extended struts (Place, a degree off its rest pitch) the main gear hung
// in the air (live, B738s at 9.72 ft on the nose wheel alone; at the
// static pitch 8.40 ft on all three, whatever height was sent).
func (i *Injector) PlaceRest(objectID uint32, pose GroundPose) error {
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
	// Not learnt as its kind's rest (settled): taken there, a pitch off
	// MovingPitchDeg would be recorded for the next of its kind.
	o.placed, o.placedAt = true, time.Time{}
	o.cgShown, o.cgShownAt, o.sentGround = o.cgFt, i.now(), 1
	p := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  pose.Position.Lat,
		Longitude: pose.Position.Lon,
		Altitude:  i.groundShown(o, pose) + o.cgFt,
		Pitch:     o.staticPitch,
		Heading:   pose.Heading,
		OnGround:  1,
	}
	i.mu.Unlock()
	return i.client.SetDataOnSimObject(i.defBase+injDefPosition, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
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
	cg, pitch := o.cgFt, o.staticPitch                // resting on its gear, not level
	if r, ok := i.rests[o.title]; ok && !o.haveRest { // not seen standing: as its kind rests
		cg, pitch = r[0], r[1]
	}
	if o.haveRest {
		cg, pitch = o.restFt, o.restPitch
	}
	if !moving { // an aircraft (vehicles are placed moving): exactly its static pitch, on all its wheels
		pitch = o.staticPitch
		o.placedAt = i.now()
	}
	o.placed = true
	o.onGroundRun++
	// Rolling: the static CG height, the height the simulator itself puts a
	// moving aircraft with the on-ground flag at. Whether it holds the wheels
	// there or takes ours, the same height: nothing to pop (live, sent its
	// rest it switched between the two, ±0.3 to 0.6 ft). Standing: its rest
	// (the struts settled), blended in.
	if pose.GroundSpeedKts > restMaxKts {
		cg = o.cgFt
	}
	// Just off the air (a landing handed to the ground), the flag held off
	// until the wheels are blended down: set at once, the simulator snapped
	// them there (live, BAW1454 -0.8 ft in a frame).
	flag := types.DWORD(1)
	if o.sentGround == 0 && math.Abs(o.cgShown-cg) > groundFlagWithinFt {
		flag = 0
	}
	cg = i.smoothCG(o, cg, flag)
	p := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  pose.Position.Lat,
		Longitude: pose.Position.Lon,
		Altitude:  i.groundShown(o, pose) + cg,
		Pitch:     pitch,
		Heading:   pose.Heading,
		OnGround:  flag,
	}
	// The speed passed on, so the wheels turn (vehicles and aircraft alike).
	p.Airspeed = types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(math.Max(0, pose.GroundSpeedKts)))
	i.traceJump(objectID, o, "ground", pose.Position, pose.Heading, pose.GroundSpeedKts)
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
		if uint32(d.DwObjectID) != obj {
			return true, nil // the slot's former aircraft, still answering (#91)
		}
		if o := i.objects[obj]; o != nil {
			g := engine.CastDataAs[injectGround](&d.DwData)
			o.groundFt, o.cgFt, o.staticPitch, o.haveGround = g.GroundFt, g.CGFt, g.StaticPitch, true
			i.ground.put(airport.LatLon{Lat: g.Lat, Lon: g.Lon}, g.GroundFt)
			rest := g.PlaneFt - g.GroundFt
			// Placed and left alone a while (standing, placed at the standing
			// rate): where the simulator settled it is its rest, not where it
			// was first seen (frozen at its static height as created); placed
			// there, it no longer sinks 0.3 ft as it stops nor rises as it
			// starts (live, RYR852 8.48 → 8.13 ft).
			settled := o.placed && !o.placedAt.IsZero() && i.now().Sub(o.placedAt) >= restSettleAfter
			if (!o.placed || settled) && g.OnGround != 0 && g.GS < restMaxKts &&
				math.Abs(rest-g.CGFt) <= restMaxOffFt && math.Abs(g.PlanePitch-g.StaticPitch) <= restMaxOffPitch {
				// Placed by us on extended struts (MovingPitchDeg): its height, and
				// the pitch it rested at before.
				// Not seen standing before (an arrival): our offset taken off
				// (live, BAW1837 holding short 1.80° → 2.80°).
				pitch := g.PlanePitch
				switch {
				case o.placed && o.haveRest:
					pitch = o.restPitch
				case o.placed:
					pitch -= MovingPitchDeg
				}
				o.restFt, o.restPitch, o.haveRest = rest, pitch, true
				if o.title != "" {
					i.rests[o.title] = [2]float64{rest, pitch}
				}
			}
		}
		return true, nil
	case types.SIMCONNECT_RECV_ID_EXCEPTION:
		e := msg.AsException()
		call, ok := i.sent[uint32(e.DwSendID)]
		if !ok {
			return false, nil
		}
		if id, err := strconv.ParseUint(strings.TrimPrefix(call, holdGearCall), 10, 32); strings.HasPrefix(call, holdGearCall) && err == nil {
			if o := i.objects[uint32(id)]; o != nil {
				if o.gearPartial {
					return true, nil // said once
				}
				o.gearPartial = true
				if o.title != "" {
					i.gearPartial[o.title] = true
				}
			}
			return true, fmt.Errorf("traffic: SimConnect exception %d on %s (parameter %d): its gear held down by the handle and nose gear only", e.DwException, call, e.DwIndex)
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
	// On the runway the aircraft rests on its gear as Place puts it (its
	// rest height and pitch), the take-off or landing pitch on top; the
	// base fades out over the first restFadeFt in the air. Placed level
	// with the static height instead, the nose came up as the take-off
	// roll began (live, UAE375's B77W looked to pop a wheelie).
	// Never on the ground under us (an arrival): its rest on extended gear
	// is above the static CG height (#854, live B738: 9.54 against 8.48 ft;
	// placed at the static height the wheels sank a foot into the runway in
	// the last half-second, and the simulator popped it up at touchdown).
	cg, base := o.cgFt*(1+RestAboveStaticShare), o.staticPitch
	if r, ok := i.rests[o.title]; ok && !o.haveRest { // not seen standing: as its kind rests
		cg, base = r[0], r[1]
	}
	if o.haveRest {
		cg, base = o.restFt, o.restPitch
	}
	// On the runway (a take-off roll, a rollout): the static CG height, as
	// the simulator itself shows a rolling aircraft with the on-ground flag
	// (live, 7.00 against 7.05, 12.06 against 12.07). It holds the wheels
	// there until the nose lifts and then takes ours: the same height,
	// nothing to pop at rotation (sent the rest it popped 0.35 to 0.68 ft).
	// In the air the height blends up from it (smoothCG).
	if pose.OnGround {
		cg = o.cgFt
		// Touching down, the flag not set yet: held off until the wheels are
		// blended down to it; set at once, the simulator snapped them there
		// (live, 9.00 → 8.48 ft in a frame).
		if o.sentGround == 0 && math.Abs(o.cgShown-cg) > groundFlagWithinFt {
			onGround = 0
		}
		if pose.Unpinned {
			onGround = 0 // a take-off roll: the height ours alone
		}
	}
	if pose.OnGround {
		o.onGroundRun++
	} else {
		o.onGroundRun = 0
	}
	w := 1.0
	if !pose.OnGround {
		w = math.Max(0, 1-math.Max(0, pose.HeightFt)/restFadeFt)
	}
	// On extended struts (MovingPitchDeg), faded with height like the rest
	// pitch (#854: on at the wheels and off the frame after, the nose jumped
	// about a degree at lift-off and touchdown), and with the nose up: gone
	// once pitched rollingFadeDeg up, so the flare keeps its full attitude
	// and the rotation lifts the nose from it smoothly. Never its rest pitch
	// on the way (offRest).
	rolling := w * movingPitch(pose.GroundSpeedKts) * math.Max(0, 1-math.Max(0, pose.PitchDeg)/rollingFadeDeg)
	p := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  pose.Position.Lat,
		Longitude: pose.Position.Lon,
		Altitude:  airAltitude(pose, i.groundUnder(o, pose.Position)) + i.smoothCG(o, o.cgFt+w*(cg-o.cgFt), onGround) + mainGearLift(pose),
		Pitch:     offRest(w*base+rolling-pose.PitchDeg, base, o.staticPitch), // SimConnect: negative is nose up
		Bank:      -pose.BankDeg,                                              // assumed like the pitch (negative right wing down); check live
		Heading:   pose.Heading,
		OnGround:  onGround,
		Airspeed:  types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(pose.GroundSpeedKts),
	}
	if pose.OnGround && pose.PitchDeg <= 0 { // wheels down, nose down: on all its wheels (MovingPitchDeg)
		p.Pitch = o.staticPitch
	}
	o.placed = true
	// Placed: no rest learnt from the samples of a roll (live, TVS495 on its
	// take-off roll learnt its own placements, 8.48 → 8.08 → 9.00 ft).
	o.placedAt = i.now()
	i.traceJump(objectID, o, "air", pose.Position, pose.Heading, pose.GroundSpeedKts)
	i.mu.Unlock()
	return i.client.SetDataOnSimObject(i.defBase+injDefPosition, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
}

// FlownPose is an aircraft as it was flown (a recorded flight's sample,
// another sim's aircraft): where its reference point was and its attitude,
// for PlaceFlown.
type FlownPose struct {
	Position airport.LatLon
	// AltFt is the reference point's altitude (PLANE ALTITUDE, feet MSL);
	// CGFt the flown aircraft's static CG height above the ground (0
	// unknown): the placed aircraft keeps its own wheels on the ground.
	AltFt, CGFt float64
	// PitchDeg nose up and BankDeg right wing down positive; Heading true.
	PitchDeg, BankDeg, Heading float64
	OnGround                   bool
	GroundSpeedKts             float64
}

// PlaceFlown places objectID as an aircraft was flown (#963): its altitude
// moved by the difference of the two aircraft's static CG heights, so a
// ghost of another model rolls on its own wheels, its attitude as flown.
// No ground height is needed.
func (i *Injector) PlaceFlown(objectID uint32, pose FlownPose) error {
	i.mu.Lock()
	o, ok := i.objects[objectID]
	if !ok || !o.taken {
		i.mu.Unlock()
		return ErrNotInjected
	}
	alt := pose.AltFt
	if pose.CGFt > 0 && o.cgFt > 0 {
		alt += o.cgFt - pose.CGFt
	}
	onGround := types.DWORD(0)
	if pose.OnGround {
		onGround = 1
		o.onGroundRun++
	} else {
		o.onGroundRun = 0
	}
	p := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  pose.Position.Lat,
		Longitude: pose.Position.Lon,
		Altitude:  alt,
		Pitch:     -pose.PitchDeg, // SimConnect: negative is nose up
		Bank:      -pose.BankDeg,  // as PlaceAir takes it
		Heading:   pose.Heading,
		OnGround:  onGround,
		Airspeed:  types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(math.Max(0, pose.GroundSpeedKts))),
	}
	o.placed = true
	o.placedAt = i.now()
	i.traceJump(objectID, o, "flown", pose.Position, pose.Heading, pose.GroundSpeedKts)
	i.mu.Unlock()
	return i.client.SetDataOnSimObject(i.defBase+injDefPosition, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
}

// AirBlendFt: below this height an injected aircraft in the air eases from
// the runway's elevation onto the ground under it.
const AirBlendFt = 100.0

// restFadeFt: in the air the rest height and pitch of the ground fade out
// over this height (PlaceAir), so lift-off and touchdown do not jump.
const restFadeFt = 50.0

// CGBlendFtPerSecond: a change of the height an aircraft rests at (its
// rest learnt as it settled, the static CG height on the runway against
// its rest on the apron) blends in at most this fast, rather than in one
// frame (live, LKPR: 0.4 to 0.5 ft pops holding short, at lift-off and
// as a landing rolled out).
const CGBlendFtPerSecond = 1.0

// mainGearLift is how far (feet) the reference point rises as pose's nose
// comes up turning on the main gear (ApproachPose.MainGearBehindM).
func mainGearLift(pose ApproachPose) float64 {
	if pose.MainGearBehindM <= 0 || pose.PitchDeg <= 0 {
		return 0
	}
	return pose.MainGearBehindM * math.Sin(pose.PitchDeg*math.Pi/180) / 0.3048
}

// groundFlagWithinFt: at touchdown the on-ground flag is set once the
// wheels are this close to where the simulator puts them.
const groundFlagWithinFt = 0.03

// smoothCG is target as sent now, with the on-ground flag onGround:
// blended from the last value sent (within a second) at
// CGBlendFtPerSecond. It never takes the height the simulator showed:
// following that, ours and its switched back and forth (live, ±0.6 ft).
// i.mu held.
func (i *Injector) smoothCG(o *injected, target float64, onGround types.DWORD) float64 {
	now := i.now()
	o.sentGround = 0
	if onGround != 0 {
		o.sentGround = 1
	}
	if o.cgShownAt.IsZero() || now.Sub(o.cgShownAt) > time.Second {
		o.cgShown = target
	} else {
		step := CGBlendFtPerSecond * now.Sub(o.cgShownAt).Seconds()
		o.cgShown += math.Max(-step, math.Min(step, target-o.cgShown))
	}
	o.cgShownAt = now
	return o.cgShown
}

// RestAboveStaticShare: a model not seen resting yet (rests: an arrival
// of a kind no departure stood as) is placed this share of its static CG
// height above it near the ground (PlaceAir), at its static pitch
// (the sim shows it a little above: A320 0.60 over 12.25 ft, B738 1.06
// over 8.48, measured with the moving pitch of #676).
const RestAboveStaticShare = 0.085

// SetModel tells the injector the title of objectID (after Takeover), so
// how the model rests (rests) serves the next of its kind.
func (i *Injector) SetModel(objectID uint32, title string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; ok {
		o.title = title
		if o.haveRest && title != "" { // seen standing before its title came
			i.rests[title] = [2]float64{o.restFt, o.restPitch}
		}
	}
}

// rollingFadeDeg: the rolling pitch (movingPitch) fades out as the nose
// comes up to this pitch, at rotation and in the flare.
const rollingFadeDeg = 3.0

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

// HoldGearDown puts objectID's gear down at once: the handle and the gear
// itself, no extension to watch. Placed in the air the first time, the
// simulator snaps the gear up; with the handle alone it then extends
// again over 4 s (live, KLM1700 at lift-off, 2026-10-03).
func (i *Injector) HoldGearDown(objectID uint32) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	o, ok := i.objects[objectID]
	if !ok || !o.taken {
		return ErrNotInjected
	}
	// A model refusing the main gears' positions (live, the FSLTL Phenom:
	// an exception every frame of the take-off roll) gets the handle and
	// the nose gear only.
	if o.gearPartial || i.gearPartial[o.title] && o.title != "" {
		g := [2]float64{1, 1}
		return i.client.SetDataOnSimObject(i.defBase+injDefGearDownPartial, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(g)), unsafe.Pointer(&g))
	}
	g := [4]float64{1, 1, 1, 1}
	err := i.client.SetDataOnSimObject(i.defBase+injDefGearDown, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(g)), unsafe.Pointer(&g))
	if o.gearSends < gearTrackSends {
		o.gearSends++
		err = i.track(fmt.Sprintf("%s%d", holdGearCall, objectID), err)
	}
	return err
}

// holdGearCall names a tracked HoldGearDown write, the object ID after it.
const holdGearCall = "hold gear down "

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

// SetThrottle sets the throttle of engines 1 to n of objectID (at most
// injMaxEngines) to percent: an injected aircraft's engines follow it
// (live, a B738 from N1 30 to 78 % in 12 s at 90 %), heard as they spool;
// left alone they idle, a take-off at idle sound.
func (i *Injector) SetThrottle(objectID uint32, n int, percent float64) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; !ok || !o.taken {
		return ErrNotInjected
	}
	v := [1]float64{percent}
	for k := 0; k < min(n, injMaxEngines); k++ {
		if err := i.track(fmt.Sprintf("throttle %d object %d", k+1, objectID),
			i.client.SetDataOnSimObject(i.defBase+injDefThrottle1+uint32(k), objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(v)), unsafe.Pointer(&v))); err != nil {
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

// Throttle of an injected aircraft by phase (SetThrottle): take-off
// thrust on the roll until MSFS AI flies it, approach power on the final,
// idle from the flare.
const (
	TakeoffThrottlePct  = 90.0
	ApproachThrottlePct = 45.0
)

// restSettleAfter: an aircraft on the ground not placed for this long (its
// standing rate) has been settled by the simulator: its rest is learnt
// again from what it shows.
const restSettleAfter = 250 * time.Millisecond
