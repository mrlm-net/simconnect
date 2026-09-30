//go:build windows
// +build windows

package traffic

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
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
	// ObjectID adopts an aircraft already standing at Parking instead of
	// spawning one (#293), e.g. one an ArrivalController parked there for
	// a turnaround. The controller takes it over from where it stands;
	// Model is still needed for its profile.
	ObjectID uint32
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
	// HoldForRunway (injected) stops at the gates onto a runway only —
	// line-up, take-off and runway crossings — until their clearance, for a
	// runway controller (#393); pushback and taxi go by themselves.
	HoldForRunway bool
	// PushbackAt (injected, without HoldForClearances) keeps the aircraft
	// on its stand until then, e.g. its scheduled departure time; zero
	// pushes after the usual short wait.
	PushbackAt time.Time
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
	// Departure is flown by MSFS AI after the injected take-off hands over
	// at ClimbHandoverFt (or Airport.ClimbHandoverFt): the SID (e.g. airport.Procedures.ResolveSID) and
	// optionally the rest of the flight plan (#315). Empty climbs straight
	// ahead (TakeoffClimb).
	Departure []airport.NavPoint
	// Aircraft is the aircraft's profile (#324); nil resolves it from
	// Model (ProfileFor). It fills Profile, Takeoff and NoseOffset where
	// those are zero and sets the flaps and pushback speed.
	Aircraft *AircraftProfile
	// Airport is the airport's limits (#335), e.g. airport.LimitsFor; nil
	// uses the global defaults. Its ClimbHandoverFt replaces
	// ClimbHandoverFt, and TaxiMaxKts and ApronMaxKts cap the injected taxi
	// speed where the motion profile is faster.
	Airport *airport.Limits
	// Deice de-ices the aircraft before it departs (#323): on the stand
	// before the pushback, or at a pad on the way to the runway. Nil: no
	// de-icing.
	Deice *Deicing
	// Tug shows a pushback tug (injected departures with a pushback): e.g.
	// NewSimObjectTug with a GSX tug title, or a third-party integration.
	// Nil pushes back without one. The controller passes it its messages.
	Tug PushbackTug
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
	// Deicing is set while the aircraft is being de-iced (#323).
	Deicing bool
	// PushbackHeld is set while the pushback waits, or stops, for traffic
	// behind the stand (#334).
	PushbackHeld bool
	// HeightFt is the height above the runway during the take-off.
	HeightFt float64
	// Request is what the crew asks for while HoldForClearances holds the
	// aircraft and it is ready (its own wait is over): "pushback" (push
	// and start-up), "taxi"; "" when it asks for nothing (#462). A
	// controller answers with the clearance (ClearPushback, ClearToTaxi).
	Request string
	// Lights is the light state the sim reports.
	Lights Lights
	// Err is set for TaxiFailed and for non-fatal warnings such as ErrTaxiStuck.
	Err error
}

// TaxiOption configures a TaxiController.
type TaxiOption func(*TaxiController)

// TaxiWithSeed seeds the controller's random choices and timing spreads
// (#343), so a run can be repeated.
func TaxiWithSeed(seed uint64) TaxiOption {
	return func(c *TaxiController) { c.rng = rand.New(rand.NewPCG(seed, 0xdea)) }
}

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
	// Level of detail (#370): how often the injected aircraft is driven.
	detail  *Detail
	detailS detailState
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
	inj    *Injector
	rng    *rand.Rand
	timing timing // this aircraft's draw of the spreads (#343)
	// De-icing (#323): the pad's node, the end of the treatment, done.
	padNode                                                 airport.NodeID
	deiceUntil                                              time.Time
	deiced                                                  bool
	sent                                                    map[uint32]string
	fast                                                    bool // monitor every frame: throttle progress events
	emittedAt                                               time.Time
	runway                                                  airport.Runway
	end                                                     airport.RunwayEnd
	runwayLength                                            float64
	lightsSet                                               bool
	gateAt, pushAt, moveAt                                  time.Time
	pushCleared, taxiCleared, lineUpCleared, takeoffCleared bool
	// climb: the waypoints MSFS AI flies after the injected climb (the
	// SID), for ClimbRoute.
	climb []types.SIMCONNECT_DATA_WAYPOINT
	pushStopped                                             bool // HoldPushback: stay on the stand
	takeoffHeld                                             bool // AbortTakeoff before the roll: wait for ClearForTakeoff, gates or not
	holdingCrossing                                         bool
	alignDist                                               float64
	takeoff                                                 *TakeoffMover
	gearUp                                                  bool
	pendingLimit                                            airport.NodeID // ClearUpTo before the taxi starts
	hasPendingLimit                                         bool
	flaps                                                   surfaceRamp
	frameAt                                                 time.Time
	tugAttached                                             bool
	pushBranch                                              airport.NodeID // taxiway the tail is pushed onto (planPushback)
	havePushBranch                                          bool
	pushJunction                                            int              // route index of the junction the tail swings at (planPushback; 1: the first)
	pushPts                                                 []airport.LatLon // the push up an alley (planPushback), nil for the fitted push
	pushTurn                                                bool             // push and turn on the apron (only taxiway at the junction is the way out)
	pushPlanned                                             *GroundPath      // the push path, planned before it starts (pushPath)
	pushTurnDir                                             float64          // the way out from the junction
	pushPose                                                *pushPose        // where the push ends (planPushPose), nil for the older plans
	faceOut                                                 bool             // a self-manoeuvring stand (standFacesOut, at the start)
	towPts                                                  []airport.LatLon // the nose gear towed forward after the push (planPushPose), nil for none
	towing                                                  bool             // the tow after the push is under way
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
	c.timing = drawTiming(c.rng)
	if req.Graph == nil || req.Model == "" {
		return fmt.Errorf("%w: Graph and Model are required", ErrBadTaxiRequest)
	}
	if req.Parking < 0 || req.Parking >= len(req.Graph.Layout.Parking) {
		return fmt.Errorf("%w: parking index %d", ErrBadTaxiRequest, req.Parking)
	}
	req.resolveAircraft()
	req.Options = withSpan(req.Options, req.Profile)
	c.padNode = -1
	if d := req.Deice; d != nil && d.Pad != nil {
		// The route passes the pad; the aircraft stops there.
		node, ok := req.Graph.NearestNode(d.Pad.Position, DeicingPadReachMeters)
		if !ok {
			return fmt.Errorf("%w: de-icing pad %q is not on a taxiway", ErrBadTaxiRequest, d.Pad.Name)
		}
		req.Options.Via = append([]airport.NodeID{node}, req.Options.Via...)
		c.padNode = node
	}
	req.Options.OwnStands = append(slices.Clone(req.Options.OwnStands), req.Parking) // not an obstacle to itself
	route, err := req.Graph.RouteToRunwayEntry(req.Parking, req.Runway, req.Entry, req.Options)
	if err != nil {
		return err
	}
	if err := entryLongEnough(req, route); err != nil {
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

	// Waypoint list and position monitor definitions (cleared first when
	// the ID block is reused).
	c.fleet.redefine(client, c.defBase+defOffWaypoints, c.defBase+defOffMonitor)
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

	if req.ObjectID != 0 {
		// Adopted: no spawn; the aircraft is already on the stand.
		c.req, c.route = req, route
		c.pushJunction = 1
		c.faceOut = c.standFacesOut()
		if c.inj != nil {
			c.planPushback()
		}
		c.track = newRouteTracker(c.route)
		c.setState(TaxiSpawning, nil)
		c.onSpawned(req.ObjectID)
		return nil
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
	c.pushJunction = 1
	c.faceOut = c.standFacesOut()
	if c.inj != nil {
		c.planPushback() // may re-plan the route from the push
	}
	c.track = newRouteTracker(c.route)
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
	if c.req.Tug != nil && c.req.Tug.Handle(msg) {
		return true
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
	c.fleet.Acknowledge(c.reqBase+reqOffSpawn, objectID) // no-op for an adopted aircraft
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
		c.takeoffCleared, c.takeoffHeld = true, false
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
	c.removeTug()
	if c.picture != nil {
		c.picture.Forget(c.objectID)
	}
	var err error
	if c.objectID != 0 {
		c.stopMonitor()
		c.detail.forget(c.objectID)
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
	c.removeTug()
	c.setState(TaxiFailed, err)
}

// removeTug takes the pushback tug away (cancel, failure).
func (c *TaxiController) removeTug() {
	if c.req.Tug != nil {
		c.note("tug", c.req.Tug.Remove())
	}
}

// setState records a state change and publishes it.
func (c *TaxiController) setState(s TaxiState, err error) {
	c.state = s
	c.emit(err, true)
	if s.Terminal() {
		c.detail.forget(c.objectID)
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

// entryLongEnough refuses an intersection departure (TaxiRequest.Entry)
// whose runway ahead is shorter than the aircraft needs: RequiredTakeoffRun
// for its take-off profile at the airport's elevation.
func entryLongEnough(req TaxiRequest, route *airport.Route) error {
	if req.Entry == "" || len(route.Nodes) == 0 {
		return nil
	}
	entries, err := req.Graph.RunwayEntries(req.Runway)
	if err != nil {
		return nil
	}
	last := route.Nodes[len(route.Nodes)-1]
	for _, e := range entries {
		if e.HoldShort != last && e.Node != last {
			continue
		}
		p := req.Takeoff
		if p == (TakeoffProfile{}) {
			p = DefaultTakeoffProfile()
		}
		need := RequiredTakeoffRun(p, TakeoffConditions{ElevationFt: convert.MetersToFeet(req.Graph.Layout.Altitude)})
		if e.Remaining < need {
			return fmt.Errorf("%w: runway %s at %s leaves %.0f m, the aircraft needs %.0f m", ErrEntryTooShort, req.Runway, req.Entry, e.Remaining, need)
		}
		return nil
	}
	return nil
}
