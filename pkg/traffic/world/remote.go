package world

import (
	"encoding/json"
	"errors"
	"fmt"

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
	}
	arrivalStart struct {
		Target  string                 `json:"target"`
		DefBase uint32                 `json:"defBase"`
		ReqBase uint32                 `json:"reqBase"`
		ICAO    string                 `json:"icao"`
		Req     traffic.ArrivalRequest `json:"req"`
	}
)

// StartDeparture starts the departure on the actuator: its events come
// from its target, subscribed before it starts.
func (r *remoteSim) StartDeparture(defBase, reqBase uint32, req traffic.TaxiRequest) (departureCtl, <-chan traffic.TaxiEvent, error) {
	w := departureStart{Target: fmt.Sprintf("dep/%d", defBase), DefBase: defBase, ReqBase: reqBase, Req: req}
	if req.Graph != nil {
		w.ICAO = req.Graph.Layout.ICAO
	}
	if t, ok := req.Tug.(*traffic.SimObjectTug); ok && t != nil {
		w.Tug = t.Title()
	}
	if f, ok := req.Fuel.(*traffic.SimObjectFuelTruck); ok && f != nil {
		w.Fuel = f.Title()
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
	w := arrivalStart{Target: fmt.Sprintf("arr/%d", defBase), DefBase: defBase, ReqBase: reqBase, Req: req}
	if req.Graph != nil {
		w.ICAO = req.Graph.Layout.ICAO
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
	*localSim
	srv   *wireServer
	send  func(wireMsg) error
	graph func(icao string) (*airport.Graph, error)
	// tug and fuel make an aircraft's tug and fuel truck from their models
	// on its request IDs (nil: none).
	tug  func(title string, reqBase uint32, prof traffic.MotionProfile) traffic.PushbackTug
	fuel func(title string, reqBase uint32, prof traffic.MotionProfile) traffic.FuelService
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
	if w.Tug != "" && a.tug != nil {
		req.Tug = a.tug(w.Tug, w.ReqBase, req.Profile)
	}
	if w.Fuel != "" && a.fuel != nil {
		req.Fuel = a.fuel(w.Fuel, w.ReqBase, req.Profile)
	}
	ctl, evs, err := a.localSim.StartDeparture(w.DefBase, w.ReqBase, req)
	if err != nil {
		return err
	}
	a.srv.add(w.Target, ctl)
	go a.pump(w.Target, func(yield func(any, error) bool) {
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
	ctl, evs, err := a.localSim.StartArrival(w.DefBase, w.ReqBase, req)
	if err != nil {
		return err
	}
	a.srv.add(w.Target, ctl)
	go a.pump(w.Target, func(yield func(any, error) bool) {
		for ev := range evs {
			if !yield(ev, ev.Err) {
				return
			}
		}
	})
	return nil
}

// pump sends target's events until they end, then forgets the target.
func (a *actuatorSim) pump(target string, events func(yield func(any, error) bool)) {
	defer a.srv.remove(target)
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
			return
		}
		m.Args = append(m.Args, b)
	}
	_ = f.send(m)
}

func (f *wireFeedOut) Airports(list []traffic.AirportRef) { f.put("airports", list) }
func (f *wireFeedOut) Weather(w nav.Weather)              { f.put("weather", w) }
func (f *wireFeedOut) ILS(r nav.NavResult)                { f.put("ils", r) }
func (f *wireFeedOut) Procedures(p airport.Procedures)    { f.put("procedures", p) }
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
		var v nav.Weather
		if err := arg(0, &v); err != nil {
			return err
		}
		to.Weather(v)
	case "ils":
		var v nav.NavResult
		if err := arg(0, &v); err != nil {
			return err
		}
		to.ILS(v)
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
