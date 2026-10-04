package traffic

import (
	"errors"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// UserPushDriver pushes the user's aircraft back along a PushRoute with the
// simulator's own pushback (TOGGLE_PUSHBACK, KEY_TUG_HEADING): it starts the
// tug, steers it every few frames toward the route behind the aircraft, and
// stops it at the route's end. For an application that does the user's
// pushback itself (not GSX): it runs only while asked to.
type UserPushDriver struct {
	client                            engine.Client
	defID, reqID, evToggle, evHeading uint32

	mu       sync.Mutex
	set      bool // definitions and events registered
	route    []airport.LatLon
	facing   float64
	state    UserPushState
	started  bool    // the tug was asked to start
	lastHdg  float64 // the heading last sent (-1 none)
	onChange func(UserPushState)
}

// UserPushState is where a user push is.
type UserPushState string

const (
	UserPushIdle    UserPushState = "idle"
	UserPushPushing UserPushState = "pushing"
	UserPushDone    UserPushState = "done"
	UserPushAborted UserPushState = "aborted"
)

// DefaultUserPushBase is the first of the driver's four SimConnect IDs: the
// data definition, its request, and the two events (base…base+3).
const DefaultUserPushBase uint32 = 9600

// Steering: the aircraft aims its tail at the route this far behind the
// point it is nearest on; it is there within userPushEndMeters of the end.
const (
	userPushLookMeters = 12.0
	userPushEndMeters  = 2.5
	userPushSteerDeg   = 1.5 // a heading is sent again once it changed this much
)

// NewUserPushDriver makes a driver on client with its IDs from base (0:
// DefaultUserPushBase). onChange, if not nil, hears every state change.
func NewUserPushDriver(client engine.Client, base uint32, onChange func(UserPushState)) *UserPushDriver {
	if base == 0 {
		base = DefaultUserPushBase
	}
	return &UserPushDriver{client: client, defID: base, reqID: base + 1, evToggle: base + 2, evHeading: base + 3, state: UserPushIdle, lastHdg: -1, onChange: onChange}
}

type userPushData struct {
	Lat, Lon, Heading, PushState, OnGround float64
}

// Start pushes along r (its Points: the main gear's way back): the tug is
// started on the next frame and steered from there.
func (d *UserPushDriver) Start(r PushRoute) error {
	if len(r.Points) < 2 {
		return errors.New("traffic: user push: a route of two points or more")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state == UserPushPushing {
		return errors.New("traffic: user push: already pushing")
	}
	if !d.set {
		c := d.client
		for i, v := range []struct{ name, unit string }{
			{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE HEADING DEGREES TRUE", "degrees"},
			{"PUSHBACK STATE", "enum"}, {"SIM ON GROUND", "bool"},
		} {
			if err := c.AddToDataDefinition(d.defID, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
				return err
			}
		}
		if err := c.MapClientEventToSimEvent(d.evToggle, "TOGGLE_PUSHBACK"); err != nil {
			return err
		}
		if err := c.MapClientEventToSimEvent(d.evHeading, "KEY_TUG_HEADING"); err != nil {
			return err
		}
		d.set = true
	}
	d.route, d.facing, d.started, d.lastHdg = append([]airport.LatLon(nil), r.Points...), r.Facing, false, -1
	d.setState(UserPushPushing)
	// Every fourth frame: steering at about 15 Hz.
	return d.client.RequestDataOnSimObject(d.reqID, d.defID, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_VISUAL_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 4, 0)
}

// Abort stops the push where it is.
func (d *UserPushDriver) Abort() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state != UserPushPushing {
		return
	}
	d.stopLocked(UserPushAborted)
}

// State is where the push is.
func (d *UserPushDriver) State() UserPushState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

// Handle takes the driver's messages; true when msg was one.
func (d *UserPushDriver) Handle(msg engine.Message) bool {
	if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return false
	}
	m := msg.AsSimObjectData()
	if uint32(m.DwRequestID) != d.reqID {
		return false
	}
	v := engine.CastDataAs[userPushData](&m.DwData)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state != UserPushPushing {
		return true
	}
	d.step(airport.LatLon{Lat: v.Lat, Lon: v.Lon}, v.Heading, int(v.PushState))
	return true
}

// step steers from pos: the tug started once, the tail aimed at the route
// behind, stopped at the end. PUSHBACK STATE 3 is no pushback.
func (d *UserPushDriver) step(pos airport.LatLon, heading float64, pushState int) {
	if !d.started {
		d.started = true
		if pushState == 3 {
			d.event(d.evToggle, 0)
		}
		return
	}
	near := 0
	for i := range d.route {
		if localDist(pos, d.route[i]) < localDist(pos, d.route[near]) {
			near = i
		}
	}
	end := d.route[len(d.route)-1]
	if near == len(d.route)-1 && localDist(pos, end) < userPushEndMeters*4 || localDist(pos, end) < userPushEndMeters {
		d.stopLocked(UserPushDone)
		return
	}
	// The point userPushLookMeters further back along the route.
	target, left := end, userPushLookMeters
	for i := near; i+1 < len(d.route); i++ {
		seg := localDist(d.route[i], d.route[i+1])
		if seg >= left {
			f := left / seg
			a, b := d.route[i], d.route[i+1]
			target = airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: a.Lon + (b.Lon-a.Lon)*f}
			break
		}
		left -= seg
	}
	// Moving tail first: the nose points away from the target.
	want := math.Mod(localBearing(target, pos)+360, 360)
	if d.lastHdg < 0 || math.Abs(headingDiff(d.lastHdg, want)) >= userPushSteerDeg {
		d.lastHdg = want
		d.event(d.evHeading, uint32(want/360*4294967295))
	}
}

// stopLocked ends the push: the tug off, the frames no longer asked for.
func (d *UserPushDriver) stopLocked(s UserPushState) {
	if d.started {
		d.event(d.evToggle, 0)
	}
	_ = d.client.RequestDataOnSimObject(d.reqID, d.defID, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	d.setState(s)
}

func (d *UserPushDriver) event(id, data uint32) {
	_ = d.client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, id, data, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY)
}

func (d *UserPushDriver) setState(s UserPushState) {
	d.state = s
	if d.onChange != nil {
		go d.onChange(s)
	}
}
