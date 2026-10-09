package world

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

var (
	_ departureCtl = (*remoteDep)(nil)
	_ arrivalCtl   = (*remoteArr)(nil)
	_ simPort      = (*remoteSim)(nil)
	_ simFeed      = (*wireFeedOut)(nil)
)

// A departure's and an arrival's start, as they go over the wire: the
// request without what is not data, and what the actuator makes it from —
// the airport's graph from its own cache, the tug and the fuel truck from
// their models.
type (
	departureStart struct {
		Target  string              `json:"target"`
		DefBase uint32              `json:"defBase"`
		ReqBase uint32              `json:"reqBase"`
		ICAO    string              `json:"icao"`
		Req     traffic.TaxiRequest `json:"req"`
		Tug     string              `json:"tug,omitempty"`
		Fuel    string              `json:"fuel,omitempty"`
		Stairs  string              `json:"stairs,omitempty"` // #831
		GPU     string              `json:"gpu,omitempty"`    // #832
		Buses   []string            `json:"buses,omitempty"`  // #887: a title per bus
		Deboard []string            `json:"deboard,omitempty"`
		// TugYaw and TugAhead: the tug's place on the nose gear (0: its own).
		TugYaw   float64 `json:"tugYaw,omitempty"`
		TugAhead float64 `json:"tugAhead,omitempty"`
	}
	arrivalStart struct {
		Target  string                 `json:"target"`
		DefBase uint32                 `json:"defBase"`
		ReqBase uint32                 `json:"reqBase"`
		ICAO    string                 `json:"icao"`
		Req     traffic.ArrivalRequest `json:"req"`
		// FollowMe is its follow-me car's model (#890), "" for none.
		FollowMe string `json:"followMe,omitempty"`
	}
)

// StartDeparture starts the departure on the actuator: its events come
// from its target, subscribed before it starts.
func (r *remoteSim) StartDeparture(defBase, reqBase uint32, req traffic.TaxiRequest) (departureCtl, <-chan traffic.TaxiEvent, error) {
	// A target of its own (#53: "dep/<defBase>" came back with a reused ID
	// block, and the old controller's end removed the new one).
	w := departureStart{Target: fmt.Sprintf("dep/%d/%d", defBase, wireTargetSeq.Add(1)), DefBase: defBase, ReqBase: reqBase, Req: req}
	if req.Graph != nil {
		w.ICAO = req.Graph.Layout.ICAO
	}
	if t, ok := req.Tug.(*traffic.SimObjectTug); ok && t != nil {
		w.Tug, w.TugYaw, w.TugAhead = t.Title(), t.YawDeg, t.AheadMeters
	}
	if f, ok := req.Fuel.(*traffic.SimObjectFuelTruck); ok && f != nil {
		w.Fuel = f.Title()
	}
	if s, ok := req.Stairs.(*traffic.SimObjectStairs); ok && s != nil {
		w.Stairs = s.Title()
	}
	if u, ok := req.GPU.(*traffic.SimObjectFuelTruck); ok && u != nil {
		w.GPU = u.Title()
	}
	for _, b := range req.Buses {
		if b, ok := b.(*traffic.SimObjectBus); ok && b != nil {
			w.Buses = append(w.Buses, b.Title())
		}
	}
	for _, b := range req.Deboard {
		if b, ok := b.(*traffic.SimObjectBus); ok && b != nil {
			w.Deboard = append(w.Deboard, b.Title())
		}
	}
	evs := r.c.subscribe(w.Target)
	if err := r.c.call(r.t, "StartDeparture", []any{w}); err != nil {
		r.c.unsubscribe(w.Target)
		return nil, nil, err
	}
	out := make(chan traffic.TaxiEvent, 64)
	go func() {
		defer close(out)
		for m := range evs {
			var ev traffic.TaxiEvent
			if len(m.Args) > 0 && json.Unmarshal(m.Args[0], &ev) == nil {
				if m.Err != "" {
					ev.Err = errors.New(m.Err)
				}
				out <- ev
			}
		}
	}()
	return &remoteDep{c: r.c, t: w.Target}, out, nil
}

// StartArrival starts the arrival on the actuator, as StartDeparture.
func (r *remoteSim) StartArrival(defBase, reqBase uint32, req traffic.ArrivalRequest) (arrivalCtl, <-chan traffic.ArrivalEvent, error) {
	w := arrivalStart{Target: fmt.Sprintf("arr/%d/%d", defBase, wireTargetSeq.Add(1)), DefBase: defBase, ReqBase: reqBase, Req: req}
	if req.Graph != nil {
		w.ICAO = req.Graph.Layout.ICAO
	}
	if f, ok := req.FollowMe.(*traffic.SimObjectFollowMe); ok && f != nil {
		w.FollowMe = f.Title()
	}
	evs := r.c.subscribe(w.Target)
	if err := r.c.call(r.t, "StartArrival", []any{w}); err != nil {
		r.c.unsubscribe(w.Target)
		return nil, nil, err
	}
	out := make(chan traffic.ArrivalEvent, 64)
	go func() {
		defer close(out)
		for m := range evs {
			var ev traffic.ArrivalEvent
			if len(m.Args) > 0 && json.Unmarshal(m.Args[0], &ev) == nil {
				if m.Err != "" {
					ev.Err = errors.New(m.Err)
				}
				out <- ev
			}
		}
	}()
	return &remoteArr{c: r.c, t: w.Target}, out, nil
}

// ── The actuator's sim port ────────────────────────────────────────────────

// actuatorSim is the sim port the actuator serves: the local one, with the
// starts taken off the wire (wireServer target "sim").
type actuatorSim struct {
	// heard: the host's OnTransmission, for the director's radio (#779).
	heard func(traffic.Transmission)
	*localSim
	srv   *wireServer
	send  func(wireMsg) error
	graph func(icao string) (*airport.Graph, error)
	reqs  chan<- string // airports to load
	// alloc is the airport's stand allocator here (stands taken, from this
	// side's scans); pushes plans the airport's standard pushes on this
	// side's graph (they are kept by graph).
	alloc  func(g *airport.Graph) *traffic.StandAllocator
	pushes func(g *airport.Graph)
	// tug and fuel make an aircraft's tug and fuel truck from their models
	// on its request IDs (nil: none).
	tug    func(w departureStart, g *airport.Graph, prof traffic.MotionProfile) traffic.PushbackTug
	fuel   func(w departureStart, g *airport.Graph, prof traffic.MotionProfile) traffic.FuelService
	stairs func(w departureStart, g *airport.Graph, prof traffic.MotionProfile) traffic.FuelService
	gpu    func(w departureStart, g *airport.Graph, prof traffic.MotionProfile) traffic.FuelService
	bus    func(w departureStart, g *airport.Graph, prof traffic.MotionProfile, title string, n int) traffic.FuelService
	// followMe makes an arrival's follow-me car (#890).
	followMe func(w arrivalStart, g *airport.Graph, prof traffic.MotionProfile) traffic.FollowMeService

	mu   sync.Mutex
	ctls []interface{ Handle(engine.Message) bool } // started off the wire
	// vehicles: each departure's tug and fuel truck, by target, for the
	// director's map (sendVehicles).
	vehicles map[string]actuatorVehicles
}

// StartDeparture starts a departure off the wire and serves its controller
// and events under its target.
func (a *actuatorSim) StartDeparture(w departureStart) error {
	g, err := a.graph(w.ICAO)
	if err != nil {
		return err
	}
	req := w.Req
	req.Graph = g
	// The push may swing through a neighbouring stand nobody holds: who
	// holds one is known here, from the scans (live: a push swung toward
	// the terminal with every stand taken for free).
	if a.alloc != nil {
		alloc := a.alloc(g)
		req.StandOccupied = func(stand int) bool { _, taken := alloc.Occupant(stand); return taken }
	}
	if a.pushes != nil {
		a.pushes(g) // the stands' standard pushes, once
	}
	if w.Tug != "" && a.tug != nil {
		req.Tug = a.tug(w, g, req.Profile)
	}
	if w.Fuel != "" && a.fuel != nil {
		req.Fuel = a.fuel(w, g, req.Profile)
	}
	if w.Stairs != "" && a.stairs != nil {
		req.Stairs = a.stairs(w, g, req.Profile)
	}
	if w.GPU != "" && a.gpu != nil {
		req.GPU = a.gpu(w, g, req.Profile)
	}
	if a.bus != nil {
		for i, title := range w.Buses {
			req.Buses = append(req.Buses, a.bus(w, g, req.Profile, title, i))
		}
		for i, title := range w.Deboard {
			req.Deboard = append(req.Deboard, a.bus(w, g, req.Profile, title, i))
		}
	}
	ctl, evs, err := a.localSim.StartDeparture(w.DefBase, w.ReqBase, req)
	if err != nil {
		return err
	}
	a.keep(ctl)
	a.srv.add(w.Target, ctl)
	tug, _ := req.Tug.(*traffic.SimObjectTug)
	fuel, _ := req.Fuel.(*traffic.SimObjectFuelTruck)
	if tug != nil || fuel != nil {
		a.mu.Lock()
		if a.vehicles == nil {
			a.vehicles = map[string]actuatorVehicles{}
		}
		a.vehicles[w.Target] = actuatorVehicles{tug: tug, fuel: fuel}
		a.mu.Unlock()
	}
	go a.pump(w.Target, ctl, func(yield func(any, error) bool) {
		for ev := range evs {
			if !yield(ev, ev.Err) {
				return
			}
		}
	})
	return nil
}

// StartArrival starts an arrival off the wire, as StartDeparture.
func (a *actuatorSim) StartArrival(w arrivalStart) error {
	g, err := a.graph(w.ICAO)
	if err != nil {
		return err
	}
	req := w.Req
	req.Graph = g
	if w.FollowMe != "" && a.followMe != nil {
		req.FollowMe = a.followMe(w, g, req.Profile)
	}
	ctl, evs, err := a.localSim.StartArrival(w.DefBase, w.ReqBase, req)
	if err != nil {
		return err
	}
	a.keep(ctl)
	a.srv.add(w.Target, ctl)
	go a.pump(w.Target, ctl, func(yield func(any, error) bool) {
		for ev := range evs {
			if !yield(ev, ev.Err) {
				return
			}
		}
	})
	return nil
}

// pump sends target's events until they end, then forgets the target.
func (a *actuatorSim) pump(target string, ctl interface{ Handle(engine.Message) bool }, events func(yield func(any, error) bool)) {
	defer a.srv.remove(target)
	defer a.drop(ctl) // done: no more messages for it (#65)
	defer func() {
		a.mu.Lock()
		delete(a.vehicles, target)
		a.mu.Unlock()
	}()
	events(func(ev any, evErr error) bool {
		b, err := json.Marshal(ev)
		if err != nil {
			return true
		}
		m := wireMsg{Kind: wireEvent, Target: target, Args: []json.RawMessage{b}}
		if evErr != nil {
			m.Err = evErr.Error()
		}
		return a.send(m) == nil
	})
	// Its events are over: the director's channel closes, as a local
	// controller's does (#52: the flight never ended on the director).
	_ = a.send(wireMsg{Kind: wireEvent, Target: target, Method: wireEnd})
}

// ── The feed over the wire ─────────────────────────────────────────────────

// wireFeedOut is the actuator's feed: what its simulator tells, sent to
// the director (wireFeedIn there).
type wireFeedOut struct{ send func(wireMsg) error }

func (f *wireFeedOut) put(kind string, vs ...any) {
	m := wireMsg{Kind: wireFeed, Method: kind}
	for _, v := range vs {
		b, err := json.Marshal(v)
		if err != nil {
			fmt.Fprintf(stdout, "⚠️  world: wire feed %s: %v\n", kind, err)
			return
		}
		m.Args = append(m.Args, b)
	}
	_ = f.send(m)
}

func (f *wireFeedOut) Airports(list []traffic.AirportRef) { f.put("airports", list) }

// Weather goes with an unknown dewpoint (NaN, which JSON cannot carry) as
// null.
func (f *wireFeedOut) Weather(w nav.Weather) {
	ww := wireWeather{Weather: w}
	if !math.IsNaN(w.DewpointC) {
		ww.DewpointC = &w.DewpointC
	}
	f.put("weather", ww)
}

// wireWeather is nav.Weather on the wire.
type wireWeather struct {
	nav.Weather
	DewpointC *float64 `json:"DewpointC"`
}

func (f *wireFeedOut) ILS(r nav.NavResult)             { f.put("ils", r) }
func (f *wireFeedOut) Procedures(p airport.Procedures) { f.put("procedures", p) }
func (f *wireFeedOut) Airways(icao string, g *nav.AirwayGraph) {
	f.put("airways", icao, g)
}
func (f *wireFeedOut) Layout(icao string, l *airport.Layout, err error) {
	e := ""
	if err != nil {
		e = err.Error()
	}
	f.put("layout", icao, l, e)
}
func (f *wireFeedOut) UserAircraft(a Aircraft, rate float64, com1 string) {
	f.put("aircraft", a, rate, com1)
}
func (f *wireFeedOut) Paused(paused bool)     { f.put("paused", paused) }
func (f *wireFeedOut) Traffic(scan []Traffic) { f.put("traffic", scan) }

// feedIn hands a feed message from the wire to the director's feed, and to
// its airport cache a layout loaded.
func feedIn(m wireMsg, to simFeed, cache *airport.Cache) error {
	arg := func(i int, v any) error {
		if i >= len(m.Args) {
			return fmt.Errorf("world: wire feed %s: argument %d missing", m.Method, i)
		}
		return json.Unmarshal(m.Args[i], v)
	}
	switch m.Method {
	case "airports":
		var v []traffic.AirportRef
		if err := arg(0, &v); err != nil {
			return err
		}
		to.Airports(v)
	case "weather":
		var v wireWeather
		if err := arg(0, &v); err != nil {
			return err
		}
		w := v.Weather
		w.DewpointC = math.NaN()
		if v.DewpointC != nil {
			w.DewpointC = *v.DewpointC
		}
		to.Weather(w)
	case "ils":
		var v nav.NavResult
		if err := arg(0, &v); err != nil {
			return err
		}
		to.ILS(v)
	case "airways":
		var icao string
		var raw json.RawMessage
		if err := arg(0, &icao); err != nil {
			return err
		}
		if err := arg(1, &raw); err != nil {
			return err
		}
		g, err := nav.ReadAirwayGraph(bytes.NewReader(raw))
		if err != nil {
			return err
		}
		to.Airways(icao, g)
	case "procedures":
		var v airport.Procedures
		if err := arg(0, &v); err != nil {
			return err
		}
		to.Procedures(v)
	case "layout":
		var icao, e string
		var l *airport.Layout
		if err := arg(0, &icao); err != nil {
			return err
		}
		_ = arg(1, &l)
		_ = arg(2, &e)
		var err error
		if e != "" {
			err = errors.New(e)
		}
		if l != nil && cache != nil {
			cache.Put(l)
		}
		to.Layout(icao, l, err)
	case "aircraft":
		var a Aircraft
		var rate float64
		var com1 string
		if err := arg(0, &a); err != nil {
			return err
		}
		_ = arg(1, &rate)
		_ = arg(2, &com1)
		to.UserAircraft(a, rate, com1)
	case "paused":
		var v bool
		if err := arg(0, &v); err != nil {
			return err
		}
		to.Paused(v)
	case "traffic":
		var v []Traffic
		if err := arg(0, &v); err != nil {
			return err
		}
		to.Traffic(v)
	default:
		return fmt.Errorf("world: wire feed: unknown %q", m.Method)
	}
	return nil
}

var _ engine.Message

// actuatorVehicles are a departure's tug and fuel truck on the actuator.
type actuatorVehicles struct {
	tug  *traffic.SimObjectTug
	fuel *traffic.SimObjectFuelTruck
}

// vehicleViews are the departures' vehicles now, by target.
func (a *actuatorSim) vehicleViews() map[string][]VehicleView {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[string][]VehicleView{}
	for t, v := range a.vehicles {
		if v.tug != nil {
			out[t] = append(out[t], vehicleView("tug", v.tug.ObjectID(), v.tug.Title(), v.tug.State(), v.tug.Track))
		}
		if v.fuel != nil {
			out[t] = append(out[t], vehicleView("fuel", v.fuel.ObjectID(), v.fuel.Title(), v.fuel.State(), v.fuel.Track))
		}
	}
	return out
}
