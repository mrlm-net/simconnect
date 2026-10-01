//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
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
	ID     int    `json:"id"`
	Kind   string `json:"kind"` // departure | arrival
	Tail   string `json:"tail"`
	ICAO   string `json:"icao"`
	dep    *traffic.TaxiController
	arr    *traffic.ArrivalController
	graph  *airport.Graph
	stands *traffic.StandAllocator
	stand  int  // parking index held for this aircraft
	left   bool // a departure has left its stand (released)
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
	defBase  uint32 // its ID block (cc.ids)
	// gates: the user gives every clearance ("hold at every clearance");
	// otherwise the tower clears it onto and across runways (#393).
	gates atomic.Bool
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
	heightFt            float64
	vacateSaid          bool
	approachSaid        bool // its approach clearance, given on the base
	readySaid           bool // a departure's "ready for departure"
	// rush: told to hurry (#510): its clearances are the expedited ones.
	rush atomic.Bool
	// handoffFt and towerAtM: where this departure goes to departure
	// (height) and to tower (meters short of the runway), varied.
	handoffFt, towerAtM float64
	// request is what the crew asks for now (TaxiEvent.Request, #462);
	// waiting one made before its clearance was done (delivered: the
	// delivery exchange finished and the aircraft with ground).
	request, waiting string
	delivered        bool
	// pushAndStart: the crew asked for the pushback and the start-up in one
	// call, approved together.
	pushAndStart bool
	// givingWay is the aircraft it gives way to (TaxiEvent.GivingWayTo),
	// told by ground once.
	givingWay uint32
	// fixes: the named points of its procedure (STAR and approach, or SID),
	// the dots of its air route on the map — not the points of the turns.
	fixes []airFix

	mu   sync.Mutex
	view ControlView
}

// ControlView is what the map shows of a controlled aircraft.
type ControlView struct {
	ID int `json:"id"`
	// ATC and Frequency: the position working it and its frequency (#416).
	ATC            string           `json:"atc,omitempty"`
	Frequency      string           `json:"frequency,omitempty"`
	Kind           string           `json:"kind"`
	ICAO           string           `json:"icao"` // its airport
	Tail           string           `json:"tail"`
	Model          string           `json:"model"`
	Stand          string           `json:"stand"`
	Runway         string           `json:"runway"`
	Procedure      string           `json:"procedure,omitempty"` // SID, or STAR → approach
	OnGround       bool             `json:"onGround"`
	PushbackHeld   bool             `json:"pushbackHeld,omitempty"` // the pushback waits for traffic behind
	Rush           bool             `json:"rush,omitempty"`         // told to hurry (#510)
	Manual         bool             `json:"manual,omitempty"`       // the user gives its clearances, no automation
	Entry          string           `json:"entry,omitempty"`        // a departure's runway entry ("" full length)
	Deicing        bool             `json:"deicing,omitempty"`      // being de-iced
	State          string           `json:"state"`
	HoldingShortOf string           `json:"holdingShortOf,omitempty"`
	AtLimit        bool             `json:"atLimit"`
	LimitNode      int              `json:"limitNode"`
	Position       airport.LatLon   `json:"position"`
	Heading        float64          `json:"heading"`
	GroundSpeed    float64          `json:"groundSpeed"`
	Lights         string           `json:"lights"`
	Error          string           `json:"error,omitempty"`
	Route          []airport.LatLon `json:"route"`
	Nodes          []airport.NodeID `json:"nodes"`
	Actions        []string         `json:"actions"` // clearances available now
	// AirRoute is what it still flies in the air: an arrival's STAR and
	// approach (with any dog-leg), a departure's SID once handed to MSFS AI;
	// Hold its hold when holding (#391, #392).
	AirRoute []airport.LatLon `json:"airRoute,omitempty"`
	// AirFixes are the named fixes still ahead on AirRoute: its dots (the
	// route itself also runs through the points of its rounded turns).
	AirFixes []airFix  `json:"airFixes,omitempty"`
	Hold     *holdView `json:"hold,omitempty"`
	Done     bool      `json:"done"`
}

type controlCenter struct {
	client engine.Client
	fleet  *traffic.Fleet
	inj    *traffic.Injector
	cmds   chan func()

	mu    sync.Mutex
	next  int
	items map[int]*controlled
	// standCheckAt: the last recheckArrivalStands (#479).
	// runwayCheckAt: the last checkRunways; runwaysNow each airport's
	// departure and arrival runway then (#456).
	runwayCheckAt time.Time
	runwaysNow    map[string]string
	standCheckAt time.Time
	models       map[string]bool                    // aircraft titles the simulator offers
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
	rejoin      func(icao, tail string)
	sequencesAt func(icao string) map[string][]traffic.SequenceEntry
	// world is the traffic picture around the centre of the world (#366):
	// every aircraft, the airports in range, a ground picture per airport.
	world *traffic.TrafficPicture
	ticks int
	// pads are an airport's de-icing pads (picked on the map, #323).
	pads func(l *airport.Layout) []airport.DeicingPad
	// weather is the latest at the user aircraft (automatic de-icing, #323).
	weather func() *nav.Weather
	// procedures gives an airport's SIDs, STARs and approaches (#315).
	procedures func(icao string) (airport.Procedures, bool)
	// The ATC game (#272): its state, the taxi graphs, the last traffic
	// scan and when the game last ran.
	game   *game
	graph  func(icao string) (*airport.Graph, error)
	scan   []Traffic
	gameAt time.Time
}

func newControlCenter(client engine.Client) *controlCenter {
	cc := &controlCenter{
		client: client, fleet: traffic.NewFleet(client), inj: traffic.NewInjector(client), clock: traffic.NewSimClock(),
		cmds: make(chan func(), 16), items: map[int]*controlled{},
		models: map[string]bool{},
		own:    map[uint32]bool{},
		ids:    traffic.NewIDBlocks(controlDefBase, controlReqBase, controlIDBlock, controlBlocks),
		detail: traffic.NewDetail(),
		stands: map[string]*traffic.StandAllocator{},
		world:  traffic.NewTrafficPicture(traffic.PictureOptions{Centre: traffic.Centre{FollowUser: true}}),
		game:   &game{},
	}
	cc.pending = newPending()
	cc.radio = traffic.NewRadio(traffic.RadioOptions{Now: cc.clock.Now, ReadBack: true,
		FrequencyOf: func(icao string, pos traffic.Position) string { _, f := cc.stationOf(icao, pos); return f },
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
			tlog.printf("%-6s %s: %s", t.Callsign, who, t.Text)
			speaker.hear(t) // the voice, when on (#419)
			if !speaker.state().On {
				heardOnCamera(t) // the camera cuts as it is said; with the voice, as it is heard
			}
		}})
	return cc
}

// do runs f in the connection goroutine and waits for it.
func (cc *controlCenter) do(f func() error) error {
	done := make(chan error, 1)
	select {
	case cc.cmds <- func() { done <- f() }:
	case <-time.After(5 * time.Second):
		return errors.New("simulator connection busy")
	}
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		return errors.New("simulator did not answer")
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
	return a
}

// tick runs every second in the connection goroutine: it scans the stands
// the ATC game.
func (cc *controlCenter) tick() {
	if now := cc.clock.Now(); now.Sub(cc.gameAt) >= time.Second {
		cc.gameAt = now
		cc.gameTick(now)
	}
	// The stand allocators are fed by the traffic picture (Allocate): no
	// scans of their own.
	if now := cc.clock.Now(); now.Sub(cc.standCheckAt) >= 10*time.Second {
		cc.standCheckAt = now
		cc.recheckArrivalStands()
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
		old, tail, obj, rwy := it.stand, it.Tail, it.objectID, it.view.Runway
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
		newStand, err := it.stands.Assign(traffic.StandRequirements{Owner: tail, Airline: airlineOf(tail), HalfSpan: half, Runway: rwy})
		if err == nil {
			err = it.arr.ChangeStand(newStand)
			if err != nil {
				it.stands.Release(newStand)
			}
		}
		if err != nil {
			_ = it.stands.Occupy(old, tail, half) // keep what it had; it will wait there
			tlog.printf("%-6s arrival: stand %s taken (%s), no other: %v", tail, l.Parking[old].Label(), why, err)
			continue
		}
		it.mu.Lock()
		it.stand, it.view.Stand = newStand, l.Parking[newStand].Label()
		mgr, model, label := it.managed, it.view.Model, it.view.Stand
		it.mu.Unlock()
		if mgr != nil {
			mgr.Describe(tail, model, label, rwy) // the board shows the new stand
		}
		tlog.printf("%-6s arrival: stand %s taken (%s), now %s", tail, l.Parking[old].Label(), why, l.Parking[newStand].Label())
	}
}

// handle passes a message to the injector and every controller.
func (cc *controlCenter) handle(msg engine.Message) bool {
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
	if cc.extra != nil && cc.extra(msg) {
		return true
	}
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
		if e := msg.AsSimObjectAndLiveryEnumeration(); uint32(e.DwRequestID) == reqModels {
			cc.addModels(msg)
			return true
		}
	}
	if ok, err := cc.inj.Handle(msg); ok {
		if err != nil {
			fmt.Printf("⚠️  injector: %v\n", err)
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
		if it.dep != nil && it.dep.Handle(msg) || it.arr != nil && it.arr.Handle(msg) {
			return true
		}
	}
	return false
}

// SpawnRequest asks for a controlled departure or arrival.
type SpawnRequest struct {
	Kind           string   `json:"kind"` // departure | arrival
	ICAO           string   `json:"icao"`
	Stand          int      `json:"stand"` // parking index
	Runway         string   `json:"runway"`
	Entry          string   `json:"entry"` // departure: runway entry taxiway
	Exit           *int     `json:"exit"`  // arrival: runway exit, an index into /api/exits; nil = the controller's choice
	Model          string   `json:"model"`
	Tail           string   `json:"tail"`
	Gates          bool     `json:"gates"`          // hold at every clearance
	InjectApproach bool     `json:"injectApproach"` // arrival: fly the approach by injection
	Tug            bool     `json:"tug"`            // departure: a pushback tug (GSX model)
	TugTitle       string   `json:"tugTitle"`       // ground vehicle title; "" = traffic.DefaultTugTitle
	TugYaw         *float64 `json:"tugYaw"`         // tug heading against the aircraft, degrees (default traffic.TugYawDeg)
	TugAhead       *float64 `json:"tugAhead"`       // tug reference point ahead of the nose gear, meters (default traffic.TugAheadMeters)
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
	// Other is the destination of a departure or the origin of an arrival
	// (ICAO): the flight follows a generated flight plan (#331).
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
}

// Turnaround dwell when none is given, and its spread.
const (
	defaultDwell = 90 * time.Second
	dwellSpread  = 0.2
)

func (cc *controlCenter) spawn(g *airport.Graph, r SpawnRequest) (*controlled, error) {
	// The runway in use: with parallels used together, an arrival takes
	// the less busy one (its stand is then found near it), a departure the
	// one nearest its stand once that is known.
	auto := r.Runway == "" || strings.EqualFold(r.Runway, "active")
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
		req := traffic.StandRequirements{Owner: r.Tail, Airline: airlineOf(r.Tail), HalfSpan: prof.SpanMeters / 2}
		if r.Kind == "arrival" {
			req.Runway = r.Runway
		}
		s, err := alloc.Assign(req)
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
	var procRoute []airport.NavPoint
	procName, expect := "", ""
	if r.planned != nil {
		procRoute, procName, expect = r.planned.route, r.planned.name, r.planned.expect
		tlog.printf("%-6s flight plan %s → %s: %s, FL%03d, %.0f NM", r.Tail, r.planned.plan.Request.Departure.ICAO, r.planned.plan.Request.Arrival.ICAO,
			r.planned.plan.Route, r.planned.plan.CruiseFL, r.planned.plan.DistanceNM)
	} else if r.Procedure {
		var err error
		if procRoute, procName, expect, err = cc.procedureFor(g, r); err != nil {
			return nil, err
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
		cc.mu.Lock()
		cc.spawnedAt = append(cc.spawnedAt, spawnPoint{tail: r.Tail, at: p.Position, altFt: alt, when: cc.clock.Now()})
		cc.mu.Unlock()
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
	it := &controlled{approach: approach, fixes: fixes, defBase: defBase, ID: n, Kind: r.Kind, Tail: r.Tail, ICAO: g.Layout.ICAO, graph: g, stands: alloc, stand: r.Stand, spoken: map[string]bool{}, removed: make(chan struct{}), cc: cc}
	it.gates.Store(r.Gates)
	var events func() (TaxiOrArrival, bool)
	switch r.Kind {
	case "departure":
		standardPlanner.want(g) // its stands' standard pushes, once
		ctl := traffic.NewTaxiController(cc.fleet, traffic.TaxiWithIDs(defBase, reqBase), traffic.TaxiWithInjector(cc.inj), traffic.TaxiWithDetail(cc.detail), traffic.TaxiWithGroundPicture(cc.world.Ground(g.Layout.ICAO)), traffic.TaxiWithClock(cc.clock.Now))
		if err := ctl.Start(traffic.TaxiRequest{Graph: g, Parking: r.Stand, Runway: r.Runway, Entry: r.Entry, ObjectID: r.adopt, PushbackAt: r.pushAt,
			Options: airport.RouteOptions{Via: r.Via, Taxiways: r.Taxiways},
			Model:   model, Livery: livery, Tail: r.Tail, HoldForClearances: true /* clearances on request, #462 */, HoldForRunway: !r.Gates, Tug: cc.tug(r, reqBase, prof), Profile: prof,
			Aircraft: &ac, Departure: procRoute, Airport: &lim, Deice: deice,
			// The push may swing through a neighbouring stand nobody holds.
			StandOccupied: func(stand int) bool { _, taken := alloc.Occupant(stand); return taken }}); err != nil {
			return nil, err
		}
		it.dep = ctl
		ch := ctl.Events()
		events = func() (TaxiOrArrival, bool) { ev, ok := <-ch; return TaxiOrArrival{dep: &ev}, ok }
	case "arrival":
		ctl := traffic.NewArrivalController(cc.fleet, traffic.ArrivalWithIDs(defBase, reqBase), traffic.ArrivalWithInjector(cc.inj), traffic.ArrivalWithDetail(cc.detail), traffic.ArrivalWithGroundPicture(cc.world.Ground(g.Layout.ICAO)), traffic.ArrivalWithClock(cc.clock.Now))
		var exit *airport.RunwayExit
		if r.Exit != nil {
			exits, err := g.RunwayExits(r.Runway)
			if err != nil || *r.Exit < 0 || *r.Exit >= len(exits) {
				return nil, fmt.Errorf("exit must be an index into /api/exits for runway %s", r.Runway)
			}
			exit = &exits[*r.Exit]
		}
		if err := ctl.Start(traffic.ArrivalRequest{Graph: g, Runway: r.Runway, Parking: r.Stand, Model: model, Livery: livery, Tail: r.Tail, Exit: exit,
			Options:          airport.RouteOptions{Via: r.Via, Taxiways: r.Taxiways},
			HoldForClearance: r.Gates, HoldAtCrossings: true, InjectApproach: r.InjectApproach || len(procRoute) > 0, Profile: prof,
			Procedure: procRoute, MissedApproach: cc.missedFor(g, r.Runway), Aircraft: &ac, Airport: &lim,
			CrosswindKts: cc.crosswind(g, r.Runway)}); err != nil {
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
		ch := ctl.Events()
		events = func() (TaxiOrArrival, bool) { ev, ok := <-ch; return TaxiOrArrival{arr: &ev}, ok }
	default:
		return nil, fmt.Errorf("kind must be departure or arrival")
	}
	it.view = ControlView{ID: n, ICAO: g.Layout.ICAO, Manual: r.Gates, Kind: r.Kind, Tail: r.Tail, Model: r.Model, Runway: r.Runway, Stand: g.Layout.Parking[r.Stand].Label(), State: "spawning", LimitNode: -1}
	tlog.printf("%-6s %s: spawned %q at %s, runway %s%s (gates %v, injected approach %v)", r.Tail, r.Kind, r.Model, it.view.Stand, r.Runway, entryNote(r.Entry), r.Gates, r.InjectApproach)
	it.setRoute()
	// Every departure starts with delivery, a SID or not: the first call,
	// then the clearance (#462).
	if procName != "" || r.Kind == "departure" {
		if procName != "" {
			it.view.Procedure = procName
			it.procSaid = procedureSaid(procName, r.planned)
		}
		info := ""
		if cc.atisLetter != nil {
			info, it.atisSaid = cc.atisLetter(g.Layout.ICAO), true
		}
		if r.Kind == "departure" {
			// The first call to delivery, then the clearance (#462).
			station, _ := cc.stationOf(g.Layout.ICAO, traffic.PosDelivery)
			dest := ""
			if r.Other != "" {
				dest = cc.airportName(r.Other)
			}
			it.climbSaid = initialClimbSaid(lim, procName)
			it.say(traffic.RequestClearance(station, r.Tail, it.view.Stand, info, dest))
			it.clearance(traffic.ClearedDeparture(r.Tail, traffic.DepartureClearance{Destination: dest, SID: it.procSaid,
				Runway: r.Runway, Level: it.climbSaid, Squawk: squawkFor(r.Tail)}))
		} else {
			// The first call to approach with its level, then the STAR.
			station, _ := cc.stationOf(g.Layout.ICAO, traffic.PosApproach)
			level := ""
			if p := it.arr.Plan(); p != nil && p.Spawn.Altitude > 0 {
				level = traffic.LevelSaidAbove(p.Spawn.Altitude, lim.TransitionAltitudeFt)
			}
			it.say(traffic.CheckIn(traffic.PosApproach, station, r.Tail, level, info))
			qnh, _ := cc.qnh()
			it.firstContact(withParams(traffic.ClearedArrival(r.Tail, it.procSaid, expect, r.Runway, ""), map[string]string{traffic.ParamQNH: qnh}))
			it.askWeather(traffic.PosApproach)
			it.view.Procedure += " → " + expect
		}
	}
	started = true
	if clash := alloc.ReserveRoute(r.Tail, it.view.Nodes); len(clash) > 0 {
		tlog.printf("%-6s %s: route overlaps the routes of %s", r.Tail, r.Kind, strings.Join(clash, ", "))
	}
	go func() {
		for {
			ev, ok := events()
			if !ok {
				it.mu.Lock()
				it.view.Done = true
				it.mu.Unlock()
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
	// What the crew asks for (#462), once the change is logged and handed
	// off (deferred first: runs last).
	defer func() {
		if ev.dep != nil && ev.dep.Request != it.request {
			it.request = ev.dep.Request
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
		gw := uint32(0)
		switch {
		case ev.dep != nil:
			gw = ev.dep.GivingWayTo
		case ev.arr != nil:
			gw = ev.arr.GivingWayTo
		}
		if gw != it.givingWay {
			it.givingWay = gw
			if gw != 0 && it.cc != nil {
				it.cc.pending.later(it.cc.clock.Now(), func() { it.tellGiveWay(gw) })
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
	}
	if e := ev.dep; e != nil {
		v.State, v.HoldingShortOf, v.AtLimit, v.LimitNode = e.State.String(), e.HoldingShortOf, e.AtLimit, int(e.LimitNode)
		v.Position, v.Heading, v.GroundSpeed, v.Lights, v.OnGround = e.Position, e.Heading, e.GroundSpeed, e.Lights.String(), e.OnGround
		if e.Err != nil {
			v.Error = e.Err.Error()
		}
		v.Actions = departureActions(e.State, e.HoldingShortOf, it.dep)
		if e.Deicing != v.Deicing {
			v.Deicing = e.Deicing
			if e.Deicing {
				tlog.printf("%-6s %s: de-icing", v.Tail, v.Kind)
			} else {
				tlog.printf("%-6s %s: de-icing done", v.Tail, v.Kind)
			}
		}
		if e.PushbackHeld != v.PushbackHeld {
			v.PushbackHeld = e.PushbackHeld
			if e.PushbackHeld {
				tlog.printf("%-6s %s: pushback holding for traffic behind the stand", v.Tail, v.Kind)
			} else {
				tlog.printf("%-6s %s: clear behind, pushing back", v.Tail, v.Kind)
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
		v.Actions = arrivalActions(e.State)
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
}

func departureActions(s traffic.TaxiState, holdingShortOf string, ctl *traffic.TaxiController) []string {
	switch s {
	case traffic.TaxiAwaitingPushback:
		if ctl != nil && ctl.FacesOut() {
			return []string{"startup", "taxi", "upto"} // taxis straight out
		}
		return []string{"pushback", "taxi", "upto"}
	case traffic.TaxiPushback, traffic.TaxiAwaitingTaxi:
		return []string{"startup", "taxi", "upto", "takeoff"}
	case traffic.TaxiTaxiing:
		return []string{"hold", "taxi", "upto", "takeoff"}
	case traffic.TaxiHoldingShort:
		if r := ctl.Route(); r != nil && holdingShortOf != r.Runway {
			return []string{"cross", "upto", "taxi"}
		}
		return []string{"lineup", "lineupbehind", "takeoff"}
	case traffic.TaxiLiningUp, traffic.TaxiLinedUp:
		return []string{"takeoff", "abort"}
	case traffic.TaxiDeparting:
		return []string{"abort"}
	}
	return nil
}

func arrivalActions(s traffic.ArrivalState) []string {
	switch s {
	case traffic.ArrivalApproaching, traffic.ArrivalLanding:
		return []string{"land", "goaround", "taxi", "upto"}
	case traffic.ArrivalRollout, traffic.ArrivalVacating, traffic.ArrivalAwaitingTaxi:
		return []string{"taxi", "upto"}
	case traffic.ArrivalTaxiing:
		return []string{"hold", "taxi", "upto"}
	case traffic.ArrivalHoldingShort:
		return []string{"cross", "upto", "taxi"}
	}
	return nil
}

// act gives a clearance to a controlled aircraft.
func (it *controlled) act(action string, node airport.NodeID) error {
	switch d := it.dep; {
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
		return d.AbortTakeoff()
	case it.arr != nil && action == "land":
		return nil // said: it lands unless sent around
	case it.arr != nil && action == "hold":
		return it.arr.HoldPosition()
	case it.arr != nil && action == "goaround":
		if err := it.arr.GoAround(); err != nil {
			return err
		}
		if it.cc.rejoin != nil {
			it.cc.rejoin(it.ICAO, it.Tail)
		}
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
			// Where departures handed to MSFS AI are now: the world scan.
			air := map[uint32]traffic.TrackedAircraft{}
			for _, a := range cc.world.Aircraft() {
				air[a.ObjectID] = a
			}
			cc.mu.Lock()
			for _, it := range cc.items {
				it.mu.Lock()
				v := it.view
				it.mu.Unlock()
				if it.arr != nil && !v.OnGround {
					v.AirRoute = it.arr.ProcedureRoute()
					v.AirFixes = fixesAhead(it.fixes, v.AirRoute)
					// Going around: the circuit's track points back to the final.
					for _, n := range it.arr.CircuitFixes() {
						v.AirFixes = append(v.AirFixes, airFix{Ident: n.Ident, LatLon: n.Position})
					}
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
				// A departure in the air: its SID still to fly, like a STAR.
				if a, ok := air[it.objectID]; it.dep != nil && ok && !a.OnGround {
					if r := it.dep.ClimbRoute(a.Position); len(r) > 0 {
						v.AirRoute, v.Position, v.Heading, v.GroundSpeed = r, a.Position, a.Heading, a.GroundKts
						v.AirFixes = fixesAhead(it.fixes, r)
					}
				}
				out = append(out, v)
			}
			cc.mu.Unlock()
		}
		writeJSON(w, out)
	})

	// GET /api/radio?icao=LKPR&n=50 — what our controllers said (#415):
	// structured transmissions, oldest first (icao empty: every airport).
	registerNetwork(mux, st) // the radio as clips, for clients on the network
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
			said = append(said, cc.radio.Transmit(icao, t))
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
		var req SpawnRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		g, err := st.cache.Graph(req.ICAO)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if req.Runway == "" || strings.EqualFold(req.Runway, "active") {
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
		// Manual (?on=1) or automatic (?on=0): who gives its clearances. Back to
		// automatic, a request still waiting is answered.
		if r.PathValue("action") == "manual" {
			on := r.URL.Query().Get("on") != "0"
			it.gates.Store(on)
			it.mu.Lock()
			it.view.Manual = on
			req := it.request
			it.mu.Unlock()
			tlog.printf("%-6s %s", it.Tail, map[bool]string{true: "under your control", false: "back to automatic control"}[on])
			if !on && req != "" {
				p := it.cc.pending
				p.later(it.clearAt(traffic.PosGround).Add(atcAnswerDelay+p.jitter(atcAnswerJitter)), func() { it.answer(req) })
			}
			w.WriteHeader(http.StatusNoContent)
			return
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
		// Network play (#511): only the position working it clears it.
		if as := positionOf(r); !mayClear(as, it) {
			http.Error(w, it.Tail+" is not on your frequency ("+as+")", http.StatusForbidden)
			return
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
		// Pushback with the start-up in one (?startup=1).
		if action == "pushback" && r.URL.Query().Get("startup") == "1" {
			action = "pushstart"
		}
		// Marked as said before it is given: the state change it causes can
		// arrive before cc.do returns, and would log it a second time.
		it.mu.Lock()
		it.spoken[action] = action != "remove"
		for _, k := range impliedBy(action) {
			it.spoken[k] = true
		}
		var m *traffic.TrafficManager
		if action == "remove" {
			m, it.managed = it.managed, nil // removed here, not failed
		}
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
			it.mu.Lock()
			delete(it.spoken, action)
			it.mu.Unlock()
			tlog.printf("%-6s %s: clearance %s refused: %v", it.Tail, it.Kind, clr, err)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		if action == "remove" {
			tlog.printf("%-6s %s: removed", it.Tail, it.Kind)
		} else {
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

// runwaySelectors keep each airport's runway in use, one for the traffic
// and its ATIS (#454), for the life of the process.
var runwaySelectors = struct {
	sync.Mutex
	m map[string]*nav.RunwaySelector
}{m: map[string]*nav.RunwaySelector{}}

func runwaySelector(icao string) *nav.RunwaySelector {
	runwaySelectors.Lock()
	defer runwaySelectors.Unlock()
	s := runwaySelectors.m[icao]
	if s == nil {
		s = &nav.RunwaySelector{}
		runwaySelectors.m[icao] = s
	}
	return s
}

// logRunwayChange logs a change of icao's runway in use, with the wind
// that made it (#465: to see every change, and why, in the traffic log).
func logRunwayChange(icao string, use nav.RunwayUse, w nav.Weather) {
	now := strings.Join(nav.Names(use.Departures), "+") + "/" + strings.Join(nav.Names(use.Arrivals), "+")
	runwaySelectors.Lock()
	before := runwayInUse[icao]
	runwayInUse[icao] = now
	runwaySelectors.Unlock()
	if before == now {
		return
	}
	head, cross := w.Components(use.Arrival.Heading)
	mode := ""
	if use.Parallel != nav.ParallelNone {
		mode = fmt.Sprintf(", %s, %.0f m apart", use.Parallel, use.SpacingM)
	}
	tlog.printf("runway in use %s: %s → %s (departures/arrivals)%s, wind %03.0f°/%.0f kt gust %.0f: headwind %.1f kt, crosswind %.1f kt on %s", icao, orNone(before), now, mode, w.WindDirTrue, w.WindKts, w.GustKts, head, cross, use.Arrival.Name)
}

var runwayInUse = map[string]string{}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
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
	return nav.Nearest(g.Layout, ends, g.Layout.Parking[stand].Position).Name
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
	name := ""
	switch {
	case len(lim.PreferredRunways) > 0:
		name = lim.PreferredRunways[0]
	case len(g.Layout.Runways) > 0:
		name = g.Layout.Runways[0].Primary.Name
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
	use := runwaySelector(g.Layout.ICAO).Choose(cc.clock.Now(), g.Layout, *w, nav.RunwayLimitsFrom(cc.limitsOf(g)))
	logRunwayChange(g.Layout.ICAO, use, *w)
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
		if r.ProcName != "" || len(list) == 0 {
			return airport.Procedure{}, fmt.Errorf("no procedure %q for runway %s", r.ProcName, r.Runway)
		}
		return list[rand.IntN(len(list))], nil
	}
	if r.Kind == "departure" {
		sid, err := pick(p.SIDsFor(r.Runway))
		if err != nil {
			return nil, "", "", err
		}
		start, alt := departureEnd(g.Layout, r.Runway)
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
	tlog.printf("%-6s turnaround: parked, departing in %s", it.Tail, it.dwell.Round(time.Second))
	select {
	case <-time.After(it.dwell):
	case <-it.departNow:
	case <-it.removed:
		return // removed while parked: nothing to depart
	}
	d := *it.turn
	d.adopt = objectID
	var dep *controlled
	err := cc.do(func() error {
		var err error
		dep, err = cc.spawn(it.graph, d)
		return err
	})
	if err != nil {
		tlog.printf("%-6s turnaround: departure failed: %v", it.Tail, err)
		return
	}
	tlog.printf("%-6s turnaround: departing from %s, runway %s (now #%d)", it.Tail, dep.view.Stand, d.Runway, dep.ID)
	cc.mu.Lock()
	delete(cc.items, it.ID)
	cc.mu.Unlock()
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
func (it *controlled) tellGiveWay(other uint32) {
	o := it.cc.byObject(other)
	if o == nil || o == it {
		return
	}
	o.mu.Lock()
	model, oh := o.view.Model, o.view.Heading
	o.mu.Unlock()
	it.mu.Lock()
	still, h := it.givingWay == other, it.view.Heading
	it.mu.Unlock()
	if !still {
		return
	}
	desc := typeSaid(traffic.ProfileFor(strings.SplitN(model, liverySep, 2)[0]).Type)
	switch d := headingDiff(h, oh); {
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
	case strings.HasPrefix(icao, "B7") && len(icao) >= 3:
		return "Boeing 7" + icao[2:3] + "7"
	case strings.HasPrefix(icao, "E1") || strings.HasPrefix(icao, "E7"):
		return "Embraer " + strings.TrimLeft(icao[1:], "0")
	case strings.HasPrefix(icao, "AT"):
		return "ATR " + icao[2:3] + "2"
	case strings.HasPrefix(icao, "CRJ"):
		return "CRJ " + icao[3:] + "00"
	}
	return icao
}

// headingDiff is b - a in degrees, within -180..180.
func headingDiff(a, b float64) float64 {
	d := math.Mod(b-a+540, 360) - 180
	return d
}

func (cc *controlCenter) byTail(tail string) *controlled {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for _, it := range cc.items {
		if it.Tail == tail {
			return it
		}
	}
	return nil
}

// ownIDs are the object IDs of the controlled aircraft.
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
	defer cc.mu.Unlock()
	if cc.items[it.ID] == it {
		delete(cc.items, it.ID)
		cc.ids.Release(it.defBase) // its controller is done: the IDs are free
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
	return cc.client.EnumerateSimObjectsAndLiveries(reqModels, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
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
	base := uintptr(unsafe.Pointer(e)) + uintptr(header)
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for i := uint32(0); i < n; i++ {
		entry := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(unsafe.Pointer(base + uintptr(i*size)))
		if t := engine.BytesToString(entry.AircraftTitle[:]); t != "" {
			if l := engine.BytesToString(entry.LiveryName[:]); l != "" {
				t += liverySep + l
			}
			cc.models[t] = true
		}
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
		tlog.printf("%s: %s → %s%s  (%.0f kt, hdg %.0f)", who, prev.State, v.State, extra, v.GroundSpeed, v.Heading)
	}
	if action := clearanceOf(v.Kind, prev.State, v.State); action != "" {
		if it.spoken[action] {
			delete(it.spoken, action) // said when given
		} else {
			var r *airport.Route
			if it.dep != nil {
				r = it.dep.Route()
			} else if p := it.arr.Plan(); p != nil {
				r = p.Route
			}
			said := v
			if said.HoldingShortOf == "" {
				said.HoldingShortOf = prev.HoldingShortOf // the runway just crossed
			}
			// (The crew's requests are said when made: onRequest, #462.)
			it.say(it.phraseView(said, r, action, -1))
		}
	}
	if v.Lights != prev.Lights && prev.Lights != "" {
		tlog.printf("%s: lights %s → %s (%s)", who, prev.Lights, v.Lights, v.State)
	}
	if v.AtLimit != prev.AtLimit {
		if v.AtLimit {
			tlog.printf("%s: holding at the clearance limit (node %d)", who, v.LimitNode)
		} else {
			tlog.printf("%s: moving on from the clearance limit", who)
		}
	}
	if v.Error != "" && v.Error != prev.Error {
		tlog.printf("%s: ⚠️ %s", who, v.Error)
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
func (cc *controlCenter) tug(r SpawnRequest, reqBase uint32, prof traffic.MotionProfile) traffic.PushbackTug {
	if !r.Tug {
		return nil
	}
	title := r.TugTitle
	if title == "" {
		title = traffic.DefaultTugTitle
	}
	t := traffic.NewSimObjectTug(cc.client, cc.inj, title, reqBase+controlIDBlock-1, prof)
	if g, err := cc.graph(r.ICAO); err == nil {
		t.Layout = g.Layout // from its depot on the vehicle roads, and back
	}
	if r.TugYaw != nil {
		t.YawDeg = *r.TugYaw
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
		r = p.Route
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
		p.later(it.clearAt(pos).Add(atcAnswerDelay+p.jitter(atcAnswerJitter)), func() {
			it.say(traffic.WeatherReport(pos, it.Tail, it.cc.windSaid(it.ICAO), qnh, alt))
		})
	})
}

// departureClimbSaid is the level departure clears a climbing departure
// to once identified.
const departureClimbSaid = "flight level 240"

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
	case "pushback":
		if it.dep != nil {
			return traffic.WithFacing(traffic.ClearedPushback(call), it.dep.PushFacing())
		}
		return traffic.ClearedPushback(call)
	case "pushstart":
		if it.dep != nil {
			return traffic.WithFacing(traffic.ClearedPushbackAndStartUp(call), it.dep.PushFacing())
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
		return traffic.ClearedLineUp(call, rwy)
	case "takeoff":
		return traffic.ClearedTakeoff(call, rwy, it.cc.windSaid(it.ICAO))
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
		return traffic.GoAround(call, "")
	case "abort":
		if v.State == traffic.TaxiDeparting.String() {
			return traffic.Stop(call)
		}
		return traffic.CancelTakeoff(call)
	}
	return traffic.Say(traffic.Transmission{Position: traffic.PosGround, Callsign: call, Intent: traffic.Intent(action)})
}

// towerHandoffMeters: a taxiing departure is handed to tower this far
// from where it holds short of its runway, give or take
// towerHandoffSpreadM, a different distance for each.
const (
	towerHandoffMeters  = 500.0
	towerHandoffSpreadM = 200.0
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
	var pos traffic.Position
	switch {
	case ev.dep != nil:
		own := ev.dep.HoldingShortOf != "" && strings.Contains(ev.dep.HoldingShortOf, it.view.Runway)
		pos = traffic.DeparturePosition(ev.dep.State, own)
		it.heightFt = ev.dep.HeightFt
		// Handed to departure once airborne and climbing away (7110.65 3-9-3:
		// about half a mile past the runway end), not at the hand-over to
		// MSFS AI.
		if it.handoffFt == 0 { // each crew its own moment
			it.handoffFt = departureHandoffFt + rand.Float64()*departureHandoffSpreadFt
			it.towerAtM = towerHandoffMeters + (2*rand.Float64()-1)*towerHandoffSpreadM
		}
		if ev.dep.State == traffic.TaxiDeparting && ev.dep.HeightFt >= it.handoffFt {
			pos = traffic.PosDeparture
		}
		// Handed to tower on the way to the runway, not at its holding point:
		// the crew calls tower ready as it gets there. Once with tower it
		// stays (a re-plan does not hand it back).
		if ev.dep.State == traffic.TaxiTaxiing && (it.atc == traffic.PosTower || ev.dep.Remaining > 0 && ev.dep.Remaining <= it.towerAtM) {
			pos = traffic.PosTower
		}
	case ev.arr != nil:
		onFinal := ev.arr.State == traffic.ArrivalApproaching && !ev.arr.OnGround && it.objectID != 0 && len(it.arr.ProcedureRoute()) == 0
		pos = traffic.ArrivalPosition(ev.arr.State, onFinal)
	default:
		return
	}
	// On the landing roll the tower tells the crew to call ground when
	// vacated (Doc 4444 12.3.4.20; #462).
	if ev.arr != nil && ev.arr.State == traffic.ArrivalRollout && !it.vacateSaid && !it.gates.Load() && it.atc == traffic.PosTower {
		gs, gf := it.cc.stationOf(it.ICAO, traffic.PosGround)
		it.say(it.rushed(traffic.WhenVacatedContact(it.Tail, traffic.PosTower, traffic.PosGround, gs, gf)))
		it.vacateSaid = true
	}
	// On the base, before the turn onto the final, approach clears the
	// approach; the crew reports established on the final, and approach
	// hands it to tower then (Doc 4444 12.4.2.2 e; CAP 413 6.27, 6.28).
	if ev.arr != nil && ev.arr.State == traffic.ArrivalApproaching && it.atc == traffic.PosApproach && pos == traffic.PosApproach &&
		!it.approachSaid && !it.gates.Load() && it.arr.TurningFinal() {
		qnh, _ := it.cc.qnh()
		it.say(traffic.ClearedApproachTo(it.Tail, traffic.ApproachClearance{Kind: it.approachKind(), Runway: it.view.Runway, QNH: qnh, ReportEstablished: true}))
		it.approachSaid = true
	}
	station, freq := it.cc.stationOf(it.ICAO, pos)
	it.view.ATC, it.view.Frequency = string(pos), freq
	if it.atc == "" || it.view.Done {
		it.atc = pos
		return
	}
	if pos == it.atc {
		return
	}
	if ev.dep != nil && it.atc == traffic.PosDelivery && !it.delivered {
		return // delivery transfers it after the clearance (clearance)
	}
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
		it.say(traffic.EstablishedReport(it.Tail, it.view.Runway))
		it.approachSaid = false // a go-around is cleared again
	}
	switch {
	case ev.arr != nil && from == traffic.PosTower && pos == traffic.PosGround && it.vacateSaid:
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
	it.say(traffic.CheckIn(pos, station, it.Tail, it.checkInReport(pos), info))
	switch {
	case ev.dep != nil && pos == traffic.PosDeparture && !it.gates.Load():
		// Departure identifies it and clears the climb on (#462).
		p := it.cc.pending
		p.later(it.clearAt(pos).Add(atcAnswerDelay+p.jitter(atcAnswerJitter)), func() {
			it.say(traffic.Identified(traffic.PosDeparture, it.Tail, departureClimbSaid))
		})
	case ev.dep != nil && pos == traffic.PosTower:
		it.askWeather(traffic.PosTower)
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
			return fmt.Sprintf("going around, climbing %.0f feet", math.Round(fx[0].AltMin*3.28084/100)*100)
		}
	case it.arr != nil && pos == traffic.PosTower:
		if kind := it.approachKind(); kind != "" {
			return "established " + kind + " runway " + rwy
		}
		return "final runway " + rwy // Doc 4444 7.3: position
	case it.arr != nil && pos == traffic.PosGround:
		return "runway vacated" // CAP 413 4.68
	}
	return ""
}

// approachKind is the approach an arrival flies ("ILS"), from its
// procedure as shown ("GOLOP 2B → ILS"); "" unknown. it.mu held.
func (it *controlled) approachKind() string {
	_, kind, _ := strings.Cut(it.view.Procedure, " → ")
	return kind
}

// stationOf is position pos at icao as said, and its frequency ("" none).
func (cc *controlCenter) stationOf(icao string, pos traffic.Position) (string, string) {
	if cc.graph == nil {
		return traffic.PositionName(pos), ""
	}
	g, err := cc.graph(icao)
	if err != nil {
		return traffic.PositionName(pos), ""
	}
	f, ok := g.Layout.FrequencyFor(freqKind(pos))
	if !ok {
		return traffic.PositionName(pos), ""
	}
	if name := aipUnitName(icao, f.String()); name != "" {
		return name, f.String()
	}
	// Named for what it is: a departure handed to the approach frequency
	// talks to approach (there is no "Ruzyne Departure", #462).
	return traffic.StationName(f.Name, kindPosition(f.Kind, pos)), f.String()
}

// departureHandoffFt: a departure is handed from tower to departure this
// high above the runway, climbing away, and up to departureHandoffSpreadFt
// higher — a different height for each: 1000 to 2500 ft.
const (
	departureHandoffFt       = 1000
	departureHandoffSpreadFt = 1500
)

// kindPosition is the position a frequency of kind is, pos when it is
// pos's own.
func kindPosition(kind string, pos traffic.Position) traffic.Position {
	switch kind {
	case airport.FreqClearance:
		return traffic.PosDelivery
	case airport.FreqGround:
		return traffic.PosGround
	case airport.FreqTower:
		return traffic.PosTower
	case airport.FreqApproach:
		return traffic.PosApproach
	case airport.FreqDeparture:
		return traffic.PosDeparture
	case airport.FreqCenter:
		return traffic.PosCenter
	}
	return pos
}

// freqKind is the airport frequency a position talks on.
func freqKind(pos traffic.Position) string {
	switch pos {
	case traffic.PosDelivery:
		return airport.FreqClearance
	case traffic.PosGround:
		return airport.FreqGround
	case traffic.PosTower:
		return airport.FreqTower
	case traffic.PosApproach:
		return airport.FreqApproach
	case traffic.PosDeparture:
		return airport.FreqDeparture
	case traffic.PosCenter:
		return airport.FreqCenter
	case traffic.PosATIS:
		return airport.FreqATIS
	}
	return ""
}

// say sends t on the radio: logged as ATC and kept for /api/radio.
func (it *controlled) say(t traffic.Transmission) {
	it.cc.radio.Transmit(it.ICAO, t)
}

// airFix is a named fix of a procedure on an air route.
type airFix struct {
	Ident string `json:"ident"`
	airport.LatLon
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
