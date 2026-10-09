//go:build windows

package flight

import (
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ReplayClient is what a UserReplay needs of a connection.
type ReplayClient interface {
	MapClientEventToSimEvent(eventID uint32, eventName string) error
	TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error
}

// DefaultReplayBase is the first data definition and client event ID a
// UserReplay takes (a block of 64).
const DefaultReplayBase uint32 = 0x7D00

// UserReplay flies the user aircraft as a Track was flown (#963): frozen
// (position, altitude, attitude), placed each frame where the sample says,
// its gear, flap lever, spoilers, lights, throttles and control surfaces
// moved as they were. Nothing else of the aircraft is touched; Stop frees it.
type UserReplay struct {
	client ReplayClient
	base   uint32

	mu      sync.Mutex
	defined bool
	events  map[string]uint32
	next    uint32
	last    *Sample
}

// NewUserReplay returns a UserReplay on client, its IDs from base
// (DefaultReplayBase when 0).
func NewUserReplay(client ReplayClient, base uint32) *UserReplay {
	if base == 0 {
		base = DefaultReplayBase
	}
	return &UserReplay{client: client, base: base, events: map[string]uint32{}, next: 1}
}

// userPose is the user aircraft's position and attitude as written.
type userPose struct {
	Lat, Lon, Alt, Pitch, Bank, Heading float64
}

var userPoseVars = []struct{ name, unit string }{
	{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALTITUDE", "feet"},
	{"PLANE PITCH DEGREES", "degrees"}, {"PLANE BANK DEGREES", "degrees"}, {"PLANE HEADING DEGREES TRUE", "degrees"},
}

// Start freezes the user aircraft for the replay.
func (u *UserReplay) Start() error {
	for _, e := range []string{"FREEZE_LATITUDE_LONGITUDE_SET", "FREEZE_ALTITUDE_SET", "FREEZE_ATTITUDE_SET"} {
		if err := u.event(e, 1); err != nil {
			return err
		}
	}
	u.mu.Lock()
	u.last = nil
	u.mu.Unlock()
	return nil
}

// Stop frees the user aircraft where the replay left it.
func (u *UserReplay) Stop() error {
	for _, e := range []string{"FREEZE_LATITUDE_LONGITUDE_SET", "FREEZE_ALTITUDE_SET", "FREEZE_ATTITUDE_SET"} {
		if err := u.event(e, 0); err != nil {
			return err
		}
	}
	return nil
}

// Apply puts the user aircraft as s says: placed every call, the rest sent
// only as it changes.
func (u *UserReplay) Apply(s Sample) error {
	if err := u.place(s); err != nil {
		return err
	}
	u.mu.Lock()
	last := u.last
	u.last = &s
	u.mu.Unlock()
	return applyChanges(last, s, u.event)
}

func (u *UserReplay) place(s Sample) error {
	u.mu.Lock()
	defined := u.defined
	u.defined = true
	u.mu.Unlock()
	if !defined {
		for i, v := range userPoseVars {
			if err := u.client.AddToDataDefinition(u.base, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
				return fmt.Errorf("flight: define %s: %w", v.name, err)
			}
		}
	}
	p := userPose{Lat: s.Lat, Lon: s.Lon, Alt: s.AltFt, Pitch: -s.Pitch, Bank: -s.Bank, Heading: s.Heading}
	return u.client.SetDataOnSimObject(u.base, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
}

// event sends key event name to the user aircraft with data.
func (u *UserReplay) event(name string, data uint32) error {
	u.mu.Lock()
	id, ok := u.events[name]
	if !ok {
		if u.next >= 64 {
			u.mu.Unlock()
			return fmt.Errorf("flight: no event IDs left for %s", name)
		}
		id = u.base + u.next
		u.next++
		if err := u.client.MapClientEventToSimEvent(id, name); err != nil {
			u.mu.Unlock()
			return err
		}
		u.events[name] = id
	}
	u.mu.Unlock()
	return u.client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, id, data,
		types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY)
}

// Change thresholds: a lever moved less than this (percent) is not sent.
const replayLeverPct = 0.5

// axis is percent (−100…100 or 0…100) as an axis event's data.
func axis(pct float64) uint32 { return uint32(int32(math.Round(pct * 163.83))) }

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// applyChanges sends what changed from last (nil: everything) to s.
func applyChanges(last *Sample, s Sample, send func(string, uint32) error) error {
	moved := func(a, b float64) bool { return last == nil || math.Abs(a-b) >= replayLeverPct }
	var prev Sample
	if last != nil {
		prev = *last
	}
	type ev struct {
		name string
		data uint32
		do   bool
	}
	evs := []ev{
		{"GEAR_SET", b2u(s.GearHandle), last == nil || prev.GearHandle != s.GearHandle},
		{"FLAPS_SET", axis(s.FlapsHandle), moved(prev.FlapsHandle, s.FlapsHandle)},
		{"SPOILERS_SET", axis(s.Spoilers), moved(prev.Spoilers, s.Spoilers)},
		{"SPOILERS_ARM_SET", b2u(s.SpoilersArmed), last == nil || prev.SpoilersArmed != s.SpoilersArmed},
		// The control surfaces as moved (the elevator's axis sign as
		// systems.Elevator takes it: to verify live).
		{"AXIS_ELEVATOR_SET", axis(-s.Elevator), moved(prev.Elevator, s.Elevator)},
		{"AXIS_AILERONS_SET", axis(s.Aileron), moved(prev.Aileron, s.Aileron)},
		{"AXIS_RUDDER_SET", axis(s.Rudder), moved(prev.Rudder, s.Rudder)},
	}
	for _, l := range []struct {
		event string
		bit   int
	}{{"NAV_LIGHTS_SET", LightNav}, {"BEACON_LIGHTS_SET", LightBeacon}, {"LANDING_LIGHTS_SET", LightLanding}, {"TAXI_LIGHTS_SET", LightTaxi},
		{"STROBES_SET", LightStrobe}, {"LOGO_LIGHTS_SET", LightLogo}, {"WING_LIGHTS_SET", LightWing}} {
		on := s.Lights&l.bit != 0
		evs = append(evs, ev{l.event, b2u(on), last == nil || (prev.Lights&l.bit != 0) != on})
	}
	for i := range min(max(s.EngineCount, 1), Engines) {
		evs = append(evs, ev{fmt.Sprintf("AXIS_THROTTLE%d_SET", i+1), axis(s.Throttle[i]), moved(prev.Throttle[i], s.Throttle[i])})
	}
	for _, e := range evs {
		if !e.do {
			continue
		}
		if err := send(e.name, e.data); err != nil {
			return err
		}
	}
	return nil
}

// GhostInjector is what a Ghost needs of the traffic Injector.
type GhostInjector interface {
	PlaceFlown(objectID uint32, pose traffic.FlownPose) error
	SetGear(objectID uint32, down bool) error
	SetFlaps(objectID uint32, percent float64) error
	SetSpoilers(objectID uint32, percent float64) error
	SetLights(objectID uint32, l traffic.Lights) error
	SetEngines(objectID uint32, n int, on bool) error
	SetThrottle(objectID uint32, n int, percent float64) error
}

// Ghost flies an AI object as a Track was flown (#963): placed by the
// traffic Injector each frame (taken over already: Injector.Takeover), its
// gear, flaps, spoilers, lights, engines and throttles following. A model
// other than the one flown keeps its own wheels on the ground (PlaceFlown).
type Ghost struct {
	inj GhostInjector
	obj uint32

	mu      sync.Mutex
	last    *Sample
	running bool
}

// NewGhost returns a Ghost flying objectID through inj.
func NewGhost(inj GhostInjector, objectID uint32) *Ghost {
	return &Ghost{inj: inj, obj: objectID}
}

// ObjectID is the object the Ghost flies.
func (g *Ghost) ObjectID() uint32 { return g.obj }

// ghostRunningN1: an engine at or above this N1 (percent) is running.
const ghostRunningN1 = 15.0

// Apply puts the ghost as s says.
func (g *Ghost) Apply(s Sample) error {
	if err := g.inj.PlaceFlown(g.obj, traffic.FlownPose{Position: airport.LatLon{Lat: s.Lat, Lon: s.Lon}, AltFt: s.AltFt, CGFt: s.CGFt,
		PitchDeg: s.Pitch, BankDeg: s.Bank, Heading: s.Heading, OnGround: s.OnGround, GroundSpeedKts: s.GS}); err != nil {
		return err
	}
	g.mu.Lock()
	last := g.last
	g.last = &s
	running := g.running
	g.mu.Unlock()
	var prev Sample
	if last != nil {
		prev = *last
	}
	first := last == nil
	if first || prev.GearPct != s.GearPct || prev.GearHandle != s.GearHandle {
		if err := g.inj.SetGear(g.obj, s.GearHandle); err != nil {
			return err
		}
	}
	if first || math.Abs(prev.FlapsPct-s.FlapsPct) >= replayLeverPct {
		if err := g.inj.SetFlaps(g.obj, s.FlapsPct); err != nil {
			return err
		}
	}
	if first || math.Abs(prev.Spoilers-s.Spoilers) >= replayLeverPct {
		if err := g.inj.SetSpoilers(g.obj, s.Spoilers); err != nil {
			return err
		}
	}
	if first || prev.Lights != s.Lights {
		l := traffic.Lights{Nav: s.Lights&LightNav != 0, Beacon: s.Lights&LightBeacon != 0, Strobe: s.Lights&LightStrobe != 0,
			Taxi: s.Lights&LightTaxi != 0, Landing: s.Lights&LightLanding != 0, Logo: s.Lights&LightLogo != 0, Wing: s.Lights&LightWing != 0}
		if err := g.inj.SetLights(g.obj, l); err != nil {
			return err
		}
	}
	n := min(max(s.EngineCount, 1), Engines)
	on := false
	thr := 0.0
	for i := range n {
		on = on || s.N1[i] >= ghostRunningN1
		thr += s.Throttle[i] / float64(n)
	}
	if first || on != running {
		if err := g.inj.SetEngines(g.obj, n, on); err != nil {
			return err
		}
		g.mu.Lock()
		g.running = on
		g.mu.Unlock()
	}
	prevThr := 0.0
	for i := range n {
		prevThr += prev.Throttle[i] / float64(n)
	}
	if first || math.Abs(prevThr-thr) >= replayLeverPct {
		return g.inj.SetThrottle(g.obj, n, thr)
	}
	return nil
}

// GhostClient is what a GhostReplay needs of a connection: an AI object
// created and removed.
type GhostClient interface {
	AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AIRemoveObject(objectID uint32, requestID uint32) error
}

// GhostReplay replays a Track as an AI aircraft on its own: Start creates
// the object (title, tail) where the Player stands, Handle takes its object
// ID, has the Injector take it over and flies it every frame as the Player
// says (Play, Pause, Seek, SetRate), Stop removes it. reqID is its creation
// and removal request; inj is the host's own Injector (its IDs clear of
// any other's, NewInjector's options).
type GhostReplay struct {
	client GhostClient
	inj    *traffic.Injector
	player *Player
	title  string
	tail   string
	req    uint32

	mu      sync.Mutex
	obj     uint32
	ghost   *Ghost
	placed  time.Time
	stopped bool
}

// NewGhostReplay returns a ghost replay of track, its Player paused at the
// start.
func NewGhostReplay(client GhostClient, inj *traffic.Injector, track *Track, title, tail string, reqID uint32) *GhostReplay {
	return &GhostReplay{client: client, inj: inj, player: NewPlayer(track), title: title, tail: tail, req: reqID}
}

// Player is the replay's clock: Play, Pause, Seek, SetRate.
func (r *GhostReplay) Player() *Player { return r.player }

// ObjectID is the ghost's object, 0 before the simulator gave it.
func (r *GhostReplay) ObjectID() uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.obj
}

// Start creates the ghost where the Player stands.
func (r *GhostReplay) Start() error {
	s, _ := r.player.Sample(time.Now())
	init := types.SIMCONNECT_DATA_INITPOSITION{Latitude: s.Lat, Longitude: s.Lon, Altitude: s.AltFt, Pitch: -s.Pitch, Bank: -s.Bank,
		Heading: s.Heading, Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(s.GS))}
	if s.OnGround {
		init.OnGround = 1
	}
	tail := r.tail
	if tail == "" {
		tail = "GHOST"
	}
	return r.client.AICreateNonATCAircraft(r.title, tail, init, r.req)
}

// ghostFrame: the ghost is placed at most this often.
const ghostFrame = time.Second / 60

// Handle takes the ghost's object ID (true), then flies it as the Player
// says on the messages that come, at most every frame (false: the
// message is everyone's).
func (r *GhostReplay) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil {
		return false
	}
	r.mu.Lock()
	obj, stopped := r.obj, r.stopped
	r.mu.Unlock()
	if stopped {
		return false
	}
	if obj == 0 {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
			return false
		}
		m := msg.AsAssignedObjectID()
		if m == nil || uint32(m.DwRequestID) != r.req {
			return false
		}
		obj = uint32(m.DwObjectID)
		if err := r.inj.Takeover(obj); err != nil {
			return true
		}
		r.mu.Lock()
		r.obj, r.ghost = obj, NewGhost(r.inj, obj)
		r.mu.Unlock()
		return true
	}
	now := time.Now()
	r.mu.Lock()
	due := now.Sub(r.placed) >= ghostFrame
	if due {
		r.placed = now
	}
	g := r.ghost
	r.mu.Unlock()
	if due && g != nil {
		s, _ := r.player.Sample(now)
		_ = g.Apply(s)
	}
	return false
}

// Stop removes the ghost.
func (r *GhostReplay) Stop() error {
	r.mu.Lock()
	r.stopped = true
	obj := r.obj
	r.mu.Unlock()
	if obj == 0 {
		return nil
	}
	r.inj.Forget(obj)
	return r.client.AIRemoveObject(obj, r.req)
}
