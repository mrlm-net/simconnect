//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
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
	cc        *controlCenter
	turn      *SpawnRequest
	dwell     time.Duration
	departNow chan struct{}
	turned    bool
	// spoken marks clearances already in the log (given on the map), so
	// the state change they cause does not log them again.
	spoken map[string]bool

	mu   sync.Mutex
	view ControlView
}

// ControlView is what the map shows of a controlled aircraft.
type ControlView struct {
	ID             int              `json:"id"`
	Kind           string           `json:"kind"`
	Tail           string           `json:"tail"`
	Model          string           `json:"model"`
	Stand          string           `json:"stand"`
	Runway         string           `json:"runway"`
	Procedure      string           `json:"procedure,omitempty"` // SID, or STAR → approach
	OnGround       bool             `json:"onGround"`
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
	Done           bool             `json:"done"`
}

type controlCenter struct {
	client engine.Client
	fleet  *traffic.Fleet
	inj    *traffic.Injector
	cmds   chan func()

	mu     sync.Mutex
	next   int
	items  map[int]*controlled
	models map[string]bool                    // aircraft titles the simulator offers
	stands map[string]*traffic.StandAllocator // by ICAO
	// picture is what the controlled aircraft know of each other and of the
	// sim's other aircraft on the ground (#334).
	picture *traffic.GroundPicture
	ticks   int
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
	return &controlCenter{
		client: client, fleet: traffic.NewFleet(client), inj: traffic.NewInjector(client),
		cmds: make(chan func(), 16), items: map[int]*controlled{},
		models:  map[string]bool{},
		stands:  map[string]*traffic.StandAllocator{},
		picture: traffic.NewGroundPicture(),
		game:    &game{},
	}
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
	return a
}

// tick runs every second in the connection goroutine: it scans the stands
// of every airport with an allocator every standScanTicks.
func (cc *controlCenter) tick() {
	if now := time.Now(); now.Sub(cc.gameAt) >= time.Second {
		cc.gameAt = now
		cc.gameTick(now)
	}
	if cc.ticks++; cc.ticks%standScanTicks != 0 {
		return
	}
	cc.mu.Lock()
	all := make([]*traffic.StandAllocator, 0, len(cc.stands))
	for _, a := range cc.stands {
		all = append(all, a)
	}
	cc.mu.Unlock()
	for _, a := range all {
		if err := a.Scan(); err != nil {
			fmt.Printf("⚠️  stand scan: %v\n", err)
		}
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

	planned *planned // Other's flight plan, resolved before the spawn

	adopt uint32 // departure: the aircraft already on the stand (turnaround)
}

// Turnaround dwell when none is given, and its spread.
const (
	defaultDwell = 90 * time.Second
	dwellSpread  = 0.2
)

func (cc *controlCenter) spawn(g *airport.Graph, r SpawnRequest) (*controlled, error) {
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
	defBase, reqBase := controlDefBase+uint32(n)*controlIDBlock, controlReqBase+uint32(n)*controlIDBlock
	it := &controlled{ID: n, Kind: r.Kind, Tail: r.Tail, ICAO: r.ICAO, graph: g, stands: alloc, stand: r.Stand, spoken: map[string]bool{}}
	var events func() (TaxiOrArrival, bool)
	switch r.Kind {
	case "departure":
		ctl := traffic.NewTaxiController(cc.fleet, traffic.TaxiWithIDs(defBase, reqBase), traffic.TaxiWithInjector(cc.inj), traffic.TaxiWithGroundPicture(cc.picture))
		if err := ctl.Start(traffic.TaxiRequest{Graph: g, Parking: r.Stand, Runway: r.Runway, Entry: r.Entry, ObjectID: r.adopt,
			Options: airport.RouteOptions{Via: r.Via, Taxiways: r.Taxiways},
			Model:   model, Livery: livery, Tail: r.Tail, HoldForClearances: r.Gates, Tug: cc.tug(r, reqBase, prof), Profile: prof,
			Aircraft: &ac, Departure: procRoute}); err != nil {
			return nil, err
		}
		it.dep = ctl
		ch := ctl.Events()
		events = func() (TaxiOrArrival, bool) { ev, ok := <-ch; return TaxiOrArrival{dep: &ev}, ok }
	case "arrival":
		ctl := traffic.NewArrivalController(cc.fleet, traffic.ArrivalWithIDs(defBase, reqBase), traffic.ArrivalWithInjector(cc.inj), traffic.ArrivalWithGroundPicture(cc.picture))
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
			HoldForClearance: r.Gates, HoldAtCrossings: r.Gates, InjectApproach: r.InjectApproach || len(procRoute) > 0, Profile: prof,
			Procedure: procRoute, Aircraft: &ac}); err != nil {
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
			it.cc, it.turn, it.departNow = cc, &d, make(chan struct{}, 1)
			it.dwell = time.Duration(float64(dwell) * (1 + dwellSpread*(2*rand.Float64()-1)))
		}
		ch := ctl.Events()
		events = func() (TaxiOrArrival, bool) { ev, ok := <-ch; return TaxiOrArrival{arr: &ev}, ok }
	default:
		return nil, fmt.Errorf("kind must be departure or arrival")
	}
	it.view = ControlView{ID: n, Kind: r.Kind, Tail: r.Tail, Model: r.Model, Runway: r.Runway, Stand: g.Layout.Parking[r.Stand].Label(), State: "spawning", LimitNode: -1}
	tlog.printf("%-6s %s: spawned %q at %s, runway %s%s (gates %v, injected approach %v)", r.Tail, r.Kind, r.Model, it.view.Stand, r.Runway, entryNote(r.Entry), r.Gates, r.InjectApproach)
	it.setRoute()
	if procName != "" {
		it.view.Procedure = procName
		if r.Kind == "departure" {
			tlog.printf("%-6s ATC: %s, cleared %s departure, runway %s", r.Tail, r.Tail, procName, r.Runway)
		} else {
			tlog.printf("%-6s ATC: %s, cleared %s arrival, expect %s approach runway %s", r.Tail, r.Tail, procName, expect, r.Runway)
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
	}
}

func (it *controlled) update(ev TaxiOrArrival) {
	it.mu.Lock()
	defer it.mu.Unlock()
	v := &it.view
	prev := *v
	defer it.logChanges(prev, ev)
	if e := ev.dep; e != nil {
		v.State, v.HoldingShortOf, v.AtLimit, v.LimitNode = e.State.String(), e.HoldingShortOf, e.AtLimit, int(e.LimitNode)
		v.Position, v.Heading, v.GroundSpeed, v.Lights, v.OnGround = e.Position, e.Heading, e.GroundSpeed, e.Lights.String(), e.OnGround
		if e.Err != nil {
			v.Error = e.Err.Error()
		}
		v.Actions = departureActions(e.State, e.HoldingShortOf, it.dep)
		// Off the stand once pushed or taxiing; the route is done when airborne.
		// (AwaitingTaxi on a face-out stand is still on it.)
		if !it.left && e.State >= traffic.TaxiPushback && e.State != traffic.TaxiAwaitingTaxi {
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
		return []string{"pushback", "taxi", "upto"}
	case traffic.TaxiPushback, traffic.TaxiAwaitingTaxi:
		return []string{"taxi", "upto", "takeoff"}
	case traffic.TaxiTaxiing:
		return []string{"hold", "taxi", "upto", "takeoff"}
	case traffic.TaxiHoldingShort:
		if r := ctl.Route(); r != nil && holdingShortOf != r.Runway {
			return []string{"cross", "upto", "taxi"}
		}
		return []string{"lineup", "takeoff"}
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
		return []string{"goaround", "taxi", "upto"}
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
	case d != nil && action == "taxi":
		d.ClearToTaxi()
	case d != nil && action == "upto":
		return d.ClearUpTo(node)
	case d != nil && action == "cross":
		d.ClearToCross()
	case d != nil && action == "lineup":
		d.ClearToLineUp()
	case d != nil && action == "takeoff":
		return d.ClearForTakeoff()
	case d != nil && action == "remove":
		return d.Cancel()
	case d != nil && action == "hold":
		return d.HoldPosition()
	case d != nil && action == "abort":
		return d.AbortTakeoff()
	case it.arr != nil && action == "hold":
		return it.arr.HoldPosition()
	case it.arr != nil && action == "goaround":
		return it.arr.GoAround()
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
			cc.mu.Lock()
			for _, it := range cc.items {
				it.mu.Lock()
				out = append(out, it.view)
				it.mu.Unlock()
			}
			cc.mu.Unlock()
		}
		writeJSON(w, out)
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
		if err := cc.do(func() error { return it.act(action, node) }); err != nil {
			tlog.printf("%-6s %s: clearance %s refused: %v", it.Tail, it.Kind, clr, err)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		if action == "remove" {
			tlog.printf("%-6s %s: removed", it.Tail, it.Kind)
		} else {
			tlog.printf("%-6s ATC: %s", it.Tail, it.phrase(action, node))
			it.mu.Lock()
			it.spoken[action] = true
			it.mu.Unlock()
		}
		if action == "remove" {
			it.stands.ReleaseOwner(it.Tail)
			cc.mu.Lock()
			delete(cc.items, id)
			cc.mu.Unlock()
		}
		w.WriteHeader(http.StatusNoContent)
	})
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
	first := ""
	for _, t := range star.RunwayTransitions {
		for _, l := range t.Legs {
			if l.HasFix() && first == "" {
				first = l.Fix
			}
		}
	}
	for _, l := range star.Legs {
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

// turnaround departs a parked arrival again (#296): after the dwell (or
// the "depart" action) a departure adopts the same aircraft on its stand,
// with the same call sign, stand reservation and runway; the arrival's
// entry leaves the list.
func (cc *controlCenter) turnaround(it *controlled, objectID uint32) {
	tlog.printf("%-6s turnaround: parked, departing in %s", it.Tail, it.dwell.Round(time.Second))
	select {
	case <-time.After(it.dwell):
	case <-it.departNow:
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
			tlog.printf("%-6s ATC: %s", it.Tail, it.phraseView(v, r, action, -1))
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
func (it *controlled) phrase(action string, node airport.NodeID) string {
	var r *airport.Route
	if it.dep != nil {
		r = it.dep.Route()
	} else if p := it.arr.Plan(); p != nil {
		r = p.Route
	}
	it.mu.Lock()
	v := it.view
	it.mu.Unlock()
	return it.phraseView(v, r, action, node)
}

// phraseView is phrase for a view the caller holds.
func (it *controlled) phraseView(v ControlView, r *airport.Route, action string, node airport.NodeID) string {
	call, rwy := it.Tail, v.Runway
	switch action {
	case "pushback":
		return call + ", push back and start-up approved"
	case "taxi":
		if it.dep != nil {
			return fmt.Sprintf("%s, taxi to holding point%s runway %s%s", call, entryPoint(r), rwy, via(r, len(r.Edges)))
		}
		return fmt.Sprintf("%s, taxi to stand %s%s", call, v.Stand, via(r, len(r.Edges)))
	case "upto":
		if r != nil {
			if i := slices.Index(r.Nodes, node); i > 0 {
				limit := r.Edges[i-1].Name
				if i < len(r.Edges) && r.Edges[i].Name != "" && r.Edges[i].Name != limit {
					limit = r.Edges[i].Name // hold short of the taxiway joined there
				}
				if limit == "" {
					return fmt.Sprintf("%s, taxi%s, hold position at the marked point", call, via(r, i))
				}
				return fmt.Sprintf("%s, taxi%s, hold short of %s", call, via(r, i), limit)
			}
		}
		return call + ", taxi to the marked point and hold"
	case "cross":
		return fmt.Sprintf("%s, cross runway %s", call, v.HoldingShortOf)
	case "lineup":
		return fmt.Sprintf("%s, runway %s, line up and wait", call, rwy)
	case "takeoff":
		return fmt.Sprintf("%s, runway %s, cleared for take-off", call, rwy)
	case "hold":
		return call + ", hold position"
	case "goaround":
		return call + ", go around, I say again, go around"
	case "abort":
		if v.State == traffic.TaxiDeparting.String() {
			return call + ", stop immediately, I say again, stop immediately"
		}
		return call + ", hold position, cancel take-off clearance, I say again, cancel take-off clearance"
	}
	return call + ", " + action
}

// via names the taxiways of the first n edges of a route: " via B2, H, A".
func via(r *airport.Route, n int) string {
	if r == nil {
		return ""
	}
	var names []string
	for _, e := range r.Edges[:min(n, len(r.Edges))] {
		if e.Name != "" && (len(names) == 0 || names[len(names)-1] != e.Name) {
			names = append(names, e.Name)
		}
	}
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
	own := map[uint32]bool{}
	for _, it := range cc.items {
		if it.dep != nil {
			own[it.dep.ObjectID()] = true
		} else if it.arr != nil {
			own[it.arr.ObjectID()] = true
		}
	}
	cc.mu.Unlock()
	now := time.Now()
	for _, t := range scan {
		if !t.OnGround || own[t.ObjectID] {
			continue
		}
		p := traffic.DefaultMotionProfile()
		if t.Span > 0 {
			p.SpanMeters = t.Span
		}
		cc.picture.Report(t.ObjectID, airport.LatLon{Lat: t.Latitude, Lon: t.Longitude}, t.Heading, p, now)
	}
}
