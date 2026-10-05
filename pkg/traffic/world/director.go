package world

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Split (#710, option 3): the director takes the World's decisions with no
// simulator of its own; the actuator, beside the simulator, keeps the
// simulator side — its connection, the controllers and their injection at
// frame rate — and serves it over a link. Decisions cross the link about
// once a second; injection never does.

// Loopback runs act as an actuator on its own simulator connection and dir
// as its director, linked in this process, until ctx ends: the split as it
// will run across a network, for checking it against the World in one
// piece. A front end serves dir.
func Loopback(ctx context.Context, act, dir *World) {
	a, d := pipe()
	act.st.actLink = a
	go act.Run(ctx)
	go func() {
		for ctx.Err() == nil {
			_ = dir.runDirector(ctx, d)
			return // a pipe does not come back
		}
	}()
}

// runDirector runs the World's decisions with its simulator side at the
// other end of l, until ctx ends or l closes.
func (w *World) runDirector(ctx context.Context, l link) error {
	st := w.st
	feedCh := make(chan wireMsg, 4096)
	c := newWireClient(l, func(m wireMsg) { feedCh <- m })
	defer l.Close()
	cc := newControlCenter(nil, st.core)
	cc.sim = &remoteSim{c: c, t: "sim"}
	c.onError = func(err error) { cc.log.printf("director: wire: %v", err) }
	stopWorld := st.startWorld(cc)
	defer stopWorld()
	feed := localFeed{st: st, cc: cc}
	// The actuator's tugs and fuel trucks, by aircraft target.
	var vmu sync.Mutex
	vehicles := map[string][]VehicleView{}
	cc.remoteVehicles = func(target string) []VehicleView {
		vmu.Lock()
		defer vmu.Unlock()
		return append([]VehicleView(nil), vehicles[target]...)
	}
	go func() {
		if err := cc.requestModels(); err != nil {
			cc.log.printf("director: model list: %v", err)
		}
		if err := cc.requestFuelTitles(); err != nil {
			cc.log.printf("director: fuel truck list: %v", err)
		}
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return nil
		case cmd := <-cc.cmds:
			cmd()
		case icao := <-w.reqs:
			go func() {
				if err := c.call("sim", "LoadAirport", []any{icao}); err != nil {
					st.finish(icao, err)
				}
			}()
		case m := <-feedCh:
			switch m.Method {
			case "models", "groundTitles":
				var titles []string
				if len(m.Args) > 0 && json.Unmarshal(m.Args[0], &titles) == nil {
					if m.Method == "models" {
						cc.addModelTitles(titles)
					} else {
						cc.addGroundTitles(titles)
					}
				}
				continue
			case "vehicles":
				var v map[string][]VehicleView
				if len(m.Args) > 0 && json.Unmarshal(m.Args[0], &v) == nil {
					vmu.Lock()
					vehicles = v
					vmu.Unlock()
				}
				continue
			}
			if err := feedIn(m, feed, st.cache); err != nil {
				cc.log.printf("director: %v", err)
			}
		case <-tick.C:
			cc.tick()
		}
	}
}

// addModelTitles adds aircraft titles the simulator offers (an actuator's).
func (cc *controlCenter) addModelTitles(titles []string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for _, t := range titles {
		cc.models[t] = true
	}
}

// ── The actuator ───────────────────────────────────────────────────────────

// actuate makes the actuator of a World on its own connection (runOn with
// st.actLink): its sim port served on the link, what its simulator tells
// sent along, its controllers handed their messages (handle).
func (st *state) actuate(ctx context.Context, cc *controlCenter, client engine.Client, l link) (*actuatorSim, simFeed) {
	out := &wireFeedOut{send: l.Send}
	srv := newWireServer()
	a := &actuatorSim{localSim: cc.sim.(*localSim), srv: srv, send: l.Send, graph: st.cache.Graph, reqs: st.requests,
		alloc: cc.allocator, pushes: st.core.pushes.want,
		tug: func(w departureStart, g *airport.Graph, prof traffic.MotionProfile) traffic.PushbackTug {
			t := traffic.NewSimObjectTug(client, cc.inj, w.Tug, w.ReqBase+controlIDBlock-1, prof)
			t.Layout = g.Layout // from its depot on the vehicle roads, and back
			if w.TugYaw != 0 {
				t.YawDeg = w.TugYaw
			}
			if w.TugAhead != 0 {
				t.AheadMeters = w.TugAhead
			}
			return t
		},
		fuel: func(w departureStart, g *airport.Graph, prof traffic.MotionProfile) traffic.FuelService {
			f := traffic.NewSimObjectFuelTruck(client, cc.inj, w.Fuel, w.ReqBase+controlIDBlock-2, prof)
			f.Layout = g.Layout
			return f
		}}
	srv.add("sim", a)
	cc.onModels = func(titles []string) { out.put("models", titles) }
	cc.onGroundTitles = func(titles []string) { out.put("groundTitles", titles) }
	// The departures' tugs and fuel trucks for the director's map, each
	// second while there are any.
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if v := a.vehicleViews(); len(v) > 0 {
					out.put("vehicles", v)
				}
			}
		}
	}()
	go func() {
		for {
			m, err := l.Recv()
			if err != nil {
				return
			}
			if m.Kind != wireCall {
				continue
			}
			go func() {
				var r wireMsg
				if err := cc.do(func() error { r = srv.dispatch(m); return nil }); err != nil {
					r = wireMsg{Kind: wireReply, ID: m.ID, Err: err.Error()}
				}
				_ = l.Send(r)
			}()
		}
	}()
	go func() {
		<-ctx.Done()
		l.Close()
	}()
	return a, teeFeed{localFeed{st: st, cc: cc}, out}
}

// LoadAirport loads an airport's layout and procedures for the director:
// they come back on the feed.
func (a *actuatorSim) LoadAirport(icao string) error {
	go func() { a.reqs <- icao }()
	return nil
}

// handle hands msg to the controllers started off the wire.
func (a *actuatorSim) handle(msg engine.Message) bool {
	a.mu.Lock()
	ctls := append([]interface{ Handle(engine.Message) bool }(nil), a.ctls...)
	a.mu.Unlock()
	for _, c := range ctls {
		if c.Handle(msg) {
			return true
		}
	}
	return false
}

// keep adds a controller started off the wire to those handed messages.
func (a *actuatorSim) keep(c interface{ Handle(engine.Message) bool }) {
	a.mu.Lock()
	a.ctls = append(a.ctls, c)
	a.mu.Unlock()
}

// teeFeed tells two feeds the same.
type teeFeed struct{ a, b simFeed }

func (t teeFeed) Airports(list []traffic.AirportRef) { t.a.Airports(list); t.b.Airports(list) }
func (t teeFeed) Weather(w nav.Weather)              { t.a.Weather(w); t.b.Weather(w) }
func (t teeFeed) ILS(r nav.NavResult)                { t.a.ILS(r); t.b.ILS(r) }
func (t teeFeed) Procedures(p airport.Procedures)    { t.a.Procedures(p); t.b.Procedures(p) }
func (t teeFeed) Layout(icao string, l *airport.Layout, err error) {
	t.a.Layout(icao, l, err)
	t.b.Layout(icao, l, err)
}
func (t teeFeed) UserAircraft(a Aircraft, rate float64, com1 string) {
	t.a.UserAircraft(a, rate, com1)
	t.b.UserAircraft(a, rate, com1)
}
func (t teeFeed) Paused(p bool)          { t.a.Paused(p); t.b.Paused(p) }
func (t teeFeed) Traffic(scan []Traffic) { t.a.Traffic(scan); t.b.Traffic(scan) }

var _ sync.Mutex
