//go:build windows
// +build windows

package traffic

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Taxi controller errors.
var (
	ErrAlreadyStarted  = errors.New("traffic: taxi controller already started")
	ErrNotHoldingShort = errors.New("traffic: aircraft is not holding short")
	ErrTaxiStuck       = errors.New("traffic: aircraft has not moved")
	ErrBadTaxiRequest  = errors.New("traffic: invalid taxi request")
)

// TaxiState is the phase of a departure taxi.
type TaxiState uint8

const (
	TaxiIdle             TaxiState = iota // not started
	TaxiSpawning                          // waiting for the simulator to create the aircraft
	TaxiAwaitingPushback                  // injected: on the stand, waiting for ClearPushback
	TaxiPushback                          // being pushed back from the stand
	TaxiAwaitingTaxi                      // injected: pushed back, waiting for ClearToTaxi
	TaxiTaxiing                           // taxiing to the hold-short point
	TaxiHoldingShort                      // stopped at a hold-short point: of the departure runway (ClearToLineUp, ClearForTakeoff) or of a crossing (ClearToCross)
	TaxiLiningUp                          // entering the runway
	TaxiLinedUp                           // injected: lined up and waiting for ClearForTakeoff
	TaxiDeparting                         // take-off roll and initial climb
	TaxiComplete                          // airborne; the controller no longer tracks the aircraft
	TaxiCancelled                         // Cancel was called
	TaxiFailed                            // an error ended the taxi
)

var taxiStateNames = [...]string{"idle", "spawning", "awaiting pushback", "pushback", "awaiting taxi", "taxiing", "holding short", "lining up", "lined up", "departing", "complete", "cancelled", "failed"}

func (s TaxiState) String() string {
	if int(s) < len(taxiStateNames) {
		return taxiStateNames[s]
	}
	return fmt.Sprintf("TaxiState(%d)", s)
}

// Terminal reports whether the state ends the controller.
func (s TaxiState) Terminal() bool { return s >= TaxiComplete }

// TaxiRequest describes a departure taxi.
type TaxiRequest struct {
	// Graph is the airport's taxi graph, e.g. from airport.Cache.Graph.
	Graph *airport.Graph
	// Parking is the stand's parking index; see airport.Layout.ParkingIndex.
	Parking int
	// Runway is the runway end to depart from, e.g. "24".
	Runway string
	// Model is the aircraft container title, e.g. "FSLTL A320 Air France SL".
	Model string
	// Livery is the livery folder name; "" selects the default livery.
	Livery string
	// Tail is the tail number or call sign shown on the aircraft.
	Tail string
	// Options control route selection.
	Options airport.RouteOptions
	// Entry is the runway entry taxiway assigned by ATC ("24 at B"); ""
	// departs from the full length (airport.Graph.RouteToRunwayEntry).
	Entry string

	// Injected departure (TaxiWithInjector): HoldForClearances stops at
	// every gate until its clearance — ClearPushback, ClearToTaxi,
	// ClearToCross, ClearToLineUp, ClearForTakeoff; without it each gate
	// clears itself after a short, varied wait. A clearance given before
	// its gate means no stop there.
	HoldForClearances bool
	// Profile is the ground motion; zero means DefaultMotionProfile.
	Profile MotionProfile
	// RollingTakeoffChance is the chance, without HoldForClearances, that
	// line-up and take-off are cleared together and the aircraft rolls
	// straight into the take-off; 0 means DefaultRollingTakeoffChance,
	// negative never.
	RollingTakeoffChance float64
	// NoseOffset is the distance from the aircraft reference point to its nose,
	// placing it on the stand (StandPoint); 0 means DefaultNoseOffsetMeters.
	NoseOffset float64
	// Takeoff is the take-off; zero means DefaultTakeoffProfile.
	Takeoff TakeoffProfile
}

// TaxiEvent reports a state change or progress of a departure taxi.
type TaxiEvent struct {
	State    TaxiState
	ObjectID uint32
	Position airport.LatLon
	// Heading is degrees true; GroundSpeed in knots.
	Heading     float64
	GroundSpeed float64
	OnGround    bool
	// Remaining is the distance to the hold-short point along the route, in
	// meters, while taxiing.
	Remaining float64
	// Taxiway is the name of the taxiway the aircraft is on, "" if unnamed.
	Taxiway string
	// HoldingShortOf names the runway while holding short.
	HoldingShortOf string
	// LimitNode is the clearance limit of a progressive taxi (ClearUpTo), -1
	// for none; AtLimit is set while the aircraft holds there.
	LimitNode airport.NodeID
	AtLimit   bool
	// HeightFt is the height above the runway during the take-off.
	HeightFt float64
	// Lights is the light state the sim reports.
	Lights Lights
	// Err is set for TaxiFailed and for non-fatal warnings such as ErrTaxiStuck.
	Err error
}

// TaxiOption configures a TaxiController.
type TaxiOption func(*TaxiController)

// TaxiWithIDs sets the first data definition ID and request ID. A controller
// uses 2 definition IDs and 4 request IDs from these bases.
func TaxiWithIDs(defBase, reqBase uint32) TaxiOption {
	return func(c *TaxiController) { c.defBase, c.reqBase = defBase, reqBase }
}

// TaxiController drives one AI aircraft from a stand to a runway: spawn,
// pushback, taxi to the hold-short point and, after ClearForTakeoff, line-up
// and take-off.
//
// Like Fleet and airport.Loader, it never reads the engine stream: the
// application passes every message to Handle. Progress and state changes are
// published on Events. A controller is single use.
//
//	ctl := traffic.NewTaxiController(fleet)
//	ctl.Start(traffic.TaxiRequest{Graph: g, Parking: 18, Runway: "24", Model: "..."})
//	for {
//	    select {
//	    case msg := <-stream: ctl.Handle(msg)
//	    case ev := <-ctl.Events(): // react; call ctl.ClearForTakeoff() when holding short
//	    }
//	}
type TaxiController struct {
	mu      sync.Mutex
	fleet   *Fleet
	defBase uint32
	reqBase uint32
	events  chan TaxiEvent
	now     func() time.Time

	req       TaxiRequest
	route     *airport.Route
	state     TaxiState
	objectID  uint32
	last      TaxiEvent
	track     *routeTracker
	stillFrom time.Time
	warned    bool

	// Injected departure (TaxiWithInjector): the shared injected ground
	// driving and the departure specifics.
	groundDrive
	inj                                                     *Injector
	rng                                                     *rand.Rand
	sent                                                    map[uint32]string
	fast                                                    bool // monitor every frame: throttle progress events
	emittedAt                                               time.Time
	runway                                                  airport.Runway
	end                                                     airport.RunwayEnd
	runwayLength                                            float64
	lightsSet                                               bool
	gateAt, pushAt, moveAt                                  time.Time
	pushCleared, taxiCleared, lineUpCleared, takeoffCleared bool
	holdingCrossing                                         bool
	alignDist                                               float64
	takeoff                                                 *TakeoffMover
	gearUp                                                  bool
	pendingLimit                                            airport.NodeID // ClearUpTo before the taxi starts
	hasPendingLimit                                         bool
	flaps                                                   surfaceRamp
	frameAt                                                 time.Time
}

// SimConnect IDs relative to the bases.
const (
	defOffWaypoints = iota
	defOffMonitor
)

const (
	reqOffSpawn = iota
	reqOffRelease
	reqOffMonitor
	reqOffRemove
)

// taxiMonitor matches the monitor data definition.
type taxiMonitor struct {
	Latitude  float64
	Longitude float64
	Heading   float64
	GroundKts float64
	OnGround  float64
	// LIGHT LANDING, TAXI, STROBE, BEACON, NAV, LOGO, WING
	Lights [7]float64
}

// currentLights is the light state the sim reports.
func (m taxiMonitor) currentLights() Lights {
	on := func(i int) bool { return m.Lights[i] != 0 }
	return Lights{Landing: on(0), Taxi: on(1), Strobe: on(2), Beacon: on(3), Nav: on(4), Logo: on(5), Wing: on(6)}
}

// NewTaxiController creates a controller that spawns and moves its aircraft
// through fleet.
func NewTaxiController(fleet *Fleet, opts ...TaxiOption) *TaxiController {
	c := &TaxiController{
		fleet:   fleet,
		defBase: DefaultTaxiDefinitionBase,
		reqBase: DefaultTaxiRequestBase,
		events:  make(chan TaxiEvent, 256),
		now:     time.Now,
		rng:     rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0xdea)),
	}
	c.groundDrive = groundDrive{ignoreRunway: -1, limitNode: -1, clock: func() time.Time { return c.now() }, record: c.note}
	c.last.LimitNode = -1
	for _, o := range opts {
		o(c)
	}
	return c
}

// Events returns the channel of state changes and progress updates. It is
// closed when the controller reaches a terminal state. Progress updates are
// dropped when the channel is full; state changes use the buffer headroom.
func (c *TaxiController) Events() <-chan TaxiEvent { return c.events }

// State returns the current state.
func (c *TaxiController) State() TaxiState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Route returns the planned route, nil before Start.
func (c *TaxiController) Route() *airport.Route {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.route
}

// ObjectID returns the aircraft's SimConnect object ID, 0 before it exists.
func (c *TaxiController) ObjectID() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.objectID
}

// Start plans the route and spawns the aircraft at the stand, facing the
// stand's heading. The taxi proceeds as the application passes messages to
// Handle.
func (c *TaxiController) Start(req TaxiRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != TaxiIdle {
		return ErrAlreadyStarted
	}
	if req.Graph == nil || req.Model == "" {
		return fmt.Errorf("%w: Graph and Model are required", ErrBadTaxiRequest)
	}
	if req.Parking < 0 || req.Parking >= len(req.Graph.Layout.Parking) {
		return fmt.Errorf("%w: parking index %d", ErrBadTaxiRequest, req.Parking)
	}
	route, err := req.Graph.RouteToRunwayEntry(req.Parking, req.Runway, req.Entry, req.Options)
	if err != nil {
		return err
	}
	rwy, end, _ := req.Graph.Layout.RunwayEnd(req.Runway)
	c.runway, c.end, c.runwayLength = rwy, end, rwy.Length
	c.sent = map[uint32]string{}
	if _, err := TaxiWaypoints(req.Graph, route); err != nil {
		return err
	}
	client := c.fleet.clientOrNil()
	if client == nil {
		return ErrNotConnected
	}

	// Waypoint list and position monitor definitions.
	if err := client.AddToDataDefinition(c.defBase+defOffWaypoints, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0); err != nil {
		return err
	}
	for i, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"},
		{"PLANE LONGITUDE", "degrees"},
		{"PLANE HEADING DEGREES TRUE", "degrees"},
		{"GROUND VELOCITY", "knots"},
		{"SIM ON GROUND", "bool"},
		{"LIGHT LANDING", "bool"}, {"LIGHT TAXI", "bool"}, {"LIGHT STROBE", "bool"}, {"LIGHT BEACON", "bool"},
		{"LIGHT NAV", "bool"}, {"LIGHT LOGO", "bool"}, {"LIGHT WING", "bool"},
	} {
		if err := client.AddToDataDefinition(c.defBase+defOffMonitor, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
			return err
		}
	}

	stand := req.Graph.Layout.Parking[req.Parking]
	standAt := StandPoint(stand, req.NoseOffset) // at the stop mark, as arrivals park
	err = c.fleet.RequestNonATC(NonATCOpts{
		Model:  req.Model,
		Livery: req.Livery,
		Tail:   req.Tail,
		Position: types.SIMCONNECT_DATA_INITPOSITION{
			Latitude:  standAt.Lat,
			Longitude: standAt.Lon,
			Altitude:  convert.MetersToFeet(req.Graph.Layout.Altitude),
			Heading:   stand.Heading,
			OnGround:  1,
		},
	}, c.reqBase+reqOffSpawn)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreationFailed, err)
	}

	c.req, c.route = req, route
	c.track = newRouteTracker(route)
	c.setState(TaxiSpawning, nil)
	return nil
}

// Handle processes one message and reports whether it belonged to this
// controller.
func (c *TaxiController) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == TaxiIdle || c.state.Terminal() {
		return false
	}
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
	case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
		m := msg.AsAssignedObjectID()
		if uint32(m.DwRequestID) != c.reqBase+reqOffSpawn || c.state != TaxiSpawning {
			return false
		}
		c.onSpawned(uint32(m.DwObjectID))
		return true
	case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
		m := msg.AsSimObjectData()
		if uint32(m.DwRequestID) != c.reqBase+reqOffMonitor || uint32(m.DwObjectID) != c.objectID {
			return false
		}
		c.onPosition(*engine.CastDataAs[taxiMonitor](&m.DwData))
		return true
	}
	return false
}

// onSpawned hands the new aircraft its waypoints and starts monitoring it.
func (c *TaxiController) onSpawned(objectID uint32) {
	c.objectID = objectID
	c.fleet.Acknowledge(c.reqBase+reqOffSpawn, objectID)
	if c.inj != nil {
		if err := c.startInjectedDeparture(); err != nil {
			c.fail(err)
		}
		return
	}
	wps, _ := TaxiWaypoints(c.req.Graph, c.route) // validated in Start
	if err := c.fleet.ReleaseControl(objectID, c.reqBase+reqOffRelease); err != nil {
		c.fail(err)
		return
	}
	if err := c.fleet.SetWaypoints(objectID, c.defBase+defOffWaypoints, wps); err != nil {
		c.fail(err)
		return
	}
	client := c.fleet.clientOrNil()
	if client == nil {
		c.fail(ErrNotConnected)
		return
	}
	if err := client.RequestDataOnSimObject(c.reqBase+reqOffMonitor, c.defBase+defOffMonitor, objectID,
		types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
		c.fail(err)
		return
	}
	c.stillFrom = c.now()
	c.setState(TaxiPushback, nil)
}

// onPosition advances the state machine from a position report.
func (c *TaxiController) onPosition(m taxiMonitor) {
	if c.inj != nil {
		c.onDepartureFrame(m)
		return
	}
	pos := airport.LatLon{Lat: m.Latitude, Lon: m.Longitude}
	c.last.Position, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pos, m.Heading, m.GroundKts, m.OnGround != 0
	seg, along := c.track.advance(pos)
	c.last.Remaining = math.Max(0, c.track.total()-along)
	c.last.Taxiway = c.track.taxiwayAt(seg)

	if m.GroundKts >= StoppedKts {
		c.stillFrom, c.warned = c.now(), false
	}
	switch c.state {
	case TaxiPushback:
		// Pushback ends once the aircraft moves forward past the junction.
		if seg >= 1 && along > c.track.cum[1] && m.GroundKts >= StoppedKts {
			c.setState(TaxiTaxiing, nil)
			return
		}
	case TaxiTaxiing:
		if c.last.Remaining <= HoldShortArrivalMeters && m.GroundKts < StoppedKts {
			c.setState(TaxiHoldingShort, nil)
			return
		}
	case TaxiLiningUp:
		if m.GroundKts > 30 {
			c.setState(TaxiDeparting, nil)
			return
		}
	case TaxiDeparting:
		if m.OnGround == 0 {
			c.stopMonitor()
			c.setState(TaxiComplete, nil)
			return
		}
	}
	if (c.state == TaxiPushback || c.state == TaxiTaxiing) && !c.warned && c.now().Sub(c.stillFrom) > StuckTimeout {
		c.warned = true
		c.emit(ErrTaxiStuck, true)
		return
	}
	c.emit(nil, false)
}

// ClearForTakeoff sends a holding-short aircraft onto the runway and into the
// take-off climb.
func (c *TaxiController) ClearForTakeoff() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj != nil {
		// Injected: takes effect at the runway hold-short, during the line-up
		// (a rolling take-off) or lined up; given earlier it waits.
		c.takeoffCleared = true
		return nil
	}
	if c.state != TaxiHoldingShort {
		return ErrNotHoldingShort
	}
	wps, err := LineUpWaypoints(c.req.Graph, c.route)
	if err != nil {
		return err
	}
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+defOffWaypoints, wps); err != nil {
		return err
	}
	c.setState(TaxiLiningUp, nil)
	return nil
}

// Cancel removes the aircraft from the simulation if it still exists, also
// after the departure completed (handed to MSFS AI), and ends the controller
// if it is still running. It is safe to call at any time.
func (c *TaxiController) Cancel() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var err error
	if c.objectID != 0 {
		c.stopMonitor()
		err = c.fleet.Remove(c.objectID, c.reqBase+reqOffRemove)
		if c.inj != nil {
			c.inj.Forget(c.objectID)
		}
		c.objectID = 0
	}
	if !c.state.Terminal() {
		c.setState(TaxiCancelled, nil)
	}
	return err
}

func (c *TaxiController) stopMonitor() {
	if client := c.fleet.clientOrNil(); client != nil && c.objectID != 0 {
		client.RequestDataOnSimObject(c.reqBase+reqOffMonitor, c.defBase+defOffMonitor, c.objectID,
			types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	}
}

func (c *TaxiController) fail(err error) {
	c.stopMonitor()
	c.setState(TaxiFailed, err)
}

// setState records a state change and publishes it.
func (c *TaxiController) setState(s TaxiState, err error) {
	c.state = s
	c.emit(err, true)
	if s.Terminal() {
		close(c.events)
	}
}

// emit publishes the current state. Progress updates (important=false) are
// dropped when the channel is full so Handle never blocks the message loop.
func (c *TaxiController) emit(err error, important bool) {
	if !important && err == nil && c.fast && c.now().Sub(c.emittedAt) < time.Second {
		return // progress at most once a second while reading every frame
	}
	c.emittedAt = c.now()
	ev := c.last
	ev.State, ev.ObjectID, ev.Err = c.state, c.objectID, err
	if important || len(c.events) < cap(c.events)-16 {
		select {
		case c.events <- ev:
		default:
		}
	}
}

// meters returns q's offset east and north of origin in meters.
func meters(origin, q airport.LatLon) (x, z float64) {
	const r = 6371008.8
	x = (q.Lon - origin.Lon) * math.Pi / 180 * r * math.Cos(origin.Lat*math.Pi/180)
	z = (q.Lat - origin.Lat) * math.Pi / 180 * r
	return x, z
}
