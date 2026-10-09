package traffic

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ErrNoTouchdown is reported when an arriving aircraft passes the runway end
// without touching down (for example with its gear up).
var ErrNoTouchdown = errors.New("traffic: aircraft did not touch down")

// ArrivalState is the phase of an arrival.
type ArrivalState uint8

const (
	ArrivalIdle         ArrivalState = iota // not started
	ArrivalSpawning                         // waiting for the simulator to create the aircraft
	ArrivalApproaching                      // on final
	ArrivalLanding                          // below LandingAGLFt, about to touch down
	ArrivalRollout                          // on the runway after touchdown
	ArrivalVacating                         // off the runway, rolling clear to the vacate stop
	ArrivalAwaitingTaxi                     // stopped clear of the runway: after-landing lights, waiting for taxi clearance
	ArrivalTaxiing                          // taxiing to the stand
	ArrivalHoldingShort                     // holding short of a runway crossing, waiting for ClearToCross
	ArrivalParking                          // on the stand's PARKING path
	ArrivalParked                           // stopped at the stand; the controller no longer moves it
	ArrivalCancelled                        // Cancel was called
	ArrivalFailed                           // an error ended the arrival
)

var arrivalStateNames = [...]string{"idle", "spawning", "approaching", "landing", "rollout", "vacating", "awaiting taxi", "taxiing", "holding short", "parking", "parked", "cancelled", "failed"}

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
	Graph   *airport.Graph `json:"-"`
	Runway  string         // runway end to land on, e.g. "24"
	Parking int            // stand parking index; see airport.Layout.ParkingIndex
	Model   string         // aircraft container title
	Livery  string
	Tail    string
	// ObjectID adopts an aircraft already flying (an en route arrival at its
	// STAR entry, #643) instead of creating one: it flies the procedure from
	// where it is, no jump to the spawn point. Procedure flights only.
	ObjectID uint32
	// SpawnNm is how far out on final the aircraft appears; 0 means DefaultSpawnNm.
	SpawnNm float64
	// CrosswindKts is the crosswind on the runway, knots, positive from its
	// right: the injected final is flown crabbed into it
	// (ApproachMover.SetCrosswind).
	CrosswindKts float64
	// Exit forces a runway exit; nil chooses one for the required rollout.
	Exit *airport.RunwayExit
	// GroundAGL sends ground waypoints at 0 ft above ground instead of the
	// airport elevation (experimental; for sloped runways and taxiways).
	GroundAGL bool
	Options   airport.RouteOptions
	// NoseOffset is the distance from the aircraft reference point to the
	// nose for stopping on the stand; 0 means DefaultNoseOffsetMeters.
	NoseOffset float64
	// NoStopWaypoint disables the active stop at the stand (for comparison).
	NoStopWaypoint bool
	// HoldForClearance keeps the aircraft stopped clear of the runway until
	// ClearToTaxi is called; otherwise it continues after AfterLandingDwell.
	HoldForClearance bool
	// AfterLandingDwell is how long the aircraft stays stopped clear of the
	// runway before taxiing on; 0 means DefaultAfterLandingDwell.
	AfterLandingDwell time.Duration
	// Profile is the ground motion of the aircraft type when the ground
	// phase is injected (ArrivalWithInjector); zero means
	// DefaultMotionProfile.
	Profile MotionProfile
	// RollThroughChance is the chance that an injected arrival not holding
	// for clearance only slows to RollThroughKts at the vacate point and taxis
	// on (a rolling clearance); 0 means DefaultRollThroughChance, negative
	// never.
	RollThroughChance float64
	// HoldAtCrossings (injected arrivals) stops the aircraft short of every
	// runway it crosses on the way to the stand until ClearToCross; without
	// it crossings are cleared in advance.
	HoldAtCrossings bool
	// InjectApproach (with ArrivalWithInjector) flies the whole arrival by
	// injection: the final approach on a stable glide path with a flare
	// and a soft touchdown (see ApproachProfile), then the rollout, exit and
	// taxi-in. Without it MSFS AI flies until the rollout.
	InjectApproach bool
	// Rollout is the injected rollout and exit; zero means
	// DefaultRolloutProfile.
	Rollout RolloutProfile
	// Approach is the injected approach; zero means DefaultApproachProfile.
	Approach ApproachProfile
	// FollowMe leads the injected taxi-in to the stand (#890): e.g.
	// NewSimObjectFollowMe. Sent once the aircraft is down, from the fleet
	// (ArrivalWithServices); nil: none. The controller passes it its
	// messages.
	FollowMe FollowMeService `json:"-"`
	// Procedure (with InjectApproach) is the STAR and approach to fly
	// before the final, e.g. from airport.Procedures.Arrival: the aircraft
	// appears at its first point, MSFS AI flies it to a join point on the
	// extended centreline (ProcedureJoinNm out), and the injected approach
	// takes over there (#315).
	Procedure []airport.NavPoint
	// Circuit (with InjectApproach, instead of Procedure) is a VFR arrival
	// through the circuit (#568): the aircraft appears at the 45° entry to
	// the downwind, MSFS AI flies the downwind and base, and the injected
	// approach takes over on the circuit's short final.
	Circuit *Circuit
	// TouchAndGos (with Circuit) is how many touch-and-goes it makes
	// before its full stop, flying the circuit again after each (#569).
	TouchAndGos int
	// StopAndGo makes each of them a stop-and-go: it brakes to a stop on
	// the runway, waits StopAndGoWait, and takes off from there (#567).
	StopAndGo bool
	// CircuitEntry (with Circuit) is the reporting point it enters over
	// (#566); nil: it appears at the 45° entry to the downwind.
	CircuitEntry *ReportingPoint
	// CircuitJoin (with CircuitEntry) is where the tower joins it
	// (CircuitJoinFor): LegFinal a straight-in approach, else the downwind.
	CircuitJoin CircuitLeg
	// MissedApproach (with InjectApproach) is the published missed approach
	// flown on a go-around (airport.Procedures.MissedApproach), then back
	// round to the final; without it the go-around flies a circuit (#394).
	MissedApproach []airport.NavPoint
	// Aircraft is the aircraft's profile (#324); nil resolves it from
	// Model (ProfileFor). It fills Profile, Approach, Rollout and
	// NoseOffset where those are zero and sets the flaps and lights.
	Aircraft *AircraftProfile
	// Airport is the airport's limits (#335), e.g. airport.LimitsFor; nil
	// uses the global defaults. Its TaxiMaxKts and ApronMaxKts cap the
	// injected taxi-in speed where the motion profile is faster.
	Airport *airport.Limits
}

// ArrivalEvent reports a state change or progress of an arrival.
type ArrivalEvent struct {
	State    ArrivalState
	ObjectID uint32
	// GivingWayTo is the aircraft (object ID) it stops to give way to while
	// taxiing in, 0 for none (TaxiEvent.GivingWayTo).
	GivingWayTo uint32
	// StoppedBy is why it stands still while taxiing in (TaxiEvent.StoppedBy).
	StoppedBy string
	// MissedJoin: MSFS AI flew past the end of its procedure without
	// joining the final (where, how): it needs a direct to the final.
	MissedJoin  string
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
	// TouchAndGo is set while it is on the runway for a touch-and-go:
	// it takes off again, not vacating (#569).
	TouchAndGo bool
	// Remaining is the distance to the stand along the taxi-in route, in
	// meters, once on the ground.
	Remaining float64
	Taxiway   string
	// HoldingShortOf names the runway the aircraft holds short of, waiting
	// for ClearToCross.
	HoldingShortOf string
	// LimitNode is the clearance limit of a progressive taxi (ClearUpTo), -1
	// for none; AtLimit is set while the aircraft holds there.
	LimitNode airport.NodeID
	AtLimit   bool
	// Lights is the light state the sim reports.
	Lights Lights
	Err    error
}

// ArrivalOption configures an ArrivalController.
type ArrivalOption func(*ArrivalController)

// ArrivalWithSeed seeds the controller's random choices and timing spreads
// (#343), so a run can be repeated.
func ArrivalWithSeed(seed uint64) ArrivalOption {
	return func(c *ArrivalController) { c.rng = rand.New(rand.NewPCG(seed, 0x5eed)) }
}

// ArrivalWithServices takes the arrival's service vehicles (its follow-me
// car, #890) from f.
func ArrivalWithServices(f ServiceFleet) ArrivalOption {
	return func(c *ArrivalController) { c.services = f }
}

// ArrivalWithIDs sets the first data definition ID and request ID; an
// ArrivalController uses 4 definition IDs and 4 request IDs from them.
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
	// services hands out the follow-me car (#890); fm is the car while it
	// is out, fmSent once it was sent for (or given up), fmStopped since
	// when the aircraft has stood held by it.
	services  ServiceFleet
	fm        FollowMeService
	fmSent    bool
	fmStopped time.Time
	// Level of detail (#370): how often the injected aircraft is driven.
	detail  *Detail
	detailS detailState
	// procSpeed is the speed assigned on the STAR by AbsorbDelay (#391).
	procSpeed float64
	// holding is the hold the arrival flies, nil when none (#392).
	holding *holdState
	mu      sync.Mutex
	fleet   *Fleet
	defBase uint32
	reqBase uint32
	events  chan ArrivalEvent
	now     func() time.Time

	req            ArrivalRequest
	plan           *ArrivalPlan
	state          ArrivalState
	objectID       uint32
	last           ArrivalEvent
	airborne       bool
	track          *routeTracker
	exitAlong      float64 // route distance of the exit node (off the runway)
	stillFrom      time.Time
	stopSent       bool
	anchor         airport.LatLon // last position that moved
	anchorAt       time.Time
	standHeading   float64
	sent           map[uint32]string // send ID → call, for attributing exceptions
	lastVS         float64           // vertical speed at the last airborne report
	warned         bool
	vacateAlong    float64 // route distance of the vacate stop
	vacateStopSent bool
	clearAt        time.Time
	cleared        bool
	wantLights     [5]float64
	lightsAt       time.Time

	// Hybrid ground phase (ArrivalWithInjector): the shared injected ground
	// driving (mover, lights, crossings) and the arrival specifics.
	groundDrive
	inj           *Injector
	fast          bool // monitor every sim frame: throttle progress events
	touchdownAt   time.Time
	clearDist     float64 // injected path distance where the aircraft is clear of the runway
	rollThrough   bool    // rolling clearance: slow at the vacate point, do not stop
	vacateDist    float64 // injected path distance of the vacate stop
	rng           *rand.Rand
	timing        timing            // this aircraft's draw of the spreads (#343)
	lightsChanged bool              // the sim reported a light change since the last event
	approach      *ApproachMover    // injected approach until the rollout hand-over
	rush          bool              // expedite vacating (Expedite, #510)
	proc          *ArrivalProcedure // STAR and approach flown by MSFS AI (Procedure)
	flyingProc    bool
	goArounds     int           // go-arounds flown (GoAround)
	tngLeft       int           // touch-and-goes still to make (ArrivalRequest.TouchAndGos)
	tng           *TakeoffMover // a touch-and-go's take-off, while it flies it
	sng           *stopAndGo    // a stop-and-go's stop, before its take-off
	// corners: proc's points as planned, before its turns are rounded
	// (proc.Waypoints is the rounded chain flown), with their names ("" for
	// none) and the next one ahead (-1: the nearest). A delay absorbed
	// re-plans from them: from the rounded points, a downwind extended cut
	// into a turn's arc and left loops in the path.
	corners     []types.SIMCONNECT_DATA_WAYPOINT
	cornerNames []string
	cornerNext  int
	// reachFirst: the first corner, a point picked on the map (#443),
	// counts as passed only once reached (off the route, the geometric test
	// passed it at once: OKRVJ told to resume own navigation 0.5 s after
	// its heading).
	reachFirst bool
	// procNext is the waypoint of proc flown to, tracked forward from a
	// known start: after a go-around (whose circuit loops back past the
	// final, where the nearest waypoint is the wrong one), a delay absorbed
	// or a hold left. -1: the nearest (a STAR does not loop).
	procNext int
	// joinMinM: -1 once off its route and reported as MissedJoin, 0
	// otherwise.
	joinMinM float64
	// lastRunwayM is its distance from the threshold at the frame before.
	lastRunwayM float64
	// circuit: flying a go-around's missed approach and circuit; the final
	// is joined again only from its last two points (align, join) — climbing
	// out along the centreline it would look established at once.
	circuit bool
	// gaGearUp: the gear is up after a go-around (raised GoAroundGearUpFt
	// above the runway; down again when the approach takes over).
	gaGearUp bool
	// tromboneNM: how far the downwind was extended on this approach
	// (AbsorbDelay).
	tromboneNM float64
	// baseCall: the downwind was extended with the base turn left to the
	// controller, said once the aircraft is there (BaseDue).
	baseCall bool
	// dogLeg: the dog-leg given on this approach (AbsorbDelay). More delay
	// moves its apex further out while it is still ahead, never a second
	// dog-leg (live, RYR1785: out, back out and round again).
	dogLeg *dogLegState
	// vectors are the radar vectors still to say for a dog-leg or an
	// extended downwind, each at its corner; vectored: off the STAR on
	// vectors, cleared for the approach with the intercept heading (#661).
	vectors           []Vector
	vectored          bool
	blend             joinBlend
	flapsPct          float64       // injected flap setting
	seq               sequence      // the landing's steps (Sequence)
	approachPhase     ApproachPhase // the approach's phase last frame
	landingFlaps      bool          // landing flaps noted (started, then set)
	landingFlapsSet   bool
	flapsUpFrom       time.Time // flaps retracting since
	approachLightsSet bool
	spoilers          surfaceRamp    // injected ground spoilers
	pendingLimit      airport.NodeID // ClearUpTo before the taxi-in starts
	hasPendingLimit   bool
	takeoverTried     bool
	emittedAt         time.Time
}

const (
	arrDefWaypoints = iota
	arrDefMonitor
	arrDefGear
	arrDefLights
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
	Lights    [5]float64 // LIGHT LANDING, TAXI, STROBE, BEACON, NAV
	Logo      float64
	Wing      float64
	AltFt     float64 // MSL: the height to join the final from (0: unknown)
}

// NewArrivalController creates a controller that spawns its aircraft through
// fleet.
func NewArrivalController(fleet *Fleet, opts ...ArrivalOption) *ArrivalController {
	c := &ArrivalController{
		fleet: fleet, defBase: DefaultArrivalDefinitionBase, reqBase: DefaultArrivalRequestBase,
		events: make(chan ArrivalEvent, 256), now: time.Now,
		rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x5eed)),
	}
	c.groundDrive = groundDrive{ignoreRunway: -1, limitNode: -1, clock: func() time.Time { return c.now() }, record: c.note}
	c.last.LimitNode = -1
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

// Sequence is the injected landing step by step, from the takeover on
// final: gear, flaps, lights, flare, touchdown, spoilers, rollout, vacating.
func (c *ArrivalController) Sequence() []SequenceStep {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seq.list()
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
	c.timing = drawTiming(c.rng)
	c.tngLeft = req.TouchAndGos
	if req.Graph == nil || req.Model == "" || req.Parking < 0 || req.Parking >= len(req.Graph.Layout.Parking) {
		return fmt.Errorf("%w: Graph, Model and a valid Parking are required", ErrBadTaxiRequest)
	}
	req.resolveAircraft()
	req.Options = withSpan(req.Options, req.Profile)
	plan, err := PlanArrival(req.Graph, req.Runway, req.Parking, ArrivalOptions{
		SpawnNm: req.SpawnNm, Exit: req.Exit, Route: req.Options, GroundAGL: req.GroundAGL, NoseOffset: req.NoseOffset,
		TouchdownKts: approachProfileOf(req).TouchdownKts, BrakeDecel: req.Rollout.BrakeDecel,
	})
	if err != nil {
		return err
	}
	if req.InjectApproach {
		if c.inj == nil {
			return fmt.Errorf("%w: InjectApproach needs ArrivalWithInjector", ErrBadTaxiRequest)
		}
		// Appear exactly where the injected approach starts.
		ap := NewApproachMover(plan.End.Threshold, plan.End.Heading, plan.SpawnNm*1852, approachProfileOf(req)).Pose()
		plan.Spawn.Latitude, plan.Spawn.Longitude = ap.Position.Lat, ap.Position.Lon
		plan.Spawn.Altitude = convert.MetersToFeet(req.Graph.Layout.Altitude) + ap.HeightFt + convert.MetersToFeet(req.Aircraft.CGHeightM)
		plan.Spawn.Airspeed = types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(ap.GroundSpeedKts)
		if req.Circuit != nil {
			proc := PlanCircuitArrivalVia(*req.Circuit, req.CircuitEntry, req.CircuitJoin)
			plan.Spawn = proc.Spawn
			c.setCorners(proc.Waypoints, nil)
			proc.Waypoints = roundedChain(airport.LatLon{Lat: proc.Spawn.Latitude, Lon: proc.Spawn.Longitude}, proc.Waypoints, MaxBankDeg(*req.Aircraft))
			// Tracked from its first point: a join from beyond the base turn
			// passes near it on the way in (live, OKKKQ from N63 taken for
			// on base, the nearest point).
			c.proc, c.procNext = proc, 0
		} else if len(req.Procedure) > 0 {
			join := math.Max(plan.SpawnNm, ProcedureJoinNm) * 1852
			jp := NewApproachMover(plan.End.Threshold, plan.End.Heading, join, approachProfileOf(req)).Pose()
			proc, err := PlanArrivalProcedure(req.Procedure, plan.End, join, convert.MetersToFeet(req.Graph.Layout.Altitude)+jp.HeightFt)
			if err != nil {
				return err
			}
			plan.Spawn = proc.Spawn
			// Its corners are the aircraft's turns (roundCorners), named.
			c.setCorners(proc.Waypoints, proc.Names)
			proc.Waypoints = roundedChain(airport.LatLon{Lat: proc.Spawn.Latitude, Lon: proc.Spawn.Longitude}, proc.Waypoints, MaxBankDeg(*req.Aircraft))
			c.proc, c.procNext = proc, -1
		}
	} else if len(req.Procedure) > 0 || req.Circuit != nil {
		return fmt.Errorf("%w: Procedure and Circuit need InjectApproach", ErrBadTaxiRequest)
	}
	client := c.fleet.clientOrNil()
	if client == nil {
		return ErrNotConnected
	}
	c.fleet.redefine(client, c.defBase+arrDefWaypoints, c.defBase+arrDefMonitor, c.defBase+arrDefGear, c.defBase+arrDefLights)
	if err := client.AddToDataDefinition(c.defBase+arrDefWaypoints, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0); err != nil {
		return err
	}
	for i, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"PLANE HEADING DEGREES TRUE", "degrees"}, {"GROUND VELOCITY", "knots"}, {"SIM ON GROUND", "bool"},
		{"VERTICAL SPEED", "feet per minute"},
		{"LIGHT LANDING", "bool"}, {"LIGHT TAXI", "bool"}, {"LIGHT STROBE", "bool"}, {"LIGHT BEACON", "bool"}, {"LIGHT NAV", "bool"},
		{"LIGHT LOGO", "bool"}, {"LIGHT WING", "bool"}, {"PLANE ALTITUDE", "feet"},
	} {
		if err := client.AddToDataDefinition(c.defBase+arrDefMonitor, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
			return err
		}
	}
	if err := client.AddToDataDefinition(c.defBase+arrDefGear, "GEAR HANDLE POSITION", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
		return err
	}
	for i, l := range []string{"LIGHT LANDING", "LIGHT TAXI", "LIGHT STROBE", "LIGHT BEACON", "LIGHT NAV"} {
		if err := client.AddToDataDefinition(c.defBase+arrDefLights, l, "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
			return err
		}
	}
	c.sent = map[uint32]string{}
	c.standHeading = req.Graph.Layout.Parking[req.Parking].Heading
	adopt := req.ObjectID != 0 && c.proc != nil
	if req.ObjectID != 0 && c.proc == nil {
		return fmt.Errorf("%w: ObjectID adopts a procedure flight only", ErrBadTaxiRequest)
	}
	if !adopt {
		if err := c.fleet.RequestNonATC(NonATCOpts{Model: req.Model, Livery: req.Livery, Tail: req.Tail, Position: plan.Spawn}, c.reqBase+arrReqSpawn); err != nil {
			return fmt.Errorf("%w: %v", ErrCreationFailed, err)
		}
	}
	c.req, c.plan = req, plan
	if adopt {
		// Flying already: on with the procedure from where it is.
		defer func() {
			if c.state == ArrivalSpawning {
				c.onSpawned(req.ObjectID)
			}
		}()
	}
	if chance := req.RollThroughChance; c.inj != nil && !req.HoldForClearance && chance >= 0 {
		if chance == 0 {
			chance = DefaultRollThroughChance
		}
		c.rollThrough = c.rng.Float64() < chance
	}
	c.track = newRouteTracker(plan.Route)
	c.exitAlong = c.track.cum[len(plan.Exit.Path)-1]
	c.vacateAlong = c.track.cum[plan.VacateIndex]
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
	// Cancelled while the simulator was still creating it: the aircraft it
	// gets after all is removed at once (#89: it was left in the sim).
	if c.state == ArrivalCancelled && c.objectID == 0 && types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		if m := msg.AsAssignedObjectID(); uint32(m.DwRequestID) == c.reqBase+arrReqSpawn {
			c.note("removed: created after the cancel", c.fleet.Remove(uint32(m.DwObjectID), c.reqBase+arrReqRemove))
			return true
		}
	}
	if c.state == ArrivalParked && c.fm != nil {
		return c.handleParked(msg) // the follow-me car on its way home (#890)
	}
	if c.state == ArrivalIdle || c.state.Terminal() {
		return false
	}
	if c.fm != nil && c.fm.Handle(msg) {
		return true
	}
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
	case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
		m := msg.AsAssignedObjectID()
		if uint32(m.DwRequestID) != c.reqBase+arrReqSpawn || c.state != ArrivalSpawning {
			return false
		}
		c.onSpawned(uint32(m.DwObjectID))
		return true
	case types.SIMCONNECT_RECV_ID_EXCEPTION:
		e := msg.AsException()
		call, ok := c.sent[uint32(e.DwSendID)]
		if !ok {
			return false
		}
		c.emit(fmt.Errorf("traffic: SimConnect exception %d on %s (parameter %d)", e.DwException, call, e.DwIndex), true)
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
	if c.proc != nil {
		if err := c.startProcedure(); err != nil {
			c.fail(err)
		}
		return
	}
	if c.req.InjectApproach {
		if err := c.startInjectedApproach(c.plan.SpawnNm * 1852); err != nil {
			c.fail(err)
		}
		return
	}
	if err := c.fleet.ReleaseControl(objectID, c.reqBase+arrReqRelease); err != nil {
		c.fail(err)
		return
	}
	// Without the gear handle down MSFS AI never touches down (#301).
	gear := [1]float64{1}
	err := client.SetDataOnSimObject(c.defBase+arrDefGear, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(gear)), unsafe.Pointer(&gear))
	c.note("gear handle down", err)
	if err != nil {
		c.fail(err)
		return
	}
	c.setLights(true, false, true, true, true, "lights approach")
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
	if m.AGL < 15 { // on the ground: in the ground picture (#334)
		pos, hdg := airport.LatLon{Lat: m.Latitude, Lon: m.Longitude}, m.Heading
		if c.mover != nil && c.state >= ArrivalVacating { // injected: where it is placed
			p := c.mover.Pose()
			pos, hdg = p.Position, p.Heading
		}
		c.reportGround(c.objectID, pos, hdg, c.now())
	}
	if l := m.currentLights(); l != c.last.Lights {
		c.last.Lights, c.lightsChanged = l, true // reported even between throttled events
	}
	if c.flyingProc {
		c.onProcedureFrame(m)
		return
	}
	if c.tng != nil || c.sng != nil {
		c.onTouchAndGoFrame()
		return
	}
	if c.approach != nil {
		c.onApproachFrame(m)
		return
	}
	if c.mover != nil {
		c.onInjectedFrame()
		return
	}
	pos := airport.LatLon{Lat: m.Latitude, Lon: m.Longitude}
	c.last.Position, c.last.AGL, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pos, m.AGL, m.Heading, m.GroundKts, m.OnGround != 0
	t := c.plan.End.Threshold
	past := calc.AlongTrackMeters(t.Lat, t.Lon, c.plan.Runway.Center.Lat, c.plan.Runway.Center.Lon, m.Latitude, m.Longitude)

	if m.OnGround == 0 {
		c.lastVS = m.VS
		c.airborne = true
	}
	// Stopped means not moving: MSFS AI keeps reporting its last commanded
	// ground speed after it stops.
	if c.anchorAt.IsZero() || calc.HaversineMeters(c.anchor.Lat, c.anchor.Lon, pos.Lat, pos.Lon) > StationaryMeters {
		c.anchor, c.anchorAt = pos, c.now()
		c.stillFrom, c.warned = c.now(), false
	}
	stationary := c.now().Sub(c.anchorAt) >= StationarySeconds*time.Second
	off := math.Abs(calc.CrossTrackMeters(c.plan.Runway.Primary.Threshold.Lat, c.plan.Runway.Primary.Threshold.Lon,
		c.plan.Runway.Secondary.Threshold.Lat, c.plan.Runway.Secondary.Threshold.Lon, m.Latitude, m.Longitude))
	// MSFS AI runs its own light logic: our light writes are applied, then
	// overridden. Re-asserting them made the lights flicker (#295), so they
	// are only set at phase changes; see the light events spike.
	if c.state >= ArrivalVacating {
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
			c.touchdownAt = c.now()
			// MSFS AI switches its lights at its own state changes, touchdown
			// among them: set the landing lights again once it has.
			c.setLights(true, false, true, true, true, "lights rollout")
			c.watchGround()
			c.setState(ArrivalRollout, nil)
			return
		case past > c.plan.Runway.Length+300:
			c.fail(ErrNoTouchdown)
			return
		}
	case ArrivalRollout:
		// Hybrid: take over on the runway once the aircraft has settled and
		// slowed, well before the exit, and drive the rest of the rollout,
		// the exit and the ground phase with the lights kept as they should be.
		if c.inj != nil && !c.takeoverTried && m.OnGround != 0 && m.GroundKts <= TakeoverKts &&
			c.now().Sub(c.touchdownAt) >= TakeoverAfterTouchdown &&
			past+c.profile().WheelbaseMeters < c.plan.Exit.Along-TakeoverBeforeExitMeters {
			c.takeoverTried = true
			if err := c.takeover(m, pos, true); err == nil {
				return
			} else {
				c.emit(err, true) // carry on with MSFS AI
			}
		}
		// Vacating once off the runway surface, near or past the planned exit.
		if off > c.plan.Runway.Width/2+RunwayClearMeters && past > c.plan.Exit.Along-100 {
			c.track.pos = c.exitAlong
			if c.inj != nil {
				if err := c.takeover(m, pos, false); err == nil {
					c.setState(ArrivalVacating, nil)
					return
				} else {
					// Carry on with MSFS AI rather than fail the arrival.
					c.mover = nil
					c.emit(err, true)
				}
			}
			c.setLights(true, false, true, true, true, "lights vacating")
			c.setState(ArrivalVacating, nil)
			return
		}
	case ArrivalVacating:
		// Stopped clear of the runway: after-landing lights, then wait.
		// Near the vacate stop: stop there actively, as at the stand, so the
		// AI does not creep on towards its last waypoint.
		if !c.vacateStopSent && c.track.pos >= c.vacateAlong-StandStopMeters {
			c.vacateStopSent = true
			c.stopHere(m, "SetWaypoints vacate stop")
		}
		if stationary && c.track.pos >= c.vacateAlong-VacateArriveMeters {
			c.setLights(false, true, false, true, true, "lights taxi")
			c.clearAt = c.now().Add(c.dwell())
			c.setState(ArrivalAwaitingTaxi, nil)
			return
		}
	case ArrivalAwaitingTaxi:
		if c.cleared || (!c.req.HoldForClearance && !c.now().Before(c.clearAt)) {
			c.startTaxi()
			return
		}
	case ArrivalTaxiing:
		if c.last.Remaining <= c.track.lastSegment()+5 {
			c.setState(ArrivalParking, nil)
			return
		}
	case ArrivalParking:
		stop := c.plan.Stop
		d := calc.HaversineMeters(stop.Lat, stop.Lon, m.Latitude, m.Longitude)
		// Past the mark: the position lies ahead of the stop point along the
		// stand heading.
		brg := calc.BearingDegrees(stop.Lat, stop.Lon, m.Latitude, m.Longitude)
		passed := d > 0.5 && math.Cos((brg-c.standHeading)*math.Pi/180) > 0
		// Stop on the mark: replace the chain with a single waypoint where
		// the aircraft is, so it does not roll on towards the overshoot point.
		if !c.stopSent && !c.req.NoStopWaypoint && (d <= StandStopMeters || passed) {
			c.stopSent = true
			c.stopHere(m, "SetWaypoints stand stop")
		}
		if stationary && (d <= ParkedMeters || c.now().Sub(c.stillFrom) > 10*time.Second) {
			c.setLights(false, false, false, true, true, "lights parked")
			c.stopMonitor()
			if d > ParkedMeters {
				c.last.Err = fmt.Errorf("traffic: stopped %.0f m from the stand", d)
			}
			c.setState(ArrivalParked, c.last.Err)
			return
		}
	}
	if (c.state == ArrivalRollout || c.state == ArrivalVacating || c.state >= ArrivalTaxiing) && !c.warned && c.now().Sub(c.stillFrom) > StuckTimeout {
		c.warned = true
		c.emit(ErrTaxiStuck, true)
		return
	}
	c.emit(nil, false)
}

// Cancel removes the aircraft if it still exists, also once it is parked,
// and ends the controller if it is still running.
func (c *ArrivalController) Cancel() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.picture != nil {
		c.picture.Forget(c.objectID)
	}
	var err error
	c.dropFollowMe()
	if c.objectID != 0 {
		c.stopMonitor()
		c.detail.forget(c.objectID)
		err = c.fleet.Remove(c.objectID, c.reqBase+arrReqRemove)
		if c.inj != nil {
			c.inj.Forget(c.objectID)
		}
		c.objectID = 0
	}
	if !c.state.Terminal() {
		c.setState(ArrivalCancelled, nil)
	}
	return err
}

func (c *ArrivalController) stopMonitor() {
	if client := c.fleet.clientOrNil(); client != nil && c.objectID != 0 {
		client.RequestDataOnSimObject(c.reqBase+arrReqMonitor, c.defBase+arrDefMonitor, c.objectID,
			types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	}
}

func (c *ArrivalController) fail(err error) {
	c.dropFollowMe()
	c.stopMonitor()
	c.setState(ArrivalFailed, err)
}

func (c *ArrivalController) setState(s ArrivalState, err error) {
	c.state = s
	c.emit(err, true)
	if s.Terminal() {
		c.detail.forget(c.objectID)
		close(c.events)
	}
}

func (c *ArrivalController) emit(err error, important bool) {
	if !important && err == nil && !c.lightsChanged && c.fast && c.now().Sub(c.emittedAt) < time.Second {
		return // progress at most once a second while reading every frame
	}
	c.emittedAt, c.lightsChanged = c.now(), false
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

// setLights sets the aircraft's lights by phase: MSFS AI runs its own light
// logic, which switched landing lights and strobes off before the aircraft
// had left the runway and never used taxi lights (#295).
func (c *ArrivalController) setLights(landing, taxi, strobe, beacon, nav bool, desc string) {
	b := func(v bool) float64 {
		if v {
			return 1
		}
		return 0
	}
	c.wantLights = [5]float64{b(landing), b(taxi), b(strobe), b(beacon), b(nav)}
	c.writeLights(desc)
}

func (c *ArrivalController) writeLights(desc string) {
	client := c.fleet.clientOrNil()
	if client == nil || c.objectID == 0 {
		return
	}
	l := c.wantLights
	c.lightsAt = c.now()
	c.note(desc, client.SetDataOnSimObject(c.defBase+arrDefLights, c.objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(l)), unsafe.Pointer(&l)))
}

// stopHere replaces the waypoint chain with a single waypoint at the
// aircraft's position, which stops it where it is.
func (c *ArrivalController) stopHere(m arrivalMonitor, desc string) {
	wp := []types.SIMCONNECT_DATA_WAYPOINT{groundAlt{agl: true}.waypoint(m.Latitude, m.Longitude, 0)}
	err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, wp)
	c.note(desc, err)
	if err != nil {
		c.emit(err, true)
	}
}

// Expedite has the crew vacate the runway faster (#510): RushExitKts
// more at the exit, for a rollout planned from now on.
func (c *ArrivalController) Expedite(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rush = on
}

// ClearToTaxi clears an aircraft waiting clear of the runway to taxi to its
// stand. Before that it takes effect as soon as the aircraft has stopped.
func (c *ArrivalController) ClearToTaxi() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleared, c.hasPendingLimit = true, false
	if c.state == ArrivalTaxiing || c.state == ArrivalHoldingShort {
		c.clearLimit() // no limit any more
	}
	if c.state == ArrivalAwaitingTaxi {
		c.startTaxi()
	}
}

// startTaxi sends the taxi-in chain from the vacate stop to the stand.
func (c *ArrivalController) startTaxi() {
	if c.mover != nil {
		if !c.lights.Taxi && !c.rollThrough {
			c.setInjectedLights(LightsTaxi, "lights taxi") // never taxi without it
		}
		c.mover.ClearHold()
		c.holdNextCrossing()
		if c.hasPendingLimit {
			c.hasPendingLimit = false
			if err := c.applyPendingLimit(c.pendingLimit); err != nil {
				c.note("clearance limit", err)
				c.emit(err, true) // #337: the caller learns the limit was not applied
			}
		}
		c.stillFrom, c.warned = c.now(), false
		c.setState(ArrivalTaxiing, nil)
		return
	}
	err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, c.plan.TaxiWaypoints)
	c.note("SetWaypoints taxi-in", err)
	if err != nil {
		c.fail(err)
		return
	}
	c.stillFrom, c.warned = c.now(), false
	c.setState(ArrivalTaxiing, nil)
}

// note records the send ID of the request just made, so a later
// SimConnect exception can name it.
func (c *ArrivalController) note(desc string, err error) {
	client := c.fleet.clientOrNil()
	if client == nil || c.sent == nil {
		return
	}
	if id, idErr := client.GetLastSentPacketID(); idErr == nil {
		noteSent(c.sent, id, desc)
	}
	_ = err
}

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

// ErrStandFixed is returned by ChangeStand once the aircraft has landed:
// its taxi-in is under way.
var ErrStandFixed = errors.New("traffic: the arrival has landed; its stand is fixed")

// ChangeStand sends the arrival to another stand (parking index), until it
// touches down: the landing and exit stay, the taxi-in is planned from the
// exit to the new stand. For a stand taken meanwhile by other traffic
// (#479: an arrival's stand is reserved when it spawns, 20–40 minutes
// before it lands).
func (c *ArrivalController) ChangeStand(parking int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.plan == nil || c.state > ArrivalLanding {
		return ErrStandFixed
	}
	g := c.req.Graph
	if parking < 0 || parking >= len(g.Layout.Parking) {
		return fmt.Errorf("%w: index %d", airport.ErrUnknownParking, parking)
	}
	// The whole ground part planned again, as Start plans it, with the same
	// exit: the route, the vacate stop, the taxi-in waypoints and the stop
	// on the stand (#87: only the route was replaced; it taxied the new
	// route and parked on the old stop).
	exit := c.plan.Exit
	req := c.req
	plan, err := PlanArrival(g, req.Runway, parking, ArrivalOptions{
		SpawnNm: c.plan.SpawnNm, Exit: &exit, Route: req.Options, GroundAGL: req.GroundAGL, NoseOffset: req.NoseOffset,
		TouchdownKts: approachProfileOf(req).TouchdownKts, BrakeDecel: req.Rollout.BrakeDecel,
	})
	if err != nil {
		return err
	}
	plan.Spawn = c.plan.Spawn
	// MSFS AI landing it (not injected, its procedure flown): the landing
	// chain to the new vacate stop.
	if !req.InjectApproach && !c.flyingProc && c.objectID != 0 && c.state >= ArrivalApproaching {
		if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, plan.Waypoints); err != nil {
			return err
		}
	}
	c.req.Parking = parking
	c.plan = plan
	c.standHeading = g.Layout.Parking[parking].Heading
	c.track = newRouteTracker(plan.Route)
	c.exitAlong = c.track.cum[len(plan.Exit.Path)-1]
	c.vacateAlong = c.track.cum[plan.VacateIndex]
	return nil
}
