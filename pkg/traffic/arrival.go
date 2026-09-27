//go:build windows
// +build windows

package traffic

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ErrNoTouchdown is reported when an arriving aircraft passes the runway end
// without touching down (for example with its gear up).
var ErrNoTouchdown = errors.New("traffic: aircraft did not touch down")

// ArrivalState is the phase of an arrival.
type ArrivalState uint8

const (
	ArrivalIdle        ArrivalState = iota // not started
	ArrivalSpawning                        // waiting for the simulator to create the aircraft
	ArrivalApproaching                     // on final
	ArrivalLanding                         // below LandingAGLFt, about to touch down
	ArrivalRollout                         // on the runway after touchdown
	ArrivalTaxiing                         // off the runway, taxiing to the stand
	ArrivalParking                         // on the stand's PARKING path
	ArrivalParked                          // stopped at the stand; the controller no longer moves it
	ArrivalCancelled                       // Cancel was called
	ArrivalFailed                          // an error ended the arrival
)

var arrivalStateNames = [...]string{"idle", "spawning", "approaching", "landing", "rollout", "taxiing", "parking", "parked", "cancelled", "failed"}

func (s ArrivalState) String() string {
	if int(s) < len(arrivalStateNames) {
		return arrivalStateNames[s]
	}
	return fmt.Sprintf("ArrivalState(%d)", s)
}

// Terminal reports whether the state ends the controller.
func (s ArrivalState) Terminal() bool { return s >= ArrivalParked }

// LandingAGLFt is the height below which an approaching aircraft counts as
// landing.
const LandingAGLFt = 200.0

// ArrivalRequest describes an arrival.
type ArrivalRequest struct {
	Graph   *airport.Graph
	Runway  string // runway end to land on, e.g. "24"
	Parking int    // stand parking index; see airport.Layout.ParkingIndex
	Model   string // aircraft container title
	Livery  string
	Tail    string
	// SpawnNm is how far out on final the aircraft appears; 0 means DefaultSpawnNm.
	SpawnNm float64
	// Exit forces a runway exit; nil chooses one for the required rollout.
	Exit *airport.RunwayExit
	// GroundAGL sends ground waypoints at 0 ft above ground instead of the
	// airport elevation (experimental; for sloped runways and taxiways).
	GroundAGL bool
	Options   airport.RouteOptions
}

// ArrivalEvent reports a state change or progress of an arrival.
type ArrivalEvent struct {
	State       ArrivalState
	ObjectID    uint32
	Position    airport.LatLon
	AGL         float64 // feet
	Heading     float64
	GroundSpeed float64 // knots
	OnGround    bool
	// Touchdown is the distance past the threshold where the aircraft
	// touched down, once it has.
	Touchdown float64
	// TouchdownFpm is the vertical speed just before touchdown (negative).
	TouchdownFpm float64
	// Remaining is the distance to the stand along the taxi-in route, in
	// meters, once on the ground.
	Remaining float64
	Taxiway   string
	Err       error
}

// ArrivalOption configures an ArrivalController.
type ArrivalOption func(*ArrivalController)

// ArrivalWithIDs sets the first data definition ID and request ID; an
// ArrivalController uses 3 definition IDs and 4 request IDs from them.
func ArrivalWithIDs(defBase, reqBase uint32) ArrivalOption {
	return func(c *ArrivalController) { c.defBase, c.reqBase = defBase, reqBase }
}

// ArrivalController lands one AI aircraft and taxis it to a stand: it spawns
// the aircraft on final, lowers its gear, and sends one waypoint chain from
// the approach through touchdown, rollout, runway exit and taxi-in to the
// stand (see PlanArrival).
//
// Like TaxiController it never reads the engine stream: pass every message to
// Handle and read progress from Events. A controller is single use.
type ArrivalController struct {
	mu      sync.Mutex
	fleet   *Fleet
	defBase uint32
	reqBase uint32
	events  chan ArrivalEvent
	now     func() time.Time

	req       ArrivalRequest
	plan      *ArrivalPlan
	state     ArrivalState
	objectID  uint32
	last      ArrivalEvent
	airborne  bool
	track     *routeTracker
	exitAlong float64 // route distance of the exit node (off the runway)
	stillFrom time.Time
	stopSent  bool
	lastVS    float64 // vertical speed at the last airborne report
	warned    bool
}

const (
	arrDefWaypoints = iota
	arrDefMonitor
	arrDefGear
)

const (
	arrReqSpawn = iota
	arrReqRelease
	arrReqMonitor
	arrReqRemove
)

type arrivalMonitor struct {
	Latitude  float64
	Longitude float64
	AGL       float64
	Heading   float64
	GroundKts float64
	OnGround  float64
	VS        float64
}

// NewArrivalController creates a controller that spawns its aircraft through
// fleet.
func NewArrivalController(fleet *Fleet, opts ...ArrivalOption) *ArrivalController {
	c := &ArrivalController{
		fleet: fleet, defBase: DefaultArrivalDefinitionBase, reqBase: DefaultArrivalRequestBase,
		events: make(chan ArrivalEvent, 256), now: time.Now,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Events returns state changes and progress; closed at a terminal state.
func (c *ArrivalController) Events() <-chan ArrivalEvent { return c.events }

// State returns the current state.
func (c *ArrivalController) State() ArrivalState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Plan returns the arrival plan (runway, exit, taxi-in route, waypoints),
// nil before Start.
func (c *ArrivalController) Plan() *ArrivalPlan {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.plan
}

// ObjectID returns the aircraft's SimConnect object ID, 0 before it exists.
func (c *ArrivalController) ObjectID() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.objectID
}

// Start plans the arrival and spawns the aircraft on final.
func (c *ArrivalController) Start(req ArrivalRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != ArrivalIdle {
		return ErrAlreadyStarted
	}
	if req.Graph == nil || req.Model == "" || req.Parking < 0 || req.Parking >= len(req.Graph.Layout.Parking) {
		return fmt.Errorf("%w: Graph, Model and a valid Parking are required", ErrBadTaxiRequest)
	}
	plan, err := PlanArrival(req.Graph, req.Runway, req.Parking, req.SpawnNm, req.Exit, req.Options, req.GroundAGL)
	if err != nil {
		return err
	}
	client := c.fleet.clientOrNil()
	if client == nil {
		return ErrNotConnected
	}
	if err := client.AddToDataDefinition(c.defBase+arrDefWaypoints, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0); err != nil {
		return err
	}
	for i, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"PLANE HEADING DEGREES TRUE", "degrees"}, {"GROUND VELOCITY", "knots"}, {"SIM ON GROUND", "bool"},
		{"VERTICAL SPEED", "feet per minute"},
	} {
		if err := client.AddToDataDefinition(c.defBase+arrDefMonitor, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
			return err
		}
	}
	if err := client.AddToDataDefinition(c.defBase+arrDefGear, "GEAR HANDLE POSITION", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
		return err
	}
	if err := c.fleet.RequestNonATC(NonATCOpts{Model: req.Model, Livery: req.Livery, Tail: req.Tail, Position: plan.Spawn}, c.reqBase+arrReqSpawn); err != nil {
		return fmt.Errorf("%w: %v", ErrCreationFailed, err)
	}
	c.req, c.plan = req, plan
	c.track = newRouteTracker(plan.Route)
	c.exitAlong = c.track.cum[len(plan.Exit.Path)-1]
	c.setState(ArrivalSpawning, nil)
	return nil
}

// Handle processes one message and reports whether it belonged to this
// controller.
func (c *ArrivalController) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == ArrivalIdle || c.state.Terminal() {
		return false
	}
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
	case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
		m := msg.AsAssignedObjectID()
		if uint32(m.DwRequestID) != c.reqBase+arrReqSpawn || c.state != ArrivalSpawning {
			return false
		}
		c.onSpawned(uint32(m.DwObjectID))
		return true
	case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
		m := msg.AsSimObjectData()
		if uint32(m.DwRequestID) != c.reqBase+arrReqMonitor || uint32(m.DwObjectID) != c.objectID {
			return false
		}
		c.onPosition(*engine.CastDataAs[arrivalMonitor](&m.DwData))
		return true
	}
	return false
}

// onSpawned releases the aircraft to our control, lowers its gear and sends
// the full waypoint chain.
func (c *ArrivalController) onSpawned(objectID uint32) {
	c.objectID = objectID
	c.fleet.Acknowledge(c.reqBase+arrReqSpawn, objectID)
	client := c.fleet.clientOrNil()
	if client == nil {
		c.fail(ErrNotConnected)
		return
	}
	if err := c.fleet.ReleaseControl(objectID, c.reqBase+arrReqRelease); err != nil {
		c.fail(err)
		return
	}
	// Without the gear handle down MSFS AI never touches down (#301).
	gear := [1]float64{1}
	if err := client.SetDataOnSimObject(c.defBase+arrDefGear, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(gear)), unsafe.Pointer(&gear)); err != nil {
		c.fail(err)
		return
	}
	if err := c.fleet.SetWaypoints(objectID, c.defBase+arrDefWaypoints, c.plan.Waypoints); err != nil {
		c.fail(err)
		return
	}
	if err := client.RequestDataOnSimObject(c.reqBase+arrReqMonitor, c.defBase+arrDefMonitor, objectID,
		types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
		c.fail(err)
		return
	}
	c.stillFrom = c.now()
	c.setState(ArrivalApproaching, nil)
}

func (c *ArrivalController) onPosition(m arrivalMonitor) {
	pos := airport.LatLon{Lat: m.Latitude, Lon: m.Longitude}
	c.last.Position, c.last.AGL, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pos, m.AGL, m.Heading, m.GroundKts, m.OnGround != 0
	t := c.plan.End.Threshold
	past := calc.AlongTrackMeters(t.Lat, t.Lon, c.plan.Runway.Center.Lat, c.plan.Runway.Center.Lon, m.Latitude, m.Longitude)

	if m.OnGround == 0 {
		c.lastVS = m.VS
		c.airborne = true
	}
	if m.GroundKts >= StoppedKts {
		c.stillFrom, c.warned = c.now(), false
	}
	off := math.Abs(calc.CrossTrackMeters(c.plan.Runway.Primary.Threshold.Lat, c.plan.Runway.Primary.Threshold.Lon,
		c.plan.Runway.Secondary.Threshold.Lat, c.plan.Runway.Secondary.Threshold.Lon, m.Latitude, m.Longitude))
	if c.state >= ArrivalTaxiing {
		seg, along := c.track.advance(pos)
		c.last.Remaining = math.Max(0, c.track.total()-along)
		c.last.Taxiway = c.track.taxiwayAt(seg)
	}

	switch c.state {
	case ArrivalApproaching:
		if m.AGL < LandingAGLFt {
			c.setState(ArrivalLanding, nil)
			return
		}
	case ArrivalLanding:
		switch {
		case m.OnGround != 0 && c.airborne:
			c.last.Touchdown, c.last.TouchdownFpm = past, c.lastVS
			c.setState(ArrivalRollout, nil)
			return
		case past > c.plan.Runway.Length+300:
			c.fail(ErrNoTouchdown)
			return
		}
	case ArrivalRollout:
		// Taxiing once off the runway surface, near or past the planned exit.
		if off > c.plan.Runway.Width/2+RunwayClearMeters && past > c.plan.Exit.Along-100 {
			c.track.pos = c.exitAlong
			c.setState(ArrivalTaxiing, nil)
			return
		}
	case ArrivalTaxiing:
		if c.last.Remaining <= c.track.lastSegment()+5 {
			c.setState(ArrivalParking, nil)
			return
		}
	case ArrivalParking:
		stand := c.plan.Route.Points[len(c.plan.Route.Points)-1]
		d := calc.HaversineMeters(stand.Lat, stand.Lon, m.Latitude, m.Longitude)
		// Stop on the mark: at (or past) the stand, replace the chain with a
		// single waypoint where the aircraft is, so it does not roll on
		// towards the overshoot point.
		if !c.stopSent && (c.last.Remaining <= StandStopMeters || d <= StandStopMeters) {
			c.stopSent = true
			stop := []types.SIMCONNECT_DATA_WAYPOINT{groundAlt{feet: c.plan.Spawn.Altitude, agl: true}.waypoint(m.Latitude, m.Longitude, 0)}
			if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, stop); err != nil {
				c.emit(err, true)
			}
		}
		if m.GroundKts < StoppedKts && (d <= ParkedMeters || c.now().Sub(c.stillFrom) > 10*time.Second) {
			c.stopMonitor()
			if d > ParkedMeters {
				c.last.Err = fmt.Errorf("traffic: stopped %.0f m from the stand", d)
			}
			c.setState(ArrivalParked, c.last.Err)
			return
		}
	}
	if c.state >= ArrivalRollout && !c.warned && c.now().Sub(c.stillFrom) > StuckTimeout {
		c.warned = true
		c.emit(ErrTaxiStuck, true)
		return
	}
	c.emit(nil, false)
}

// Cancel removes the aircraft (if it exists) and ends the controller.
func (c *ArrivalController) Cancel() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Terminal() {
		return nil
	}
	var err error
	if c.objectID != 0 {
		c.stopMonitor()
		err = c.fleet.Remove(c.objectID, c.reqBase+arrReqRemove)
	}
	c.setState(ArrivalCancelled, nil)
	return err
}

func (c *ArrivalController) stopMonitor() {
	if client := c.fleet.clientOrNil(); client != nil && c.objectID != 0 {
		client.RequestDataOnSimObject(c.reqBase+arrReqMonitor, c.defBase+arrDefMonitor, c.objectID,
			types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	}
}

func (c *ArrivalController) fail(err error) {
	c.stopMonitor()
	c.setState(ArrivalFailed, err)
}

func (c *ArrivalController) setState(s ArrivalState, err error) {
	c.state = s
	c.emit(err, true)
	if s.Terminal() {
		close(c.events)
	}
}

func (c *ArrivalController) emit(err error, important bool) {
	ev := c.last
	ev.State, ev.ObjectID, ev.Err = c.state, c.objectID, err
	if important || len(c.events) < cap(c.events)-16 {
		select {
		case c.events <- ev:
		default:
		}
	}
}

// Route tracking window: a position is matched only to route segments
// between trackBehind meters behind and trackAhead meters ahead of the
// progress so far, so a route that doubles back or crosses a runway the
// aircraft is still on cannot make progress jump.
const (
	trackBehind = 30.0
	trackAhead  = 250.0
)

// routeTracker projects positions onto a route polyline with monotonic
// progress.
type routeTracker struct {
	route *airport.Route
	cum   []float64 // cumulative distance at each route point
	pos   float64   // progress so far, meters along the route
}

// advance matches p to the route within the tracking window and returns the
// segment and the (never decreasing) progress along the route.
func (t *routeTracker) advance(p airport.LatLon) (seg int, along float64) {
	pts := t.route.Points
	best := math.Inf(1)
	seg, along = -1, t.pos
	for i := 1; i < len(pts); i++ {
		if t.cum[i] < t.pos-trackBehind || t.cum[i-1] > t.pos+trackAhead {
			continue
		}
		ax, az := meters(p, pts[i-1])
		bx, bz := meters(p, pts[i])
		dx, dz := bx-ax, bz-az
		l2 := dx*dx + dz*dz
		f := 0.0
		if l2 > 0 {
			f = math.Max(0, math.Min(1, -(ax*dx+az*dz)/l2))
		}
		cx, cz := ax+dx*f, az+dz*f
		if d := cx*cx + cz*cz; d < best {
			best, seg, along = d, i-1, t.cum[i-1]+f*(t.cum[i]-t.cum[i-1])
		}
	}
	if seg < 0 {
		seg = t.segmentAt(t.pos)
	}
	if along > t.pos {
		t.pos = along
	}
	return seg, t.pos
}

// segmentAt returns the segment containing route distance d.
func (t *routeTracker) segmentAt(d float64) int {
	for i := 1; i < len(t.cum); i++ {
		if t.cum[i] >= d {
			return i - 1
		}
	}
	return len(t.cum) - 2
}

func newRouteTracker(r *airport.Route) *routeTracker {
	t := &routeTracker{route: r, cum: make([]float64, len(r.Points))}
	for i := 1; i < len(r.Points); i++ {
		a, b := r.Points[i-1], r.Points[i]
		t.cum[i] = t.cum[i-1] + calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
	}
	return t
}

func (t *routeTracker) total() float64 { return t.cum[len(t.cum)-1] }

// lastSegment is the length of the route's final segment (the PARKING path).
func (t *routeTracker) lastSegment() float64 {
	n := len(t.cum)
	return t.cum[n-1] - t.cum[n-2]
}

// taxiwayAt returns the name of segment seg or the nearest named one before it.
func (t *routeTracker) taxiwayAt(seg int) string {
	for i := min(seg, len(t.route.Edges)-1); i >= 0; i-- {
		if n := t.route.Edges[i].Name; n != "" {
			return n
		}
	}
	return ""
}
