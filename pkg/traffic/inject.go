//go:build windows
// +build windows

package traffic

import (
	"fmt"
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
	haveGround     bool
	lights         Lights
	lightsSent     bool
}

// Injector definition, request and event offsets.
const (
	injDefPosition = iota
	injDefGround
)

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

type injectGround struct{ GroundFt, CGFt float64 }

// InjectorOption configures an Injector.
type InjectorOption func(*Injector)

// InjectorWithIDs sets the SimConnect ID bases: 2 definition IDs, 2 request
// IDs per aircraft (up to 50 aircraft) and 10 event IDs are used.
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
	for k, v := range []string{"GROUND ALTITUDE", "STATIC CG TO GROUND"} {
		if err := i.track("define "+v, c.AddToDataDefinition(i.defBase+injDefGround, v, "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(k))); err != nil {
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
	if _, ok := i.objects[objectID]; ok {
		return nil
	}
	if err := i.register(); err != nil {
		return err
	}
	slot := -1
	for k, used := range i.slots {
		if !used {
			slot = k
			break
		}
	}
	if slot < 0 {
		return ErrInjectorFull
	}
	req := i.reqBase + 2*uint32(slot)
	if err := i.track(fmt.Sprintf("AIReleaseControl object %d", objectID), i.client.AIReleaseControl(objectID, req)); err != nil {
		return err
	}
	for _, e := range []int{injEvtFreezeLatLon, injEvtFreezeAlt, injEvtFreezeAtt} {
		if err := i.event(objectID, e, true); err != nil {
			return err
		}
	}
	if err := i.track(fmt.Sprintf("request ground height object %d", objectID),
		i.client.RequestDataOnSimObject(req+1, i.defBase+injDefGround, objectID, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)); err != nil {
		return err
	}
	i.slots[slot] = true
	i.objects[objectID] = &injected{slot: slot}
	i.byRequest[req+1] = objectID
	return nil
}

// Driven reports whether objectID is driven by the injector.
func (i *Injector) Driven(objectID uint32) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, ok := i.objects[objectID]
	return ok
}

// Place puts objectID at pose, on the ground. Call it at InjectHz. It
// returns ErrGroundUnknown until the first ground height has arrived.
func (i *Injector) Place(objectID uint32, pose GroundPose) error {
	i.mu.Lock()
	o, ok := i.objects[objectID]
	if !ok {
		i.mu.Unlock()
		return ErrNotInjected
	}
	if !o.haveGround {
		i.mu.Unlock()
		return ErrGroundUnknown
	}
	p := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  pose.Position.Lat,
		Longitude: pose.Position.Lon,
		Altitude:  o.groundFt + o.cgFt,
		Heading:   pose.Heading,
		OnGround:  1,
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
	if !ok {
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

// Release unfreezes objectID and stops driving it. The aircraft stays where
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
			o.groundFt, o.cgFt, o.haveGround = g.GroundFt, g.CGFt, true
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
