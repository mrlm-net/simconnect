// Command airport-map loads the complete ground layout of an airport from the
// simulator with pkg/airport (runways, taxi paths, taxi points, taxi names and
// parking spots) and serves it on an interactive Leaflet map at
// http://127.0.0.1:8080, with departure routes computed by pkg/airport.
//
// It is a visual debugging tool for taxi routing: every feature on the map
// shows its raw facility index and TYPE value in a popup, so the taxi graph
// can be checked against what SimConnect actually returns.
//
//	go run ./cmd/airport-map                      # live, default LKPR
//	go run ./cmd/airport-map -dump                # also save <ICAO>.json
//	go run ./cmd/airport-map -file LKPR.json      # offline, no simulator
package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The page: web/index.html with its styles and scripts. web/classic.html
// is the previous page, served at /classic during the redesign.
//

// User aircraft position (and the simulation rate), polled once per
// second; the pause state as the simulator reports it (#413).
const (
	defAircraft uint32 = 2000
	reqAircraft uint32 = 2001
	evPause     uint32 = 2010
	evCom1Set   uint32 = 2011 // COM_RADIO_SET_HZ: the map tunes the user's COM1
	// The simulation from the map: pause and resume, rate up and down.
	evPauseOn  uint32 = 2012
	evPauseOff uint32 = 2013
	evRateUp   uint32 = 2014
	evRateDown uint32 = 2015
	// evFrame: every rendered frame; the camera is set on it, in step
	// with the picture.
	evFrame uint32 = 2016
)

// simEvents are the simulator events the map's pause and rate buttons send.
var simEvents = map[string]struct {
	id    uint32
	event string
}{
	"pause":  {evPauseOn, "PAUSE_ON"},
	"resume": {evPauseOff, "PAUSE_OFF"},
	"faster": {evRateUp, "SIM_RATE_INCR"},
	"slower": {evRateDown, "SIM_RATE_DECR"},
}

// Live traffic scan, requested every second.
const (
	defTraffic    uint32 = 2002
	reqTraffic    uint32 = 2003
	trafficRadius uint32 = traffic.MaxScanRadiusMeters // meters around the user aircraft: SimConnect's maximum
)

// trafficRaw matches the defTraffic data definition.
type trafficRaw struct {
	Title                                          [256]byte
	AtcID                                          [32]byte
	State                                          [256]byte
	Lat, Lon, AGL, GS, Heading, VS, OnGround, Gear float64
	Landing, Taxi, Strobe, Beacon, Nav             float64
	SpanFt                                         float64
	AltFt                                          float64 // MSL
}

// Traffic is one aircraft near the user, served at /api/traffic.
type Traffic struct {
	ObjectID uint32 `json:"objectId"`
	// Route: one of ours flown by MSFS AI, the way it still flies (its
	// plan, then its STAR); nil otherwise.
	Route       []airport.LatLon `json:"route,omitempty"`
	Title       string           `json:"title"`
	Tail        string           `json:"tail"`
	State       string           `json:"state"`
	Latitude    float64          `json:"lat"`
	Longitude   float64          `json:"lon"`
	AGL         float64          `json:"agl"`
	GroundKts   float64          `json:"groundKts"`
	Heading     float64          `json:"heading"`
	VerticalFpm float64          `json:"vs"`
	OnGround    bool             `json:"onGround"`
	Gear        float64          `json:"gear"`
	// Lights lists the lights that are on: L landing, T taxi, S strobe, B beacon, N nav.
	Lights string `json:"lights"`
	User   bool   `json:"user"`
	// Ours: driven by our controllers (spawned on the map or scheduled);
	// the rest is other traffic — MSFS AI, other add-ons.
	Ours bool `json:"ours"`
	// Span is the wing span in meters; Alt the altitude above sea level in feet.
	Span float64 `json:"span"`
	Alt  float64 `json:"alt"`
	// Type is the ICAO type designator from the model title ("B738"),
	// "" when not known.
	Type string `json:"type,omitempty"`
}

type aircraftRaw struct {
	Latitude  float64
	Longitude float64
	Heading   float64
	GroundKts float64
	OnGround  float64
	SimRate   float64 // SIMULATION RATE
	Com1      float64 // COM ACTIVE FREQUENCY:1, MHz
	Camera    float64 // CAMERA STATE: the simulator's camera now
	CamView   float64 // CAMERA VIEW TYPE AND INDEX:1: its view
	Zulu      float64 // ZULU TIME: seconds since midnight UTC, the simulator's
	Local     float64 // LOCAL TIME: seconds since local midnight
	Day       float64 // ZULU DAY OF MONTH
	Month     float64 // ZULU MONTH OF YEAR
	Year      float64 // ZULU YEAR
	DayPart   float64 // TIME OF DAY: 0 dawn, 1 day, 2 dusk, 3 night
}

// Aircraft is the user aircraft position served at /api/aircraft.
type Aircraft struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	Heading   float64 `json:"heading"`
	GroundKts float64 `json:"groundKts"`
	OnGround  bool    `json:"onGround"`
	// SimRate and Paused: the simulator's rate and pause (traffic follows
	// them, #413).
	SimRate float64 `json:"simRate"`
	Paused  bool    `json:"paused"`
	// Camera is the simulator's CAMERA STATE now (its numbering differs
	// between versions: see simCameraStates).
	Camera  int `json:"camera"`
	CamView int `json:"cameraView"`
	// ZuluSec and LocalSec: the simulator's time of day, seconds since
	// midnight UTC and local (at the user's aircraft).
	ZuluSec  float64 `json:"zuluSec"`
	LocalSec float64 `json:"localSec"`
	// The simulator's UTC date, and the part of the day at the aircraft
	// (0 dawn, 1 day, 2 dusk, 3 night).
	ZuluDay   int       `json:"zuluDay"`
	ZuluMonth int       `json:"zuluMonth"`
	ZuluYear  int       `json:"zuluYear"`
	DayPart   int       `json:"dayPart"`
	Updated   time.Time `json:"updated"`
}

// airportResponse is the /api/airport payload: the Layout plus when it was
// fetched.
type airportResponse struct {
	*airport.Layout
	FetchedAt time.Time `json:"fetchedAt"`
}

type state struct {
	// keepOnStop: Options.KeepOnStop.
	keepOnStop bool
	// dropped counts the host's messages Feed dropped (queue full): a
	// layout loaded meanwhile may miss records (live, MyCrew: LKPR with no
	// runways), so it is loaded again.
	dropped atomic.Uint64
	// actLink, when set, makes the World an actuator: its simulator side
	// served on it to a director (#710).
	actLink link
	// core is the traffic engine's state that outlives a connection (#710).
	core *core
	// reviewDir holds GeoJSON overlays for review (GET /api/overlay).
	reviewDir string
	cache     *airport.Cache

	// sim sends a pause or rate event to the simulator (simEvents); nil
	// while not connected.
	sim func(action string) error

	// camera is the camera operator of the live connection (nil without).
	camera *cameraMan

	mu         sync.Mutex
	fetched    map[string]time.Time
	waiters    map[string][]chan error
	aircraft   *Aircraft
	traffic    []Traffic
	trafficAt  time.Time
	live       bool
	control    *controlCenter // traffic control while connected (#322)
	schedule   *scheduler     // scheduled traffic while connected (#368)
	sequences  *sequences     // landing sequences while connected (#390)
	separation *sepMonitor    // airborne separation (#395)
	towers     *towers        // runway controllers (#393)
	conflicts  *conflictWatch // airborne conflicts and resolutions (#395)
	tcas       *tcasWatch     // TCAS of ours (#450)
	// procedures are the SIDs, STARs and approaches by ICAO (#312).
	procedures map[string]airport.Procedures
	// airportRefs: every airport the simulator knows, worldwide, where it
	// is (ICAO → its list entry), asked once a connection: an overflight's
	// ends without loading them (planOverflight).
	airportRefs map[string]traffic.AirportRef
	// requests asks the connection to load an airport (load); airways is
	// the airway graph for flight plans (#331), nil for direct routes.
	requests chan<- string
	airways  *nav.AirwayGraph
	// airwaysGiven is Options.Airways; airwaysBy each airport's read
	// from the sim (#799), merged into airways.
	airwaysGiven *nav.AirwayGraph
	airwaysBy    map[string]*nav.AirwayGraph
	// weather is the latest at the user aircraft; atis the information
	// services by ICAO (#357).
	weather *nav.Weather
	atis    map[string]*nav.ATISService
	// pads are the de-icing pads picked on the map (#323); pushes the
	// pushbacks drawn by hand.
	pads   *padStore
	pushes *pushStore
	// stations: the airports' own ATC stations (#722).
	stations *stationStore
}

func (s *state) setLive(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live = v
}

// finish records the outcome of a fetch and wakes everyone waiting for it.
// standardPushModel is the aircraft the stands' standard pushes are
// planned for: a narrow-body, the stands' usual user.
const standardPushModel = "FSLTL_B738_RYR"

// pushPlanQueue (core.pushes) plans every stand's standard push of an airport
// (traffic.PlanStandardPushes): a stand then pushes the same way whatever
// the runway. Only airports we push back at — planned when the first
// departure appears there, not for every airport loaded (destinations
// included: a minute or two of a core each, all at once) — and one airport
// at a time, in the background.
type pushPlanQueue struct {
	log  *trafficLog
	once sync.Once
	mu   sync.Mutex
	seen map[string]bool
	ch   chan *airport.Graph
}

// standardPushDuty: the planner works this share of the time on one core,
// resting after each stand, so it never runs a core flat out.
const standardPushDuty = 0.3

// standardPushFile is where an airport's standard pushes are kept between
// runs ("" none): planned once per layout, loaded on later starts.
func standardPushFile(icao string) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "mrlm-simconnect", "airport-map", "pushes", icao+".json")
}

// planStandardPushes loads g's saved standard pushes, plans the stands
// still missing one at a time at standardPushDuty, and saves them.
func planStandardPushes(log *trafficLog, g *airport.Graph) {
	icao, file := g.Layout.ICAO, standardPushFile(g.Layout.ICAO)
	loaded := 0
	if f, err := os.Open(file); err == nil {
		n, err := traffic.LoadStandardPushes(f, g)
		f.Close()
		if err != nil && !errors.Is(err, traffic.ErrStandardStale) {
			tlog.printf("%s: standard pushbacks: %v", icao, err)
		}
		loaded = n
	}
	start, worked := time.Now(), time.Duration(0)
	for i := range g.Layout.Parking {
		t := time.Now()
		traffic.PlanStandardPushes(g, standardPushModel, []int{i}) // a loaded stand is skipped
		d := time.Since(t)
		worked += d
		time.Sleep(time.Duration(float64(d) * (1 - standardPushDuty) / standardPushDuty))
	}
	if loaded == len(g.Layout.Parking) {
		tlog.printf("%s: standard pushbacks of %d stands loaded", icao, loaded)
		return
	}
	tlog.printf("%s: standard pushbacks of %d stands planned (%d loaded) in %s, %s of work", icao, len(g.Layout.Parking)-loaded, loaded, time.Since(start).Round(time.Second), worked.Round(time.Second))
	if file == "" {
		return
	}
	var buf bytes.Buffer
	if _, err := traffic.SaveStandardPushes(&buf, g); err != nil {
		return
	}
	err := os.MkdirAll(filepath.Dir(file), 0o755)
	if err == nil {
		err = os.WriteFile(file, buf.Bytes(), 0o644)
	}
	if err != nil {
		tlog.printf("%s: standard pushbacks not saved: %v", icao, err)
	}
}

// want queues g's airport, once.
func (q *pushPlanQueue) want(g *airport.Graph) {
	q.once.Do(func() {
		q.seen, q.ch = map[string]bool{}, make(chan *airport.Graph, 64)
		go func() {
			for g := range q.ch {
				planStandardPushes(q.log, g)
			}
		}()
	})
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.seen[g.Layout.ICAO] {
		return
	}
	q.seen[g.Layout.ICAO] = true
	select {
	case q.ch <- g:
	default:
	}
}

func (s *state) finish(icao string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.fetched[icao] = time.Now().UTC()
	}
	for _, w := range s.waiters[icao] {
		w <- err
	}
	delete(s.waiters, icao)
}

// startWorld wires the World's decisions to cc — the front end's hooks,
// the schedule, sequences, separation, conflicts, towers and their ticks —
// and returns what stops them: on the World's own connection (runOn) or
// a director's, with the simulator side elsewhere (#710).
func (st *state) startWorld(cc *controlCenter) (stopWorld func()) {
	// The front end: the open maps, the voice and the camera hear it all.
	cc.onChange = func(topic string) {
		hub.publish(topic) // the open maps
		if h := st.core.hooks.OnChange; h != nil {
			h(topic)
		}
	}
	// The host hears each transmission on a goroutine of its own, in order:
	// called where it is said, with an aircraft's lock held, a host that
	// asked the World for its views from the callback deadlocked (E28).
	heard := make(chan traffic.Transmission, heardQueue)
	var heardMu sync.Mutex
	heardClosed := false
	go func() {
		for t := range heard {
			if h := st.core.hooks.OnTransmission; h != nil {
				h(t) // the host: its voice (#419), which cuts the camera as heard
			} else {
				heardOnCamera(t) // no voice: the camera cuts as it is said
			}
		}
	}()
	cc.onTransmission = func(t traffic.Transmission) {
		cc.changed("radio") // the open maps fetch it now (push.go)
		heardMu.Lock()
		defer heardMu.Unlock()
		if heardClosed {
			return
		}
		select {
		case heard <- t:
		default:
			fmt.Fprintf(stdout, "⚠️  transmission not heard (the host is %d behind): %s\n", heardQueue, t.Text)
		}
	}
	stopHeard := func() {
		heardMu.Lock()
		defer heardMu.Unlock()
		if !heardClosed {
			heardClosed = true
			close(heard)
		}
	}
	cc.graph = st.cache.Graph
	cc.layout = st.cache.Layout
	cc.pads = st.pads.forAirport
	cc.localStations = st.stations.forAirport
	cc.weather = func() *nav.Weather {
		st.mu.Lock()
		defer st.mu.Unlock()
		return st.weather
	}
	cc.procedures = func(icao string) (airport.Procedures, bool) {
		st.mu.Lock()
		defer st.mu.Unlock()
		p, ok := st.procedures[icao]
		return p, ok
	}
	// Scheduled traffic: its own goroutine, as it waits for the connection.
	sched := newScheduler(st, cc)
	cc.extra = sched.handle
	seqs := newSequences(cc, sched)
	sep := newSepMonitor(cc.log)
	cw := newConflictWatch(sched)
	tcw := newTCASWatch(sched)
	cc.tcasView = tcw.view
	seqs.inConflict = cw.inConflict // conflict holds last until the conflict is over
	seqs.engaged = cw.engaged       // no shortcut undoes a resolution (#785)
	cc.climbStopped = cw.isStopped
	tw := newTowers(cc, sched)
	cc.landCleared = tw.landCleared
	cc.departureCleared, cc.departuresAhead = tw.departureCleared, tw.departuresAhead
	// A go-around is sequenced again (#394), and cleared to land again on
	// its next approach (#486).
	cc.rejoin = func(icao, tail string) {
		seqs.rejoin(icao, tail)
		tw.forgetLanding(tail)
		tw.dropBehind(tail) // nobody waits to line up behind an arrival that went around
	}
	cc.followed = seqs.behind
	// A call sign spawned again (a scene replayed): no clearance remembered.
	cc.forgetTower = tw.forgetTail
	cc.forgetFlight = func(tail string) { seqs.forget(tail); cw.forget(tail) }
	cc.lineUpBehind = tw.behindNext
	cc.behindSaid = func(it *controlled) string { return tw.arrivalSaid(tw.nextArrival(it)) }
	cc.sequencesAt = seqs.at
	cc.toFinal = func(icao, tail string) {
		if err := seqs.approachAction(icao, tail, "direct"); err != nil {
			cc.log.printf("%-6s arrival: direct to the final failed: %v", tail, err)
		}
	}
	cc.saidCallsign = sched.cfg.SaidCallsign // telephony as the schedule has it (#462)
	cc.namedAirport = sched.cfg.AirportName
	cc.atisLetter = st.atisLetter
	stop := make(chan struct{}) // this connection only
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		var atisAt time.Time // the last ATIS refresh (#418)
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				now := cc.clock.Now() // traffic time (#413)
				// Each part on its own: a panic in one is logged and the rest
				// go on, the next second too (E27: it ended the World).
				safe := func(part string, f func()) { recoverTick(part, f) }
				if now.Sub(atisAt) >= time.Minute {
					atisAt = now
					safe("atis", func() { st.atisTick(now, cc, sched.airports()) })
				}
				safe("pending", func() { cc.pending.run(now) })  // clearances and actions in radio order (#462)
				safe("agenda", func() { cc.agenda.run(now) })    // the controllers' calls, most urgent first
				safe("runways", func() { cc.checkRunways(now) }) // a new runway in use re-plans the traffic (#456)
				safe("schedule", func() { sched.tick(now) })
				safe("sequences", func() { seqs.tick(now) })
				air := cc.world.Aircraft()
				// Ours by our call sign, not the object's first ATC ID: a
				// turnaround flies on in the arrival's object (live, KLM1433
				// seen as KLM185, not steered as our departure).
				tails := cc.ownTails()
				for i := range air {
					if tail := tails[air[i].ObjectID]; tail != "" {
						air[i].Tail = tail
					}
				}
				safe("separation", func() {
					sep.needed = cc.separationNeeded(air, sched.airports())
					sep.tick(now, air)
				})
				safe("conflicts", func() { cw.tick(now, air) })
				safe("tcas", func() { tcw.tick(now, air) })
				safe("towers", func() { tw.tick(now) })
			}
		}
	}()
	st.mu.Lock()
	st.control, st.schedule, st.sequences, st.separation, st.towers, st.conflicts, st.tcas = cc, sched, seqs, sep, tw, cw, tcw
	st.mu.Unlock()

	st.setLive(true)
	return func() {
		close(stop)
		stopHeard()
		st.mu.Lock()
		st.control, st.schedule, st.sequences, st.separation, st.towers, st.conflicts, st.tcas = nil, nil, nil, nil, nil, nil, nil
		st.mu.Unlock()
		st.setLive(false)
	}
}

// heardQueue: transmissions waiting for the host (OnTransmission) at most;
// beyond, one is dropped with a warning rather than blocking the engine.
const heardQueue = 256

// runOn runs the traffic on a connected client whose messages arrive on
// stream (the client's own, or a host's fed through World.Feed), until ctx
// ends (ctx.Err()) or stream closes (nil: the simulator went away).
func runOn(ctx context.Context, st *state, client engine.Client, stream <-chan engine.Message, requests <-chan string, dumpDir string) error {
	if err := client.SubscribeToSystemEvent(evFrame, "Frame"); err != nil {
		fmt.Fprintln(stdout, "❌ SubscribeToSystemEvent(Frame):", err)
	}
	if e, ok := client.(systemEventStater); ok {
		e.SetSystemEventState(evFrame, types.SIMCONNECT_STATE_OFF) // on with the camera
	}
	if err := client.SubscribeToSystemEvent(evPause, "Pause"); err != nil {
		fmt.Fprintln(stdout, "❌ SubscribeToSystemEvent(Pause):", err)
	}
	// Following COM1 both ways: a frequency picked on the map tunes it.
	if err := client.MapClientEventToSimEvent(evCom1Set, "COM_RADIO_SET_HZ"); err != nil {
		fmt.Fprintln(stdout, "❌ MapClientEventToSimEvent(COM_RADIO_SET_HZ):", err)
	}
	for _, e := range simEvents {
		if err := client.MapClientEventToSimEvent(e.id, e.event); err != nil {
			fmt.Fprintf(stdout, "❌ MapClientEventToSimEvent(%s): %v\n", e.event, err)
		}
	}
	for i, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"},
		{"PLANE LONGITUDE", "degrees"},
		{"PLANE HEADING DEGREES TRUE", "degrees"},
		{"GROUND VELOCITY", "knots"},
		{"SIM ON GROUND", "bool"},
		{"SIMULATION RATE", "number"},
		{"COM ACTIVE FREQUENCY:1", "MHz"},
		{"CAMERA STATE", "number"},
		{"CAMERA VIEW TYPE AND INDEX:1", "number"},
		{"ZULU TIME", "seconds"},
		{"LOCAL TIME", "seconds"},
		{"ZULU DAY OF MONTH", "number"},
		{"ZULU MONTH OF YEAR", "number"},
		{"ZULU YEAR", "number"},
		{"TIME OF DAY", "enum"},
	} {
		if err := client.AddToDataDefinition(defAircraft, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
			fmt.Fprintf(stdout, "❌ AddToDataDefinition(%q): %v\n", v.name, err)
		}
	}
	if err := client.RequestDataOnSimObject(reqAircraft, defAircraft, types.SIMCONNECT_OBJECT_ID_USER,
		types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
		fmt.Fprintf(stdout, "❌ RequestDataOnSimObject: %v\n", err)
	}

	// Live traffic: title, tail, AI state, position and flight parameters.
	client.AddToDataDefinition(defTraffic, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)
	client.AddToDataDefinition(defTraffic, "ATC ID", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 1)
	client.AddToDataDefinition(defTraffic, "AI TRAFFIC STATE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 2)
	for i, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"GROUND VELOCITY", "knots"}, {"PLANE HEADING DEGREES TRUE", "degrees"}, {"VERTICAL SPEED", "feet per minute"},
		{"SIM ON GROUND", "bool"}, {"GEAR TOTAL PCT EXTENDED", "percent"}, // native 0–1; "percent" returns it unscaled
		{"LIGHT LANDING", "bool"}, {"LIGHT TAXI", "bool"}, {"LIGHT STROBE", "bool"}, {"LIGHT BEACON", "bool"}, {"LIGHT NAV", "bool"},
		{"WING SPAN", "feet"}, {"PLANE ALTITUDE", "feet"},
	} {
		client.AddToDataDefinition(defTraffic, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i+3))
	}
	var (
		scan   []Traffic
		userID uint32
		// Last position per object: aircraft moved by position injection
		// (pkg/traffic Injector) report 0 kt, so their speed is derived.
		lastPos = map[uint32]fix{}
	)

	// The loader sends facility requests; this loop hands it every message.
	ids := st.core.libIDs()
	loader := airport.NewLoader(client, airport.LoaderWithCache(st.cache), airport.LoaderWithIDs(ids.loaderDef, ids.loaderReq),
		airport.LoaderWithTimeout(loaderTimeout)) // its error before the callers' 30 s
	procLoader := airport.NewProcedureLoaderWithIDs(client, ids.procDef, ids.procReq)
	// The runways' ILS: frequency and name from their navaid records.
	navLoader := nav.NewNavLoaderWithIDs(client, ids.navDef, ids.navReq, 8)
	st.core.resetILS() // lookups of a connection before: never answered now
	// Weather at the user aircraft, whenever it changes.
	weather := nav.NewWeatherReader(client, weatherDefID, weatherReqID)
	if err := weather.Subscribe(); err != nil {
		fmt.Fprintf(stdout, "⚠️  weather: %v\n", err)
	}

	// Traffic control: controllers live in this goroutine; HTTP handlers
	// queue commands to it.
	cc := newControlCenter(client, st.core)
	stopWorld := st.startWorld(cc)
	defer stopWorld()
	// COM1 tuned from the map, in this connection's goroutine.
	if h := st.core.hooks.OnTune; h != nil {
		defer h(nil)
		h(func(mhz float64) error {
			return cc.do(func() error {
				return client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, evCom1Set, uint32(math.Round(mhz*1e6)),
					types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY)
			})
		})
	}
	// What the loop reads goes to the World through the feed (#710).
	var feed simFeed = localFeed{st: st, cc: cc}
	// An actuator (#710): its simulator side served to a director.
	var act *actuatorSim
	st.mu.Lock()
	actLink := st.actLink
	st.mu.Unlock()
	if actLink != nil {
		act, feed = st.actuate(ctx, cc, client, actLink)
	}
	// The airways around each airport loaded, from the sim (#799).
	airways := newAirwayKeeper(client, feed, st.core.hooks.DataDir, st.core.hooks.AirwaysMaxAge, ids.crawlDef, ids.crawlReq, st.core.log.printf)
	// The airports around, for the traffic picture: now and every minute.
	airports := traffic.NewAirportLister(client, ids.airportList)
	if err := airports.Request(); err != nil {
		fmt.Fprintf(stdout, "⚠️  airport list: %v\n", err)
	}
	// Every airport worldwide, once: where an overflight's ends are, not
	// loaded in full for it (live: KJFK's taxiways loaded at LKPR for a
	// Bucharest–New York overflight, the frame loop stood still meanwhile).
	everywhere := traffic.NewAirportLister(client, ids.airportList+1)
	if err := everywhere.RequestAll(); err != nil {
		fmt.Fprintf(stdout, "⚠️  worldwide airport list: %v\n", err)
	}
	if err := cc.requestFuelTitles(); err != nil {
		fmt.Fprintf(stdout, "⚠️  fuel truck list: %v\n", err)
	}
	if err := cc.requestModels(); err != nil {
		fmt.Fprintf(stdout, "❌ model list: %v\n", err)
	}
	// The camera on our traffic: cut to the aircraft on the radio as the
	// call is heard.
	st.mu.Lock()
	st.sim = func(action string) error {
		e, ok := simEvents[action]
		if !ok {
			return fmt.Errorf("action: pause, resume, faster or slower")
		}
		return cc.do(func() error {
			return client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, e.id, 0, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY)
		})
	}
	st.mu.Unlock()
	defer func() {
		st.mu.Lock()
		st.sim = nil
		st.mu.Unlock()
	}()
	cam := newCameraMan(cc, client)
	// The view the simulator's camera is on: stepping starts from it.
	cam.sim.stateNow = func() (int, bool) {
		st.mu.Lock()
		defer st.mu.Unlock()
		if st.aircraft == nil {
			return 0, false
		}
		return st.aircraft.Camera, true
	}
	cam.sim.viewNow = func() (int, bool) {
		st.mu.Lock()
		defer st.mu.Unlock()
		if st.aircraft == nil {
			return 0, false
		}
		return st.aircraft.CamView, true
	}
	cam.frames = func(on bool) {
		state := types.SIMCONNECT_STATE_OFF
		if on {
			state = types.SIMCONNECT_STATE_ON
		}
		if e, ok := client.(systemEventStater); ok {
			go cc.do(func() error { return e.SetSystemEventState(evFrame, state) })
		}
	}
	// The camera goes back to the simulator however this connection ends:
	// a camera left acquired stays stuck for the user.
	defer func() {
		if cam.dir != nil {
			cam.dir.Release()
		}
	}()
	cameraHeard.Store(&cam)
	defer cameraHeard.Store(nil)
	st.mu.Lock()
	st.camera = cam
	st.mu.Unlock()
	defer func() {
		st.mu.Lock()
		st.camera = nil
		st.mu.Unlock()
	}()
	camTick := time.NewTicker(cameraRate)
	defer camTick.Stop()
	var lastFrame time.Time          // the simulator's last frame event
	droppedAt := map[string]uint64{} // by ICAO: Feed's drop count when its layout was asked
	var excWindow time.Time          // SimConnect exceptions logged since
	excLogged := 0
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			// Stopped, the connection perhaps still open: ours leave the
			// simulator (Options.KeepOnStop keeps them).
			if !st.keepOnStop {
				st.mu.Lock()
				sched := st.schedule
				st.mu.Unlock()
				removeOurs(cc, sched)
			}
			return ctx.Err()

		case cmd := <-cc.cmds:
			cmd()

		case now := <-camTick.C:
			// Only when the simulator sends no frames (its menu): on frames
			// the camera moves with the picture, no double or missed steps.
			if now.Sub(lastFrame) > 3*cameraRate {
				cam.tick(now)
			}

		case icao := <-requests:
			fmt.Fprintf(stdout, "🛫 Fetching facility data for %s...\n", icao)
			droppedAt[strings.ToUpper(icao)] = st.dropped.Load()
			tlog.printf("%s: loading the airport from the simulator", strings.ToUpper(icao)) // in the log too: a host may not show stdout
			if err := loader.Request(icao); err != nil {
				feed.Layout(icao, nil, err)
			}
			if err := procLoader.Request(icao); err != nil {
				fmt.Fprintf(stdout, "❌ procedures of %s: %v\n", icao, err)
			}

		case now := <-tick.C:
			cc.tick()
			// The airports around once a minute (#70: ticks never counted, asked
			// every second, a multi-part list reset half-way).
			if cc.ticks%60 == 0 {
				airports.Request()
			}
			cc.ticks++
			// Every aircraft within TrafficRadius of the user aircraft; the scan
			// restarts with its first entry, not here (#81: a reply still coming
			// in was cut, aircraft vanished for a cycle).
			client.RequestDataOnSimObjectType(reqTraffic, defTraffic, trafficRadius, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
			airways.tick(now)
			for _, r := range navLoader.Expire(now) {
				feed.ILS(r)
			}
			st.core.retryILS(navLoader)
			for _, res := range loader.Expire(now) {
				tlog.printf("%s: airport not loaded: %v", res.ICAO, res.Err)
				fmt.Fprintf(stdout, "❌ %v\n", res.Err)
				feed.Layout(res.ICAO, nil, res.Err)
			}

		case msg, ok := <-stream:
			if !ok {
				fmt.Fprintln(stdout, "📴 Simulator disconnected")
				for _, icao := range loader.Pending() {
					feed.Layout(icao, nil, errors.New("simulator disconnected"))
				}
				return nil
			}
			if msg.Err != nil {
				fmt.Fprintf(stdout, "❌ Stream error: %v\n", msg.Err)
				continue
			}

			if list, ok := everywhere.Handle(msg); ok {
				refs := make(map[string]traffic.AirportRef, len(list))
				for _, a := range list {
					refs[strings.ToUpper(a.ICAO)] = a
				}
				st.mu.Lock()
				st.airportRefs = refs
				st.mu.Unlock()
				tlog.printf("airports worldwide: %d", len(refs))
				continue
			}
			if list, ok := airports.Handle(msg); ok {
				feed.Airports(list)
				continue
			}
			if wx, ok := weather.Handle(msg); ok {
				feed.Weather(wx)
				continue
			}
			if r, done := navLoader.Handle(msg); done {
				feed.ILS(r)
				continue
			}
			if airways.handle(msg) {
				continue
			}
			if p, done := procLoader.Handle(msg); done {
				feed.Procedures(p)
				airways.want(p, time.Now())
				continue
			}
			if res, done := loader.Handle(msg); done {
				// Messages dropped while it loaded: some of its records may be
				// missing; not kept, loaded again when next asked.
				if at, ok := droppedAt[res.ICAO]; ok && res.Err == nil && st.dropped.Load() != at {
					st.cache.Invalidate(res.ICAO)
					res.Err = fmt.Errorf("%s: %d messages dropped while it loaded (Options.QueueSize): load it again", res.ICAO, st.dropped.Load()-at)
				}
				delete(droppedAt, res.ICAO)
				if res.Err != nil {
					tlog.printf("%s: airport not loaded: %v", res.ICAO, res.Err)
				} else {
					tlog.printf("%s: airport loaded: %d runways, %d stands, %d taxi points", res.ICAO, len(res.Layout.Runways), len(res.Layout.Parking), len(res.Layout.TaxiPoints))
				}
				if res.Err != nil {
					fmt.Fprintf(stdout, "❌ %v\n", res.Err)
				} else {
					l := res.Layout
					fmt.Fprintf(stdout, "🏁 %s %s: %d runways, %d parking, %d taxi points, %d taxi paths, %d names\n",
						l.ICAO, l.Name, len(l.Runways), len(l.Parking), len(l.TaxiPoints), len(l.TaxiPaths), len(l.TaxiNames))
					if l.HasTower {
						fmt.Fprintf(stdout, "🗼 %s tower %.5f, %.5f at %.0f m (airport %.0f m)\n", l.ICAO, l.Tower.Lat, l.Tower.Lon, l.TowerAltitude, l.Altitude)
					}
					if dumpDir != "" {
						writeDump(dumpDir, res.Raw)
					}
					st.core.requestILS(navLoader, l)
				}
				feed.Layout(res.ICAO, res.Layout, res.Err)
				continue
			}
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT_FRAME {
				lastFrame = time.Now()
				cc.clock.Frame() // the simulator standing still (a model loading): the traffic too
				cam.tick(lastFrame)
				continue
			}
			if cam.handle(msg) {
				continue
			}
			if act != nil && act.handle(msg) {
				continue
			}
			if cc.handle(msg) {
				continue
			}

			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Fprintf(stdout, "⚠️  SimConnect exception %d (sendID=%d, index=%d)\n", e.DwException, e.DwSendID, e.DwIndex)
				// In the log too, at most excLogPerMinute a minute: what the
				// simulator refused (a host may not show stderr).
				if time.Since(excWindow) > time.Minute {
					excWindow, excLogged = time.Now(), 0
				}
				if excLogged++; excLogged <= excLogPerMinute {
					tlog.printf("simconnect exception %d (send %d, index %d)", e.DwException, e.DwSendID, e.DwIndex)
				}

			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				if uint32(d.DwRequestID) != reqAircraft {
					continue
				}
				a := engine.CastDataAs[aircraftRaw](&d.DwData)
				userID = uint32(d.DwObjectID) // the user aircraft's real object ID, as by-type scans report it
				com1 := ""
				if a.Com1 > 0 {
					com1 = airport.FormatMHz(a.Com1)
				}
				feed.UserAircraft(Aircraft{Latitude: a.Latitude, Longitude: a.Longitude, Heading: a.Heading,
					GroundKts: a.GroundKts, OnGround: a.OnGround != 0, Camera: int(a.Camera), CamView: int(a.CamView), ZuluSec: a.Zulu, LocalSec: a.Local,
					ZuluDay: int(a.Day), ZuluMonth: int(a.Month), ZuluYear: int(a.Year), DayPart: int(a.DayPart)}, a.SimRate, com1)

			case types.SIMCONNECT_RECV_ID_EVENT:
				if e := msg.AsEvent(); uint32(e.UEventID) == evPause {
					feed.Paused(e.DwData != 0)
				}

			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE:
				d := msg.AsSimObjectDataBType()
				if uint32(d.DwRequestID) != reqTraffic {
					continue
				}
				t := engine.CastDataAs[trafficRaw](&d.DwData)
				id, now := uint32(d.DwObjectID), time.Now()
				if t.GS < 0.5 {
					t.GS = derivedKts(lastPos[id], t.Lat, t.Lon, now)
				}
				vs := derivedFpm(lastPos[id], t.AltFt, now)
				lastPos[id] = fix{lat: t.Lat, lon: t.Lon, at: now, altFt: t.AltFt, vs: vs}
				if uint32(d.DwEntryNumber) <= 1 {
					scan = scan[:0] // a new scan
				}
				scan = append(scan, Traffic{
					ObjectID: uint32(d.DwObjectID), Title: engine.BytesToString(t.Title[:]), Tail: engine.BytesToString(t.AtcID[:]),
					State: engine.BytesToString(t.State[:]), Latitude: t.Lat, Longitude: t.Lon, AGL: t.AGL, GroundKts: t.GS,
					Heading: t.Heading, VerticalFpm: vs, OnGround: t.OnGround != 0, Gear: t.Gear, Lights: lights(t), Span: t.SpanFt * 0.3048, Alt: t.AltFt, User: uint32(d.DwObjectID) == userID || uint32(d.DwObjectID) == types.SIMCONNECT_OBJECT_ID_USER,
				})
				if uint32(d.DwEntryNumber) >= uint32(d.DwOutOf) {
					feed.Traffic(scan)
					scan = nil
				}
			}
		}
	}
}

// writeDump saves the raw facility records, the format pkg/airport tests and
// -file read.
func writeDump(dir string, raw airport.RawAirport) {
	path := filepath.Join(dir, raw.ICAO+".json")
	b, err := json.MarshalIndent(raw, "", "  ")
	if err == nil {
		err = os.WriteFile(path, b, 0o644)
	}
	if err != nil {
		fmt.Fprintf(stdout, "❌ Dump %s: %v\n", path, err)
		return
	}
	fmt.Fprintf(stdout, "💾 Wrote %s\n", path)
}

// ── HTTP side ──────────────────────────────────────────────────────────────

// load returns a cached layout or fetches it from the simulator.
func (s *state) load(ctx context.Context, icao string, refresh bool, requests chan<- string) (*airport.Layout, error) {
	if l, ok := s.cache.Layout(icao); ok && !refresh {
		return l, nil
	}
	s.mu.Lock()
	if !s.live {
		s.mu.Unlock()
		return nil, fmt.Errorf("simulator not connected and %s not loaded from file", icao)
	}
	done := make(chan error, 1)
	first := len(s.waiters[icao]) == 0
	s.waiters[icao] = append(s.waiters[icao], done)
	s.mu.Unlock()

	// Given up: no longer waiting, so the next load asks again (#71: a first
	// waiter cancelled before its request stayed, and the airport could never
	// be loaded again).
	leave := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		ws := s.waiters[icao]
		for i, w := range ws {
			if w == done {
				ws = append(ws[:i:i], ws[i+1:]...)
				break
			}
		}
		if len(ws) == 0 {
			delete(s.waiters, icao)
		} else {
			s.waiters[icao] = ws
		}
	}
	if first {
		select {
		case requests <- icao:
		case <-ctx.Done():
			leave()
			return nil, ctx.Err()
		}
	}
	select {
	case err := <-done:
		if err != nil {
			return nil, err
		}
		l, _ := s.cache.Layout(icao)
		return l, nil
	case <-ctx.Done():
		leave()
		return nil, ctx.Err()
	}
}

// Register serves the traffic engine's HTTP API (/api/...) on mux: the
// airport map's API, also what a remote client of the World calls.
func (w *World) Register(mux *http.ServeMux) {
	st, requests := w.st, w.st.requests
	icaoParam := func(r *http.Request) string {
		return strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("icao")))
	}

	mux.HandleFunc("GET /api/airport", func(w http.ResponseWriter, r *http.Request) {
		icao := icaoParam(r)
		if icao == "" || len(icao) > 8 {
			http.Error(w, "icao query parameter required", http.StatusBadRequest)
			return
		}
		l, err := st.load(r.Context(), icao, r.URL.Query().Get("refresh") != "", requests)
		if err != nil {
			status := http.StatusBadGateway
			if !st.isLive() {
				status = http.StatusServiceUnavailable
			}
			http.Error(w, err.Error(), status)
			return
		}
		st.mu.Lock()
		at := st.fetched[icao]
		st.mu.Unlock()
		writeJSON(w, airportResponse{Layout: l, FetchedAt: at})
	})

	// GET /api/geojson?icao=LKPR — the layout as a GeoJSON FeatureCollection.
	registerControl(mux, st)
	registerProcedures(mux, st)
	registerGame(mux, st)
	registerAirportInfo(mux, st)
	// GET /api/overlay?name=LKPR-A1-24-after — a GeoJSON overlay from the
	// review folder (<dump-dir>/review), drawn with ?overlay= on the page:
	// push and taxi-start paths written by the tests, for review.
	mux.HandleFunc("GET /api/overlay", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" || strings.Trim(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") != "" {
			http.Error(w, "bad overlay name", http.StatusBadRequest)
			return
		}
		b, err := os.ReadFile(filepath.Join(st.reviewDir, name+".geojson"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/geo+json")
		w.Write(b)
	})
	registerPush(mux)
	registerCamera(mux, st)
	registerTowers(mux, st)
	registerCircuits(mux, st.core, func() *controlCenter { st.mu.Lock(); defer st.mu.Unlock(); return st.control })
	registerVFRPoints(mux, st.core)
	registerDeicing(mux, st)
	registerWorld(mux, st)
	registerSchedule(mux, st)
	registerReal(mux, st)
	registerTCAS(mux, st)
	registerSequence(mux, st)
	registerSeparation(mux, st)
	registerRunways(mux, st)
	registerApproach(mux, st)
	registerStatus(mux, st)
	registerPushback(mux, st)
	registerStations(mux, st)
	registerPlayer(mux, st)
	registerCorridor(mux, st)

	mux.HandleFunc("GET /api/geojson", func(w http.ResponseWriter, r *http.Request) {
		l, ok := st.cache.Layout(icaoParam(r))
		if !ok {
			http.Error(w, "airport not loaded", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/geo+json")
		json.NewEncoder(w).Encode(l.FeatureCollection())
	})

	// GET /api/route?icao=LKPR&from=18&to=24 — departure route from a parking
	// index to a runway end, computed with pkg/airport.
	mux.HandleFunc("GET /api/route", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		g, err := st.cache.Graph(icaoParam(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		from, err := strconv.Atoi(q.Get("from"))
		if err != nil {
			http.Error(w, "from must be a parking index", http.StatusBadRequest)
			return
		}
		opts, err := routeOptions(g, q)
		if err != nil {
			routeError(w, err)
			return
		}
		route, err := g.RouteToRunwayEntry(from, q.Get("to"), q.Get("entry"), opts)
		if err != nil {
			routeError(w, err)
			return
		}
		writeJSON(w, route)
	})

	// GET /api/node?icao=LKPR&lat=..&lon=.. — the taxi node nearest to a
	// point, for picking via points of a custom route (#340).
	mux.HandleFunc("GET /api/node", func(w http.ResponseWriter, r *http.Request) {
		g, err := st.cache.Graph(icaoParam(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
		lon, _ := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
		n, names, d := nearestNode(g, airport.LatLon{Lat: lat, Lon: lon})
		if n.ID < 0 || d > 150 {
			http.Error(w, "no taxiway near this point", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"id": n.ID, "position": n.Position, "taxiways": names, "distance": d})
	})

	// GET /api/entries?icao=LKPR&runway=24 — entries onto a runway end for
	// departures ("24 at B"), full length first.
	mux.HandleFunc("GET /api/entries", func(w http.ResponseWriter, r *http.Request) {
		g, err := st.cache.Graph(icaoParam(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		entries, err := g.RunwayEntries(r.URL.Query().Get("runway"))
		if err != nil {
			routeError(w, err)
			return
		}
		type entry struct {
			airport.RunwayEntry
			Position airport.LatLon `json:"position"` // where the entry meets the runway
			// With ?model=: the runway the type needs to take off here
			// (traffic.RequiredTakeoffRun at this elevation) and whether the
			// entry leaves enough of it.
			Required float64 `json:"required,omitempty"`
			OK       bool    `json:"ok"`
		}
		need := 0.0
		if model := r.URL.Query().Get("model"); model != "" {
			title, _, _ := strings.Cut(model, liverySep)
			need = traffic.RequiredTakeoffRun(traffic.TakeoffProfileFor(title),
				traffic.TakeoffConditions{ElevationFt: convert.MetersToFeet(g.Layout.Altitude)})
		}
		out := make([]entry, len(entries))
		for i, e := range entries {
			out[i] = entry{e, g.Nodes[e.RunwayNode].Position, need, e.Remaining >= need}
		}
		writeJSON(w, out)
	})

	// GET /api/exits?icao=LKPR&runway=24 — exits for aircraft landing on a
	// runway end, nearest the threshold first.
	mux.HandleFunc("GET /api/exits", func(w http.ResponseWriter, r *http.Request) {
		g, err := st.cache.Graph(icaoParam(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		exits, err := g.RunwayExits(r.URL.Query().Get("runway"))
		if err != nil {
			routeError(w, err)
			return
		}
		type exit struct {
			airport.RunwayExit
			Position airport.LatLon `json:"position"` // where the exit leaves the runway
		}
		out := make([]exit, len(exits))
		for i, e := range exits {
			out[i] = exit{e, g.Nodes[e.RunwayNode].Position}
		}
		writeJSON(w, out)
	})

	// GET /api/arrival?icao=LKPR&runway=24&to=18[&exit=3] — taxi-in route from
	// a runway exit to a parking index. exit is an index into /api/exits;
	// without it the exit is chosen as the arrival controller would.
	mux.HandleFunc("GET /api/arrival", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		g, err := st.cache.Graph(icaoParam(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		to, err := strconv.Atoi(q.Get("to"))
		if err != nil {
			http.Error(w, "to must be a parking index", http.StatusBadRequest)
			return
		}
		ro, err := routeOptions(g, q)
		if err != nil {
			routeError(w, err)
			return
		}
		opts := traffic.ArrivalOptions{Route: ro}
		if s := q.Get("exit"); s != "" {
			exits, err := g.RunwayExits(q.Get("runway"))
			if err != nil {
				routeError(w, err)
				return
			}
			i, err := strconv.Atoi(s)
			if err != nil || i < 0 || i >= len(exits) {
				http.Error(w, "exit must be an index into /api/exits", http.StatusBadRequest)
				return
			}
			opts.Exit = &exits[i]
		}
		plan, err := traffic.PlanArrival(g, q.Get("runway"), to, opts)
		if err != nil {
			routeError(w, err)
			return
		}
		writeJSON(w, struct {
			Route  *airport.Route     `json:"route"`
			Exit   airport.RunwayExit `json:"exit"`
			Vacate airport.LatLon     `json:"vacate"` // where the aircraft stops clear of the runway
			Stop   airport.LatLon     `json:"stop"`   // reference point on the stand
		}{plan.Route, plan.Exit, plan.Route.Points[plan.VacateIndex], plan.Stop})
	})

	// GET /api/traffic — every aircraft within MaxScanRadiusMeters (200 km) of the user aircraft.
	mux.HandleFunc("GET /api/traffic", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		t, at := st.traffic, st.trafficAt
		st.mu.Unlock()
		if time.Since(at) > 5*time.Second {
			t = nil
		}
		if t == nil {
			t = []Traffic{}
		}
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		st.mu.Lock()
		sched := st.schedule
		st.mu.Unlock()
		var routes map[uint32][]airport.LatLon
		if sched != nil {
			routes = sched.enrouteRoutes()
		}
		if cc != nil {
			ours, tails := cc.ownIDs(), cc.ownTails()
			t = append([]Traffic(nil), t...)
			for i := range t {
				t[i].Ours = ours[t[i].ObjectID]
				t[i].Route = routes[t[i].ObjectID] // ours flown by MSFS AI: the way it goes
				if tail := tails[t[i].ObjectID]; tail != "" {
					t[i].Tail = tail // our call sign, not the object's first ATC ID
				}
				t[i].Type = typeOfTitle(t[i].Title)
			}
		}
		writeJSON(w, t)
	})

	// POST /api/sim?action=pause|resume|faster|slower — the simulation from
	// the map; its state comes with /api/aircraft (paused, simRate).
	mux.HandleFunc("POST /api/sim", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		f := st.sim
		st.mu.Unlock()
		if f == nil {
			http.Error(w, "not connected to the simulator", http.StatusServiceUnavailable)
			return
		}
		if err := f(r.URL.Query().Get("action")); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/aircraft", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		a := st.aircraft
		st.mu.Unlock()
		if a == nil || time.Since(a.Updated) > 5*time.Second {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, a)
	})
}

func (s *state) isLive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.live
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

// lights summarises the light SimVars as letters: L landing, T taxi,
// S strobe, B beacon, N nav.
func lights(t *trafficRaw) string {
	var b strings.Builder
	for _, l := range []struct {
		on     float64
		letter byte
	}{{t.Landing, 'L'}, {t.Taxi, 'T'}, {t.Strobe, 'S'}, {t.Beacon, 'B'}, {t.Nav, 'N'}} {
		if l.on != 0 {
			b.WriteByte(l.letter)
		}
	}
	return b.String()
}

// routeError maps pkg/airport routing errors to HTTP statuses.
// routeOptions are the route query's options (#340): runwayPaths, the
// custom route (via=node,node… and taxiways=A,B…, checked against the
// airport) and the aircraft's size from model=, so a custom route never
// takes it where it does not fit.
func routeOptions(g *airport.Graph, q url.Values) (airport.RouteOptions, error) {
	o := airport.RouteOptions{UseRunwayPaths: q.Get("runwayPaths") != ""}
	for _, v := range strings.Split(q.Get("via"), ",") {
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return o, fmt.Errorf("via: %q is not a node", v)
		}
		o.Via = append(o.Via, airport.NodeID(n))
	}
	for _, t := range strings.Split(q.Get("taxiways"), ",") {
		if t = strings.ToUpper(strings.TrimSpace(t)); t != "" {
			o.Taxiways = append(o.Taxiways, t)
		}
	}
	if m := q.Get("model"); m != "" {
		model, _, _ := strings.Cut(m, liverySep)
		o.HalfSpan = traffic.MotionProfileFor(model).SpanMeters / 2
	}
	return o, g.ValidateRouteOptions(o)
}

// nearestNode is the taxi node nearest to p (stands excluded), with the
// names of the taxiways meeting there.
func nearestNode(g *airport.Graph, p airport.LatLon) (airport.Node, []string, float64) {
	best, bd := airport.Node{ID: -1}, math.Inf(1)
	for _, n := range g.Nodes {
		if n.Kind == airport.NodeParking {
			continue
		}
		if d := calc.HaversineMeters(p.Lat, p.Lon, n.Position.Lat, n.Position.Lon); d < bd {
			best, bd = n, d
		}
	}
	var names []string
	if best.ID >= 0 {
		for _, e := range g.Adj[best.ID] {
			if e.Name != "" && !slices.Contains(names, e.Name) {
				names = append(names, e.Name)
			}
		}
	}
	return best, names, bd
}

func routeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, airport.ErrNoRoute) {
		status = http.StatusUnprocessableEntity
	}
	http.Error(w, err.Error(), status)
}

// fix is an aircraft position at a time.
type fix struct {
	lat, lon float64
	at       time.Time
	altFt    float64 // feet MSL
	vs       float64 // vertical speed derived so far (fpm)
}

// derivedFpm is the vertical speed from the altitude change since the last
// fix, smoothed: VERTICAL SPEED is wrong for the aircraft we place (live, an
// arrival descending on final read +700 fpm and showed climbing).
func derivedFpm(prev fix, altFt float64, now time.Time) float64 {
	dt := now.Sub(prev.at).Seconds()
	if prev.at.IsZero() || dt < 0.2 || dt > 10 {
		return 0
	}
	return 0.5*prev.vs + 0.5*(altFt-prev.altFt)/dt*60
}

// derivedKts is the ground speed from the distance moved since the last
// fix, in knots; 0 without a usable previous fix.
func derivedKts(prev fix, lat, lon float64, now time.Time) float64 {
	dt := now.Sub(prev.at).Seconds()
	if prev.at.IsZero() || dt < 0.2 || dt > 10 {
		return 0
	}
	return calc.HaversineMeters(prev.lat, prev.lon, lat, lon) / dt / 0.514444
}

// typeOfTitle is the ICAO type designator of a model title ("FSLTL_B738_RYR"
// → "B738", traffic.ProfileFor), remembered per title; "" when unknown.
func typeOfTitle(title string) string {
	typeTitlesMu.Lock()
	defer typeTitlesMu.Unlock()
	if t, ok := typeTitles[title]; ok {
		return t
	}
	t := traffic.ProfileFor(title).Type
	typeTitles[title] = t
	return t
}

var (
	typeTitlesMu sync.Mutex
	typeTitles   = map[string]string{}
)

// excLogPerMinute: SimConnect exceptions written to the traffic log a
// minute at most.
const excLogPerMinute = 20

// loaderTimeout: an airport load not answered by then ends in
// airport.ErrTimeout, before the 30 s its callers wait (a director's
// SetSchedule saw only "context deadline exceeded").
const loaderTimeout = 25 * time.Second

// recoverTick runs f, one part of the World's second, and logs a panic in
// it with its stack (once a minute per part) instead of ending the World.
func recoverTick(part string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			tickPanics.Lock()
			defer tickPanics.Unlock()
			if time.Since(tickPanics.at[part]) < time.Minute {
				return
			}
			tickPanics.at[part] = time.Now()
			tlog.printf("world: %s panicked: %v", part, r)
			fmt.Fprintf(stdout, "❌ world: %s panicked: %v\n%s\n", part, r, debug.Stack())
		}
	}()
	f()
}

// tickPanics: when each part last logged a panic.
var tickPanics = struct {
	sync.Mutex
	at map[string]time.Time
}{at: map[string]time.Time{}}

// removeOurs removes every aircraft (and with them their tugs and service
// vehicles) and en route flight of ours from the simulator, on the
// connection's own goroutine — the World stopped (RunOn's ctx ended) while
// the connection lives on: a host that restarts its traffic on the same
// connection found the old aircraft left frozen where they were. Errors
// are ignored: a connection already gone has nothing to remove.
func removeOurs(cc *controlCenter, sched *scheduler) {
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		it.managed = nil
		it.mu.Unlock()
		_ = it.act("remove", -1)
		it.removeOnce.Do(func() { close(it.removed) })
	}
	if sched == nil {
		return
	}
	sched.mu.Lock()
	var enroute []uint32
	for _, e := range sched.enroute {
		if e.objectID != 0 {
			enroute = append(enroute, e.objectID)
		}
	}
	sched.mu.Unlock()
	for _, id := range enroute {
		_ = cc.sim.RemoveObject(id, reqRemoveEnroute)
	}
}
