package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Traffic control (#322): the map hosts injected arrivals and departures
// and gives their clearances — gates, runway crossings and progressive taxi
// ("taxi up to here" on the route). Controllers run in the connection
// goroutine; HTTP handlers queue commands to it.

// Controller SimConnect ID bases: each controller gets its own block.
const (
	controlDefBase uint32 = 20000
	controlReqBase uint32 = 30000
	controlIDBlock uint32 = 10
	// controlBlocks: controlled aircraft at once; their ID blocks are
	// reused once they are gone (#370).
	controlBlocks uint32 = 128
)

// Stand allocator ID bases, one block per airport, and how often (in
// connection ticks of a second) the stands are scanned for sim aircraft.
const (
	standDefBase   uint32 = 8200
	standReqBase   uint32 = 8300
	standIDBlock   uint32 = 4
	standScanTicks        = 10
)

type controlled struct {
	// tug: a departure's pushback tug (SimObjectTug), for its way on the map.
	tug *traffic.SimObjectTug
	// fuel: a departure's fuel truck (#582), for its way on the map.
	fuel   *traffic.SimObjectFuelTruck
	ID     int          `json:"id"`
	Kind   string       `json:"kind"` // departure | arrival
	Tail   string       `json:"tail"`
	ICAO   string       `json:"icao"`
	dep    departureCtl // nil: not a departure
	arr    arrivalCtl   // nil: not an arrival
	graph  *airport.Graph
	stands *traffic.StandAllocator
	stand  int  // parking index held for this aircraft
	left   bool // a departure has left its stand (released)
	// jetwayGone: its jetway went home for the push (#572), not docked again.
	jetwayGone bool
	// groundStop: held on the stand by the schedule's ground stop; ground
	// does not clear its push meanwhile.
	groundStop bool
	// boardWaitSince: when it began waiting for its jetway to dock.
	boardWaitSince time.Time
	// tugAfter: its tug comes once the jetway is home (drawn at the spawn);
	// tugStand: its stand for that. Atomic: read inside the controller's
	// frame, which must not take it.mu (tugWaitsForJetway).
	tugAfter atomic.Bool
	// other is its other airport (SpawnRequest.Other), for a new stand.
	other string
	// overnight: on its stand through the night (#1294): remote stands
	// first (StandRequirements.Overnight).
	overnight bool

	tugStand atomic.Int32
	// Turnaround (arrival): the departure to start once parked, and
	// departNow to start it before the dwell is over.
	cc         *controlCenter
	turn       *SpawnRequest
	dwell      time.Duration
	departNow  chan struct{}
	turned     bool
	removed    chan struct{} // closed when the aircraft is removed: no turnaround any more
	removeOnce sync.Once
	// spoken marks clearances already in the log (given on the map), so
	// the state change they cause does not log them again.
	spoken map[string]bool
	// managed is the traffic manager of a scheduled flight (#368): it
	// hears the controller's progress. objectID is the aircraft once known.
	managed  *traffic.TrafficManager
	objectID uint32
	// firstPending: its first-contact clearance (the STAR) is not said yet;
	// the approach clearance waits for it (live: ILS before the STAR).
	firstPending atomic.Bool
	// tcasRA: its crew flies a TCAS RA (#450): approach gives it nothing
	// until clear of conflict.
	tcasRA atomic.Bool
	// finished: its controller is done (view.Done), read without it.mu;
	// doneAt when.
	finished atomic.Bool
	doneAt   time.Time
	// observed: a real aircraft's sighting (#841), nil one of the schedule.
	observed *traffic.Sighting
	// deliver: a parked real aircraft's call to delivery, made when its
	// departure is seen (nil: made already); a night stop's at deliverAt
	// (#1294).
	deliver   func()
	deliverAt time.Time
	// clearanceDue: its departure clearance, asked for on a staffed
	// delivery and not given yet (action "clearance"; given by the World
	// once delivery is no longer staffed; amended by the player first,
	// #1164). it.mu.
	clearanceDue *traffic.DepartureClearance
	// arrivalDue: an arrival's clearance (its STAR) after its first call to
	// a staffed approach, not given yet (action "clearance"). it.mu.
	arrivalDue *traffic.Transmission
	// handoffDue: the position it is in now, off a staffed station that
	// does not hand off by itself; the player's "handoff" (handoffGiven)
	// transfers it (#1096). it.mu.
	handoffDue   traffic.Position
	handoffGiven bool
	// pushHeldWho: whom its push was last logged held for (logPushHeld).
	pushHeldWho string
	// clearedFt: the initial climb of its departure clearance (feet), the
	// level a staffed departure climbs to and no further until the player
	// clears it (climbCapped once set); cruiseFt its flight plan's level,
	// asked for (higherAt: when). landCleared: the player said "cleared to
	// land" (#1096). it.mu.
	clearedFt, cruiseFt float64
	climbCapped         bool
	levelSet            bool // held at clearedFt once handed over (crewAsksHigher)
	// approachCleared: the player cleared its approach on a staffed
	// approach; limitHeld: it holds at its clearance limit, the end of its
	// STAR, for want of one (crewAtLimit, #1096).
	approachCleared, limitHeld bool
	higherAt                   time.Time
	landCleared                bool
	defBase                    uint32 // its ID block (cc.ids)
	// gates: the user gives every clearance ("hold at every clearance");
	// otherwise the tower clears it onto and across runways (#393).
	gates atomic.Bool
	// playerAt: when the player last gave it an instruction (unix ns), the
	// safety net's deadline counted from it (#1096).
	playerAt atomic.Int64
	// approach: an arrival's STAR and approach points (the sequencer, #390).
	approach []airport.LatLon
	// atc is the position working it now (#416); atisSaid: its pilot has
	// given the ATIS letter (#418).
	atc      traffic.Position
	atisSaid bool
	// procSaid is its SID or STAR as said ("BALTU 7D"), climbSaid a
	// departure's initial climb ("5000 feet"), heightFt its height on the
	// climb-out, vacateSaid the tower's "when vacated contact ground" (#462).
	procSaid, climbSaid string
	// radarSaid, radarFt: the level departure clears it to once
	// identified (departureLevel), its filed cruise level when lower.
	radarSaid string
	radarFt   float64
	heightFt  float64
	// vacateSaid: set once, by whichever event comes first (live, OKECF
	// and TVS122 told twice in the same millisecond).
	vacateSaid   atomic.Bool
	approachSaid bool // its approach clearance, given on the base
	// emergencyLevel: its emergency (mayday, panpan; "" none) and the code
	// it squawked before 7700, given back when cancelled (emergency.go).
	emergencyLevel, preEmergencySquawk string
	// manualSet: its manual control set by its own toggle, which a staffed
	// station's does not override (staffed.go).
	manualSet bool
	// afterDepFt: the climb restriction tower gave it after departure (feet
	// MSL, 0 none: takeoff_plan.go).
	afterDepFt float64
	// progState: its controller's state last reported as progress;
	// closestNM an arrival's least distance from the field so far.
	progState string
	closestNM float64
	// lastATC, lastATCAt: the last ATC call said to it, and when (say).
	sayMu     sync.Mutex
	lastATC   string
	lastATCAt time.Time
	taxiSaid  string // the last taxi clearance said to it (#1269)
	visual    bool // cleared for a visual approach, as the crew asked (#766)
	// circuit: a VFR arrival's circuit (#568); downwindSaid once it has
	// reported downwind.
	circuit      *traffic.Circuit
	downwindSaid bool
	baseSaid     bool // the base leg reported, at a field without a tower (#1305)
	crossSaid    bool // the crosswind leg reported, at a field without a tower (#1307)
	// wakeCautioned: told of a heavier aircraft ahead, this circuit (#1307);
	// landedNoted: its landing noted for those behind it.
	wakeCautioned atomic.Bool
	landedNoted   bool
	ifrCancelled bool // cancelled IFR with approach, at a field without a tower (#1305)
	// tngRolling: on the runway for a touch-and-go; once in the circuit
	// again its calls start afresh (#569).
	tngRolling bool
	stopAndGo  bool // its touch-and-goes are stop-and-goes
	// vfrPoint: the reporting point it enters or leaves by (#566).
	vfrPoint *traffic.ReportingPoint
	// circuitJoin: where the tower joins it (LegFinal: straight in).
	circuitJoin traffic.CircuitLeg
	// exitTwy: the taxiway an arrival vacated by, for its report.
	exitTwy string
	// standTakenSaid: the last "stand taken, no other" logged (E30).
	standTakenSaid string
	readySaid      bool // a departure's "ready for departure"
	// askedEntry: the intersection the crew asked to depart from with its
	// taxi request, answered with the taxi clearance (#621).
	askedEntry string
	// entryRemaining: the runway left from the intersection it was given
	// (meters, 0 none); entryDeclined: its crew declines the one just
	// offered (#1030).
	entryRemaining float64
	entryDeclined  bool
	// askedRunway: the runway the crew asked to depart from instead (#621),
	// answered with the taxi clearance; "" none.
	askedRunway string
	// identified: departure has identified it (its check-in answered,
	// "identified[, climb to ...]"), it.mu (#698).
	identified bool
	// identifiedAt: when it was identified (levelInTrail).
	identifiedAt time.Time
	// directAnswer: the crew's request for direct asked before it was
	// identified, answered in the same call, it.mu.
	directAnswer func() (traffic.Transmission, bool)
	// rush: told to hurry (#510): its clearances are the expedited ones.
	rush atomic.Bool
	// joinPending: a VFR arrival's join call not said yet; no sequencing
	// instruction before it (live, OKIMV "extend downwind" then "join
	// right downwind", #711). placeSaid: its place in the circuit as told.
	joinPending atomic.Bool
	// wake: its wake category, said in its first calls (initial).
	wake      traffic.WakeCategory
	placeSaid atomic.Int32
	// extendSaid: told to extend its downwind on this circuit (said once,
	// however often it is extended further).
	extendSaid atomic.Bool
	// goAroundCalling: gone around and handed to approach, not taken in
	// with "radar contact" yet: approach has nothing else to say to it
	// (live, TVS354 told "number 5, extend downwind" before tower's
	// "contact Ruzyne Radar").
	goAroundCalling atomic.Bool
	// vectorsSaid counts the vectors said on their own (each turn as it
	// comes): a sequence call waiting for the frequency leaves out the
	// vector it was decided with once a later one went out first.
	vectorsSaid atomic.Int32
	// vectoringSaid: the approach it is vectored for was said, with its
	// first vector off the STAR (vectored).
	vectoringSaid atomic.Bool
	// handoffFt and towerAtM: where this departure goes to departure
	// (height) and to tower (meters short of the runway), varied.
	handoffFt, towerAtM float64
	// called: the request its crew has said (a request waits for delivery
	// and boarding), what a controller is answering (#1308).
	called string
	// request is what the crew asks for now (TaxiEvent.Request, #462);
	// waiting one made before its clearance was done (delivered: the
	// delivery exchange finished and the aircraft with ground).
	request, waiting string
	delivered        bool
	// pushAndStart: the crew asked for the pushback and the start-up in one
	// call, approved together.
	pushAndStart bool
	// givingWay is the aircraft it gives way to (TaxiEvent.GivingWayTo),
	// told by ground once; toldWay the last one it was told to give way to
	// (tellGiveWay), not told again.
	givingWay uint32
	// giveWayHoldShort is the taxiway it stops short of to give way
	// (TaxiEvent.GiveWayHoldShort); heldShortOf the one it was told to hold
	// short of, so it is told to continue once clear.
	giveWayHoldShort, heldShortOf string
	toldWay                       uint32
	toldWayAt                     time.Time // when it was last told to give way
	// stoppedBy is why it stands still taxiing (TaxiEvent.StoppedBy).
	stoppedBy string
	// pushHeldBy: the object its push holds for, as last logged.
	pushHeldBy uint32
	// missedJoin is the arrival's last MissedJoin, acted on once.
	missedJoin string
	// fixes: the named points of its procedure (STAR and approach, or SID),
	// the dots of its air route on the map — not the points of the turns.
	fixes []airFix

	mu   sync.Mutex
	view ControlView
	// spot: view's runway and position for station, which runs with it.mu
	// held and without; stored with them (keepSpot).
	spot atomic.Pointer[stationSpot]
}

// stationSpot is where it is for picking its station.
type stationSpot struct {
	runway string
	pos    airport.LatLon
}

// keepSpot stores view's runway and position in spot; it.mu is held.
func (it *controlled) keepSpot() {
	it.spot.Store(&stationSpot{runway: it.view.Runway, pos: it.view.Position})
}

// ControlView is what the map shows of a controlled aircraft.
// tugView is a departure's tug: where it is and the way it still drives.
type tugView struct {
	Position airport.LatLon   `json:"position"`
	Heading  float64          `json:"heading"`
	Route    []airport.LatLon `json:"route,omitempty"`
}

// VehicleView is one of an aircraft's ground vehicles (the pushback tug,
// the fuel truck): what it is, the sim object (a host matches it to the
// objects it shows), where it is, the way still ahead and its state.
type VehicleView struct {
	Kind     string               `json:"kind"` // tug, fuel
	ObjectID uint32               `json:"objectId"`
	Title    string               `json:"title"`
	State    traffic.VehicleState `json:"state"`
	Position airport.LatLon       `json:"position"`
	Heading  float64              `json:"heading"`
	Route    []airport.LatLon     `json:"route,omitempty"`
}

func vehicleView(kind string, id uint32, title string, state traffic.VehicleState, track func() (traffic.GroundPose, []airport.LatLon, bool)) VehicleView {
	v := VehicleView{Kind: kind, ObjectID: id, Title: title, State: state}
	if p, route, ok := track(); ok {
		v.Position, v.Heading, v.Route = p.Position, p.Heading, route
	}
	return v
}

type ControlView struct {
	ID int `json:"id"`
	// ATC and Frequency: the position working it and its frequency (#416).
	ATC       string `json:"atc,omitempty"`
	Frequency string `json:"frequency,omitempty"`
	Kind      string `json:"kind"`
	// Rules: the flight rules it flies under, "IFR" or "VFR" (a circuit
	// arrival).
	Rules  string `json:"rules"`
	ICAO   string `json:"icao"` // its airport
	Tail   string `json:"tail"`
	Squawk string `json:"squawk,omitempty"` // a departure's SSR code
	// Tug: its pushback tug while it drives (from its depot or home), with
	// the way still ahead.
	Tug  *tugView `json:"tug,omitempty"`
	Fuel *tugView `json:"fuel,omitempty"` // its fuel truck while it drives (#582)
	// Rescue: the fire trucks or the ambulance out for it (rescue.go).
	Rescue []tugView `json:"rescue,omitempty"`
	// Vehicles: its ground vehicles with their object and state (#710).
	Vehicles     []VehicleView `json:"vehicles,omitempty"`
	Model        string        `json:"model"`
	Stand        string        `json:"stand"`
	Runway       string        `json:"runway"`
	Procedure    string        `json:"procedure,omitempty"` // SID, or STAR → approach
	OnGround     bool          `json:"onGround"`
	PushbackHeld bool          `json:"pushbackHeld,omitempty"` // the pushback waits for traffic behind
	Rush         bool          `json:"rush,omitempty"`         // told to hurry (#510)
	Manual       bool          `json:"manual,omitempty"`       // the user gives its clearances, no automation
	Entry        string        `json:"entry,omitempty"`        // a departure's runway entry ("" full length)
	Deicing      bool          `json:"deicing,omitempty"`      // being de-iced
	State        string        `json:"state"`
	// Request is what the crew has called for and is waiting on, in the
	// engine's own words: "pushback", "start_up", "taxi"; "" when it has not
	// called, as with delivery still or boarding (#462, #1308).
	//
	// IT IS WHAT THE OFFERED LIST CANNOT SAY. Actions names what a controller
	// MAY give; a departure cleared to push and start is offered nothing at
	// all, correctly, and a strip with no buttons and no reason reads as a
	// stuck game. This is the other half: whose move it is.
	Request string `json:"request,omitempty"`
	// ClearedEarly are the clearances a player gave it ahead of its crew: not
	// called for yet, or before its IFR clearance (#1308). They are taken, as
	// a controller can expedite; a host decides what they mean.
	ClearedEarly []string `json:"clearedEarly,omitempty"`
	// ActionParams names, of Actions, those that cannot be given on their own
	// and the query parameter they need: "upto" → "node", "climb" → "level"
	// (#1308).
	ActionParams map[string]string `json:"actionParams,omitempty"`
	// SayingUntil: what is booked on the radio to or from it has been said by
	// then, on the World's clock (#1312). A clearance is taken at once and
	// said when the frequency clears: until this time it is queued; absent or
	// past, nothing is.
	SayingUntil    *time.Time `json:"sayingUntil,omitempty"`
	HoldingShortOf string     `json:"holdingShortOf,omitempty"`
	// TaxiRemainingM: a departure's taxi still to go to its runway (m).
	TaxiRemainingM float64        `json:"taxiRemainingM,omitempty"`
	AtLimit        bool           `json:"atLimit"`
	LimitNode      int            `json:"limitNode"`
	Position       airport.LatLon `json:"position"`
	Heading        float64        `json:"heading"`
	// AltFt: its altitude (feet MSL) as the simulator reports it; 0 not seen.
	AltFt       float64          `json:"altFt,omitempty"`
	GroundSpeed float64          `json:"groundSpeed"`
	Lights      string           `json:"lights"`
	Error       string           `json:"error,omitempty"`
	Route       []airport.LatLon `json:"route"`
	Nodes       []airport.NodeID `json:"nodes"`
	Actions     []string         `json:"actions"` // clearances available now
	// AirRoute is what it still flies in the air: an arrival's STAR and
	// approach (with any dog-leg), a departure's SID once handed to MSFS AI;
	// Hold its hold when holding (#391, #392).
	AirRoute []airport.LatLon `json:"airRoute,omitempty"`
	// Real: a real aircraft from a feed (#841), ObservedID its own ID there
	// and Registration its registration ("" unknown).
	// Instruction is the last approach instruction given on the map
	// (#443), InstructionAt the point it was about, if any.
	Instruction   string          `json:"instruction,omitempty"`
	InstructionAt *airport.LatLon `json:"instructionAt,omitempty"`
	// TCAS is its advisory now (#450): TA or RA; nil none.
	TCAS *TCASView `json:"tcas,omitempty"`
	Real bool      `json:"real,omitempty"`
	// Seeded: placed from a real-world snapshot at the start (World.Seed,
	// "real start"), flown on as ours; not Real.
	Seeded       bool   `json:"seeded,omitempty"`
	ObservedID   string `json:"observedId,omitempty"`
	Registration string `json:"registration,omitempty"`
	// AirFixes are the named fixes still ahead on AirRoute: its dots (the
	// route itself also runs through the points of its rounded turns).
	AirFixes []airFix  `json:"airFixes,omitempty"`
	Hold     *holdView `json:"hold,omitempty"`
	// Sequence is an arrival's place in its airport's landing sequence
	// (number, leader, spacing); nil when not sequenced.
	Sequence *traffic.SequenceEntry `json:"sequence,omitempty"`
	Done     bool                   `json:"done"`
}

type controlCenter struct {
	// runwayQueues: departures that may wait at a runway end, by "ICAO
	// end" (runwayQueue, intersections.go).
	runwayQueues sync.Map
	// core is the engine state that outlives this connection; log its
	// traffic log (console, file, /api/control/log).
	core *core
	log  *trafficLog
	// startedAt: when this connection's traffic started (jetwaysReady).
	startedAt time.Time
	// onTransmission hears every transmission once logged; onChange is told
	// a part of the picture changed ("control", "radio"): the front end's
	// voice, camera and open maps. Set before the connection runs; nil none.
	onTransmission func(traffic.Transmission)
	onChange       func(topic string)
	// assigned is each aircraft's last clearance as said (its level, its
	// approach), named when it returns to it after a TCAS RA.
	assignedMu sync.Mutex
	assigned   map[string]string
	client     engine.Client
	fleet      *traffic.Fleet
	// sim is what it does to the simulator beside the controllers (#710);
	// onModels is told the aircraft titles found (an actuator's, for its
	// director).
	sim      simPort
	onModels func([]string)
	// onGroundTitles is told the ground vehicle titles found (an
	// actuator's); remoteVehicles gives a remote aircraft's vehicles (a
	// director's, nil otherwise).
	onGroundTitles func([]string)
	remoteVehicles func(target string) []VehicleView
	inj            *traffic.Injector
	cmds           chan func()

	mu    sync.Mutex
	next  int
	items map[int]*controlled
	// standCheckAt: the last recheckArrivalStands (#479).
	// runwayCheckAt: the last checkRunways; runwaysNow each airport's
	// departure and arrival runway then (#456).
	runwayCheckAt time.Time
	runwaysNow    map[string]string
	standCheckAt  time.Time
	// keepClearAt, keepClearKey: the last keepClear and the places taken then.
	keepClearAt  time.Time
	keepClearKey string
	// opposedTried: pairs on routes the opposite way along one taxiway
	// looked at (keepClearOfOncoming), by "tail|tail", when.
	opposedTried map[string]time.Time
	models       map[string]bool                    // aircraft titles the simulator offers
	fuelTitles   fuelTitles                         // fuel vehicles the simulator offers (#582)
	groundTitles []string                           // every ground vehicle title the simulator offers, sorted
	stands       map[string]*traffic.StandAllocator // by ICAO
	// picture is what the controlled aircraft know of each other and of the
	// sim's other aircraft on the ground (#334).
	// ids hands out the controllers' ID blocks and takes them back (#370);
	// detail drives far and standing aircraft on fewer frames.
	ids *traffic.IDBlocks
	// spawnedAt: where arrivals appeared lately (the scan sees them only a
	// second later).
	spawnedAt []spawnPoint
	// runways keep each airport's runway in use.
	detail *traffic.Detail
	// radio carries what our controllers say (#415): logged as ATC, served
	// at /api/radio.
	radio *traffic.Radio
	// clock is traffic time: the simulation rate, stopped while paused
	// (#413). Traffic runs on it; logs and data freshness on the wall clock.
	clock *traffic.SimClock
	// own are aircraft of ours not driven by a controller: enroute and
	// overflying MSFS AI of the scheduled traffic (#369). extra handles
	// the scheduler's messages.
	own   map[uint32]bool
	extra func(engine.Message) bool
	// pending runs clearances and actions at their traffic time (#462).
	pending *pending
	// climbStopped reports a climb or descent stopped for traffic and not
	// yet cleared on (the conflict watch); nil none.
	climbStopped func(cs string) bool
	// landCleared reports an arrival cleared to land (the tower); nil none.
	landCleared func(cs string) bool
	// departureCleared reports a departure cleared to line up or take off,
	// departuresAhead the departures to go before one (the tower); nil none.
	departureCleared func(cs string) bool
	departuresAhead  func(icao, runway, cs string) int
	// tcasView is one of ours' TCAS advisory now (#450); nil hook or quiet: nil.
	tcasView func(objectID uint32) *TCASView
	// agenda: the controllers' calls, most urgent first (agenda.go).
	agenda *agenda
	// saidCallsign writes a call sign as said (#462); set once the schedule
	// exists.
	saidCallsign func(cs string) string
	// namedAirport is an airport as a clearance names it (#462).
	namedAirport func(icao string) string
	// atisLetter is an airport's current ATIS letter for first calls (#418).
	atisLetter func(icao string) string
	// lineUpBehind clears a departure to line up behind the next arrival,
	// behindSaid names that arrival (#509): the towers'.
	lineUpBehind func(it *controlled) error
	behindSaid   func(it *controlled) string
	// rejoin sequences an arrival afresh after a go-around (#394);
	// sequencesAt gives an airport's landing sequences by runway (#396).
	rejoin func(icao, tail string)
	// followed keeps tail landing after lead, as the tower told it
	// ("number 2, follow …"; ApproachSequencer.Behind).
	followed func(icao, tail, lead string)
	// forgetTower drops what the tower gave a call sign (spawned again).
	forgetTower func(tail string)
	// forgetFlight drops what approach and the conflict watch remember of a
	// call sign, spawned again as a new flight (#75).
	forgetFlight func(tail string)
	sequencesAt  func(icao string) map[string][]traffic.SequenceEntry
	// staffedAt, staffedAny: the last look at staffed stations, and whether
	// anyone was on one then (staffed.go).
	staffedAt  time.Time
	staffedAny bool
	// rescue: the fire and rescue crews out (rescue.go).
	rescue rescueCrews
	// changeDeadline: when a runway change's departures from the other end
	// of arrival runway end key ("LKPR 24") should have gone; false none.
	changeDeadline func(key string) (time.Time, bool)
	// plannedTraffic: everyone airborne but a call sign, with the planned
	// way ahead of ours (scheduler.plannedTraffic); nil without a schedule.
	plannedTraffic func(callsign string) ([]traffic.TrackedAircraft, map[string][]traffic.RoutePoint)
	// toFinal clears an arrival that missed its join direct to the final.
	toFinal func(icao, tail string)
	// world is the traffic picture around the centre of the world (#366):
	// every aircraft, the airports in range, a ground picture per airport.
	world *traffic.TrafficPicture
	ticks int
	// pads are an airport's de-icing pads (picked on the map, #323).
	pads func(l *airport.Layout) []airport.DeicingPad
	// localStations are an airport's own ATC stations (#722).
	localStations func(icao string) []traffic.Station
	// vehATC: the service vehicles' calls and clearances (#752).
	vehATC  *vehicleATC
	vehOnce sync.Once
	// weather is the latest at the user aircraft (automatic de-icing, #323).
	weather func() *nav.Weather
	// procedures gives an airport's SIDs, STARs and approaches (#315).
	procedures func(icao string) (airport.Procedures, bool)
	// The ATC game (#272): its state, the taxi graphs, the last traffic
	// scan and when the game last ran.
	game  *game
	graph func(icao string) (*airport.Graph, error)
	// layout gives a loaded airport's layout (the cache), for where on the
	// airfield other traffic is (TrafficPicture, #623).
	layout func(icao string) (*airport.Layout, bool)
	scan   []Traffic
	// scanNow: the same for the camera (spots.go).
	scanNow atomic.Pointer[scanned]
	gameAt time.Time
	// jw: jetways for our traffic (jetways.go, #572); airportRefs the
	// airports the simulator knows (a retract plan's destination).
	jw          jetwayState
	airportRefs func() map[string]traffic.AirportRef
	// landed is the last landing per airport and runway (noteLanded, #1307).
	landed sync.Map
	// selfAnnounceOff: tuned off (ScheduleTuning.SelfAnnounce, #1303).
	selfAnnounceOff atomic.Bool
}

func newControlCenter(client engine.Client, k *core) *controlCenter {
	log := k.log
	cc := &controlCenter{
		core: k, log: log, client: client, fleet: traffic.NewFleet(client), inj: traffic.NewInjector(client, traffic.InjectorWithIDs(k.libIDs().injDef, k.libIDs().injReq, k.libIDs().injEvt)), clock: traffic.NewSimClock(), startedAt: traffic.Now(),
		cmds: make(chan func(), 16), items: map[int]*controlled{},
		models:   map[string]bool{},
		own:      map[uint32]bool{},
		assigned: map[string]string{},
		ids:      traffic.NewIDBlocks(controlDefBase, controlReqBase, controlIDBlock, controlBlocks),
		detail:   traffic.NewDetail(),
		stands:   map[string]*traffic.StandAllocator{},
		game:     &game{log: log},
	}
	// Other traffic's phase by where it is on the airfield: the loaded
	// layouts (cc.layout, set once the cache is there).
	cc.world = traffic.NewTrafficPicture(traffic.PictureOptions{Centre: traffic.Centre{FollowUser: true},
		Layout: func(icao string) *airport.Layout {
			if cc.layout == nil {
				return nil
			}
			if l, ok := cc.layout(icao); ok {
				return l
			}
			return nil
		}})
	cc.sim = &localSim{client: client, fleet: cc.fleet, inj: cc.inj, detail: cc.detail, world: cc.world, clock: cc.clock, logf: cc.log.printf}
	cc.pending = newPending()
	cc.agenda = &agenda{radio: func(icao, freq string) time.Time { return cc.radio.ClearAt(icao, freq) }, log: cc.log.printf}
	cc.radio = traffic.NewRadio(traffic.RadioOptions{Now: cc.clock.Now, ReadBack: true,
		// Crews answer their own way (#721); POST /api/radio/variety.
		Variety:     &traffic.Variety{Seed: 721},
		FrequencyOf: func(icao string, pos traffic.Position) string { _, f := cc.stationOf(icao, pos); return f },
		// One person on several frequencies: one voice (#722).
		ControllerOf: func(icao, freq string) string { return traffic.ControllerOn(cc.stations(icao), freq) },
		// A field without a tower: its crews announce themselves (#1296).
		Uncontrolled: cc.uncontrolled,
		// Faster on a busy frequency and after a safety call, as a
		// controller speeds up.
		TempoOf: func(icao, freq string) float64 { return cc.agenda.tempo(icao, freq, cc.clock.Now()) },
		// Call signs as said, in the text and so in the voice (#462).
		SaidCallsign: func(cs string) string {
			if f := cc.saidCallsign; f != nil {
				return f(cs)
			}
			return defaultSchedule.SaidCallsign(cs)
		},
		OnTransmission: func(t traffic.Transmission) {
			who := "ATC"
			if t.Pilot {
				who = "pilot"
			}
			cc.log.printf("%-6s %s: %s", t.Callsign, who, t.Text)
			if s, ok := traffic.AssignedClearance(t); ok {
				cc.assignedMu.Lock()
				cc.assigned[t.Callsign] = s
				cc.assignedMu.Unlock()
			}
			if f := cc.onTransmission; f != nil {
				f(t) // the front end: its voice, camera, open maps
			}
		}})
	return cc
}

// initial is t, its first call to an ATS unit, with "heavy" or "super"
// after the call sign for its wake category (Doc 4444 4.9.2).
func (it *controlled) initial(t traffic.Transmission) traffic.Transmission {
	return traffic.WithWake(t, it.wake)
}

// changed tells the front end a part of the picture changed.
func (cc *controlCenter) changed(topic string) {
	if f := cc.onChange; f != nil {
		f(topic)
	}
}

// do runs f in the connection goroutine and waits for it.
func (cc *controlCenter) do(f func() error) error {
	done := make(chan error, 1)
	// 0 waiting, 1 running, 2 given up: what times out never runs later, and
	// what runs is waited for (#74: reported failed, it ran on anyway: an
	// orphan spawn, a stand released under an aircraft).
	var state atomic.Int32
	run := func() {
		if state.CompareAndSwap(0, 1) {
			done <- f()
		}
	}
	select {
	case cc.cmds <- run:
	case <-traffic.After(5 * time.Second):
		return errors.New("simulator connection busy")
	}
	select {
	case err := <-done:
		return err
	case <-traffic.After(10 * time.Second):
		if state.CompareAndSwap(0, 2) {
			return errors.New("simulator did not answer")
		}
		return <-done // running: its own result
	}
}

// allocator returns the stand allocator of g's airport, creating it.
func (cc *controlCenter) allocator(g *airport.Graph) *traffic.StandAllocator {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if a := cc.stands[g.Layout.ICAO]; a != nil {
		return a
	}
	k := uint32(len(cc.stands))
	a := traffic.NewStandAllocator(cc.client, g, traffic.StandWithIDs(standDefBase+k*standIDBlock, standReqBase+k*standIDBlock))
	cc.stands[g.Layout.ICAO] = a
	cc.world.Allocate(g.Layout.ICAO, a) // fed from the picture's scans
	// Every stand's taxi-in from each runway end worked out now, in the
	// background: an arrival's stand pick then never runs the searches on
	// the connection's goroutine (docs/traffic-smoothness.md).
	var ends []string
	for _, r := range g.Layout.Runways {
		ends = append(ends, r.Primary.Name, r.Secondary.Name)
	}
	go a.WarmTaxiIn(ends)
	return a
}

// tick runs every second in the connection goroutine: it scans the stands
// the ATC game.
func (cc *controlCenter) tick() {
	if now := cc.clock.Now(); now.Sub(cc.gameAt) >= time.Second {
		cc.gameAt = now
		cc.timed("game", func() { cc.gameTick(now) })
		cc.timed("rescue", func() { cc.rescueTick(now) })
		cc.timed("calls due", func() { cc.callsDue(now) })
	}
	// The stand allocators are fed by the traffic picture (Allocate): no
	// scans of their own.
	if now := cc.clock.Now(); now.Sub(cc.standCheckAt) >= 10*time.Second {
		cc.standCheckAt = now
		cc.timed("arrival stands", cc.recheckArrivalStands)
		cc.timed("prune", cc.pruneDone)
	}
	cc.timed("keep clear", cc.keepClear)
	cc.timed("jetways", func() { cc.jetwayTick(cc.clock.Now()) })
}

// callsDue makes the first call of the parked aircraft whose time came:
// night stops calling delivery DepartureLead before their STD (#1294).
func (cc *controlCenter) callsDue(now time.Time) {
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		deliver := it.deliver
		if deliver == nil || it.deliverAt.IsZero() || now.Before(it.deliverAt) {
			it.mu.Unlock()
			continue
		}
		it.deliver, it.deliverAt = nil, time.Time{}
		it.mu.Unlock()
		deliver()
	}
}

// timed runs f, logging it when it takes longer than slowStep: the tick
// runs on the connection's goroutine, and its slow parts hold every
// aircraft back (docs/traffic-smoothness.md).
func (cc *controlCenter) timed(name string, f func()) {
	t0 := traffic.Now()
	f()
	if d := traffic.Since(t0); d > slowStep {
		cc.log.printf("slow: tick's %s took %s (traffic frames held back)", name, d.Round(time.Millisecond))
	}
}

// pruneDone drops the flights whose controller is done and whose aircraft
// has gone from the simulator (#79: they stayed for good, their ID blocks
// ran out after 128, ErrNoIDs).
func (cc *controlCenter) pruneDone() {
	present := map[uint32]bool{}
	for _, a := range cc.world.Aircraft() {
		present[a.ObjectID] = true
	}
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		id, since := it.objectID, traffic.Since(it.doneAt)
		it.mu.Unlock()
		// One that never got its aircraft waits a minute: a late creation
		// still reaches it, to be removed (#89).
		if it.finished.Load() && (id != 0 && !present[id] || id == 0 && since > time.Minute) {
			cc.forget(it)
		}
	}
}

// recheckArrivalStands moves an arrival that has not landed to another
// stand when other traffic has parked on its own (#479: reserved when it
// spawned, 20–40 minutes before; a reservation does not stop MSFS AI or the
// user). Runs in the connection goroutine.
func (cc *controlCenter) recheckArrivalStands() {
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		if it.arr == nil || it.stands == nil || it.arr.State() > traffic.ArrivalLanding {
			continue
		}
		it.mu.Lock()
		old, tail, obj, rwy, model := it.stand, it.Tail, it.objectID, it.view.Runway, it.view.Model
		it.mu.Unlock()
		why := it.stands.TakenFrom(old, tail, obj)
		if why == "" {
			continue
		}
		half := traffic.DefaultHalfSpanMeters
		if o, ok := it.stands.Occupant(old); ok && o.HalfSpan > 0 {
			half = o.HalfSpan
		}
		it.stands.Release(old)
		l := it.graph.Layout
		newStand, err := it.stands.Assign(traffic.StandRequirements{Owner: tail, Airline: airlineOf(tail), HalfSpan: half, Runway: rwy, NoJetway: traffic.NoJetway(model), Overnight: it.overnight, Bridged: cc.jetwayStands(it.graph.Layout.ICAO), Prefer: cc.jetwayGates(it.graph.Layout.ICAO), Other: it.other})
		if err == nil {
			err = it.arr.ChangeStand(newStand)
			if err != nil {
				it.stands.Release(newStand)
			}
		}
		if err != nil {
			_ = it.stands.Occupy(old, tail, half) // keep what it had; it will wait there
			// Once per stand and reason: looked at every 10 s, the same line
			// filled the log while it lasted (E30).
			msg := fmt.Sprintf("stand %s taken (%s), no other: %v", l.Parking[old].Label(), why, err)
			it.mu.Lock()
			said := it.standTakenSaid == msg
			it.standTakenSaid = msg
			it.mu.Unlock()
			if !said {
				cc.log.printf("%-6s arrival: %s", tail, msg)
			}
			continue
		}
		it.mu.Lock()
		it.stand, it.view.Stand = newStand, l.Parking[newStand].Label()
		it.tugStand.Store(int32(newStand))
		mgr, model, label := it.managed, it.view.Model, it.view.Stand
		it.mu.Unlock()
		if mgr != nil {
			mgr.Describe(tail, model, label, rwy) // the board shows the new stand
		}
		cc.log.printf("%-6s arrival: stand %s taken (%s), now %s", tail, l.Parking[old].Label(), why, l.Parking[newStand].Label())
	}
}

// handle passes a message to the injector and every controller.
func (cc *controlCenter) handle(msg engine.Message) bool {
	if svc := cc.jetwayBuilt(); svc != nil && svc.Handle(msg) {
		return true
	}
	cc.mu.Lock()
	allocs := make([]*traffic.StandAllocator, 0, len(cc.stands))
	for _, a := range cc.stands {
		allocs = append(allocs, a)
	}
	cc.mu.Unlock()
	for _, a := range allocs {
		if a.Handle(msg) {
			return true
		}
	}
	if cc.rescueHandle(msg) {
		return true
	}
	if cc.extra != nil && cc.extra(msg) {
		return true
	}
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
		switch e := msg.AsSimObjectAndLiveryEnumeration(); uint32(e.DwRequestID) {
		case reqModels:
			cc.addModels(msg)
			return true
		case reqGroundVehicles:
			cc.addFuelTitles(msg)
			return true
		}
	}
	if ok, err := cc.inj.Handle(msg); ok {
		if err != nil {
			fmt.Fprintf(stdout, "⚠️  injector: %v\n", err)
		}
		return true
	}
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		t0 := traffic.Now()
		if it.dep != nil && it.dep.Handle(msg) || it.arr != nil && it.arr.Handle(msg) {
			if d := traffic.Since(t0); d > slowStep {
				it.mu.Lock()
				state := it.view.State // under its lock: update writes it on another goroutine
				it.mu.Unlock()
				cc.core.log.printf("%-6s slow: its frame took %s (%s)", it.Tail, d.Round(time.Millisecond), state)
			}
			return true
		}
	}
	return false
}

// SpawnRequest asks for a controlled departure or arrival.
type SpawnRequest struct {
	Kind   string `json:"kind"` // departure | arrival
	ICAO   string `json:"icao"`
	Stand  int    `json:"stand"` // parking index
	Runway string `json:"runway"`
	Entry  string `json:"entry"` // departure: runway entry taxiway
	Exit   *int   `json:"exit"`  // arrival: runway exit, an index into /api/exits; nil = the controller's choice
	// StandUse: the stands it takes first (#833): "" a passenger
	// airline's (gates), "ga" (GA ramps; a circuit flight always), "cargo".
	StandUse string `json:"standUse,omitempty"`
	// PushInMin: a departure's push this many minutes from now, its stand
	// services (fuel, stairs, the tug) timed by it; 0: none set.
	PushInMin float64 `json:"pushInMin,omitempty"`
	Model     string  `json:"model"`
	Tail      string  `json:"tail"`
	// Squawk: a departure's SSR code, four octal digits; "": its own
	// (squawkFor).
	Squawk         string `json:"squawk"`
	Gates          bool   `json:"gates"`          // hold at every clearance
	InjectApproach bool   `json:"injectApproach"` // arrival: fly the approach by injection (the default: see AILanding)
	// AILanding: MSFS AI flies the approach and lands (only when asked, an
	// explicit injectApproach false; it lands at -1000 fpm). Every other
	// arrival flies our injected approach.
	AILanding bool `json:"-"`
	Tug       bool `json:"tug"` // departure: a pushback tug (GSX model)
	// Fuel: a departure is refuelled on its stand when it waits long
	// enough (a schedule's), by a fuel truck or hydrant dispenser (#582).
	Fuel     bool     `json:"fuel"`
	TugTitle string   `json:"tugTitle"` // ground vehicle title; "" = traffic.DefaultTugTitle
	TugYaw   *float64 `json:"tugYaw"`   // tug heading against the aircraft, degrees (default traffic.TugYawDeg)
	TugAhead *float64 `json:"tugAhead"` // tug reference point ahead of the nose gear, meters (default traffic.TugAheadMeters)
	// Turnaround (arrival): once parked the same aircraft departs again
	// from its stand after DwellSec (±20 %, default 90 s; the "depart"
	// action skips the wait), from the same runway (#296).
	Turnaround bool    `json:"turnaround"`
	DwellSec   float64 `json:"dwellSec"`
	// Procedure: a departure flies a SID after the take-off, an arrival
	// appears at a STAR's first fix and flies it and the approach (#315).
	// ProcName picks the SID or STAR; "" picks one for the runway.
	Procedure bool   `json:"procedure"`
	ProcName  string `json:"procName"`
	// Circuit: a VFR flight through the circuit (#568), the runway end's
	// as set on the map (circuits.go). An arrival appears at the 45° entry
	// to the downwind and lands from the base; a departure leaves the
	// circuit towards ExitBearing (true degrees from the field; nil: at
	// random) to a point VFRExitNM out (Circuit.Departure), without a
	// departure clearance.
	Circuit     bool     `json:"circuit"`
	ExitBearing *float64 `json:"exitBearing"`
	// TouchAndGos: a VFR arrival makes this many touch-and-goes before its
	// full stop, flying the circuit again after each (#569).
	TouchAndGos int `json:"touchAndGos"`
	// StopAndGo: each of them a stop-and-go (#567).
	StopAndGo bool `json:"stopAndGo"`
	// VFRPoint: the reporting point a VFR flight enters or leaves by
	// (#566, vfrpoints.go); "" one of the airport's by its call sign, or
	// none when the airport has none (ExitBearing set: none).
	VFRPoint string `json:"vfrPoint"`
	// Other is the destination of a departure or the origin of an arrival
	// (ICAO): the flight follows a generated flight plan (#331), and takes
	// the stand areas of its side of the Schengen border first
	// (traffic.StandAreaFor).
	Other string `json:"other"`
	// Via and Taxiways shape the taxi route (#340): route nodes to pass
	// and taxiways to follow, in order.
	Via      []airport.NodeID `json:"via"`
	Taxiways []string         `json:"taxiways"`
	// Deice (departure, #323): "" none, "auto" when the weather calls for
	// it, "stand", or "pad" (the first via point, else the airport's pad).
	Deice    string  `json:"deice"`
	DeiceSec float64 `json:"deiceSec"`

	planned *planned // Other's flight plan, resolved before the spawn

	adopt  uint32    // departure: the aircraft already on the stand (turnaround)
	pushAt time.Time // departure: stay on the stand until then (its STD)
	// parked: a real aircraft standing with no departure yet (#841): it
	// calls delivery only when one comes (controlled.deliver); a night
	// stop (#1294) at callAt.
	parked bool
	callAt time.Time
	// overnight: on its stand through the night (#1294): remote stands
	// first (#1298).
	overnight bool
	// offBlock: an arrival's turnaround departure time, for its stand
	// (StandRequirements.OffBlock).
	offBlock time.Time
}

// Turnaround dwell when none is given, and its spread.
const (
	defaultDwell = 90 * time.Second
	dwellSpread  = 0.2
)

func (cc *controlCenter) spawn(g *airport.Graph, r SpawnRequest) (*controlled, error) {
	spawnAt, startTook := traffic.Now(), time.Duration(0)
	defer func() {
		if d := traffic.Since(spawnAt); d > slowStep {
			cc.log.printf("%-6s slow spawn: %s (%s), its controller started in %s", r.Tail, d.Round(time.Millisecond), r.Kind, startTook.Round(time.Millisecond))
		}
	}()
	// The runway in use: with parallels used together, an arrival takes
	// the less busy one (its stand is then found near it), a departure the
	// one nearest its stand once that is known.
	auto := r.Runway == "" || strings.EqualFold(r.Runway, "active")
	if cc.forgetFlight != nil && r.Tail != "" && r.adopt == 0 {
		cc.forgetFlight(r.Tail)
	}
	if cc.forgetTower != nil && r.Tail != "" && r.adopt == 0 {
		cc.forgetTower(r.Tail)
	}
	if auto {
		r.Runway = cc.pickRunway(g, r.Kind == "arrival", r.Stand)
	}
	cc.mu.Lock()
	cc.next++
	n := cc.next
	cc.mu.Unlock()
	if r.Model == "" {
		r.Model = "FSLTL A320 Air France SL"
	}
	model, livery, _ := strings.Cut(r.Model, liverySep)
	// Airframe of the type: wheelbase (where the tug connects), span (stands).
	ac := traffic.ProfileFor(model)
	prof := ac.Motion
	if r.Tail == "" {
		r.Tail = fmt.Sprintf("MAP%02d", n)
	}
	// The squawk asked for, else its own; never an emergency code.
	if r.Squawk != "" {
		if !validSquawk(r.Squawk) {
			return nil, fmt.Errorf("squawk %q: four octal digits (0–7), not 7500, 7600 or 7700", r.Squawk)
		}
	} else if r.Kind == "departure" {
		r.Squawk = squawkFor(r.Tail)
	}
	// A call sign picked on the map: letters and digits, as said on the radio.
	if len(r.Tail) < 2 || len(r.Tail) > 8 || strings.IndexFunc(r.Tail, func(c rune) bool { return (c < 'A' || c > 'Z') && (c < '0' || c > '9') }) >= 0 {
		return nil, fmt.Errorf("call sign %q: 2 to 8 letters and digits", r.Tail)
	}
	// One aircraft per call sign (a turnaround adopts its own arrival): a
	// second one would share its stand reservation and its log.
	if it := cc.byTail(r.Tail); it != nil && r.adopt == 0 {
		it.mu.Lock()
		done := it.view.Done
		it.mu.Unlock()
		if !done {
			return nil, fmt.Errorf("%s is already flying", r.Tail)
		}
	}
	// The stand: assigned (-1) or the one asked for, if nobody holds it.
	alloc := cc.allocator(g)
	if r.Stand < 0 {
		req := traffic.StandRequirements{Owner: r.Tail, Airline: airlineOf(r.Tail), HalfSpan: prof.SpanMeters / 2, OffBlock: r.pushAt, NoJetway: traffic.NoJetway(r.Model), Overnight: r.overnight, Bridged: cc.jetwayStands(g.Layout.ICAO), Prefer: cc.jetwayGates(g.Layout.ICAO), Other: r.Other}
		if r.Kind == "arrival" {
			req.Runway, req.OffBlock = r.Runway, r.offBlock
		}
		// By its use (#833): a light aircraft (VFR) on a GA ramp where the
		// airport has one free (#568), an airliner at a gate, a freighter
		// on a cargo stand; then on another that fits.
		use := r.StandUse
		if r.Circuit {
			use = standGA
		}
		s, err := assignStand(alloc, req, use)
		if err != nil {
			return nil, err
		}
		r.Stand = s
	} else if err := alloc.Occupy(r.Stand, r.Tail, prof.SpanMeters/2); err != nil {
		return nil, err
	}
	if auto && r.Kind != "arrival" {
		r.Runway = cc.runwayFor(g, false, r.Stand)
	}
	started := false
	defer func() {
		if !started {
			alloc.ReleaseOwner(r.Tail)
		}
	}()
	// On a jetway gate, not before the jetways are ready: placed earlier,
	// it stands where no helper's door meets it (live, QTR605 at B14
	// spawned a minute before the gates were filled, 18 m off its jetway).
	if r.Kind == "departure" && r.adopt == 0 && g.Layout.Parking[r.Stand].IsGate() && !cc.jetwaysReady(g.Layout.ICAO) {
		return nil, fmt.Errorf("%w: the jetways at %s are not ready yet", traffic.ErrSpawnBlocked, g.Layout.ICAO)
	}
	var procRoute []airport.NavPoint
	procName, expect := "", ""
	if r.Circuit {
		r.Procedure, r.planned = false, nil // VFR through the circuit: no SID or STAR, no IFR plan
	}
	if r.planned != nil {
		procRoute, procName, expect = r.planned.route, r.planned.name, r.planned.expect
		if fp := r.planned.plan; fp != nil {
			cc.log.printf("%-6s flight plan %s → %s: %s, FL%03d, %.0f NM", r.Tail, fp.Request.Departure.ICAO, fp.Request.Arrival.ICAO,
				fp.Route, fp.CruiseFL, fp.DistanceNM)
		}
	} else if r.Procedure {
		var err error
		if procRoute, procName, expect, err = cc.procedureFor(g, r); err != nil {
			return nil, err
		}
	} else if r.Circuit && r.Kind == "departure" {
		// VFR out of the circuit towards its exit (#568).
		c, err := traffic.NewCircuit(g.Layout, r.Runway, cc.core.circuitConfig(g.Layout.ICAO, r.Runway), ac)
		if err != nil {
			return nil, err
		}
		exit := rand.Float64() * 360
		if r.ExitBearing != nil {
			exit = *r.ExitBearing
		}
		procRoute, procName = c.Departure(exit), "VFR "+traffic.CompassName(exit)
		if pt := cc.core.vfrPointFor(g.Layout.ICAO, r.VFRPoint, r.Tail); pt != nil && r.ExitBearing == nil {
			procRoute, procName = c.DepartureVia(*pt), "VFR via "+pt.Name // out by a reporting point (#566)
		}
	}
	// Nobody appears on top of other traffic: an arrival waits while an
	// aircraft is near its STAR entry, or appeared there in the last minute.
	if r.Kind == "arrival" && len(procRoute) > 0 {
		p := procRoute[0]
		alt := math.Max(p.AltMax, p.AltMin) / 0.3048
		if who := cc.nearAirborne(p.Position, alt, r.Tail, cc.clock.Now()); who != "" {
			return nil, fmt.Errorf("%w: %s is near %s; try again in a minute", traffic.ErrSpawnBlocked, who, p.Ident)
		}
		sp := spawnPoint{tail: r.Tail, at: p.Position, altFt: alt, when: cc.clock.Now()}
		cc.mu.Lock()
		cc.spawnedAt = append(cc.spawnedAt, sp)
		cc.mu.Unlock()
		// Not spawned after all: the point is free again (#83: a failed spawn
		// blocked the STAR entry for a minute).
		defer func() {
			if started {
				return
			}
			cc.mu.Lock()
			for i, x := range cc.spawnedAt {
				if x == sp {
					cc.spawnedAt = append(cc.spawnedAt[:i:i], cc.spawnedAt[i+1:]...)
					break
				}
			}
			cc.mu.Unlock()
		}()
	}
	// The airport's limits (#335): climb-out hand-over from the SIDs, taxi speeds.
	var procs *airport.Procedures
	if cc.procedures != nil {
		if p, ok := cc.procedures(g.Layout.ICAO); ok {
			procs = &p
		}
	}
	lim := airport.LimitsFor(g.Layout, procs)
	var deice *traffic.Deicing
	if r.Kind == "departure" {
		var err error
		if deice, r.Via, err = cc.deicingFor(g, r); err != nil {
			return nil, err
		}
	}
	defBase, reqBase, err := cc.ids.Acquire()
	if err != nil {
		return nil, err
	}
	defer func() {
		if !started {
			cc.ids.Release(defBase)
		}
	}()
	var approach []airport.LatLon
	if r.Kind == "arrival" {
		for _, n := range procRoute {
			approach = append(approach, n.Position)
		}
	}
	var fixes []airFix
	for _, p := range procRoute {
		if p.Ident != "" {
			fixes = append(fixes, airFix{Ident: p.Ident, LatLon: p.Position})
		}
	}
	it := &controlled{approach: approach, fixes: fixes, defBase: defBase, ID: n, Kind: r.Kind, Tail: r.Tail, ICAO: g.Layout.ICAO, graph: g, stands: alloc, stand: r.Stand, other: r.Other, overnight: r.overnight, spoken: map[string]bool{}, removed: make(chan struct{}), cc: cc}
	it.tugAfter.Store(rand.Float64() < TugAfterJetwayShare)
	it.tugStand.Store(int32(r.Stand))
	it.gates.Store(r.Gates)
	it.wake = traffic.WakeFor(model).ICAO
	var events func() (TaxiOrArrival, bool)
	switch r.Kind {
	case "departure":
		cc.core.pushes.want(g, r.Stand) // its stand's standard push, planned on demand
		tug := cc.tug(r, reqBase, prof, ac.WingspanM)
		if t, ok := tug.(*traffic.SimObjectTug); ok {
			it.tug = t // its way shown on the map
		}
		var fuel traffic.FuelService
		if f := cc.fuelTruck(r, g, reqBase, prof); f != nil {
			fuel, it.fuel = f, f
		}
		var stairs, gpu traffic.FuelService
		if s := cc.stairs(r, g, reqBase, prof); s != nil {
			stairs = s
		}
		if u := cc.gpu(r, g, reqBase, prof); u != nil {
			gpu = u
		}
		buses := cc.buses(g, reqBase, prof, stairs)
		var deboard []traffic.FuelService
		if r.adopt != 0 { // a turnaround: the passengers off first
			deboard = cc.buses(g, reqBase, prof, stairs)
		}
		nose, side := cc.standPlace(g, r.Stand, model)
		startAt := traffic.Now()
		ctl, ch, err := cc.sim.StartDeparture(defBase, reqBase, traffic.TaxiRequest{Graph: g, Parking: r.Stand, Runway: r.Runway, Entry: r.Entry, ISADeviationC: cc.isaDeviation(g), ObjectID: r.adopt, PushbackAt: r.pushAt, NoseOffset: nose, StandSide: side,
			Options: airport.RouteOptions{Via: r.Via, Taxiways: r.Taxiways, HoldShortQueue: cc.holdShortQueue(g, r.Runway), LowVisibility: cc.lowVisibility(g, r.Runway)},
			Model:   model, Livery: livery, Tail: r.Tail, HoldForClearances: true /* clearances on request, #462 */, HoldForRunway: !r.Gates, Tug: tug, TugWait: it.tugWaitsForJetway, Fuel: fuel, Stairs: stairs, GPU: gpu, Buses: buses, Deboard: deboard, Profile: prof,
			Aircraft: &ac, Departure: procRoute, VFR: r.Circuit, Airport: &lim, Deice: deice,
			// The push may swing through a neighbouring stand nobody holds.
			StandOccupied: func(stand int) bool { _, taken := alloc.Occupant(stand); return taken },
			PowerOut:      powerOut(g.Layout.Parking[r.Stand], ac.WingspanM)})
		startTook = traffic.Since(startAt)
		if err != nil {
			return nil, err
		}
		it.dep = ctl
		events = func() (TaxiOrArrival, bool) { ev, ok := <-ch; return TaxiOrArrival{dep: &ev}, ok }
	case "arrival":
		var exit *airport.RunwayExit
		if r.Exit != nil {
			exits, err := g.RunwayExits(r.Runway)
			if err != nil || *r.Exit < 0 || *r.Exit >= len(exits) {
				return nil, fmt.Errorf("exit must be an index into /api/exits for runway %s", r.Runway)
			}
			exit = &exits[*r.Exit]
		}
		var circuit *traffic.Circuit
		// In over a reporting point, where the airport has some (#566); the
		// tower joins it from there (CircuitJoinFor): the downwind of the
		// circuit on its side, or straight in from the final's sector, never
		// across the runway.
		var entryPoint *traffic.ReportingPoint
		join := traffic.LegDownwind
		cfg := cc.core.circuitConfig(g.Layout.ICAO, r.Runway)
		if r.Circuit {
			entryPoint = cc.core.vfrPointFor(g.Layout.ICAO, r.VFRPoint, r.Tail)
			it.vfrPoint = entryPoint
			if entryPoint == nil && cfg.OverheadJoin {
				// An overhead join needs somewhere to come from: 6 NM out, in a
				// direction by its call sign.
				h := fnv32(r.Tail)
				entryPoint = &traffic.ReportingPoint{Position: offsetLatLon(airport.LatLon{Lat: g.Layout.Latitude, Lon: g.Layout.Longitude}, float64(h%360), 6*1852)}
			}
			it.stopAndGo = r.StopAndGo
			if entryPoint != nil {
				if c, j, err := traffic.CircuitJoinFor(g.Layout, r.Runway, cfg, entryPoint.Position); err == nil {
					cfg, join = c, j
				}
			}
			it.circuitJoin = join
		}
		if r.Circuit {
			c, err := traffic.NewCircuit(g.Layout, r.Runway, cfg, ac)
			if err != nil {
				return nil, err
			}
			circuit, it.circuit = &c, &c
			for _, p := range c.Points {
				it.approach = append(it.approach, p.Position) // its way on the map
			}
		}
		nose, side := cc.standPlace(g, r.Stand, model)
		startAt := traffic.Now()
		ctl, ach, err := cc.sim.StartArrival(defBase, reqBase, traffic.ArrivalRequest{Graph: g, Runway: r.Runway, Parking: r.Stand, NoseOffset: nose, StandSide: side, Model: model, Livery: livery, Tail: r.Tail, ObjectID: r.adopt, Exit: exit, Circuit: circuit, TouchAndGos: r.TouchAndGos, StopAndGo: r.StopAndGo, CircuitEntry: entryPoint, CircuitJoin: join,
			Options:          airport.RouteOptions{Via: r.Via, Taxiways: r.Taxiways},
			HoldForClearance: r.Gates, HoldAtCrossings: true, InjectApproach: !r.AILanding, Profile: prof, FollowMe: cc.followMe(r, g, reqBase, prof),
			Procedure: procRoute, MissedApproach: cc.missedFor(g, r.Runway), Aircraft: &ac, Airport: &lim,
			CrosswindKts: cc.crosswind(g, r.Runway)})
		startTook = traffic.Since(startAt)
		if err != nil {
			return nil, err
		}
		it.arr = ctl
		if r.Turnaround {
			d := r
			d.Kind, d.Turnaround, d.Entry, d.Exit, d.ProcName, d.Other, d.planned = "departure", false, "", nil, "", "", nil
			d.Via, d.Taxiways = nil, nil // the custom route was the taxi-in's
			dwell := defaultDwell
			if r.DwellSec > 0 {
				dwell = time.Duration(r.DwellSec * float64(time.Second))
			}
			it.turn, it.departNow = &d, make(chan struct{}, 1)
			it.dwell = time.Duration(float64(dwell) * (1 + dwellSpread*(2*rand.Float64()-1)))
		}
		events = func() (TaxiOrArrival, bool) { ev, ok := <-ach; return TaxiOrArrival{arr: &ev}, ok }
	default:
		return nil, fmt.Errorf("kind must be departure or arrival")
	}
	it.view = ControlView{ID: n, ICAO: g.Layout.ICAO, Squawk: r.Squawk, Manual: r.Gates, Kind: r.Kind, Rules: flightRules(r), Tail: r.Tail, Model: r.Model, Runway: r.Runway, Stand: g.Layout.Parking[r.Stand].Label(), State: "spawning", LimitNode: -1}
	it.keepSpot()
	cc.log.printf("%-6s %s: spawned %q at %s, runway %s%s (gates %v, injected approach %v)", r.Tail, r.Kind, r.Model, it.view.Stand, r.Runway, entryNote(r.Entry), r.Gates, r.Kind == "arrival" && !r.AILanding)
	it.setRoute()
	// Every departure starts with delivery, a SID or not: the first call,
	// then the clearance (#462).
	if procName != "" || r.Kind == "departure" || it.circuit != nil {
		if procName != "" {
			it.view.Procedure = procName
			it.procSaid = procedureSaid(procName, r.planned)
		}
		// VFR: no level from radar (live, OKQQS out VFR via NOVEMBER told
		// "climb to flight level 240").
		if r.Kind == "departure" && !r.Circuit {
			it.radarSaid, it.radarFt = departureLevel(r.planned, lim)
		}
		info := ""
		if cc.atisLetter != nil {
			info, it.atisSaid = cc.atisLetter(g.Layout.ICAO), true
		}
		if r.Kind == "departure" && r.Circuit {
			// VFR: no departure clearance; its first call is to ground,
			// for start-up or taxi (#568).
			_, freq := cc.stationOf(g.Layout.ICAO, traffic.PosGround)
			it.delivered, it.atc = true, traffic.PosGround
			it.view.ATC, it.view.Frequency = string(traffic.PosGround), freq
		} else if r.Kind == "departure" {
			// The first call to delivery, then the clearance (#462).
			station, _ := cc.stationOf(g.Layout.ICAO, traffic.PosDelivery)
			dest := ""
			if r.Other != "" {
				dest = cc.clearanceLimit(r.Other)
			}
			it.climbSaid, it.clearedFt = initialClimbSaid(lim, procName), lim.InitialClimbFor(procName)
			if r.planned != nil && r.planned.plan != nil {
				it.cruiseFt = float64(r.planned.plan.CruiseFL) * 100
			}
			deliver := func() {
				it.say(it.initial(traffic.RequestClearance(station, r.Tail, it.view.Stand, info, dest)))
				dc := traffic.DepartureClearance{Destination: dest, SID: it.procSaid,
					Runway: r.Runway, Level: it.climbSaid, Squawk: it.view.Squawk}
				clr := traffic.ClearedDeparture(r.Tail, dc)
				// Delivery staffed: the clearance waits for the player
				// (action "clearance"), as every other one does (#1156).
				if _, staffed := cc.core.staffedAt(g.Layout.ICAO, traffic.PosDelivery, station); staffed {
					it.mu.Lock()
					it.clearanceDue, it.view.Manual = &dc, true
					it.mu.Unlock()
					it.gates.Store(true)
					cc.log.printf("%-6s on a staffed station (%s): its clearance the player's", r.Tail, station)
					return
				}
				it.clearance(clr)
			}
			if r.parked {
				it.deliver, it.deliverAt = deliver, r.callAt // a real aircraft parked: it calls once its departure is seen (#841); a night stop at callAt (#1294)
			} else {
				deliver()
			}
		} else if it.circuit != nil {
			// VFR: the first call to the tower, for landing, then the join
			// (Doc 4444 12.3.4.13 a, b, d).
			station, _ := cc.stationOf(g.Layout.ICAO, traffic.PosTower)
			p := it.arr.Plan()
			pos, level := "", ""
			if p != nil {
				pos = positionSaid(g.Layout, airport.LatLon{Lat: p.Spawn.Latitude, Lon: p.Spawn.Longitude})
				level = fmt.Sprintf("%.0f feet", math.Round(p.Spawn.Altitude/100)*100)
			}
			if it.vfrPoint != nil {
				pos = "over " + it.vfrPoint.Name
			}
			it.say(it.initial(traffic.VFRForLanding(station, r.Tail, typeSaid(traffic.ProfileFor(model).Type), pos, level, info)))
			qnh, _ := cc.qnh()
			it.joinPending.Store(true)
			it.call(traffic.PosTower, prioApproach, func() {
				defer it.joinPending.Store(false)
				if it.circuitJoin == traffic.LegFinal {
					it.say(traffic.StraightIn(r.Tail, r.Runway, cc.windSaid(g.Layout.ICAO), qnh))
					return
				}
				where := string(it.circuit.Side) + " downwind"
				switch it.circuitJoin {
				case traffic.LegBase:
					where = string(it.circuit.Side) + " base" // from the approach side (#567)
				case traffic.LegOverhead:
					where = "overhead" // the standard overhead join (#567)
				}
				it.say(traffic.JoinCircuit(r.Tail, where, r.Runway, cc.windSaid(g.Layout.ICAO), qnh, ""))
			})
		} else {
			// The first call to approach with its level, then the STAR.
			station, _ := cc.stationOf(g.Layout.ICAO, traffic.PosApproach)
			level := ""
			if p := it.arr.Plan(); p != nil && p.Spawn.Altitude > 0 {
				level = traffic.LevelSaidAbove(p.Spawn.Altitude, lim.TransitionAltitudeFt)
			}
			// Adopted in the air (en route, a real aircraft): its own level,
			// not the plan's (live: "flight level 081" at 3,860 ft).
			if r.adopt != 0 {
				for _, a := range cc.world.Aircraft() {
					if a.ObjectID == r.adopt && a.AltFt > 0 {
						level = traffic.LevelSaidAbove(a.AltFt, lim.TransitionAltitudeFt)
					}
				}
			}
			it.say(it.initial(traffic.CheckIn(traffic.PosApproach, station, r.Tail, level, info)))
			qnh, _ := cc.qnh()
			clr := withParams(traffic.ClearedArrival(r.Tail, it.procSaid, expect, r.Runway, ""), map[string]string{traffic.ParamQNH: qnh})
			// Approach staffed: the clearance waits for the player (action
			// "clearance"), as delivery's does (#1096); the crew flies its
			// flight plan meanwhile.
			if _, staffed := cc.core.staffedAt(g.Layout.ICAO, traffic.PosApproach, station); staffed {
				it.arrivalDue, it.view.Manual = &clr, true
				it.gates.Store(true)
				cc.log.printf("%-6s on a staffed station (%s): its clearance the player's", r.Tail, station)
			} else {
				it.firstContact(clr)
			}
			it.askWeather(traffic.PosApproach)
			it.view.Procedure += " → " + expect
		}
	}
	started = true
	if clash := alloc.ReserveRoute(r.Tail, it.view.Nodes); len(clash) > 0 {
		cc.log.printf("%-6s %s: route overlaps the routes of %s", r.Tail, r.Kind, strings.Join(clash, ", "))
	}
	go func() {
		for {
			ev, ok := events()
			if !ok {
				it.mu.Lock()
				it.view.Done, it.doneAt = true, traffic.Now()
				it.mu.Unlock()
				it.finished.Store(true)
				return
			}
			it.update(ev)
		}
	}()
	cc.mu.Lock()
	cc.items[n] = it
	cc.mu.Unlock()
	return it, nil
}

// TaxiOrArrival is one event of either controller.
type TaxiOrArrival struct {
	dep *traffic.TaxiEvent
	arr *traffic.ArrivalEvent
}

func (it *controlled) setRoute() {
	var r *airport.Route
	if it.dep != nil {
		r = it.dep.Route()
	} else if p := it.arr.Plan(); p != nil {
		r = p.Route
	}
	if r != nil {
		it.view.Route, it.view.Nodes = r.Points, r.Nodes
		if it.dep != nil {
			it.view.Entry = r.Entry
		}
	}
}

func (it *controlled) update(ev TaxiOrArrival) {
	it.mu.Lock()
	defer it.mu.Unlock()
	v := &it.view
	prev := *v
	// On the stand: the route with its push, planned in the background
	// (TaxiWithPushInBackground), drawn once it is in.
	if ev.dep != nil && it.dep != nil && (ev.dep.State == traffic.TaxiSpawning || ev.dep.State == traffic.TaxiAwaitingPushback) {
		it.setRoute()
	}
	if ev.arr != nil && (ev.arr.State == traffic.ArrivalVacating || ev.arr.State == traffic.ArrivalRollout) && ev.arr.Taxiway != "" {
		it.exitTwy = ev.arr.Taxiway
	}
	// A change worth showing now (not each move): the open maps fetch it
	// at once (push.go). Positions come with the regular poll.
	defer func() {
		if prev.State != v.State || prev.ATC != v.ATC || prev.AtLimit != v.AtLimit || prev.LimitNode != v.LimitNode ||
			prev.Lights != v.Lights || prev.Error != v.Error || prev.Done != v.Done || prev.Manual != v.Manual || !slices.Equal(prev.Actions, v.Actions) {
			it.cc.changed("control")
		}
	}()
	// What the crew asks for (#462), once the change is logged and handed
	// off (deferred first: runs last).
	defer func() {
		if ev.dep != nil && ev.dep.Request != it.request {
			it.request, it.called = ev.dep.Request, ""
			if it.request != "" {
				it.onRequest(it.request)
			}
		}
		// Stopped short of a runway to cross: the crew reports it, "holding
		// short of runway 12 at F" (the departure's own runway is the ready
		// call instead).
		cross, at, holding := "", "", false
		switch {
		case ev.dep != nil:
			holding = ev.dep.State == traffic.TaxiHoldingShort && ev.dep.HoldingShortOf != "" && !strings.Contains(ev.dep.HoldingShortOf, it.view.Runway)
			cross, at = ev.dep.HoldingShortOf, ev.dep.Taxiway
		case ev.arr != nil:
			holding = ev.arr.State == traffic.ArrivalHoldingShort && ev.arr.HoldingShortOf != ""
			cross, at = ev.arr.HoldingShortOf, ev.arr.Taxiway
		}
		if holding && prev.State != v.State {
			it.say(traffic.HoldingShortReport(it.Tail, oneDesignator(cross), at))
		}
		// Holding short of its own runway, with tower since the way there:
		// the ready call.
		if ev.dep != nil && ev.dep.State == traffic.TaxiHoldingShort && !holding && it.atc == traffic.PosTower && !it.readySaid {
			it.readySaid = true
			entry := ""
			if r := it.dep.Route(); r != nil {
				entry = r.Entry
			}
			it.say(traffic.ReadyForDeparture(it.Tail, it.view.Runway, entry))
		}
		// Giving way where routes cross: ground says so, once (outside the
		// locks: it looks at the other aircraft).
		gw, hs := uint32(0), ""
		switch {
		case ev.dep != nil:
			gw, hs = ev.dep.GivingWayTo, ev.dep.GiveWayHoldShort
		case ev.arr != nil:
			gw, hs = ev.arr.GivingWayTo, ev.arr.GiveWayHoldShort
		}
		it.giveWayHoldShort = hs
		if gw != it.givingWay {
			it.givingWay = gw
			if gw != 0 && it.cc != nil {
				it.cc.pending.later(it.cc.clock.Now(), func() { it.tellGiveWay(gw) })
			}
			// Told to hold short for it: released once it has passed.
			if gw == 0 && it.heldShortOf != "" && it.cc != nil {
				it.heldShortOf = ""
				it.cc.pending.later(it.cc.clock.Now(), func() { it.say(traffic.ContinueTaxi(it.Tail)) })
			}
		}
		// Standing still while taxiing: why, logged once it lasts
		// (live, OKSTM stopped short of the holding point unexplained).
		by := ""
		switch {
		case ev.dep != nil:
			by = ev.dep.StoppedBy
		case ev.arr != nil:
			by = ev.arr.StoppedBy
		}
		if by != it.stoppedBy {
			it.stoppedBy = by
			if by != "" && it.cc != nil {
				it.cc.pending.later(it.cc.clock.Now().Add(stoppedLogAfter), func() { it.logStopped(by) })
			}
		}
		// Past its procedure's end without joining the final: logged, and
		// cleared direct to it (live, TVS1972 flew on 38 minutes).
		if ev.arr != nil && ev.arr.MissedJoin != it.missedJoin {
			it.missedJoin = ev.arr.MissedJoin
			if it.missedJoin != "" && it.cc != nil {
				it.cc.log.printf("%-6s arrival: %s; direct to the final", it.Tail, it.missedJoin)
				if f := it.cc.toFinal; f != nil {
					go f(it.ICAO, it.Tail) // through the connection's goroutine, not on it
				}
			}
		}
	}()
	defer it.handoff(ev) // after the change is logged (deferred: runs last)
	defer it.logChanges(prev, ev)
	if it.cc != nil {
		it.cc.reportOwn(it.ICAO, ev)
	}
	first := it.objectID == 0
	if e := ev.dep; e != nil && e.ObjectID != 0 {
		it.objectID = e.ObjectID
	}
	if e := ev.arr; e != nil && e.ObjectID != 0 {
		it.objectID = e.ObjectID
	}
	if m := it.managed; m != nil && first && it.objectID != 0 {
		m.Attach(it.Tail, it.objectID)
	}
	if e := ev.dep; e != nil && it.managed != nil && e.State == traffic.TaxiComplete && it.objectID != 0 {
		// On its way, still ours until it leaves the area (#369).
		it.cc.world.SetOwn(it.objectID, traffic.PhaseEnroute, "")
	}
	if m := it.managed; m != nil {
		if st, ok, err := managedStatus(ev); err != nil {
			// Failed or cancelled: out of the sim, and the manager decides
			// (another attempt, or the flight is over).
			it.managed = nil
			go func() {
				it.cc.do(func() error { return it.cc.remove(it) })
				m.Failed(it.Tail, err, it.cc.clock.Now())
			}()
		} else if ok {
			m.Update(it.Tail, st, it.cc.clock.Now())
		}
		// Its controller moved on: progress, the stuck time from now.
		cur := ""
		if ev.dep != nil {
			cur = ev.dep.State.String()
		} else if ev.arr != nil {
			cur = ev.arr.State.String()
		}
		if cur != "" && cur != it.progState {
			it.progState = cur
			m.Progress(it.Tail, it.cc.clock.Now())
		}
		// An arrival closer to the field than ever: progress, however long
		// its approach (live, LKPR: CSA253 removed "stuck approaching for
		// 50m" on short final, cleared to land, after two runway changes).
		if ev.arr != nil && ev.arr.State == traffic.ArrivalApproaching {
			if g, err := it.cc.graph(it.ICAO); err == nil {
				p := it.view.Position
				nm := calc.HaversineNM(p.Lat, p.Lon, g.Layout.Latitude, g.Layout.Longitude)
				if it.closestNM == 0 || nm < it.closestNM-1 {
					it.closestNM = nm
					m.Progress(it.Tail, it.cc.clock.Now())
				}
			}
		}
		if e := ev.dep; e != nil {
			// In the runway's queue: holding short, lining up, rolling, or in the
			// line for it (live, LKPR: departures stopped nose to tail short
			// of the holding point counted as taxiing, the next ones held on
			// their stands).
			queued := e.State == traffic.TaxiTaxiing && e.Remaining > 0 && e.Remaining <= runwayQueueM
			m.SetAtRunway(it.Tail, queued || e.State >= traffic.TaxiHoldingShort && e.State <= traffic.TaxiDeparting)
		}
	}
	if e := ev.dep; e != nil {
		v.State, v.HoldingShortOf, v.AtLimit, v.LimitNode = e.State.String(), e.HoldingShortOf, e.AtLimit, int(e.LimitNode)
		v.TaxiRemainingM = e.Remaining
		v.Position, v.Heading, v.GroundSpeed, v.Lights, v.OnGround = e.Position, e.Heading, e.GroundSpeed, e.Lights.String(), e.OnGround
		if e.Err != nil {
			v.Error = e.Err.Error()
		}
		v.Request = it.called
		v.SayingUntil = it.sayingUntil()
		v.Actions = askedFor(departureActions(e.State, e.HoldingShortOf, it.dep), v.State, it.delivered, it.called)
		if it.clearanceDue != nil {
			v.Actions = append([]string{"clearance"}, v.Actions...)
		}
		if it.handoffDue != "" {
			v.Actions = append(v.Actions, "handoff")
		}
		if e.State == traffic.TaxiComplete && it.atc == traffic.PosDeparture && it.gates.Load() {
			v.Actions = append(v.Actions, "climb") // ?level=feet (#1096)
		}
		if e.Deicing != v.Deicing {
			v.Deicing = e.Deicing
			if e.Deicing {
				it.cc.log.printf("%-6s %s: de-icing", v.Tail, v.Kind)
			} else {
				it.cc.log.printf("%-6s %s: de-icing done", v.Tail, v.Kind)
			}
		}
		if e.PushbackHeld != v.PushbackHeld || e.PushbackHeld && e.PushbackHeldBy != it.pushHeldBy {
			v.PushbackHeld, it.pushHeldBy = e.PushbackHeld, e.PushbackHeldBy
			if e.PushbackHeld {
				// Named outside it.mu (held here): byObject takes each one's lock.
				go it.logPushHeld(v.Tail, v.Kind, e.PushbackHeldBy)
			} else {
				it.pushHeldWho = ""
				it.cc.log.printf("%-6s %s: clear behind, pushing back", v.Tail, v.Kind)
			}
		}
		// Off the stand once taxiing; the route is done when airborne. Not
		// before: a push held for traffic can sit on the stand for minutes,
		// and pushed back (AwaitingTaxi) it is still on or just behind it —
		// the next aircraft would be spawned on top of it.
		if !it.left && e.State >= traffic.TaxiTaxiing {
			it.left = true
			it.stands.Release(it.stand)
		}
		if e.State.Terminal() {
			it.stands.ReleaseRoute(it.Tail)
		}
	}
	if e := ev.arr; e != nil {
		v.State, v.HoldingShortOf, v.AtLimit, v.LimitNode = e.State.String(), e.HoldingShortOf, e.AtLimit, int(e.LimitNode)
		v.Position, v.Heading, v.GroundSpeed, v.Lights, v.OnGround = e.Position, e.Heading, e.GroundSpeed, e.Lights.String(), e.OnGround
		if e.Err != nil {
			v.Error = e.Err.Error()
		}
		v.SayingUntil = it.sayingUntil()
		v.Actions = arrivalActions(e.State)
		if it.arrivalDue != nil {
			v.Actions = append([]string{"clearance"}, v.Actions...)
		}
		if e.State == traffic.ArrivalApproaching && it.atc == traffic.PosApproach && it.gates.Load() && !it.manualSet && !it.approachCleared && it.arrivalDue == nil && it.circuit == nil {
			v.Actions = append(v.Actions, "approach")
		}
		if it.handoffDue != "" {
			v.Actions = append(v.Actions, "handoff")
		}
		// Cleared to land only on the final: its STAR and approach flown.
		if e.State == traffic.ArrivalApproaching && (e.OnGround || it.objectID == 0 || len(it.arr.ProcedureRoute()) > 0) {
			v.Actions = slices.DeleteFunc(v.Actions, func(a string) bool { return a == "land" })
		}
		switch e.State {
		case traffic.ArrivalParked:
			it.stands.ReleaseRoute(it.Tail)
			if it.turn != nil && !it.turned {
				it.turned = true
				v.Actions = []string{"depart"}
				go it.cc.turnaround(it, e.ObjectID)
			}
		case traffic.ArrivalCancelled, traffic.ArrivalFailed:
			it.stands.ReleaseOwner(it.Tail)
		}
	}
	it.keepSpot()
}

// notAgain drops the clearances this aircraft has already been given.
//
// A CONTROLLER DOES NOT SAY A THING TWICE. The taxi state cannot tell them
// apart — "awaiting pushback" looks identical whether the push has been
// cleared or not — so a list built from the state alone went on offering
// "pushback" to an aircraft already cleared for push-and-start, and "start up"
// to one already cleared to start (#1231). The controller itself knows, and
// `Cleared` answers true only for the clearances that are once-only: taxi, a
// progressive limit, hold position and a crossing all mean something said
// again and are never withheld.
//
// A nil controller is "not known", which keeps the full list rather than
// hiding a clearance that may still be due.
func notAgain(ctl departureCtl, actions []string) []string {
	if ctl == nil {
		return actions
	}
	return slices.DeleteFunc(actions, func(a string) bool { return ctl.Cleared(a) })
}

func departureActions(s traffic.TaxiState, holdingShortOf string, ctl departureCtl) []string {
	switch s {
	case traffic.TaxiAwaitingPushback:
		if ctl != nil && ctl.FacesOut() {
			// No push to ask for: the crew's one call is "request start up
			// and taxi", and both are given together.
			return notAgain(ctl, []string{"startup", "taxi", "upto"}) // taxis straight out
		}
		// NO TAXI CLEARANCE ON THE STAND. The crew has asked for a push, with
		// a tug about to connect; it has not asked to taxi and could not go
		// if it were told to. A taxi clearance here is a clearance for a
		// conversation that has not happened — the player's complaint was
		// exactly that, "the sequence of commands … cannot give taxi when it
		// hasn't called ground". It comes on offer at TaxiAwaitingTaxi, when
		// the tug is clear and the crew calls ready (TaxiEvent.Request
		// "taxi").
		return notAgain(ctl, []string{"pushback", "pushstart"})
	case traffic.TaxiPushback:
		// Being pushed, tug attached: start-up is the only thing left to say.
		// (Nothing at all, once that is given — which is correct: the next
		// word on the frequency is the crew's, asking to taxi.)
		return notAgain(ctl, []string{"startup"})
	case traffic.TaxiAwaitingTaxi:
		// The push is done and the crew is asking: this is where the taxi
		// clearance belongs. NO "takeoff" HERE — a take-off clearance to an
		// aircraft standing on the apron is not a thing anyone says, and
		// offering it taught an order of the job that does not exist (#1231).
		return notAgain(ctl, []string{"startup", "taxi", "upto"})
	case traffic.TaxiTaxiing:
		// Nor here: a departure taxiing is minutes and a mile from the
		// runway, and the runway is not even known to be free yet. `upto` is
		// the clearance that means something while it taxis, and `hold` is
		// the one that stops it.
		return notAgain(ctl, []string{"hold", "taxi", "upto"})
	case traffic.TaxiHoldingShort:
		if r := ctl.Route(); r != nil && holdingShortOf != r.Runway {
			return notAgain(ctl, []string{"cross", "upto", "taxi"})
		}
		return notAgain(ctl, []string{"lineup", "lineupbehind", "takeoff"})
	case traffic.TaxiLiningUp, traffic.TaxiLinedUp:
		return []string{"takeoff", "abort"}
	case traffic.TaxiDeparting:
		return []string{"abort"}
	}
	return nil
}

// askedFor keeps of a departure's actions on its stand those its crew has come
// to (#1308). The taxi state alone says nothing about the conversation:
// "awaiting pushback" holds from the spawn, through delivery and before the
// crew's call. Nothing is offered until it has its clearance and has called.
func askedFor(actions []string, state string, delivered bool, called string) []string {
	if !onStand(state) {
		return actions
	}
	if !delivered {
		return nil
	}
	var asked []string
	switch called {
	case "pushback":
		asked = []string{"pushback", "pushstart"}
	case "start_up":
		asked = []string{"startup"}
		if state == traffic.TaxiAwaitingPushback.String() {
			asked = append(asked, "taxi", "upto") // taxis straight out: one call, one answer
		}
	case "taxi":
		asked = []string{"taxi", "upto"}
	}
	return slices.DeleteFunc(actions, func(a string) bool { return !slices.Contains(asked, a) })
}

// onStand: the taxi states before it taxis.
func onStand(state string) bool {
	switch state {
	case traffic.TaxiAwaitingPushback.String(), traffic.TaxiPushback.String(), traffic.TaxiAwaitingTaxi.String():
		return true
	}
	return false
}

// clearedEarly: a player's action comes ahead of the crew's order (#1308):
// not among those its crew has come to on the stand, or a runway clearance
// before its IFR clearance. it.mu is held.
func (it *controlled) clearedEarly(action string) bool {
	if it.dep == nil {
		return false
	}
	switch action {
	case "lineup", "lineupbehind", "takeoff":
		if !onStand(it.view.State) {
			return !it.delivered
		}
		return true
	case "pushback", "pushstart", "startup", "taxi", "upto":
		return onStand(it.view.State) && !slices.Contains(askedFor([]string{action}, it.view.State, it.delivered, it.called), action)
	}
	return false
}

// actionParams are the query parameters of the offered actions that need one
// (ControlView.ActionParams).
func actionParams(actions []string) map[string]string {
	var out map[string]string
	for _, a := range actions {
		p := ""
		switch a {
		case "upto", "vacate":
			p = "node"
		case "climb":
			p = "level"
		}
		if p != "" {
			if out == nil {
				out = map[string]string{}
			}
			out[a] = p
		}
	}
	return out
}

// vacateAt sends the arrival off the runway by the exit the player picked,
// named by any node on it (#1308 follow-up: "vacate where").
//
// A NODE, NOT AN EXIT, IS WHAT A HOST CAN SEND. The exits are the engine's own
// geometry — a runway node, a path off it, a side and a taxiway name — and a
// map click is a point. So the pick is matched against the exits of the runway
// it is landing on, by any node on the exit's path: a player clicking the
// turn-off, the taxiway behind it or the runway centreline where it begins all
// mean the same exit.
//
// Refused rather than approximated when the click matches no exit. "The nearest
// exit to wherever you clicked" is how an aeroplane ends up taking a turn-off
// it cannot make at its speed, and bestExit already weighs that properly.
func (it *controlled) vacateAt(node airport.NodeID) error {
	if it.arr == nil {
		return errors.New("not an arrival")
	}
	x, err := it.exitAt(node)
	if err != nil {
		return err
	}
	if err := it.arr.VacateAt(x); err != nil {
		return err
	}
	it.mu.Lock()
	it.view.Instruction = "vacate " + strings.TrimSpace(x.Side.String()+" "+x.Taxiway)
	it.mu.Unlock()
	it.cc.changed("control")
	return nil
}

// exitAt is the exit off the arrival's runway that node is on.
func (it *controlled) exitAt(node airport.NodeID) (airport.RunwayExit, error) {
	p := it.arr.Plan()
	if p == nil || it.graph == nil {
		return airport.RunwayExit{}, errors.New("vacate: this arrival has no runway plan yet")
	}
	exits, err := it.graph.RunwayExits(p.End.Name)
	if err != nil {
		return airport.RunwayExit{}, err
	}
	x, ok := exitAtNode(exits, node)
	if !ok {
		return airport.RunwayExit{}, fmt.Errorf("vacate: no exit off runway %s there", p.End.Name)
	}
	return x, nil
}

// exitAtNode is the exit node belongs to: the runway node it leaves by, the
// first taxiway node off the surface, or anything on the path between them.
//
// ALL THREE, because all three are the same turn-off to the person clicking.
// Matching only the taxiway node would refuse a click on the runway end of the
// very exit being named, which is where the turn-off is drawn on a chart.
func exitAtNode(exits []airport.RunwayExit, node airport.NodeID) (airport.RunwayExit, bool) {
	for _, x := range exits {
		if x.RunwayNode == node || x.Node == node || slices.Contains(x.Path, node) {
			return x, true
		}
	}
	return airport.RunwayExit{}, false
}

// arrivalActions is what a controller may say to an arrival in state s.
//
// THE OFFER FOLLOWS THE PHASE, as it now does for a departure (#1308). This
// answered "land, go around, taxi, up to" to an aeroplane ON FINAL: a taxi
// clearance to something a thousand feet in the air, which the engine would
// take and which teaches an order of the job that does not exist. The ground
// clearances begin where ground begins — off the runway (ArrivalPosition) —
// and the roll itself is the crew's, not the controller's: the aeroplane is
// stopping, and the next word on the frequency is its call to ground.
func arrivalActions(s traffic.ArrivalState) []string {
	switch s {
	case traffic.ArrivalApproaching, traffic.ArrivalLanding:
		// `vacate` names the exit it leaves the runway by, and is the tower's
		// to give before the wheels are down: after that the rollout is a
		// braking profile already being flown to the old one (VacateAt).
		return []string{"land", "goaround", "vacate"}
	case traffic.ArrivalVacating, traffic.ArrivalAwaitingTaxi:
		return []string{"taxi", "upto"}
	case traffic.ArrivalTaxiing:
		return []string{"hold", "taxi", "upto"}
	case traffic.ArrivalHoldingShort:
		return []string{"cross", "upto", "taxi"}
	}
	return nil
}

// positionSaid is at as said from l's airport: "10 miles south".
func positionSaid(l *airport.Layout, at airport.LatLon) string {
	miles := math.Max(1, math.Round(calc.HaversineNM(l.Latitude, l.Longitude, at.Lat, at.Lon)))
	unit := "miles"
	if miles == 1 {
		unit = "mile"
	}
	return fmt.Sprintf("%.0f %s %s", miles, unit, traffic.CompassName(calc.BearingDegrees(l.Latitude, l.Longitude, at.Lat, at.Lon)))
}

// flightRules: a circuit arrival flies VFR (#568), everything else IFR on
// its procedures or plan.
func flightRules(r SpawnRequest) string {
	if r.Circuit {
		return "VFR"
	}
	return "IFR"
}

// act gives a clearance to a controlled aircraft.
func (it *controlled) act(action string, node airport.NodeID) error {
	switch d := it.dep; {
	case action == "clearance":
		return it.giveClearance()
	case action == "handoff":
		// The player's "contact (next)": transferred on its next look.
		it.mu.Lock()
		defer it.mu.Unlock()
		if it.handoffDue == "" {
			return errors.New("nothing to hand over")
		}
		it.handoffGiven = true
		return nil
	// Already given, to the word: refused rather than transmitted a second
	// time. The offered list drops it too (notAgain), but the offer is
	// advice and this is the gate — a host that clears by name, a stale
	// strip or a double click all arrive here (#1269). `upto` is absent on
	// purpose: whether it repeats depends on the node, which only
	// ClearUpTo can judge. So is `taxi`: a changed route or runway still
	// goes out, and the same words stop earlier (taxiInForce).
	case d != nil && action != "upto" && action != "taxi" && d.Cleared(action):
		return fmt.Errorf("%s: %w", action, traffic.ErrAlreadyCleared)
	case d != nil && action == "pushback":
		d.ClearPushback()
	case d != nil && action == "pushstart":
		d.ClearStartUp()
		d.ClearPushback()
	case d != nil && action == "startup":
		d.ClearStartUp()
	case d != nil && action == "taxi":
		d.ClearToTaxi()
	case d != nil && action == "upto":
		return d.ClearUpTo(node)
	case d != nil && action == "cross":
		d.ClearToCross()
	case d != nil && action == "lineup":
		d.ClearToLineUp()
	case d != nil && action == "lineupbehind":
		if it.cc.lineUpBehind == nil {
			return errors.New("no tower")
		}
		return it.cc.lineUpBehind(it)
	case d != nil && action == "takeoff":
		return d.ClearForTakeoff()
	case d != nil && action == "remove":
		return d.Cancel()
	case d != nil && action == "hold":
		return d.HoldPosition()
	case d != nil && action == "abort":
		if err := d.AbortTakeoff(); err != nil {
			return err
		}
		// Lined up again or back at the holding point: the tower clears it
		// anew, not "given already".
		if it.cc.forgetTower != nil {
			it.cc.forgetTower(it.Tail)
		}
		return nil
	case it.arr != nil && action == "approach":
		// The player's approach clearance (#1096): out of the hold at its
		// clearance limit, if it holds there, onto the approach.
		it.mu.Lock()
		held := it.limitHeld
		it.approachCleared, it.approachSaid, it.limitHeld = true, true, false
		it.mu.Unlock()
		if held {
			if _, _, holding := it.arr.Holding(); holding {
				return it.arr.LeaveHold()
			}
		}
		return nil
	case it.arr != nil && action == "land":
		it.mu.Lock()
		it.landCleared = true // said: it lands unless sent around
		it.mu.Unlock()
		return nil
	case it.arr != nil && action == "hold":
		return it.arr.HoldPosition()
	case it.arr != nil && action == "goaround":
		if err := it.arr.GoAround(); err != nil {
			return err
		}
		if it.cc.rejoin != nil {
			it.cc.rejoin(it.ICAO, it.Tail)
		}
	case it.arr != nil && action == "vacate":
		return it.vacateAt(node)
	case it.arr != nil && action == "taxi":
		it.arr.ClearToTaxi()
	case it.arr != nil && action == "upto":
		return it.arr.ClearUpTo(node)
	case it.arr != nil && action == "cross":
		it.arr.ClearToCross()
	case it.arr != nil && action == "remove":
		return it.arr.Cancel()
	case it.arr != nil && action == "depart" && it.departNow != nil:
		select {
		case it.departNow <- struct{}{}:
		default:
		}
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	return nil
}

// views are our aircraft as the map and a host see them: ControlView
// with the route still to fly and the ground vehicles.
func (cc *controlCenter) views() []ControlView {
	out := []ControlView{}
	// Where departures handed to MSFS AI are now: the world scan.
	air := map[uint32]traffic.TrackedAircraft{}
	for _, a := range cc.world.Aircraft() {
		air[a.ObjectID] = a
	}
	cc.mu.Lock()
	for _, it := range cc.items {
		it.mu.Lock()
		v, id, fixes, o := it.view, it.objectID, it.fixes, it.observed
		it.mu.Unlock()
		v.ActionParams = actionParams(v.Actions)
		if cc.tcasView != nil && id != 0 {
			v.TCAS = cc.tcasView(id)
		}
		if o != nil {
			v.Real, v.Seeded, v.ObservedID, v.Registration = !o.Seeded, o.Seeded, o.ID, o.Registration
		}
		if it.arr != nil && !v.OnGround {
			if v.AirRoute = it.arr.ProcedureCorners(); v.AirRoute == nil {
				v.AirRoute = it.arr.ProcedureRoute()
			}
			v.AirFixes = fixesAhead(fixes, v.AirRoute)
			// Going around: the circuit's track points back to the final.
			v.AirFixes = append(v.AirFixes, goAroundFixes(it.arr.CircuitFixes(), it.circuit != nil)...)
			// On the final, the procedure flown: the line to the
			// threshold and down the runway to where its taxi starts.
			if len(v.AirRoute) == 0 && it.graph != nil {
				if _, end, ok := it.graph.Layout.RunwayEnd(v.Runway); ok {
					v.AirRoute = []airport.LatLon{end.Threshold}
					if len(v.Route) > 0 {
						v.AirRoute = append(v.AirRoute, v.Route[0])
					}
				}
			}
			if h, alt, ok := it.arr.Holding(); ok {
				v.Hold = &holdView{Ident: h.Ident, AltFt: alt, Racetrack: h.Racetrack(alt)}
			}
		}
		remote := false
		if rd, ok := it.dep.(*remoteDep); ok && cc.remoteVehicles != nil {
			// Split (#710): its tug and fuel truck drive on the actuator,
			// which tells how.
			remote = true
			for _, rv := range cc.remoteVehicles(rd.t) {
				v.Vehicles = append(v.Vehicles, rv)
				if rv.State == traffic.VehicleWaiting || rv.State == traffic.VehicleRemoved {
					continue
				}
				tv := &tugView{Position: rv.Position, Heading: rv.Heading, Route: rv.Route}
				if rv.Kind == "tug" {
					v.Tug = tv
				} else {
					v.Fuel = tv
				}
			}
		}
		if it.tug != nil && !remote {
			if p, route, ok := it.tug.Track(); ok {
				v.Tug = &tugView{Position: p.Position, Heading: p.Heading, Route: route}
			}
		}
		if it.fuel != nil && !remote {
			if p, route, ok := it.fuel.Track(); ok {
				v.Fuel = &tugView{Position: p.Position, Heading: p.Heading, Route: route}
			}
		}
		// Its altitude as the simulator reports it, for a scope's label
		// without /api/traffic (#1152).
		if a, ok := air[id]; ok {
			v.AltFt = math.Round(a.AltFt)
		}
		// A departure in the air: its SID still to fly, like a STAR.
		if a, ok := air[id]; it.dep != nil && ok && !a.OnGround {
			if r := it.dep.ClimbRoute(a.Position); len(r) > 0 {
				v.AirRoute, v.Position, v.Heading, v.GroundSpeed = r, a.Position, a.Heading, a.GroundKts
				v.AirFixes = fixesAhead(fixes, r)
			}
		}
		// Its ground vehicles, with what they say of themselves (#710).
		if it.tug != nil && !remote {
			v.Vehicles = append(v.Vehicles, vehicleView("tug", it.tug.ObjectID(), it.tug.Title(), it.tug.State(), it.tug.Track))
		}
		if it.fuel != nil && !remote {
			v.Vehicles = append(v.Vehicles, vehicleView("fuel", it.fuel.ObjectID(), it.fuel.Title(), it.fuel.State(), it.fuel.Track))
		}
		v.Rescue = cc.rescueTrack(it.Tail)
		out = append(out, v)
	}
	cc.mu.Unlock()
	// Each arrival's place in its airport's landing sequence (asked
	// outside cc.mu: the sequencer has its own locks).
	if cc.sequencesAt != nil {
		seq := map[string]map[string]traffic.SequenceEntry{}
		for i := range out {
			v := &out[i]
			if v.Kind != "arrival" {
				continue
			}
			bys, ok := seq[v.ICAO]
			if !ok {
				bys = map[string]traffic.SequenceEntry{}
				for _, list := range cc.sequencesAt(v.ICAO) {
					for _, e := range list {
						bys[e.Callsign] = e
					}
				}
				seq[v.ICAO] = bys
			}
			if e, ok := bys[v.Tail]; ok {
				v.Sequence = &e
			}
		}
	}
	return out
}

// registerControl adds the traffic control API to mux.
func registerControl(mux *http.ServeMux, st *state) {
	center := func(w http.ResponseWriter) *controlCenter {
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		if cc == nil {
			http.Error(w, "not connected to the simulator", http.StatusServiceUnavailable)
		}
		return cc
	}

	// GET /api/control — controlled aircraft.
	// GET /api/stands?icao=X — who holds which stand: reservations of the
	// controlled traffic and aircraft the scan found on stands.
	mux.HandleFunc("GET /api/stands", func(w http.ResponseWriter, r *http.Request) {
		cc := center(w)
		if cc == nil {
			return
		}
		g, err := st.cache.Graph(r.URL.Query().Get("icao"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		type standView struct {
			Index int    `json:"index"`
			Label string `json:"label"`
			traffic.Occupant
		}
		out := []standView{}
		for i, o := range cc.allocator(g).Occupancy() {
			out = append(out, standView{Index: i, Label: g.Layout.Parking[i].Label(), Occupant: o})
		}
		sort.Slice(out, func(a, b int) bool { return out[a].Index < out[b].Index })
		writeJSON(w, out)
	})

	mux.HandleFunc("GET /api/control", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		out := []ControlView{}
		if cc != nil {
			out = cc.views()
		}
		writeJSON(w, out)
	})

	// GET /api/radio?icao=LKPR&n=50 — what our controllers said (#415):
	// structured transmissions, oldest first (icao empty: every airport).
	mux.HandleFunc("GET /api/radio", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		out := []traffic.Transmission{}
		if cc != nil {
			n, _ := strconv.Atoi(r.URL.Query().Get("n"))
			if n <= 0 {
				n = 50
			}
			out = append(out, cc.radio.Recent(strings.ToUpper(r.URL.Query().Get("icao")), n)...)
		}
		writeJSON(w, out)
	})

	// GET /api/radio/variety — the radio's variety (#721): {"enabled", "seed",
	// "sayAgain", "readbackError"}; POST the same to change it
	// ("enabled":false: every crew answers at once, as worded).
	type varietyView struct {
		Enabled bool `json:"enabled"`
		traffic.Variety
	}
	mux.HandleFunc("GET /api/radio/variety", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		v := varietyView{}
		if cc != nil {
			if cur := cc.radio.Variety(); cur != nil {
				v = varietyView{Enabled: true, Variety: *cur}
			}
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("POST /api/radio/variety", func(w http.ResponseWriter, r *http.Request) {
		var v varietyView
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		if cc == nil {
			http.Error(w, "not connected", http.StatusServiceUnavailable)
			return
		}
		if v.Enabled {
			cc.radio.SetVariety(&v.Variety)
		} else {
			cc.radio.SetVariety(nil)
		}
		writeJSON(w, v)
	})

	// POST /api/radio/pilot {icao, callsign, intent, tags} — a call from the
	// user as pilot (#417), e.g. a voice recogniser's result: "request_taxi"
	// is answered with a taxi clearance from where the user aircraft is,
	// "readback" is checked against the last clearance to the call sign (a
	// wrong one is corrected), anything else gets "say again". Returns what
	// ATC said.
	mux.HandleFunc("POST /api/radio/pilot", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		cc, own := st.control, st.aircraft
		st.mu.Unlock()
		if cc == nil {
			http.Error(w, "not connected", http.StatusServiceUnavailable)
			return
		}
		var call struct {
			ICAO     string            `json:"icao"`
			Callsign string            `json:"callsign"`
			Intent   string            `json:"intent"`
			Tags     map[string]string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		icao := strings.ToUpper(call.ICAO)
		said := []traffic.Transmission{}
		say := func(t traffic.Transmission) {
			// Nothing is said to a tower that is not there (#1296).
			if tx := cc.radio.Transmit(icao, t); tx.Text != "" {
				said = append(said, tx)
			}
		}
		switch call.Intent {
		case string(traffic.IntentRequestTaxi):
			g, err := cc.graph(icao)
			if err != nil || own == nil {
				say(traffic.SayAgain(traffic.PosGround, call.Callsign))
				break
			}
			from, ok := g.NearestNode(airport.LatLon{Lat: own.Latitude, Lon: own.Longitude}, 200)
			// The runway in use nearest the user's aircraft.
			rwy := nav.Nearest(g.Layout, cc.runwaysInUse(g, false), airport.LatLon{Lat: own.Latitude, Lon: own.Longitude}).Name
			route, err := g.RouteToRunwayFrom(from, -1, rwy, "", airport.RouteOptions{})
			if !ok || err != nil {
				say(traffic.SayAgain(traffic.PosGround, call.Callsign))
				break
			}
			say(traffic.ClearedTaxiToRunway(call.Callsign, rwy, route.Entry, route.SpokenTaxiways(-1)))
		case string(traffic.IntentReadback):
			var last *traffic.Transmission
			for _, t := range cc.radio.Recent(icao, 100) {
				if !t.Pilot && t.Callsign == call.Callsign && t.Intent != traffic.IntentCorrection {
					t := t
					last = &t
				}
			}
			if last == nil {
				say(traffic.SayAgain(traffic.PosTower, call.Callsign))
				break
			}
			if c, ok := traffic.CheckReadback(*last, call.Tags); !ok {
				say(c)
			}
		default:
			say(traffic.SayAgain(traffic.PosTower, call.Callsign))
		}
		writeJSON(w, said)
	})

	// GET /api/control/log — the recent traffic log, newest last.
	mux.HandleFunc("GET /api/control/log", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, tlog.recent(200))
	})

	// GET /api/models — the aircraft titles the simulator can spawn.
	mux.HandleFunc("GET /api/models", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		out := []string{}
		if cc != nil {
			out = cc.modelList()
		}
		writeJSON(w, out)
	})

	// POST /api/control — spawn a controlled departure or arrival.
	mux.HandleFunc("POST /api/control", func(w http.ResponseWriter, r *http.Request) {
		cc := center(w)
		if cc == nil {
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var req SpawnRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Kind == "departure" && req.PushInMin > 0 {
			req.pushAt = cc.clock.Now().Add(time.Duration(req.PushInMin * float64(time.Minute)))
		}
		// MSFS AI lands an arrival only when asked: injectApproach false.
		var given map[string]json.RawMessage
		if _ = json.Unmarshal(body, &given); given["injectApproach"] != nil && !req.InjectApproach {
			req.AILanding = true
		}
		g, err := st.cache.Graph(req.ICAO)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		// A departure without its stand yet and no flight plan: spawn picks
		// the parallel nearest the stand it assigns.
		if (req.Runway == "" || strings.EqualFold(req.Runway, "active")) && (req.Kind == "arrival" || req.Stand >= 0 || req.Other != "") {
			req.Runway = cc.pickRunway(g, req.Kind == "arrival", req.Stand)
		}
		if req.Other != "" {
			p, err := planFor(r.Context(), st, g, req)
			if err != nil {
				tlog.printf("%s flight plan with %s failed: %v", req.Kind, req.Other, err)
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			req.planned = p
		}
		var it *controlled
		if err := cc.do(func() (e error) { it, e = cc.spawn(g, req); return e }); err != nil {
			tlog.printf("%s spawn at stand %d, runway %s failed: %v", req.Kind, req.Stand, req.Runway, err)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		it.mu.Lock()
		defer it.mu.Unlock()
		writeJSON(w, it.view)
	})

	// GET /api/control/{id}/{action}/phrase[?node=N] — what that clearance
	// would say, not given: the controller's transmission (its text in
	// the airport's phraseology) and the crew's readback.
	mux.HandleFunc("GET /api/control/{id}/{action}/phrase", func(w http.ResponseWriter, r *http.Request) {
		cc := center(w)
		if cc == nil {
			return
		}
		id, _ := strconv.Atoi(r.PathValue("id"))
		cc.mu.Lock()
		it := cc.items[id]
		cc.mu.Unlock()
		if it == nil {
			http.Error(w, "no such aircraft", http.StatusNotFound)
			return
		}
		node := airport.NodeID(-1)
		if s := r.URL.Query().Get("node"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil {
				http.Error(w, "node: a node ID", http.StatusBadRequest)
				return
			}
			node = airport.NodeID(n)
		}
		// NOTHING TO SAY STAYS NOTHING. Wording an empty transmission renders its
		// empty callsign and empty intent into the phraseology's own punctuation —
		// `", "` — so the page swapped a sentence nobody would say for a fragment
		// nobody would say. An action with no phrase right now answers empty, and
		// the page shows no line at all.
		t := it.phrase(r.PathValue("action"), node)
		if t.Intent == "" {
			writeJSON(w, ClearancePhrase{})
			return
		}
		said, rb, ok := cc.radio.Worded(it.ICAO, t)
		out := ClearancePhrase{Controller: said}
		if ok {
			out.Readback = &rb
		}
		writeJSON(w, out)
	})
	// POST /api/control/{id}/{action}[?node=N] — a clearance.
	mux.HandleFunc("POST /api/control/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		cc := center(w)
		if cc == nil {
			return
		}
		id, _ := strconv.Atoi(r.PathValue("id"))
		cc.mu.Lock()
		it := cc.items[id]
		cc.mu.Unlock()
		if it == nil {
			http.Error(w, "no such aircraft", http.StatusNotFound)
			return
		}
		// Network play (#511): only the position working it changes it, its
		// manual control, entry and rush included.
		if as := positionOf(r); !mayClear(as, it) {
			http.Error(w, it.Tail+" is not on your frequency ("+as+")", http.StatusForbidden)
			return
		}
		// Manual (?on=1) or automatic (?on=0): who gives its clearances. Back to
		// automatic, a request still waiting is answered.
		if r.PathValue("action") == "manual" {
			it.setManual(r.URL.Query().Get("on") != "0")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Stand (?stand=N, a parking index; #443): an arrival not yet down
		// goes to another stand, picked on the map; it is reserved for it.
		if r.PathValue("action") == "standto" && it.arr != nil {
			to, err := strconv.Atoi(r.URL.Query().Get("stand"))
			if err != nil || it.stands == nil || to < 0 || to >= len(it.graph.Layout.Parking) {
				http.Error(w, "stand: a parking index of this airport", http.StatusBadRequest)
				return
			}
			it.mu.Lock()
			old, tail, rwy := it.stand, it.Tail, it.view.Runway
			it.mu.Unlock()
			half := traffic.DefaultHalfSpanMeters
			if o, ok := it.stands.Occupant(old); ok && o.HalfSpan > 0 {
				half = o.HalfSpan
			}
			if to != old {
				if why := it.stands.TakenFrom(to, tail, 0); why != "" || !it.stands.Free(to, half) {
					http.Error(w, it.graph.Layout.Parking[to].Label()+" is not free", http.StatusConflict)
					return
				}
				if err := it.stands.Occupy(to, tail, half); err != nil {
					http.Error(w, err.Error(), http.StatusConflict)
					return
				}
				if err := cc.do(func() error { return it.arr.ChangeStand(to) }); err != nil {
					it.stands.Release(to)
					http.Error(w, err.Error(), http.StatusUnprocessableEntity)
					return
				}
				it.stands.Release(old)
			}
			l := it.graph.Layout
			it.mu.Lock()
			it.stand, it.view.Stand = to, l.Parking[to].Label()
			it.tugStand.Store(int32(to))
			it.view.Instruction, it.view.InstructionAt = "stand "+l.Parking[to].Label(), &l.Parking[to].Position
			mgr, model, label := it.managed, it.view.Model, it.view.Stand
			it.mu.Unlock()
			if mgr != nil {
				mgr.Describe(tail, model, label, rwy)
			}
			cc.log.printf("%-6s arrival: stand %s (on the map, was %s)", tail, label, l.Parking[old].Label())
			cc.changed("control")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Climb (?level=feet, #1096): a departure handed over, cleared on
		// from the level it was held at.
		if r.PathValue("action") == "climb" && it.dep != nil {
			ft, err := strconv.ParseFloat(r.URL.Query().Get("level"), 64)
			if err != nil || ft <= 0 {
				http.Error(w, "level: feet", http.StatusBadRequest)
				return
			}
			if err := it.climbTo(ft); err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Clearance amended (?runway=20&sid=RANU2A&level=4000; #1164): the
		// departure clearance off another runway, with another SID or a
		// lower initial climb, said as applied.
		if q := r.URL.Query(); r.PathValue("action") == "clearance" && it.dep != nil && (q.Has("runway") || q.Has("sid") || q.Has("level")) {
			a := ClearanceAmend{Runway: q.Get("runway"), SID: q.Get("sid")}
			if s := q.Get("level"); s != "" {
				ft, err := strconv.ParseFloat(s, 64)
				if err != nil || ft <= 0 {
					http.Error(w, "level: feet", http.StatusBadRequest)
					return
				}
				a.LevelFt = ft
			}
			if err := it.amendClearance(a); err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
		}
		// Entry (?entry=B, "" full length): the departure takes its runway
		// from another intersection, re-planned from where it is; taxiing,
		// ground gives the new route.
		if r.PathValue("action") == "entry" && it.dep != nil {
			entry := r.URL.Query().Get("entry")
			if err := cc.do(func() error { return it.dep.ChangeEntry(entry) }); err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			it.gates.Store(true) // the user's clearance: manual from here
			it.mu.Lock()
			it.view.Manual = true
			it.setRoute()
			state := it.view.State
			it.mu.Unlock()
			tlog.printf("%-6s runway entry %s", it.Tail, orNone(entry))
			if state == traffic.TaxiTaxiing.String() { // on along the new route at once
				it.say(it.phrase("taxi", -1))
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Via (?nodes=12,40,7; #443): the departure taxis to its runway
		// through these nodes, picked on the map, re-planned from where it
		// is; taxiing, ground gives the new route.
		if r.PathValue("action") == "via" && it.dep != nil {
			var via []airport.NodeID
			for _, s := range strings.Split(r.URL.Query().Get("nodes"), ",") {
				n, err := strconv.Atoi(strings.TrimSpace(s))
				if err != nil {
					http.Error(w, "nodes: comma-separated node IDs", http.StatusBadRequest)
					return
				}
				via = append(via, airport.NodeID(n))
			}
			if err := cc.do(func() error { return it.dep.TaxiVia(via) }); err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			it.gates.Store(true) // the user's clearance: manual from here
			it.mu.Lock()
			it.view.Manual = true
			it.setRoute()
			state := it.view.State
			p := it.graph.Nodes[via[len(via)-1]].Position
			it.view.Instruction, it.view.InstructionAt = fmt.Sprintf("taxi via %d points", len(via)), &p
			it.mu.Unlock()
			tlog.printf("%-6s taxi via %d points (on the map)", it.Tail, len(via))
			if state == traffic.TaxiTaxiing.String() { // on along the new route at once
				it.say(it.phrase("taxi", -1))
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Rush (#510): ?on=1 or 0; the crew hurries, the clearances say it.
		if r.PathValue("action") == "rush" {
			on := r.URL.Query().Get("on") != "0"
			it.rush.Store(on)
			err := cc.do(func() error {
				if it.dep != nil {
					it.dep.Expedite(on)
				} else if it.arr != nil {
					it.arr.Expedite(on)
				}
				return nil
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			it.mu.Lock()
			it.view.Rush = on
			it.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Follow on the ground (?tail= the leader): it stays behind it
		// wherever their ways meet, told ground's way.
		if r.PathValue("action") == "follow" {
			lead := cc.byTail(r.URL.Query().Get("tail"))
			if lead == nil || lead == it || lead.ICAO != it.ICAO {
				http.Error(w, "tail: another aircraft of ours at the airport", http.StatusBadRequest)
				return
			}
			if !cc.follow(it, lead) {
				http.Error(w, "no aircraft yet", http.StatusConflict)
				return
			}
			it.say(traffic.FollowTaxi(it.Tail, cc.followSaid(it, lead)))
			it.mu.Lock()
			it.view.Instruction, it.view.InstructionAt = "follow "+lead.Tail, nil
			it.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Network play (#511): only the position working it clears it.
		if as := positionOf(r); !mayClear(as, it) {
			http.Error(w, it.Tail+" is not on your frequency ("+as+")", http.StatusForbidden)
			return
		}
		// No limit given is not a limit behind it (#1308): "not ahead on the
		// taxi route" sent a host looking at the route.
		if a := r.PathValue("action"); (a == "upto" || a == "vacate") && r.URL.Query().Get("node") == "" {
			what := map[string]string{"upto": "the clearance limit", "vacate": "the exit"}[a]
			http.Error(w, a+": ?node= "+what+" is required", http.StatusBadRequest)
			return
		}
		if a := r.PathValue("action"); a != "remove" && a != "locate" {
			it.playerAt.Store(cc.clock.Now().UnixNano())
		}
		node := airport.NodeID(-1)
		if s := r.URL.Query().Get("node"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil {
				http.Error(w, "node must be a graph node ID", http.StatusBadRequest)
				return
			}
			node = airport.NodeID(n)
		}
		action := r.PathValue("action")
		clr := action
		if node >= 0 {
			clr = fmt.Sprintf("%s node %d", action, node)
		}
		// A clearance from the user takes the aircraft over: no automatic
		// answers or tower clearances for it any more (they would clash).
		if action != "remove" && action != "locate" && !it.gates.Load() {
			it.gates.Store(true)
			it.mu.Lock()
			it.view.Manual = true
			it.mu.Unlock()
			tlog.printf("%-6s under your control", it.Tail)
		}
		// The taxi clearance it is following already: not said again.
		if (action == "taxi" || action == "upto") && it.taxiInForce(action, node) {
			tlog.printf("%-6s %s: clearance %s in force already, not said again", it.Tail, it.Kind, clr)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Pushback with the start-up in one (?startup=1).
		if action == "pushback" && r.URL.Query().Get("startup") == "1" {
			action = "pushstart"
		}
		// Marked as said before it is given: the state change it causes can
		// arrive before cc.do returns, and would log it a second time.
		it.mu.Lock()
		wasSpoken, hadSpoken := it.spoken[action]
		it.spoken[action] = action != "remove"
		for _, k := range impliedBy(action) {
			it.spoken[k] = true
		}
		var m *traffic.TrafficManager
		if action == "remove" {
			m, it.managed = it.managed, nil // removed here, not failed
		}
		it.mu.Unlock()
		it.mu.Lock()
		early := it.clearedEarly(action)
		it.mu.Unlock()
		// A pushback facing a compass direction ("east"): planned again so.
		facing := r.URL.Query().Get("facing")
		do := func() error { return it.act(action, node) }
		if (action == "pushback" || action == "pushstart") && facing != "" && it.dep != nil {
			start := action == "pushstart"
			do = func() error {
				if start {
					it.dep.ClearStartUp()
				}
				return it.dep.ClearPushbackFacing(facing)
			}
		}
		if err := cc.do(do); err != nil {
			// As it was: refused, the one given before still counts as said,
			// or its state change would say it again (#1321).
			it.mu.Lock()
			if hadSpoken {
				it.spoken[action] = wasSpoken
			} else {
				delete(it.spoken, action)
			}
			it.mu.Unlock()
			tlog.printf("%-6s %s: clearance %s refused: %v", it.Tail, it.Kind, clr, err)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		if early {
			it.mu.Lock()
			it.view.ClearedEarly = append(slices.Clone(it.view.ClearedEarly), action)
			it.mu.Unlock()
			tlog.printf("%-6s %s: clearance %s ahead of its crew", it.Tail, it.Kind, clr)
		}
		defer func() {
			it.mu.Lock()
			it.view.SayingUntil = it.sayingUntil()
			it.mu.Unlock()
		}()
		switch action {
		case "remove":
			tlog.printf("%-6s %s: removed", it.Tail, it.Kind)
		case "clearance", "handoff":
			// Said by the exchange they start (live in the test: "Airfrans
			// 1288, clearance" before the clearance itself).
		default:
			it.say(it.phrase(action, node))
		}
		if action == "remove" {
			it.stands.ReleaseOwner(it.Tail)
			it.removeOnce.Do(func() { close(it.removed) })
			cc.forget(it)
			if m != nil {
				m.Remove(it.Tail, cc.clock.Now()) // off the schedule too
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// defaultSchedule is the built-in schedule data: telephony and airport
// names before the schedule exists.
var defaultSchedule = traffic.DefaultScheduleConfig()

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// isaDeviation is the temperature above ISA at g's airport now, °C (0
// without weather): a warm day lengthens the take-off (#1232).
func (cc *controlCenter) isaDeviation(g *airport.Graph) float64 {
	if cc.weather == nil {
		return 0
	}
	w := cc.weather()
	if w == nil || math.IsNaN(w.TempC) {
		return 0
	}
	return w.TempC - (15 - 1.98*g.Layout.Altitude/0.3048/1000)
}

// unableRunway: the crew of it refuses runway, too short for its take-off,
// on the frequency of the position working it, and keeps its own (#1232).
func (it *controlled) unableRunway(runway string, err error) {
	it.mu.Lock()
	own, pos := it.view.Runway, it.atc
	it.mu.Unlock()
	if pos == "" {
		pos = traffic.PosDelivery
	}
	it.cc.log.printf("%-6s runway %s refused by the crew: %v", it.Tail, runway, err)
	it.say(traffic.RunwayTooShort(pos, it.Tail, runway, own))
}

// crosswind is the crosswind on runway end rwy now, knots, positive from
// its right (0 without weather): arrivals crab into it on final.
func (cc *controlCenter) crosswind(g *airport.Graph, rwy string) float64 {
	if cc.weather == nil {
		return 0
	}
	w := cc.weather()
	_, end, ok := g.Layout.RunwayEnd(rwy)
	if w == nil || !ok {
		return 0
	}
	return w.WindKts * math.Sin((w.WindDirTrue-end.Heading)*math.Pi/180)
}

// activeRunway is the runway in use at g's airport now: from the weather
// (wind, limits, preferential runways), else the first preferred runway,
// else the first runway end. Departures and arrivals may differ. With
// parallels in use together, the first of them (runwayFor picks one per
// flight).
func (cc *controlCenter) activeRunway(g *airport.Graph, arrival bool) string {
	ends := cc.runwaysInUse(g, arrival)
	if len(ends) == 0 {
		return ""
	}
	return ends[0].Name
}

// runwayFor is the runway in use a flight from or to stand takes: with
// parallels in use together, the one nearest the stand (a negative stand:
// the first).
func (cc *controlCenter) runwayFor(g *airport.Graph, arrival bool, stand int) string {
	ends := cc.runwaysInUse(g, arrival)
	if len(ends) == 0 {
		return ""
	}
	if stand < 0 || stand >= len(g.Layout.Parking) {
		return ends[0].Name
	}
	if !arrival && len(ends) > 1 {
		if r := cc.balancedDeparture(g, ends, stand); r != "" {
			return r
		}
	}
	return nav.Nearest(g.Layout, ends, g.Layout.Parking[stand].Position).Name
}

// departureQueueM: each departure already on the ground for a runway counts
// as this much more taxi to it (about a departure interval's), so with
// parallels in use together a busy one hands departures to the other.
const departureQueueM = 900.0

// runwayQueueM: a departure taxiing this near its runway is in the line
// for it, at the runway (TrafficManager.SetAtRunway), not on the taxiways.
const runwayQueueM = 400.0

// balancedDeparture is the departure runway of parallels in use together
// for a flight from stand: the shortest taxi there, with each departure
// already waiting for it or taxiing to it counted as departureQueueM more
// (live, LROP: every stand is nearest 26L, and 26R stood empty); "" when
// no route is found.
func (cc *controlCenter) balancedDeparture(g *airport.Graph, ends []airport.RunwayEnd, stand int) string {
	queue := map[string]int{}
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		if v.Kind == "departure" && v.OnGround && strings.EqualFold(v.ICAO, g.Layout.ICAO) {
			queue[v.Runway]++
		}
	}
	best, bestCost := "", math.Inf(1)
	for _, e := range ends {
		r, err := g.RouteToRunway(stand, e.Name, airport.RouteOptions{})
		if err != nil {
			continue
		}
		if cost := r.Length + float64(queue[e.Name])*departureQueueM; cost < bestCost {
			best, bestCost = e.Name, cost
		}
	}
	return best
}

// DepartureRunway is the runway the World would give a departure from
// stand (its label) at icao now: with parallels in use together, the
// shorter taxi with the queues counted (balancedDeparture), as for its own
// traffic; "" when the airport is not loaded. For the player's runway.
func (w *World) DepartureRunway(icao, stand string) string {
	st := w.st
	st.mu.Lock()
	t := st.towers
	st.mu.Unlock()
	if t == nil {
		return ""
	}
	g, err := t.cc.graph(icao)
	if err != nil {
		return ""
	}
	idx := -1
	if stand != "" {
		if i, err := g.Layout.ParkingIndex(stand); err == nil {
			idx = i
		}
	}
	return t.cc.runwayFor(g, false, idx)
}

// pickRunway is the runway in use for a new flight: from or to stand, the
// nearest of parallels used together; an arrival without a stand yet, the
// one with the fewest arrivals in its sequence.
func (cc *controlCenter) pickRunway(g *airport.Graph, arrival bool, stand int) string {
	ends := cc.runwaysInUse(g, arrival)
	if len(ends) < 2 || !arrival || stand >= 0 {
		return cc.runwayFor(g, arrival, stand)
	}
	var seqs map[string][]traffic.SequenceEntry
	if cc.sequencesAt != nil {
		seqs = cc.sequencesAt(g.Layout.ICAO)
	}
	best := ends[0].Name
	own := func(rwy string) int { // not the adjacent final's, given to it too
		n := 0
		for _, e := range seqs[rwy] {
			if e.Runway == "" {
				n++
			}
		}
		return n
	}
	for _, e := range ends[1:] {
		if own(e.Name) < own(best) {
			best = e.Name
		}
	}
	return best
}

// runwaysInUse are the runway ends in use for departures or arrivals at
// g's airport: several with parallel runways used together.
func (cc *controlCenter) runwaysInUse(g *airport.Graph, arrival bool) []airport.RunwayEnd {
	if use, ok := cc.runwayUse(g); ok {
		ends := use.Departures
		if arrival {
			ends = use.Arrivals
		}
		if len(ends) > 0 && ends[0].Name != "" {
			return ends
		}
	}
	lim := cc.limitsOf(g)
	// Without weather: the first preferred runway, else the first runway,
	// either open (none open: none).
	shut := nav.RunwayLimits{Closed: cc.core.closed(g.Layout.ICAO)}
	name := ""
	if end := cc.core.usedRunway(g.Layout.ICAO); end != "" && !shut.ClosedEnd(end) {
		name = end // made the runway in use (UseRunway)
	}
	for _, n := range lim.PreferredRunways {
		if name != "" {
			break
		}
		if !shut.ClosedEnd(n) {
			name = n
			break
		}
	}
	for _, r := range g.Layout.Runways {
		if name == "" && !shut.ClosedEnd(r.Primary.Name) && !shut.ClosedEnd(r.Secondary.Name) {
			name = r.Primary.Name
		}
	}
	if _, end, ok := g.Layout.RunwayEnd(name); ok {
		return []airport.RunwayEnd{end}
	}
	return nil
}

// limitsOf are g's airport limits, with its procedures when known.
func (cc *controlCenter) limitsOf(g *airport.Graph) airport.Limits {
	var procs *airport.Procedures
	if cc.procedures != nil {
		if p, ok := cc.procedures(g.Layout.ICAO); ok {
			procs = &p
		}
	}
	return airport.LimitsFor(g.Layout, procs)
}

// runwayUse is the runway configuration in use at g's airport now, from
// the weather; false without weather.
func (cc *controlCenter) runwayUse(g *airport.Graph) (nav.RunwayUse, bool) {
	if cc.weather == nil {
		return nav.RunwayUse{}, false
	}
	w := cc.weather()
	if w == nil {
		return nav.RunwayUse{}, false
	}
	// The runway in use holds through wind shifts near a limit (#391); the
	// ATIS says the same (#454).
	use := cc.core.runwaySelector(g.Layout.ICAO).Choose(cc.clock.Now(), g.Layout, *w, cc.core.runwayLimits(g, cc.limitsOf(g)))
	cc.core.logRunwayChange(g.Layout.ICAO, use, *w)
	return use, use.Departure.Name != ""
}

// reportOwn tells the traffic picture what one of our aircraft is doing:
// its controller knows better than a scan.
func (cc *controlCenter) reportOwn(icao string, ev TaxiOrArrival) {
	var id uint32
	var phase traffic.Phase
	done := false
	switch {
	case ev.dep != nil:
		e := ev.dep
		id, done = e.ObjectID, e.State.Terminal()
		switch e.State {
		case traffic.TaxiSpawning, traffic.TaxiAwaitingPushback:
			phase = traffic.PhaseParked
		case traffic.TaxiLiningUp, traffic.TaxiLinedUp:
			phase = traffic.PhaseRunway
		case traffic.TaxiDeparting:
			phase = traffic.PhaseRunway
			if !e.OnGround {
				phase = traffic.PhaseDeparting
			}
		default:
			phase = traffic.PhaseTaxiing
		}
	case ev.arr != nil:
		e := ev.arr
		id, done = e.ObjectID, e.State.Terminal() && e.State != traffic.ArrivalParked
		switch e.State {
		case traffic.ArrivalSpawning, traffic.ArrivalApproaching:
			phase = traffic.PhaseArriving
		case traffic.ArrivalLanding, traffic.ArrivalRollout:
			phase = traffic.PhaseRunway
		case traffic.ArrivalParked:
			phase = traffic.PhaseParked
		default:
			phase = traffic.PhaseTaxiing
		}
	}
	if id == 0 {
		return
	}
	if done {
		cc.world.ForgetOwn(id) // handed to MSFS AI, cancelled or failed: a scan tells from here
		return
	}
	cc.world.SetOwn(id, phase, icao)
}

// deicingFor is the de-icing a departure asks for (#323): on the stand, at
// a pad (the first via point, taken off the via list, else the airport's
// first pad), or with "auto" whatever icing weather calls for. It returns
// the via points left.
func (cc *controlCenter) deicingFor(g *airport.Graph, r SpawnRequest) (*traffic.Deicing, []airport.NodeID, error) {
	mode, via := r.Deice, r.Via
	pads := airport.LimitsFor(g.Layout, nil).DeicingPads
	if cc.pads != nil {
		pads = cc.pads(g.Layout)
	}
	if mode == "auto" {
		mode = ""
		if cc.weather != nil {
			if w := cc.weather(); w != nil && nav.IcingConditions(*w) {
				mode = "stand"
				if len(pads) > 0 {
					mode = "pad"
				}
			}
		}
	}
	d := &traffic.Deicing{Dwell: time.Duration(r.DeiceSec * float64(time.Second))}
	switch mode {
	case "":
		return nil, via, nil
	case "stand":
		return d, via, nil
	case "pad":
		switch {
		case len(via) > 0:
			d.Pad = &airport.DeicingPad{Name: "via point 1", Position: g.Nodes[via[0]].Position}
			via = via[1:]
		case len(pads) > 0:
			d.Pad = &pads[0]
		default:
			return nil, via, errors.New("no de-icing pad: pick pads in Charts → De-icing pads, or make one the first via point of a custom route")
		}
		return d, via, nil
	}
	return nil, via, fmt.Errorf("de-icing %q: want auto, stand or pad", r.Deice)
}

// procedureFor resolves the SID (departure) or the STAR and approach
// (arrival) of r's runway: r.ProcName, or one picked at random. It returns
// the points, the procedure's name and, for an arrival, the approach
// type to expect ("ILS").
func (cc *controlCenter) procedureFor(g *airport.Graph, r SpawnRequest) ([]airport.NavPoint, string, string, error) {
	if cc.procedures == nil {
		return nil, "", "", errors.New("procedures not available")
	}
	p, ok := cc.procedures(g.Layout.ICAO)
	if !ok {
		return nil, "", "", fmt.Errorf("procedures of %s not loaded (yet)", g.Layout.ICAO)
	}
	pick := func(list []airport.Procedure) (airport.Procedure, error) {
		for _, x := range list {
			if strings.EqualFold(x.Name, r.ProcName) {
				return x, nil
			}
		}
		if len(list) == 0 {
			// A fact about the airport, not the caller (#1164: Orly has no
			// SID off 02).
			return airport.Procedure{}, fmt.Errorf("runway %s has no published procedure", r.Runway)
		}
		if r.ProcName != "" {
			return airport.Procedure{}, fmt.Errorf("no procedure %q for runway %s", r.ProcName, r.Runway)
		}
		return list[rand.IntN(len(list))], nil
	}
	if r.Kind == "departure" {
		sid, err := pick(p.SIDsFor(r.Runway))
		if err != nil {
			return nil, "", "", err
		}
		start, alt := g.Layout.DepartureStart(r.Runway)
		pts, err := p.ResolveSID(sid.Name, r.Runway, "", start, alt)
		return pts, sid.Name, "", err
	}
	star, err := pick(p.STARsFor(r.Runway))
	if err != nil {
		return nil, "", "", err
	}
	// Where it is entered: a STAR is flown enroute transition → common
	// route → runway transition, so its first fix is on the common route,
	// else on this runway's transition.
	first := ""
	legs := slices.Clone(star.Legs)
	for _, t := range star.RunwayTransitions {
		if strings.TrimLeft(t.Runway, "0") == strings.TrimLeft(r.Runway, "0") || t.Runway == "ALL" {
			legs = append(legs, t.Legs...)
			break
		}
	}
	for _, l := range legs {
		if l.HasFix() && first == "" {
			first = l.Fix
		}
	}
	pts, err := p.Arrival(r.Runway, first)
	if err != nil {
		return nil, "", "", err
	}
	app, _ := p.BestApproach(r.Runway)
	kind, _, _ := strings.Cut(app.Name, " ")
	return pts, star.Name, kind, nil
}

// missedFor is the published missed approach of the approach to runway
// flown on a go-around (#394); nil when unknown (a circuit instead).
func (cc *controlCenter) missedFor(g *airport.Graph, runway string) []airport.NavPoint {
	if cc.procedures == nil {
		return nil
	}
	p, ok := cc.procedures(g.Layout.ICAO)
	if !ok {
		return nil
	}
	app, ok := p.BestApproach(runway)
	if !ok {
		return nil
	}
	m, err := p.MissedApproach(app.Name)
	if err != nil {
		return nil
	}
	return m
}

// turnaround departs a parked arrival again (#296): after the dwell (or
// the "depart" action) a departure adopts the same aircraft on its stand,
// with the same call sign, stand reservation and runway; the arrival's
// entry leaves the list.
func (cc *controlCenter) turnaround(it *controlled, objectID uint32) {
	cc.log.printf("%-6s turnaround: parked, departing in %s", it.Tail, it.dwell.Round(time.Second))
	select {
	case <-traffic.After(it.dwell):
	case <-it.departNow:
	case <-it.removed:
		return // removed while parked: nothing to depart
	}
	d := *it.turn
	d.adopt = objectID
	// From the stand it is on now: changed since it spawned (#72: a stand
	// picked on the map or retaken, and the departure left the old one).
	it.mu.Lock()
	d.Stand = it.stand
	it.mu.Unlock()
	var dep *controlled
	err := cc.do(func() error {
		var err error
		dep, err = cc.spawn(it.graph, d)
		return err
	})
	if err != nil {
		cc.log.printf("%-6s turnaround: departure failed: %v", it.Tail, err)
		return
	}
	dep.mu.Lock()
	stand := dep.view.Stand
	dep.mu.Unlock()
	cc.log.printf("%-6s turnaround: departing from %s, runway %s (now #%d)", it.Tail, stand, d.Runway, dep.ID)
	cc.forget(it) // its ID block released too
}

// object is its sim object ID (0: not known yet), taking it.mu.
func (it *controlled) object() uint32 {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.objectID
}

// byTail is the controlled aircraft of a call sign, nil if none.
// byObject is the controlled aircraft with that object ID, nil if none.
func (cc *controlCenter) byObject(id uint32) *controlled {
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		own := it.objectID == id
		it.mu.Unlock()
		if own {
			return it
		}
	}
	return nil
}

// tellGiveWay is ground telling it to give way to other (by object ID):
// "CSA1, give way to the Airbus A320 passing left to right" — the other's
// type, and how it passes relative to this aircraft's heading.
// stoppedLogAfter: standing still taxiing this long is logged with why.
const stoppedLogAfter = 20 * time.Second

// logPushHeld logs a push held for traffic behind the stand, the aircraft
// named by its call sign, once while the name stays the same (live, LKPR:
// TVS512 logged held for TVS439 several times a second, the object in the
// way changing between TVS439 and its tug).
func (it *controlled) logPushHeld(tail, kind string, by uint32) {
	cc := it.cc
	who := "traffic"
	if o := cc.byObject(by); o != nil && by != 0 {
		who = o.Tail
	} else if by != 0 {
		who = fmt.Sprintf("object %d", by)
	}
	it.mu.Lock()
	same := it.pushHeldWho == who
	it.pushHeldWho = who
	it.mu.Unlock()
	if !same {
		cc.log.printf("%-6s %s: pushback holding for %s behind the stand", tail, kind, who)
	}
}

// logStopped logs why it stands still taxiing, if it still does for the
// same reason, with the object IDs named by their call signs.
func (it *controlled) logStopped(by string) {
	it.mu.Lock()
	still := it.stoppedBy == by
	it.mu.Unlock()
	if !still {
		return
	}
	said := objectIDs.ReplaceAllStringFunc(by, func(s string) string {
		id, _ := strconv.ParseUint(s[1:], 10, 32)
		if o := it.cc.byObject(uint32(id)); o != nil {
			return o.Tail
		}
		return "object " + s[1:]
	})
	it.cc.log.printf("%-6s %s: stopped %s — %s", it.Tail, it.Kind, stoppedLogAfter, said)
}

// giveWayCallGap: an aircraft is told to give way at most once in this
// long, whoever to (live, LKPR: CSA1146 told six times in 30 s as the
// one to give way to changed between a Dash 8, an A321 and a Citation).
const giveWayCallGap = 30 * time.Second

// holdShortShare: of the give-way calls where it stops short of another
// taxiway, the share said as "hold short of (taxiway)".
const holdShortShare = 0.5

// objectIDs are the object IDs in a StoppedBy reason.
var objectIDs = regexp.MustCompile(`#[0-9]+`)

func (it *controlled) tellGiveWay(other uint32) {
	o := it.cc.byObject(other)
	if o == nil || o == it {
		return
	}
	o.mu.Lock()
	model, oh, parking := o.view.Model, o.view.Heading, o.arr != nil && o.view.State == traffic.ArrivalParking.String()
	o.mu.Unlock()
	// Turning into its stand ahead, not passing: nothing to say, the crew
	// waits for it as for any traffic ahead (live, LKPR: EZY489 told to give
	// way to CSA1065 "passing left to right" as it turned in to C18).
	if parking || o.arr != nil && o.turningIn() {
		return
	}
	it.mu.Lock()
	now := it.cc.clock.Now()
	still, h, told := it.givingWay == other, it.view.Heading, it.toldWay == other || now.Sub(it.toldWayAt) < giveWayCallGap
	if still {
		it.toldWay = other
		if !told {
			it.toldWayAt = now
		}
	}
	// Short of another taxiway: sometimes "hold short of C" instead, then
	// "continue taxi" once the traffic has passed (the user).
	hs := ""
	if still && !told && it.giveWayHoldShort != "" && rand.Float64() < holdShortShare {
		hs, it.heldShortOf = it.giveWayHoldShort, it.giveWayHoldShort
	}
	it.mu.Unlock()
	if !still || told {
		return
	}
	if hs != "" {
		it.say(traffic.HoldShort(it.Tail, hs))
		return
	}
	desc := typeSaid(traffic.ProfileFor(strings.SplitN(model, liverySep, 2)[0]).Type)
	// Pushing back, or pushed and about to taxi: said so, not by its
	// heading, which in a push points away from where it goes (live,
	// LKPR: TVS770 told "passing right to left" for TVS1309 pushing onto J,
	// which then came at it head-on).
	var gp *traffic.GroundPicture
	if it.cc.world != nil {
		gp = it.cc.world.Ground(it.ICAO)
	}
	switch d := headingDiff(h, oh); {
	case gp != nil && gp.Pushing(other):
		desc += " pushing back"
	case gp != nil && gp.Waiting(other):
		desc += " about to taxi"
	case gp != nil && gp.SameWay(it.object(), other):
		// Onto its way ahead, the same direction: followed (live, CSA429
		// told "passing left to right" for TVS1169 pushed onto H1 ahead).
		gp.Follow(it.object(), other)
		it.say(traffic.FollowTaxi(it.Tail, desc))
		return
	case d >= 30 && d <= 150:
		desc += " passing left to right"
	case d <= -30 && d >= -150:
		desc += " passing right to left"
	case d > -30 && d < 30:
		desc += " ahead"
	default:
		desc += " coming the other way"
	}
	it.say(traffic.GiveWay(it.Tail, desc))
}

// standNoseOffset is the stand offset for model on stand: at a gate the
// simulator's own (gateNoseOffset), so a jetway's helper parks with its
// door where ours is; elsewhere, or not known, 0 (the profile's).
func (cc *controlCenter) standNoseOffset(g *airport.Graph, stand int, model string) float64 {
	nose, _ := cc.standPlace(g, stand, model)
	return nose
}

// standPlace is model's stand offset and side on stand (TaxiRequest
// NoseOffset, StandSide): at a gate as the simulator parks there, moved by
// the stand's rule (traffic.StandRuleFor) where it has one.
func (cc *controlCenter) standPlace(g *airport.Graph, stand int, model string) (nose, side float64) {
	nose, side = cc.standPlaceSim(g, stand, model)
	if g == nil || stand < 0 || stand >= len(g.Layout.Parking) {
		return nose, side
	}
	m := strings.SplitN(model, liverySep, 2)[0]
	prof := traffic.ProfileFor(m)
	r, ok := traffic.StandRuleFor(g.Layout.ICAO, g.Layout.Parking[stand].Label(), prof.WingspanM)
	if !ok {
		return nose, side
	}
	// At a jetway gate with the jetways on, where the simulator parks: the
	// jetway reaches no further (live, LKPR C22: a B738 9 m back, the
	// jetway docked short of it; the user: the jetway wins).
	if jetwaysEnabled() && cc != nil {
		if svc := cc.jetwayBuilt(); svc != nil && svc.HasJetway(g.Layout.ICAO, stand) {
			r.ForwardM = 0
		}
	}
	if nose <= 0 { // as the controllers take a 0 (StandPoint)
		nose = traffic.DefaultNoseOffsetMeters
	}
	return nose - r.ForwardM, side + r.RightM
}

// standPlaceSim is standPlace without the stand's rule.
func (cc *controlCenter) standPlaceSim(g *airport.Graph, stand int, model string) (nose, side float64) {
	if g == nil || stand < 0 || stand >= len(g.Layout.Parking) || !g.Layout.Parking[stand].IsGate() {
		return 0, 0
	}
	m := strings.SplitN(model, liverySep, 2)[0]
	helper := ""
	if jetwaysEnabled() && cc != nil {
		if svc := cc.jetwayBuilt(); svc != nil {
			helper = gateHelper(svc, g, stand, m, cc.hasModel)
		}
	}
	inset, side := SimNoseGearInsetM, 0.0
	if cc != nil {
		inset, side = cc.gatePlace(g, stand)
	}
	if helper == "" {
		side = 0 // no helper: no jetway to meet
	}
	if _, ok := cc.gateVariant(g, stand, m); ok {
		side = 0 // on the line: the helper's door makes up for the gate
		return gateNoseOffset(m, helper, inset), 0
	}
	return gateNoseOffset(m, helper, inset), gateSide(m, helper, side)
}

// typeSaid is an ICAO type designator as a controller says it: "Airbus
// A320", "Boeing 737", "Embraer 190", "ATR 72"; "aircraft" when unknown.
func typeSaid(icao string) string {
	switch {
	case icao == "":
		return "aircraft"
	case len(icao) == 4 && icao[0] == 'A' && icao[3] == 'N':
		return "Airbus A3" + icao[1:3] + "neo" // A20N: A320neo
	case strings.HasPrefix(icao, "A3") && len(icao) >= 4:
		return "Airbus A3" + icao[2:4]
	case len(icao) == 4 && icao[0] == 'B' && icao[1] == '3' && icao[3] == 'M':
		return "Boeing 737 MAX" // B38M
	case icao == "BCS1", icao == "BCS3":
		return "Airbus A220"
	case strings.HasPrefix(icao, "DH8"):
		return "Dash 8"
	case icao == "PC24":
		return "Pilatus PC-24"
	case icao == "PC12":
		return "Pilatus PC-12"
	case len(icao) == 4 && icao[0] == 'C' && (icao[1] == '5' || icao[1] == '6' || icao[1] == '7'):
		return "Citation" // C56X Excel, C68A Latitude, C700 Longitude
	case strings.HasPrefix(icao, "C25"):
		return "Citation" // C25A, C25B: the CJs
	case icao == "C208":
		return "Caravan"
	case icao == "DA42" || icao == "DA62":
		return "Diamond " + icao[:2] + "-" + icao[2:] // "Diamond DA-62"
	case icao == "E50P" || icao == "E55P":
		return "Phenom"
	case strings.HasPrefix(icao, "LJ"):
		return "Learjet"
	case icao == "E545" || icao == "E550":
		return "Praetor"
	case strings.HasPrefix(icao, "B7") && len(icao) >= 3:
		return "Boeing 7" + icao[2:3] + "7"
	case strings.HasPrefix(icao, "E1") || strings.HasPrefix(icao, "E7"):
		return "Embraer " + strings.TrimLeft(icao[1:], "0")
	case strings.HasPrefix(icao, "AT"):
		return "ATR " + icao[2:3] + "2"
	case strings.HasPrefix(icao, "CRJ"):
		return "CRJ " + icao[3:] + "00"
	case icao == "C152", icao == "C172":
		return "Cessna " + icao[1:]
	case icao == "P28A":
		return "Piper PA-28"
	case icao == "DA40":
		return "Diamond DA40"
	case icao == "SR22":
		return "Cirrus SR22"
	}
	return icao
}

// headingDiff is b - a in degrees, within -180..180.
func headingDiff(a, b float64) float64 {
	d := math.Mod(b-a+540, 360) - 180
	return d
}

// byTail is the flight of a call sign: one still going before a finished
// one (#79: a done flight answered for its successor).
func (cc *controlCenter) byTail(tail string) *controlled {
	cc.mu.Lock()
	var found []*controlled
	for _, it := range cc.items {
		if it.Tail == tail {
			found = append(found, it)
		}
	}
	cc.mu.Unlock()
	var done *controlled
	for _, it := range found {
		if !it.finished.Load() {
			return it
		}
		done = it
	}
	return done
}

// ownIDs are the object IDs of the controlled aircraft.
// ownTails are the call signs of our aircraft by object: a turnaround
// flies on as a new flight in the same object, whose ATC ID the simulator
// keeps (live, TVS1124 showed as TVS1482 on the runway). The flight still
// going wins over a finished one.
func (cc *controlCenter) ownTails() map[uint32]string {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	out := map[uint32]string{}
	for _, it := range cc.items {
		it.mu.Lock()
		id, tail, done := it.objectID, it.Tail, it.view.Done
		it.mu.Unlock()
		if id == 0 {
			continue
		}
		if _, have := out[id]; !have || !done {
			out[id] = tail
		}
	}
	return out
}

func (cc *controlCenter) ownIDs() map[uint32]bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	out := map[uint32]bool{}
	for id := range cc.own {
		out[id] = true
	}
	for _, it := range cc.items {
		it.mu.Lock()
		if it.objectID != 0 {
			out[it.objectID] = true
		}
		it.mu.Unlock()
	}
	return out
}

// addOwn and dropOwn mark aircraft of ours without a controller.
func (cc *controlCenter) addOwn(id uint32) {
	cc.mu.Lock()
	cc.own[id] = true
	cc.mu.Unlock()
}

func (cc *controlCenter) dropOwn(id uint32) {
	cc.mu.Lock()
	delete(cc.own, id)
	cc.mu.Unlock()
}

// holdView is a hold on the map: its fix, level and racetrack.
type holdView struct {
	Ident     string           `json:"ident"`
	AltFt     float64          `json:"altFt"`
	Racetrack []airport.LatLon `json:"racetrack"`
}

// spawnPoint is where an arrival appeared, and when.
type spawnPoint struct {
	tail  string
	at    airport.LatLon
	altFt float64
	when  time.Time
}

// Spawn separation: nobody appears within these of an airborne aircraft
// (the in-trail minimum is sepMinNM).
const (
	entryClearNM = 6.0 // 5 NM kept, and a mile for the leader slowing down
	entryClearFt = 2000.0
)

// nearAirborne names an airborne aircraft (any: ours or not) near p at
// altFt (0: any altitude), or one of ours that appeared there in the last
// minute; "" when clear. The aircraft tail itself does not count.
func (cc *controlCenter) nearAirborne(p airport.LatLon, altFt float64, tail string, now time.Time) string {
	near := func(q airport.LatLon, alt float64) bool {
		return calc.HaversineNM(q.Lat, q.Lon, p.Lat, p.Lon) < entryClearNM && (altFt == 0 || alt == 0 || math.Abs(alt-altFt) < entryClearFt)
	}
	for _, a := range cc.world.Aircraft() {
		if !a.OnGround && a.Tail != tail && near(a.Position, a.AltFt) {
			if a.Tail != "" {
				return a.Tail
			}
			return a.Title
		}
	}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	keep := cc.spawnedAt[:0]
	who := ""
	for _, sp := range cc.spawnedAt {
		if now.Sub(sp.when) > time.Minute {
			continue
		}
		keep = append(keep, sp)
		if who == "" && sp.tail != tail && near(sp.at, sp.altFt) {
			who = sp.tail
		}
	}
	cc.spawnedAt = keep
	return who
}

// forget drops a controlled aircraft from the list (its aircraft stays).
func (cc *controlCenter) forget(it *controlled) {
	cc.mu.Lock()
	if cc.items[it.ID] == it {
		delete(cc.items, it.ID)
		cc.ids.Release(it.defBase) // its controller is done: the IDs are free
	}
	v := cc.vehATC
	cc.mu.Unlock()
	// Its vehicles' calls go with it: an object ID used again is a new
	// vehicle, asking anew (#64).
	if v != nil {
		if it.tug != nil {
			v.forget(it.tug.ObjectID())
		}
		if it.fuel != nil {
			v.forget(it.fuel.ObjectID())
		}
	}
}

// remove takes a controlled aircraft out of the simulator and the list; in
// the connection goroutine.
func (cc *controlCenter) remove(it *controlled) error {
	it.mu.Lock()
	it.managed = nil
	it.mu.Unlock()
	err := it.act("remove", -1)
	it.stands.ReleaseOwner(it.Tail)
	it.removeOnce.Do(func() { close(it.removed) })
	cc.forget(it)
	return err
}

// reqModels asks the simulator for its aircraft titles (the model list).
const reqModels uint32 = 2004

// liverySep joins an aircraft title and its livery in the model list (MSFS
// 2024 spawns a title with an explicit livery).
const liverySep = " :: "

// requestModels enumerates the aircraft the simulator can spawn.
func (cc *controlCenter) requestModels() error {
	return cc.sim.ListModels()
}

// addModels collects the titles of one enumeration message.
func (cc *controlCenter) addModels(msg engine.Message) {
	e := msg.AsSimObjectAndLiveryEnumeration()
	n := uint32(e.DwArraySize)
	if n == 0 {
		return
	}
	header := uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_LIST_TEMPLATE{})) // 28 bytes
	size := uint32(unsafe.Sizeof(types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY{}))
	if n*size > uint32(msg.DwSize)-header {
		return
	}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	var added []string
	for i := uint32(0); i < n; i++ {
		entry := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(unsafe.Add(unsafe.Pointer(e), header+i*size)) // no uintptr kept: checkptr (-race) killed it
		if t := engine.BytesToString(entry.AircraftTitle[:]); t != "" {
			if l := engine.BytesToString(entry.LiveryName[:]); l != "" {
				t += liverySep + l
			}
			cc.models[t] = true
			added = append(added, t)
		}
	}
	if cc.onModels != nil && len(added) > 0 {
		go cc.onModels(added)
	}
}

// modelList returns the aircraft titles, sorted.
func (cc *controlCenter) modelList() []string {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	out := make([]string, 0, len(cc.models))
	for t := range cc.models {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// logChanges logs what an event changed about the aircraft.
func (it *controlled) logChanges(prev ControlView, ev TaxiOrArrival) {
	v := it.view
	who := fmt.Sprintf("%-6s %s", v.Tail, v.Kind)
	if v.State != prev.State {
		extra := ""
		if e := ev.arr; e != nil && e.State == traffic.ArrivalRollout && e.Touchdown != 0 {
			extra = fmt.Sprintf(" — touchdown %.0f m past the threshold, %.0f fpm, %.0f kt", e.Touchdown, e.TouchdownFpm, e.GroundSpeed)
		}
		if v.HoldingShortOf != "" {
			extra += " of " + v.HoldingShortOf
		}
		it.cc.log.printf("%s: %s → %s%s  (%.0f kt, hdg %.0f)", who, prev.State, v.State, extra, v.GroundSpeed, v.Heading)
	}
	if action := clearanceOf(v.Kind, prev.State, v.State); action != "" {
		if it.spoken[action] {
			delete(it.spoken, action) // said when given
		} else {
			var r *airport.Route
			if it.dep != nil {
				r = it.dep.Route()
			} else if p := it.arr.Plan(); p != nil {
				r = p.TaxiRoute()
			}
			said := v
			if said.HoldingShortOf == "" {
				said.HoldingShortOf = prev.HoldingShortOf // the runway just crossed
			}
			// (The crew's requests are said when made: onRequest, #462.)
			// An empty one is a phrase that does not exist for this action now (the
			// departure clearance after it has been given); saying it would invent one.
			if t := it.phraseView(said, r, action, -1); t.Intent != "" {
				it.say(t)
			}
		}
	}
	if v.Lights != prev.Lights && prev.Lights != "" {
		it.cc.log.printf("%s: lights %s → %s (%s)", who, prev.Lights, v.Lights, v.State)
	}
	if v.AtLimit != prev.AtLimit {
		if v.AtLimit {
			it.cc.log.printf("%s: holding at the clearance limit (node %d)", who, v.LimitNode)
		} else {
			it.cc.log.printf("%s: moving on from the clearance limit", who)
		}
	}
	if v.Error != "" && v.Error != prev.Error {
		it.cc.log.printf("%s: ⚠️ %s", who, v.Error)
	}
}

func entryNote(e string) string {
	if e == "" {
		return ""
	}
	return " at " + e
}

// airlineOf reads an airline code from a callsign-style tail ("BAW851" →
// "BAW"); "" when the tail is not one.
func airlineOf(tail string) string {
	if len(tail) < 4 {
		return ""
	}
	for i, c := range tail {
		switch {
		case i < 3 && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z'):
			return ""
		case i == 3:
			if c < '0' || c > '9' {
				return ""
			}
			return strings.ToUpper(tail[:3])
		}
	}
	return ""
}

// tug is the pushback tug of a departure, if asked for: a GSX tug model
// driven by the injector, created with the last request ID of the
// aircraft's block.
func (cc *controlCenter) tug(r SpawnRequest, reqBase uint32, prof traffic.MotionProfile, spanM float64) traffic.PushbackTug {
	if !r.Tug || !cc.core.vehicleOn(traffic.VehicleTug) {
		return nil
	}
	title := r.TugTitle
	if title == "" {
		title = traffic.TugTitleFor(spanM) // sized to the aircraft
	}
	t := traffic.NewSimObjectTug(cc.client, cc.inj, title, reqBase+controlIDBlock-1, prof)
	if g, err := cc.graph(r.ICAO); err == nil {
		t.Layout = g.Layout // from its depot on the vehicle roads, and back
		cc.giveATC(t, g.Layout, "tug")
	}
	if r.TugYaw != nil {
		t.YawDeg = *r.TugYaw
	}
	if ahead, ok := traffic.TugAheadFor(title); ok && r.TugAhead == nil {
		t.AheadMeters = ahead
	}
	if r.TugAhead != nil {
		t.AheadMeters = *r.TugAhead
	}
	return t
}

// phrase is the clearance as ATC says it (ICAO phraseology), e.g.
// "AFR1383, taxi to holding point runway 24 via B2, H, A".
func (it *controlled) phrase(action string, node airport.NodeID) traffic.Transmission {
	var r *airport.Route
	if it.dep != nil {
		r = it.dep.Route()
	} else if p := it.arr.Plan(); p != nil {
		r = p.TaxiRoute()
	}
	it.mu.Lock()
	v := it.view
	it.mu.Unlock()
	return it.rushed(it.phraseView(v, r, action, node))
}

// rushed is clearance t expedited when the aircraft is told to hurry
// (#510): "cleared for immediate take-off", "expedite crossing" …
func (it *controlled) rushed(t traffic.Transmission) traffic.Transmission {
	if it.rush.Load() {
		return traffic.Rushed(t)
	}
	return t
}

// qnh is the QNH at the user aircraft, hPa as said ("" without weather),
// and the same as an altimeter setting in inches ×100 for the FAA.
func (cc *controlCenter) qnh() (hPa, inches string) {
	if cc.weather == nil {
		return "", ""
	}
	w := cc.weather()
	if w == nil || w.QNHhPa <= 0 {
		return "", ""
	}
	return fmt.Sprintf("%d", int(math.Floor(w.QNHhPa))), fmt.Sprintf("%04d", int(math.Round(w.QNHhPa*0.0295300*100)))
}

// withParams is t said again with params added.
func withParams(t traffic.Transmission, params map[string]string) traffic.Transmission {
	p := map[string]string{}
	for k, v := range t.Params {
		p[k] = v
	}
	for k, v := range params {
		if v != "" {
			p[k] = v
		}
	}
	t.Params = p
	return traffic.Say(t)
}

// weatherShare: how often a crew asks for the weather on its check-in with
// tower (departures) or approach (arrivals).
const weatherShare = 0.15

// queueSaidShare: how often tower tells a departure checking in how many go
// before it, with traffic.QueueSaidFrom or more ahead; else "report ready".
const queueSaidShare = 0.5

// askWeather has the crew of it ask pos for the weather now and then, and
// the controller answer with the wind and QNH.
func (it *controlled) askWeather(pos traffic.Position) {
	if rand.Float64() >= weatherShare || it.gates.Load() {
		return
	}
	p := it.cc.pending
	p.later(it.clearAt(pos).Add(crewActDelay+p.jitter(crewActJitter)), func() {
		it.say(traffic.RequestWeather(pos, it.Tail))
		qnh, alt := it.cc.qnh()
		it.callAfter(pos, prioTaxi, it.clearAt(pos), func() {
			it.say(traffic.WeatherReport(pos, it.Tail, it.cc.windSaid(it.ICAO), qnh, alt))
		})
	})
}

// departureClimbSaid is the level departure clears a climbing departure
// to once identified, its filed cruise level when lower (departureLevel).
const (
	departureClimbSaid = "flight level 240"
	departureClimbFt   = 24000.0
)

// departureLevel is the level departure clears a departure with plan p
// (nil none) to: FL240, or its cruise level when lower (live, a short
// flight filed at FL150 cleared to FL240).
func departureLevel(p *planned, lim airport.Limits) (string, float64) {
	if p == nil || p.plan == nil || p.plan.CruiseFL <= 0 || float64(p.plan.CruiseFL)*100 >= departureClimbFt {
		return departureClimbSaid, departureClimbFt
	}
	ft := float64(p.plan.CruiseFL) * 100
	return traffic.LevelSaidAbove(ft, lim.TransitionAltitudeFt), ft
}

// departureTrailWithin: a departure identified this soon after another on
// the same route, cleared to the same level, is given another level.
const departureTrailWithin = 8 * time.Minute

// levelInTrail gives a departure the level departure clears it to, below
// the one a departure ahead of it on the same route got when that was
// identified within departureTrailWithin: in trail on one route they are
// not at one level (live, RYR1427 and TVS1620 both at FL240 on BALTU,
// closing, turned apart by the conflict watch). 2000 ft lower each, at
// most twice.
func (it *controlled) levelInTrail() {
	it.mu.Lock()
	ft, route := it.radarFt, it.view.Procedure
	it.mu.Unlock()
	if ft <= 0 || route == "" {
		return
	}
	now := it.cc.clock.Now()
	taken := map[float64]bool{}
	it.cc.mu.Lock()
	others := make([]*controlled, 0, len(it.cc.items))
	for _, o := range it.cc.items {
		others = append(others, o)
	}
	it.cc.mu.Unlock()
	for _, o := range others {
		if o == it || o.dep == nil || o.ICAO != it.ICAO {
			continue
		}
		o.mu.Lock()
		if o.identified && o.view.Procedure == route && now.Sub(o.identifiedAt) < departureTrailWithin {
			taken[o.radarFt] = true
		}
		o.mu.Unlock()
	}
	for i := 0; i <= 2 && taken[ft]; i++ {
		ft -= 2000
	}
	if taken[ft] {
		return // all taken: its own, the conflict watch keeps them apart
	}
	it.mu.Lock()
	if ft != it.radarFt {
		it.radarFt, it.radarSaid = ft, traffic.LevelSaidAbove(ft, it.cc.taOf(it.ICAO))
	}
	it.mu.Unlock()
}

// phraseView is phrase for a view the caller holds.
func (it *controlled) phraseView(v ControlView, r *airport.Route, action string, node airport.NodeID) traffic.Transmission {
	call, rwy := it.Tail, v.Runway
	spoken := func(n int) []string {
		if r == nil {
			return nil
		}
		return r.SpokenTaxiways(min(n, len(r.Edges)))
	}
	switch action {
	case "clearance":
		it.mu.Lock()
		due, arr := it.clearanceDue, it.arrivalDue
		it.mu.Unlock()
		if due != nil {
			return traffic.ClearedDeparture(call, *due)
		}
		if arr != nil {
			return *arr
		}
		// GIVEN ALREADY: there is no clearance to say, so say nothing. Falling
		// through reached the catch-all at the end of this switch, which builds a
		// transmission out of the action's NAME — "UAE168, clearance", attributed to
		// Ground, which is neither the words nor the position. `logChanges` skips an
		// empty intent, and `/phrase` answers with an empty controller transmission
		// rather than a sentence nobody would say.
		return traffic.Transmission{}
	case "approach":
		if it.arr != nil {
			qnh, _ := it.cc.qnh()
			return traffic.ClearedApproachTo(call, traffic.ApproachClearance{Kind: it.approachKind(), Runway: rwy, QNH: qnh, ReportEstablished: true})
		}
	case "handoff":
		it.mu.Lock()
		from, to := it.atc, it.handoffDue
		it.mu.Unlock()
		if to != "" {
			station, freq := it.station(to)
			return traffic.Handoff(call, from, to, station, freq)
		}
		// NOBODY TO HAND IT TO: no transmission, for the same reason as the
		// clearance above — the catch-all would otherwise say "UAE168, handoff".
		return traffic.Transmission{}
	case "pushback":
		if it.dep != nil {
			return reportReadyForTaxi(traffic.WithFacing(traffic.ClearedPushback(call), it.dep.PushFacingSaid()), it.dep)
		}
		return traffic.ClearedPushback(call)
	case "pushstart":
		if it.dep != nil {
			return reportReadyForTaxi(traffic.WithFacing(traffic.ClearedPushbackAndStartUp(call), it.dep.PushFacingSaid()), it.dep)
		}
		return traffic.ClearedPushbackAndStartUp(call)
	case "startup":
		return traffic.ClearedStartUp(call)
	case "taxi":
		// Told to hold position: on along the route it was cleared.
		if v.AtLimit && v.LimitNode < 0 {
			return traffic.ContinueTaxi(call)
		}
		n := 0
		if r != nil {
			n = len(r.Edges)
		}
		if it.dep != nil {
			entry := ""
			if r != nil {
				entry = r.Entry
			}
			return traffic.ClearedTaxiToRunway(call, rwy, entry, spoken(n))
		}
		return traffic.ClearedTaxiToStand(call, v.Stand, spoken(n))
	case "vacate":
		if it.arr != nil {
			if x, err := it.exitAt(node); err == nil {
				return traffic.VacateAtExit(call, traffic.PosTower, x.Side.String(), x.Taxiway)
			}
		}
		return traffic.Transmission{} // no exit named: not the catch-all's "CSA1, vacate"
	case "upto":
		if r != nil {
			if i := slices.Index(r.Nodes, node); i > 0 {
				limit := r.Edges[i-1].Name
				if i < len(r.Edges) && r.Edges[i].Name != "" && r.Edges[i].Name != limit {
					limit = r.Edges[i].Name // hold short of the taxiway joined there
				}
				return traffic.ClearedTaxiUpTo(call, spoken(i), limit)
			}
		}
		return traffic.ClearedTaxiUpTo(call, nil, "")
	case "cross":
		return traffic.ClearedCross(call, oneDesignator(v.HoldingShortOf)) // one designator (#462)
	case "lineup":
		return traffic.Backtracked(traffic.AtEntry(traffic.ClearedLineUp(call, rwy), v.Entry), it.dep != nil && v.Entry == "" && it.graph != nil && !it.graph.ThresholdEntry(rwy))
	case "takeoff":
		return traffic.AtEntry(traffic.ClearedTakeoff(call, rwy, it.cc.windSaid(it.ICAO)), v.Entry)
	case "hold":
		return traffic.HoldPosition(call)
	case "land":
		return traffic.ClearedToLand(call, rwy, it.cc.windSaid(it.ICAO))
	case "lineupbehind":
		what := "aircraft"
		if it.cc.behindSaid != nil {
			what = it.cc.behindSaid(it)
		}
		return traffic.ClearedLineUpBehind(call, what, rwy)
	case "goaround":
		return it.goAround("")
	case "abort":
		if v.State == traffic.TaxiDeparting.String() {
			return traffic.Stop(call)
		}
		return traffic.CancelTakeoff(call)
	}
	return traffic.Say(traffic.Transmission{Position: traffic.PosGround, Callsign: call, Intent: traffic.Intent(action)})
}

// reportReadyForTaxi adds "report ready for taxi" to a push approval, when
// the taxi clearance is still to come.
//
// IT IS THE OTHER HALF OF THE GATE. departureActions stops offering taxi to
// an aircraft on its stand, which is right — and on its own leaves the
// player with a strip that went quiet and no reason given. The approval now
// says what the next call will be, and the crew makes it (TaxiEvent.Request
// "taxi" → "request taxi"), so the exchange reads as the exchange it is.
//
// Not said when the aircraft is cleared to taxi already — a push given after
// a taxi clearance (the engine allows it, and a scripted scenario does it)
// has nothing left to report.
func reportReadyForTaxi(t traffic.Transmission, ctl departureCtl) traffic.Transmission {
	if ctl == nil || ctl.Cleared("taxi") {
		return t
	}
	return traffic.WithReportReady(t, "ready for taxi")
}

// towerHandoffMeters: a taxiing departure is handed to tower this far
// from where it holds short of its runway, give or take
// towerHandoffSpreadM, a different distance for each.
const (
	towerHandoffMeters  = 500.0
	towerHandoffSpreadM = 200.0
	// goAroundHandoffAfter: tower hands a go-around to approach this long
	// after "going around", climbing away.
	goAroundHandoffAfter = 30 * time.Second
)

// handoff moves the aircraft to the position working it now (#416): a
// change is said by the position handing over ("contact Praha Tower
// 118.105"). it.mu held.
func (it *controlled) handoff(ev TaxiOrArrival) {
	// Removed (a scene ending, the user, a failure): nobody hands it over —
	// an arrival cancelled on the final is not "runway vacated".
	if ev.arr != nil && (ev.arr.State == traffic.ArrivalCancelled || ev.arr.State == traffic.ArrivalFailed) ||
		ev.dep != nil && (ev.dep.State == traffic.TaxiCancelled || ev.dep.State == traffic.TaxiFailed) {
		return
	}
	// Rolling for a staffed departure: it climbs to the level of its
	// clearance and no further until the player clears it (#1096; its
	// crew asks for higher there, crewAsksHigher).
	if ev.dep != nil && ev.dep.State == traffic.TaxiDeparting && !it.climbCapped && it.clearedFt > 0 {
		it.climbCapped = true
		at, _ := it.station(traffic.PosDeparture)
		if _, staffed := it.cc.core.staffedAt(it.ICAO, traffic.PosDeparture, at); staffed {
			ft := it.clearedFt
			if it.afterDepFt > 0 && it.afterDepFt < ft {
				ft = it.afterDepFt
			}
			if it.dep.LimitClimb(ft) {
				it.cc.log.printf("%-6s departure staffed: climbs to %.0f ft until cleared higher", it.Tail, ft)
			}
		}
	}
	var pos traffic.Position
	switch {
	case ev.dep != nil:
		own := ev.dep.HoldingShortOf != "" && strings.Contains(ev.dep.HoldingShortOf, it.view.Runway)
		// VFR leaving a field without a tower: no one to check in with (#1314).
		if _, nobody := it.cc.uncontrolled(it.ICAO); nobody && it.view.Rules == "VFR" && ev.dep.State == traffic.TaxiComplete {
			return
		}
		pos = traffic.DeparturePosition(ev.dep.State, own)
		if pos == traffic.PosDelivery && it.delivered {
			pos = traffic.PosGround // VFR: no clearance, ground from the start (#568)
		}
		it.heightFt = ev.dep.HeightFt
		// Handed to departure once airborne and climbing away (7110.65 3-9-3:
		// about half a mile past the runway end), not at the hand-over to
		// MSFS AI.
		if it.handoffFt == 0 { // each crew its own moment
			it.handoffFt = departureHandoffFt + rand.Float64()*departureHandoffSpreadFt
			it.towerAtM = towerHandoffMeters + (2*rand.Float64()-1)*towerHandoffSpreadM
		}
		// A VFR departure stays with the tower out of the circuit: no radar
		// departure, no IFR climb (live, OKVFD: "climb to flight level 240").
		if ev.dep.State == traffic.TaxiDeparting && ev.dep.HeightFt >= it.handoffFt && it.view.Rules != "VFR" {
			pos = traffic.PosDeparture
		}
		// Handed to tower on the way to the runway, not at its holding point:
		// the crew calls tower ready as it gets there. Once with tower it
		// stays (a re-plan does not hand it back).
		if ev.dep.State == traffic.TaxiTaxiing && (it.atc == traffic.PosTower || ev.dep.Remaining > 0 && ev.dep.Remaining <= it.towerAtM) {
			pos = traffic.PosTower
		}
	case ev.arr != nil:
		// A VFR circuit arrival is the tower's from its first call (Doc 4444
		// 12.3.4.13): no approach clearance, no "established".
		onFinal := ev.arr.State == traffic.ArrivalApproaching && !ev.arr.OnGround && it.objectID != 0 && (len(it.arr.ProcedureRoute()) == 0 || it.circuit != nil)
		pos = traffic.ArrivalPosition(ev.arr.State, onFinal)
		if it.circuit != nil && pos == traffic.PosApproach {
			pos = traffic.PosTower // VFR in the circuit: the tower from its first call, spawning too
		}
	default:
		return
	}
	// After a touch-and-go, in the circuit again: its reports and the
	// tower's clearances start afresh, and it is sequenced again (#569).
	if ev.arr != nil && ev.arr.TouchAndGo {
		it.tngRolling = true
	}
	if ev.arr != nil && it.tngRolling && !ev.arr.TouchAndGo && ev.arr.State == traffic.ArrivalApproaching {
		it.tngRolling, it.downwindSaid, it.baseSaid, it.crossSaid, it.landedNoted = false, false, false, false, false
		it.vacateSaid.Store(false)
		it.wakeCautioned.Store(false)
		if _, ok := it.cc.uncontrolled(it.ICAO); ok {
			it.say(it.circuitReport("upwind")) // off again (#1307)
		}
		if it.cc.rejoin != nil {
			go it.cc.rejoin(it.ICAO, it.Tail)
		}
	}
	if ev.arr != nil && ev.arr.State == traffic.ArrivalRollout && !it.landedNoted {
		it.landedNoted = true
		it.cc.noteLanded(it.ICAO, it.view.Runway, it.view.Model)
	}
	// A VFR arrival reports downwind abeam the threshold (12.3.4.14 a).
	if ev.arr != nil && it.circuit != nil && !it.downwindSaid && ev.arr.State == traffic.ArrivalApproaching {
		if dw, ok := it.circuit.Point(traffic.LegDownwind); ok && calc.HaversineMeters(ev.arr.Position.Lat, ev.arr.Position.Lon, dw.Position.Lat, dw.Position.Lon) < 500 {
			it.downwindSaid = true
			report := "downwind"
			if it.arr.TouchAndGosLeft() > 0 {
				report = "downwind, touch and go"
				if it.stopAndGo {
					report = "downwind, stop and go"
				}
			}
			it.say(it.circuitReport(report))
			// The tower gives its place in the landing sequence (#569).
			it.call(traffic.PosTower, prioApproach, func() {
				// Told with an extended downwind already: not again.
				if n, tr, lead := it.circuitPlace(); n > 0 && int32(n) != it.placeSaid.Load() {
					it.placeSaid.Store(int32(n))
					it.say(traffic.FollowTraffic(it.Tail, n, tr))
					// As told: it lands after the one it follows, whatever the
					// predictions say once it turns in.
					if lead != "" && it.cc.followed != nil {
						it.cc.followed(it.ICAO, it.Tail, lead)
					}
				}
				it.wakeCaution()
			})
		}
	}
	// Nobody to ask them at a field without a tower: crosswind and base are
	// reported too, and each circuit round again, also after a go-around
	// (#1305, #1307).
	if ev.arr != nil && it.circuit != nil && ev.arr.State == traffic.ArrivalApproaching {
		if _, ok := it.cc.uncontrolled(it.ICAO); ok {
			if !it.crossSaid && it.arr.OnCrosswind() {
				it.crossSaid, it.downwindSaid, it.baseSaid = true, false, false
				it.say(it.circuitReport("crosswind"))
			}
			if !it.baseSaid && it.arr.OnBase() {
				it.baseSaid, it.crossSaid = true, false
				it.say(it.circuitReport("base"))
			}
		}
	}
	// On the landing roll the tower tells the crew to call ground when
	// vacated (Doc 4444 12.3.4.20; #462).
	if ev.arr != nil && ev.arr.State == traffic.ArrivalRollout && !ev.arr.TouchAndGo && !it.tngRolling && it.arr.TouchAndGosLeft() == 0 && !it.vacateSaid.Load() && !it.gates.Load() && it.atc == traffic.PosTower && it.vacateSaid.CompareAndSwap(false, true) {
		gs, gf := it.station(traffic.PosGround)
		tx := traffic.WhenVacatedContact(it.Tail, traffic.PosTower, traffic.PosGround, gs, gf)
		// Exits on both sides near its own: told which way and where
		// (the user).
		if p := it.arr.Plan(); p != nil && it.graph != nil && it.graph.ExitsBothSides(p.End.Name, p.Exit) {
			tx = traffic.WhenVacatedVia(it.Tail, traffic.PosTower, traffic.PosGround, gs, gf, p.Exit.Side.String(), p.Exit.Taxiway)
		}
		it.say(it.rushed(tx))
	}
	// On the base, before the turn onto the final, approach clears the
	// approach; the crew reports established on the final, and approach
	// hands it to tower then (Doc 4444 12.4.2.2 e; CAP 413 6.27, 6.28).
	// Off the STAR on a dog-leg or an extended downwind: approach vectors
	// it, each turn as it comes (#661).
	// A VFR downwind extended with "I'll call your base": the tower calls
	// it at the extended base turn, once the traffic it follows is on the
	// final inside it by its spacing; till then it flies on downwind.
	if ev.arr != nil && it.circuit != nil && ev.arr.State == traffic.ArrivalApproaching && !it.gates.Load() && it.arr.BaseDue() {
		if lead, toGo, gap := it.leaderToGo(); lead != "" && toGo > it.arr.CircuitFinalNM()-gap && it.arr.HoldDownwind() {
			it.cc.log.printf("%-6s tower: base held, %s still %.1f NM out", it.Tail, lead, toGo)
		} else {
			it.call(traffic.PosTower, prioLanding, func() { it.say(traffic.CircuitInstruction(it.Tail, traffic.InstrTurnBase)) })
		}
	}
	if ev.arr != nil && ev.arr.State == traffic.ArrivalApproaching && it.atc == traffic.PosApproach && pos == traffic.PosApproach &&
		!it.approachSaid && !it.gates.Load() && !it.arr.TurningFinal() {
		// Not with the approach clearance due: its intercept heading is the
		// vector then (live, EZY1672 heard "fly heading 140, for spacing"
		// and "turn left heading 060 to intercept" in the same second).
		if v, ok := it.arr.VectorDue(); ok {
			it.vectorsSaid.Add(1)
			it.say(it.vectored(v, it.cc.magVar(it.ICAO)))
		}
	}
	if ev.arr != nil && ev.arr.State == traffic.ArrivalApproaching && it.atc == traffic.PosApproach && pos == traffic.PosApproach &&
		!it.approachSaid && !it.gates.Load() && !it.firstPending.Load() && it.arr.TurningFinal() &&
		// Not for a runway closed or no longer in use, the change to come
		// (live, AUA133 "cleared ILS approach runway 06" 5 s after 06 closed).
		it.cc.core.arrivalRunwayInUse(it.ICAO, it.view.Runway) {
		qnh, _ := it.cc.qnh()
		// In good visibility some crews ask for a visual approach (#766);
		// not on vectors: there the clearance carries the heading to
		// intercept (live, OKZSU "cleared visual approach" 31 s into its
		// downwind, its turn in never said).
		if _, vectored := it.arr.InterceptHeading(); !vectored && it.wantsVisual() {
			it.say(traffic.RequestVisual(traffic.PosApproach, it.Tail))
			it.say(traffic.ClearedVisual(traffic.PosApproach, it.Tail, it.view.Runway))
			it.visual = true
		} else {
			ac := traffic.ApproachClearance{Kind: it.approachKind(), Runway: it.view.Runway, QNH: qnh, ReportEstablished: true}
			// On vectors: the heading to intercept with the clearance (#661).
			// The heading flown as told: from where it is to the final on it
			// (the user: told 060, it follows 060); one it cannot fly there
			// is not said, it flies its route on.
			if h, ok := it.arr.InterceptHeading(); ok {
				mv := it.cc.magVar(it.ICAO)
				h = math.Round((h+mv)/traffic.HeadingStepDeg)*traffic.HeadingStepDeg - mv // as said, in 5° steps
				if it.arr.InterceptOnHeading(h) {
					ac.Intercept, ac.Turn = traffic.HeadingSaid(h, mv), traffic.TurnTo(ev.arr.Heading, h)
					if g, err := it.cc.graph(it.ICAO); err == nil {
						if _, end, ok := g.Layout.RunwayEnd(it.view.Runway); ok {
							ac.FromTouchdownNM = int(math.Round(calc.HaversineNM(ev.arr.Position.Lat, ev.arr.Position.Lon, end.Threshold.Lat, end.Threshold.Lon)))
						}
					}
				}
			}
			it.say(traffic.ClearedApproachTo(it.Tail, ac))
		}
		it.approachSaid = true
	}
	station, freq := it.station(pos)
	was := it.view.Frequency
	it.view.ATC, it.view.Frequency = string(pos), freq
	if it.atc == "" || it.view.Done {
		it.atc = pos
		return
	}
	if pos == it.atc {
		// Into another station's sector of the same position (Ground to
		// Apron, #722): handed over, on the ground only, not mid-clearance.
		if was != "" && freq != "" && was != freq && pos == traffic.PosGround && !it.gates.Load() {
			// Pushing back: it stays with the station that cleared the push
			// until it is done, then calls the next when ready for taxi
			// (#1268; a conditional transfer, Doc 4444 12.3.1.4 b), its
			// own transmission after the push's readback (CAP 413 2.58).
			if ev.dep != nil && ev.dep.State == traffic.TaxiPushback {
				it.view.Frequency = was
				return
			}
			tx := traffic.Transmission{Position: pos, Frequency: was, Callsign: it.Tail, Intent: traffic.IntentContact,
				Params: map[string]string{traffic.ParamPosition: string(pos), traffic.ParamStation: station, traffic.ParamFreq: freq}}
			if ev.dep != nil && ev.dep.State == traffic.TaxiAwaitingTaxi {
				tx.Params[traffic.ParamWhen] = "when ready for taxi"
			}
			it.say(tx)
		}
		return
	}
	if ev.dep != nil && it.atc == traffic.PosDelivery && !it.delivered {
		// Delivery transfers it after the clearance (clearance); until then
		// it is delivery's, on the view too: a client clearing as the
		// position the view names was refused by mayClear (#1155).
		it.view.ATC, it.view.Frequency = string(it.atc), was
		return
	}
	// Out of a staffed station the player hands it over (#1096): the crew
	// stays on the player's frequency until told to contact the next
	// (action "handoff"), unless the station hands off by itself.
	if !it.handoffGiven {
		at, _ := it.station(it.atc)
		if s, staffed := it.cc.core.staffedAt(it.ICAO, it.atc, at); staffed && !s.AutoHandoff {
			if it.handoffDue != pos {
				it.cc.log.printf("%-6s leaving the staffed %s: the player hands it to %s", it.Tail, it.atc, pos)
			}
			it.handoffDue = pos
			it.view.ATC, it.view.Frequency = string(it.atc), was
			return
		}
	}
	it.handoffGiven, it.handoffDue = false, ""
	from := it.atc
	it.atc = pos
	// Approach clears the arrival for its approach before handing it to
	// tower on the final.
	if ev.arr != nil && from == traffic.PosApproach && pos == traffic.PosTower && !it.gates.Load() {
		// Cleared on the base; else (on the final already, as it appeared)
		// now. The crew reports established, then approach hands it over.
		if !it.approachSaid {
			qnh, _ := it.cc.qnh()
			it.say(traffic.ClearedApproachTo(it.Tail, traffic.ApproachClearance{Kind: it.approachKind(), Runway: it.view.Runway, QNH: qnh, ReportEstablished: true}))
		}
		if !it.visual {
			it.say(traffic.EstablishedReport(it.Tail, it.view.Runway))
		}
		it.approachSaid = false // a go-around is cleared again
		// At a field without a tower, the field in sight: the crew cancels
		// IFR with approach before going to the advisory frequency (#1305).
		if !it.ifrCancelled && it.unworked() && it.fieldInSight() {
			it.ifrCancelled = true
			it.say(traffic.CancelIFR(station, it.Tail))
			it.say(traffic.IFRCancelled(it.Tail))
		}
	}
	// Gone around: tower says "go around" and hears "going around" first,
	// then hands it to approach once climbing away, and the crew calls in
	// once (live, AUA529 heard "contact Ruzyne Radar" before "go around").
	if ev.arr != nil && from == traffic.PosTower && pos == traffic.PosApproach {
		it.goAroundCalling.Store(true)
		p := it.cc.pending
		p.later(it.cc.clock.Now().Add(goAroundHandoffAfter+p.jitter(atcAnswerJitter)), func() {
			it.say(traffic.Handoff(it.Tail, from, pos, station, freq))
			it.say(it.initial(traffic.CheckIn(pos, station, it.Tail, it.checkInReport(pos), "")))
			// Approach takes it in: radar contact, its level, the approach to
			// expect; vectors and the sequence follow.
			it.call(traffic.PosApproach, prioApproach, func() {
				it.mu.Lock()
				rwy, kind := it.view.Runway, it.approachKind()
				it.mu.Unlock()
				it.say(traffic.RadarContactAfterGoAround(it.Tail, it.goAroundLevel(), kind, rwy))
				it.goAroundCalling.Store(false)
			})
		})
		return
	}
	switch {
	case ev.arr != nil && from == traffic.PosTower && pos == traffic.PosGround && it.vacateSaid.Load():
		// Told on the landing roll: the crew calls ground when vacated.
	default:
		it.say(traffic.Handoff(it.Tail, from, pos, station, freq))
	}
	// A departure on its stand makes its first call to ground when it is
	// ready: the push request (#462).
	if ev.dep != nil && pos == traffic.PosGround && ev.dep.State == traffic.TaxiAwaitingPushback {
		return
	}
	// The pilot's first call on the new frequency (#417), with the ATIS
	// letter on the first of all (#418).
	info := ""
	if !it.atisSaid && it.cc.atisLetter != nil {
		info, it.atisSaid = it.cc.atisLetter(it.ICAO), true
	}
	checkIn := traffic.CheckIn(pos, station, it.Tail, it.checkInReport(pos), info)
	// At a field without a tower that is its announcement inbound (#1305).
	if ev.arr != nil && pos == traffic.PosTower && it.view.Rules != "VFR" {
		if _, ok := it.cc.uncontrolled(it.ICAO); ok {
			checkIn = withParams(checkIn, map[string]string{traffic.ParamCircuit: positionSaid(it.graph.Layout, ev.arr.Position)})
		}
	}
	it.say(it.initial(checkIn))
	// Nobody at the field to clear an IFR departure: approach releases it
	// (#1305).
	if ev.dep != nil && pos == traffic.PosTower && it.unworked() {
		it.say(traffic.ReleasedForDeparture(it.Tail))
	}
	switch {
	case ev.dep != nil && pos == traffic.PosDeparture && (it.gates.Load() || it.staffedNow(pos)):
		// Staffed: no "identified, climb to"; it levels at the level of its
		// clearance once handed over (crewAsksHigher), the climb past its
		// SID the player's (#1096).
	case ev.dep != nil && pos == traffic.PosDeparture && !it.gates.Load():
		// Departure identifies it and clears the climb on (#462).
		p := it.cc.pending
		it.callAfter(pos, prioApproach, it.clearAt(pos), func() {
			// Stopped for traffic since the check-in: identified only, the
			// climb is cleared on once clear of it (live, TVS524 "stop climb
			// at 4000 feet" then "identified, climb to flight level 240").
			it.levelInTrail()
			it.mu.Lock()
			level := it.radarSaid
			it.mu.Unlock()
			if stopped := it.cc.climbStopped; stopped != nil && stopped(it.Tail) {
				level = ""
			}
			// Above the arrivals only clear of them (release_plan.go).
			holdClimb := false
			var restrict traffic.Transmission // a level restriction with the climb
			if level != "" {
				it.mu.Lock()
				ft := it.radarFt
				it.mu.Unlock()
				if who := it.climbMeets(ft); who != "" {
					// A level until a fix past the arrival, else the climb held.
					if fix, below, route, ok := it.climbRestriction(ft); ok {
						if err := it.cc.do(func() error { return it.dep.Reroute(route) }); err == nil {
							restrict = traffic.CrossAt(traffic.PosDeparture, it.Tail, fix, below, it.cc.taOf(it.ICAO), false)
							it.cc.log.printf("%-6s departure: cross %s at or below %.0f ft, clear of %s", it.Tail, fix, below, who)
						}
					}
					if restrict.Text == "" {
						level, holdClimb = "", true
						it.cc.log.printf("%-6s departure: climb held, it would meet %s", it.Tail, who)
					}
				}
			}
			it.mu.Lock()
			it.identified, it.identifiedAt = true, it.cc.clock.Now()
			answer := it.directAnswer
			it.directAnswer = nil
			it.mu.Unlock()
			if level != "" && restrict.Text == "" {
				it.climbOn()
			}
			tx := traffic.Identified(traffic.PosDeparture, it.Tail, level)
			if restrict.Text != "" {
				tx = traffic.Joined(tx, restrict)
			}
			if answer != nil {
				if u, ok := answer(); ok {
					tx = traffic.Joined(tx, u)
				}
			}
			it.say(tx)
			if holdClimb {
				p.later(it.cc.clock.Now().Add(climbRecheckEvery), it.climbWhenClear)
			}
		})
	case ev.dep != nil && pos == traffic.PosTower:
		it.askWeather(traffic.PosTower)
		// The tower answers the check-in: line-up or take-off when it gives
		// it then, else report ready, or hold short behind those to depart
		// first (CAP 413 4.19, 4.20; live, TVS1359 heard nothing for 2m50s).
		it.callAfter(pos, prioCheckInReply, it.clearAt(pos), func() {
			if cleared := it.cc.departureCleared; cleared != nil && cleared(it.Tail) {
				return
			}
			it.mu.Lock()
			rwy := it.view.Runway
			it.mu.Unlock()
			ahead := 0
			if n := it.cc.departuresAhead; n != nil {
				ahead = n(it.ICAO, rwy, it.Tail)
			}
			// The queue said now and then, not every time (the user: "still
			// not always").
			if rand.Float64() >= queueSaidShare {
				ahead = 0
			}
			it.say(traffic.DepartureOrder(it.Tail, rwy, ahead))
		})
	case ev.arr != nil && pos == traffic.PosTower:
		// The tower answers the check-in at once: cleared to land when it
		// can be, else "continue approach" (12.3.4.15; live, AUA762 heard
		// nothing for 1m45s with a departure on the runway). After the
		// runway's calls (prioCheckInReply; offline LKPR, KLM1688 cleared for
		// take-off late behind "continue approach").
		it.callAfter(pos, prioCheckInReply, it.clearAt(pos), func() {
			if cleared := it.cc.landCleared; cleared != nil && cleared(it.Tail) {
				return
			}
			it.say(traffic.CircuitInstruction(it.Tail, traffic.InstrContinue))
		})
	}
}

// checkInReport is what the pilot reports on first calling pos.
func (it *controlled) checkInReport(pos traffic.Position) string {
	rwy := it.view.Runway
	switch {
	case it.dep != nil && pos == traffic.PosGround:
		return "stand " + it.view.Stand
	case it.dep != nil && pos == traffic.PosTower:
		// Handed over on the way (towerHandoffMeters): taxiing, the ready call
		// once holding short; there already, ready.
		entry := ""
		if r := it.dep.Route(); r != nil {
			entry = r.Entry
		}
		if it.view.State != traffic.TaxiHoldingShort.String() {
			return traffic.TaxiingToSaid(rwy, entry)
		}
		it.readySaid = true
		return traffic.HoldingShortSaid(rwy, entry) + ", ready for departure"
	case it.dep != nil && (pos == traffic.PosDeparture || pos == traffic.PosApproach):
		// Passing and cleared levels, the SID (Doc 4444 4.11.3; CAP 413 6.2).
		passing := math.Round((it.heightFt+it.graph.Layout.Altitude*3.28084)/100) * 100
		s := fmt.Sprintf("passing %.0f feet", passing)
		if it.climbSaid != "" {
			s += " climbing " + it.climbSaid
		}
		if it.procSaid != "" {
			s += ", " + it.procSaid
		}
		return s
	case it.arr != nil && pos == traffic.PosApproach:
		// Back from a go-around: climbing to the circuit's altitude.
		if fx := it.arr.CircuitFixes(); len(fx) > 0 {
			passing := math.Round((it.heightFt+it.graph.Layout.Altitude*3.28084)/100) * 100
			return fmt.Sprintf("going around, passing %.0f feet climbing %s", passing, it.goAroundLevel())
		}
	case it.arr != nil && pos == traffic.PosTower:
		if kind := it.approachKind(); kind != "" && !it.visual {
			return "established " + kind + " runway " + rwy
		}
		return "final runway " + rwy // Doc 4444 7.3: position
	case it.arr != nil && pos == traffic.PosGround:
		// CAP 413 4.68 "runway vacated", with the runway and the exit.
		if it.exitTwy != "" {
			return "runway " + rwy + " vacated at " + it.exitTwy
		}
		return "runway " + rwy + " vacated"
	}
	return ""
}

// approachKind is the approach an arrival flies ("ILS"), from its
// procedure as shown ("GOLOP 2B → ILS"); "" unknown. it.mu held.
// vectored is radar vector v to the arrival, the first off the STAR with
// the approach it leads to: "vectoring for ILS approach runway 24, fly
// heading 060, for spacing" (traffic.Vectoring).
func (it *controlled) vectored(v traffic.Vector, magVar float64) traffic.Transmission {
	tx := traffic.Vectored(it.Tail, v, magVar)
	if v.Fix == "" && !it.vectoringSaid.Swap(true) {
		tx = traffic.Vectoring(tx, it.approachKind(), it.view.Runway)
	}
	return tx
}

func (it *controlled) approachKind() string {
	_, kind, _ := strings.Cut(it.view.Procedure, " → ")
	return kind
}

// stationOf is position pos at icao as said, and its frequency ("" none):
// the position's station without a sector (#722), else traffic.StationFor
// at the airport's layout.
func (cc *controlCenter) stationOf(icao string, pos traffic.Position) (string, string) {
	if s, ok := traffic.PickStation(cc.stations(icao), pos, traffic.Where{}); ok {
		return s.Name, s.Freq
	}
	return traffic.PositionName(pos), ""
}

// departureHandoffFt: a departure is handed from tower to departure this
// high above the runway, climbing away, and up to departureHandoffSpreadFt
// higher — a different height for each: 1000 to 2500 ft.
const (
	departureHandoffFt       = 1000
	departureHandoffSpreadFt = 1500
)

// say sends t on the radio: logged as ATC and kept for /api/radio.
func (it *controlled) say(t traffic.Transmission) {
	// On the frequency of the station working it where it is (#722).
	if t.Frequency == "" && t.Position != "" && t.Position != traffic.PosATIS {
		_, t.Frequency = it.station(t.Position)
	}
	// The same ATC call to it twice in a moment is said once: two rules
	// deciding the same thing in one look (live, LKPR: EZY1029 "contact
	// Ruzyne Tower" twice in a millisecond, TVS252's sequence call too).
	if !t.Pilot && t.Text != "" {
		now := traffic.Now()
		it.sayMu.Lock()
		dup := t.Text == it.lastATC && now.Sub(it.lastATCAt) < sayTwiceWithin
		it.lastATC, it.lastATCAt = t.Text, now
		it.sayMu.Unlock()
		if dup {
			it.cc.log.printf("%-6s the same call again within %s, said once: %s", it.Tail, sayTwiceWithin, t.Text)
			return
		}
	}
	if t.Intent == traffic.IntentTaxi || t.Intent == traffic.IntentTaxiLimit {
		it.sayMu.Lock()
		it.taxiSaid = t.Text
		it.sayMu.Unlock()
	}
	it.cc.radio.Transmit(it.ICAO, t)
}

// taxiInForce reports a taxi clearance (action "taxi", or "upto" node) that
// is the one in force word for word, its limit the same: said again it is
// only noise on the frequency, and a readback (#1269). A changed route,
// runway or limit goes out, and so does a taxi after a hold position.
func (it *controlled) taxiInForce(action string, node airport.NodeID) bool {
	n := node
	if action == "taxi" {
		n = -1
	}
	var same bool
	switch {
	case it.dep != nil:
		same = it.dep.TaxiInForce(n)
	case it.arr != nil:
		same = it.arr.TaxiInForce(n)
	}
	if !same {
		return false
	}
	it.sayMu.Lock()
	said := it.taxiSaid
	it.sayMu.Unlock()
	return said != "" && it.phrase(action, node).Text == said
}

// sayingUntil is when what is booked on the radio for it has been said, nil
// when nothing is (ControlView.SayingUntil).
func (it *controlled) sayingUntil() *time.Time {
	at := it.cc.radio.AircraftClearAt(it.ICAO, it.Tail)
	if !at.After(it.cc.clock.Now()) {
		return nil
	}
	return &at
}

// sayTwiceWithin: the same ATC call to one aircraft this soon again is a
// duplicate, not a repeat.
const sayTwiceWithin = 2 * time.Second

// airFix is a named fix of a procedure on an air route.
type airFix struct {
	Ident string `json:"ident"`
	airport.LatLon
}

// goAroundFixes are the points of a go-around shown on its way back
// (traffic.ArrivalController.CircuitFixes): a VFR circuit arrival's
// circuit points, an IFR arrival's fixes only — it is on vectors, not
// flying a circuit (live, DLH792 shown "BASE" after its go-around).
func goAroundFixes(fx []airport.NavPoint, vfrCircuit bool) []airFix {
	var out []airFix
	for _, n := range fx {
		if !vfrCircuit && traffic.CircuitPointName(n.Ident) {
			continue
		}
		out = append(out, airFix{Ident: n.Ident, LatLon: n.Position})
	}
	return out
}

// fixesAhead are the fixes near route (within the mile a rounded turn
// passes inside its fix): those behind the aircraft are off it.
func fixesAhead(fixes []airFix, route []airport.LatLon) []airFix {
	var out []airFix
	for _, f := range fixes {
		for _, p := range route {
			if calc.HaversineNM(f.Lat, f.Lon, p.Lat, p.Lon) < 1.5 {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// via names the taxiways of the first n edges of a route: " via B2, H, A".
func via(r *airport.Route, n int) string {
	if r == nil {
		return ""
	}
	names := r.SpokenTaxiways(min(n, len(r.Edges)))
	if len(names) == 0 {
		return ""
	}
	return " via " + strings.Join(names, ", ")
}

// entryPoint is the named holding point of an intersection departure
// (" B" in "holding point B runway 24"), "" for full length.
func entryPoint(r *airport.Route) string {
	if r == nil || r.Entry == "" {
		return ""
	}
	return " " + r.Entry
}

// impliedBy are the clearances a clearance covers, not said on their own
// when the state changes: a take-off clearance from the holding point is
// no "line up and wait", a taxi clearance on the stand no "pushback", one
// up to a limit no "taxi".
func impliedBy(action string) []string {
	switch action {
	case "takeoff", "lineupbehind":
		return []string{"lineup"}
	case "pushstart":
		return []string{"pushback", "startup"}
	case "taxi":
		return []string{"pushback"}
	case "upto":
		return []string{"pushback", "taxi"}
	}
	return nil
}

// clearanceOf names the clearance a state change carries out ("" none):
// with gates off the controller clears itself, and the log still shows
// what ATC said.
func clearanceOf(kind, from, to string) string {
	switch {
	case from == to:
		return ""
	case to == traffic.TaxiPushback.String() && kind == "departure":
		return "pushback"
	case to == traffic.TaxiLiningUp.String() && kind == "departure":
		return "lineup"
	case to == traffic.TaxiDeparting.String() && kind == "departure":
		return "takeoff"
	case from == traffic.TaxiHoldingShort.String() && to == traffic.TaxiTaxiing.String():
		return "cross" // arrivals share the state names
	case to == traffic.TaxiTaxiing.String():
		return "taxi"
	}
	return ""
}

// reportTraffic puts the sim's other aircraft on the ground (MSFS AI, the
// user) into the ground picture, so the controlled aircraft stop for them
// too. The controlled ones report themselves.
func (cc *controlCenter) reportTraffic(scan []Traffic) {
	cc.mu.Lock()
	cc.scan = scan
	cc.mu.Unlock()
	cc.scanNow.Store(&scanned{at: traffic.Now(), list: scan})
	for _, t := range scan {
		if t.User {
			cc.detail.SetViewer(airport.LatLon{Lat: t.Latitude, Lon: t.Longitude})
		}
	}
	obs := make([]traffic.Observation, 0, len(scan))
	for _, t := range scan {
		obs = append(obs, traffic.Observation{ObjectID: t.ObjectID, Title: t.Title, Tail: t.Tail,
			Position: airport.LatLon{Lat: t.Latitude, Lon: t.Longitude}, AGLFt: t.AGL, AltFt: t.Alt, GroundKts: t.GroundKts,
			Heading: t.Heading, VSFpm: t.VerticalFpm, OnGround: t.OnGround, User: t.User, SpanM: t.Span})
	}
	// Ground pictures and stand allocators get it from here; our own
	// aircraft report themselves (SetOwn in update).
	cc.world.Observe(cc.clock.Now(), obs)
}

// gaRamps are the general aviation parking types: where a VFR light
// aircraft parks (#568).
var gaRamps = []types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{
	types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL,
	types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_MEDIUM, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_LARGE,
	types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_EXTRA,
}

// powerOutMaxSpanM: an aircraft this small or smaller (business jets up to
// a Citation Latitude or a Praetor 600, live: pushed from S14A and S20A
// facing the taxiway) leaves a GA ramp
// stand under its own power, no tug (traffic.TaxiRequest.PowerOut), where
// its loop out fits; else it is pushed.
const powerOutMaxSpanM = 25.0

// powerOut reports whether a departure of span spanM from stand st taxis
// out under its own power: a small aircraft on a general aviation ramp.
func powerOut(st airport.Parking, spanM float64) bool {
	if spanM <= 0 || spanM > powerOutMaxSpanM || st.IsGate() {
		return false
	}
	switch st.Type {
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_MEDIUM, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_LARGE,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_EXTRA:
		return true
	}
	return false
}

// weatherNow is the weather in use, nil when none is known.
func (it *controlled) weatherNow() *nav.Weather {
	if it.cc.weather == nil {
		return nil
	}
	return it.cc.weather()
}

// goodVisibility: visibility of 8 km or more under no ceiling below 3000
// ft, not in cloud (Doc 4444 6.5.3.3).
func goodVisibility(w *nav.Weather) bool {
	return w.VisibilityM >= 8000 && (w.CeilingFt <= 0 || w.CeilingFt >= 3000) && !w.InCloud
}

// fieldInSight: a crew can cancel IFR for the field ahead; unknown weather
// or visibility counts as good, as for VFR flights (scheduler.visual).
func (it *controlled) fieldInSight() bool {
	w := it.weatherNow()
	if w == nil {
		return true
	}
	c := *w
	if c.VisibilityM <= 0 {
		c.VisibilityM = 8000
	}
	return goodVisibility(&c)
}

// unworked: an IFR flight of ours at a field without a tower, approach not
// the player's: approach's calls for it are ours to say (#1305).
func (it *controlled) unworked() bool {
	if it.view.Rules == "VFR" || it.gates.Load() || it.staffedNow(traffic.PosApproach) {
		return false
	}
	_, ok := it.cc.uncontrolled(it.ICAO)
	return ok
}

// circuitReport is the crew's report of report, its position in the
// circuit; with the side and runway a field without a tower needs (#1305).
// The upwind has no side.
func (it *controlled) circuitReport(report string) traffic.Transmission {
	if _, ok := it.cc.uncontrolled(it.ICAO); !ok {
		return traffic.CircuitReport(it.Tail, report)
	}
	if report != "upwind" {
		report = string(it.circuit.Side) + " " + report
	}
	return withParams(traffic.CircuitReport(it.Tail, report), map[string]string{traffic.ParamRunway: it.circuit.Runway})
}

// wantsVisual: this arrival's crew asks for a visual approach (#766):
// about one crew in five (by its call sign), in good visibility.
func (it *controlled) wantsVisual() bool {
	w := it.weatherNow()
	if it.circuit != nil || w == nil || !goodVisibility(w) {
		return false
	}
	h := fnv.New32a()
	h.Write([]byte(it.Tail))
	return h.Sum32()%5 == 0
}
