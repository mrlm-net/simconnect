//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
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

type controlled struct {
	ID    int    `json:"id"`
	Kind  string `json:"kind"` // departure | arrival
	Tail  string `json:"tail"`
	ICAO  string `json:"icao"`
	dep   *traffic.TaxiController
	arr   *traffic.ArrivalController
	graph *airport.Graph

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
	models map[string]bool // aircraft titles the simulator offers
}

func newControlCenter(client engine.Client) *controlCenter {
	return &controlCenter{
		client: client, fleet: traffic.NewFleet(client), inj: traffic.NewInjector(client),
		cmds: make(chan func(), 16), items: map[int]*controlled{},
		models: map[string]bool{},
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

// handle passes a message to the injector and every controller.
func (cc *controlCenter) handle(msg engine.Message) bool {
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
	Kind           string `json:"kind"` // departure | arrival
	ICAO           string `json:"icao"`
	Stand          int    `json:"stand"` // parking index
	Runway         string `json:"runway"`
	Entry          string `json:"entry"` // departure: runway entry taxiway
	Model          string `json:"model"`
	Tail           string `json:"tail"`
	Gates          bool   `json:"gates"`          // hold at every clearance
	InjectApproach bool   `json:"injectApproach"` // arrival: fly the approach by injection
}

func (cc *controlCenter) spawn(g *airport.Graph, r SpawnRequest) (*controlled, error) {
	cc.mu.Lock()
	cc.next++
	n := cc.next
	cc.mu.Unlock()
	if r.Model == "" {
		r.Model = "FSLTL A320 Air France SL"
	}
	if r.Tail == "" {
		r.Tail = fmt.Sprintf("MAP%02d", n)
	}
	defBase, reqBase := controlDefBase+uint32(n)*controlIDBlock, controlReqBase+uint32(n)*controlIDBlock
	it := &controlled{ID: n, Kind: r.Kind, Tail: r.Tail, ICAO: r.ICAO, graph: g}
	var events func() (TaxiOrArrival, bool)
	switch r.Kind {
	case "departure":
		ctl := traffic.NewTaxiController(cc.fleet, traffic.TaxiWithIDs(defBase, reqBase), traffic.TaxiWithInjector(cc.inj))
		if err := ctl.Start(traffic.TaxiRequest{Graph: g, Parking: r.Stand, Runway: r.Runway, Entry: r.Entry,
			Model: r.Model, Tail: r.Tail, HoldForClearances: r.Gates}); err != nil {
			return nil, err
		}
		it.dep = ctl
		ch := ctl.Events()
		events = func() (TaxiOrArrival, bool) { ev, ok := <-ch; return TaxiOrArrival{dep: &ev}, ok }
	case "arrival":
		ctl := traffic.NewArrivalController(cc.fleet, traffic.ArrivalWithIDs(defBase, reqBase), traffic.ArrivalWithInjector(cc.inj))
		if err := ctl.Start(traffic.ArrivalRequest{Graph: g, Runway: r.Runway, Parking: r.Stand, Model: r.Model, Tail: r.Tail,
			HoldForClearance: r.Gates, HoldAtCrossings: r.Gates, InjectApproach: r.InjectApproach}); err != nil {
			return nil, err
		}
		it.arr = ctl
		ch := ctl.Events()
		events = func() (TaxiOrArrival, bool) { ev, ok := <-ch; return TaxiOrArrival{arr: &ev}, ok }
	default:
		return nil, fmt.Errorf("kind must be departure or arrival")
	}
	it.view = ControlView{ID: n, Kind: r.Kind, Tail: r.Tail, Model: r.Model, Runway: r.Runway, Stand: g.Layout.Parking[r.Stand].Label(), State: "spawning", LimitNode: -1}
	tlog.printf("%-6s %s: spawned %q at %s, runway %s%s (gates %v, injected approach %v)", r.Tail, r.Kind, r.Model, it.view.Stand, r.Runway, entryNote(r.Entry), r.Gates, r.InjectApproach)
	it.setRoute()
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
		v.Position, v.Heading, v.GroundSpeed, v.Lights = e.Position, e.Heading, e.GroundSpeed, e.Lights.String()
		if e.Err != nil {
			v.Error = e.Err.Error()
		}
		v.Actions = departureActions(e.State, e.HoldingShortOf, it.dep)
	}
	if e := ev.arr; e != nil {
		v.State, v.HoldingShortOf, v.AtLimit, v.LimitNode = e.State.String(), e.HoldingShortOf, e.AtLimit, int(e.LimitNode)
		v.Position, v.Heading, v.GroundSpeed, v.Lights = e.Position, e.Heading, e.GroundSpeed, e.Lights.String()
		if e.Err != nil {
			v.Error = e.Err.Error()
		}
		v.Actions = arrivalActions(e.State)
	}
}

func departureActions(s traffic.TaxiState, holdingShortOf string, ctl *traffic.TaxiController) []string {
	switch s {
	case traffic.TaxiAwaitingPushback:
		return []string{"pushback", "taxi", "upto"}
	case traffic.TaxiPushback, traffic.TaxiAwaitingTaxi, traffic.TaxiTaxiing:
		return []string{"taxi", "upto", "takeoff"}
	case traffic.TaxiHoldingShort:
		if r := ctl.Route(); r != nil && holdingShortOf != r.Runway {
			return []string{"cross", "upto", "taxi"}
		}
		return []string{"lineup", "takeoff"}
	case traffic.TaxiLiningUp, traffic.TaxiLinedUp:
		return []string{"takeoff"}
	}
	return nil
}

func arrivalActions(s traffic.ArrivalState) []string {
	switch s {
	case traffic.ArrivalApproaching, traffic.ArrivalLanding, traffic.ArrivalRollout, traffic.ArrivalVacating,
		traffic.ArrivalAwaitingTaxi, traffic.ArrivalTaxiing:
		return []string{"taxi", "upto"}
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
	case it.arr != nil && action == "taxi":
		it.arr.ClearToTaxi()
	case it.arr != nil && action == "upto":
		return it.arr.ClearUpTo(node)
	case it.arr != nil && action == "cross":
		it.arr.ClearToCross()
	case it.arr != nil && action == "remove":
		return it.arr.Cancel()
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
		tlog.printf("%-6s %s: cleared %s", it.Tail, it.Kind, clr)
		if action == "remove" {
			cc.mu.Lock()
			delete(cc.items, id)
			cc.mu.Unlock()
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// reqModels asks the simulator for its aircraft titles (the model list).
const reqModels uint32 = 2004

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
	size := (uint32(msg.DwSize) - header) / n
	base := uintptr(unsafe.Pointer(e)) + uintptr(header)
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for i := uint32(0); i < n; i++ {
		entry := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(unsafe.Pointer(base + uintptr(i*size)))
		if t := engine.BytesToString(entry.AircraftTitle[:]); t != "" {
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
