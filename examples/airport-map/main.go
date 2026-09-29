//go:build windows
// +build windows

// Command airport-map loads the complete ground layout of an airport from the
// simulator with pkg/airport (runways, taxi paths, taxi points, taxi names and
// parking spots) and serves it on an interactive Leaflet map at
// http://127.0.0.1:8080, with departure routes computed by pkg/airport.
//
// It is a visual debugging tool for taxi routing: every feature on the map
// shows its raw facility index and TYPE value in a popup, so the taxi graph
// can be checked against what SimConnect actually returns.
//
//	go run ./examples/airport-map                      # live, default LKPR
//	go run ./examples/airport-map -dump                # also save <ICAO>.json
//	go run ./examples/airport-map -file LKPR.json      # offline, no simulator
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

//go:embed index.html
var web embed.FS

// User aircraft position, polled once per second.
const (
	defAircraft uint32 = 2000
	reqAircraft uint32 = 2001
)

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
	ObjectID    uint32  `json:"objectId"`
	Title       string  `json:"title"`
	Tail        string  `json:"tail"`
	State       string  `json:"state"`
	Latitude    float64 `json:"lat"`
	Longitude   float64 `json:"lon"`
	AGL         float64 `json:"agl"`
	GroundKts   float64 `json:"groundKts"`
	Heading     float64 `json:"heading"`
	VerticalFpm float64 `json:"vs"`
	OnGround    bool    `json:"onGround"`
	Gear        float64 `json:"gear"`
	// Lights lists the lights that are on: L landing, T taxi, S strobe, B beacon, N nav.
	Lights string `json:"lights"`
	User   bool   `json:"user"`
	// Ours: driven by our controllers (spawned on the map or scheduled);
	// the rest is other traffic — MSFS AI, other add-ons.
	Ours bool `json:"ours"`
	// Span is the wing span in meters; Alt the altitude above sea level in feet.
	Span float64 `json:"span"`
	Alt  float64 `json:"alt"`
}

type aircraftRaw struct {
	Latitude  float64
	Longitude float64
	Heading   float64
	GroundKts float64
	OnGround  float64
}

// Aircraft is the user aircraft position served at /api/aircraft.
type Aircraft struct {
	Latitude  float64   `json:"lat"`
	Longitude float64   `json:"lon"`
	Heading   float64   `json:"heading"`
	GroundKts float64   `json:"groundKts"`
	OnGround  bool      `json:"onGround"`
	Updated   time.Time `json:"updated"`
}

// airportResponse is the /api/airport payload: the Layout plus when it was
// fetched.
type airportResponse struct {
	*airport.Layout
	FetchedAt time.Time `json:"fetchedAt"`
}

type state struct {
	cache *airport.Cache

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
	// procedures are the SIDs, STARs and approaches by ICAO (#312).
	procedures map[string]airport.Procedures
	// requests asks the connection to load an airport (load); airways is
	// the airway graph for flight plans (#331), nil for direct routes.
	requests chan<- string
	airways  *nav.AirwayGraph
	// weather is the latest at the user aircraft; atis the information
	// services by ICAO (#357).
	weather *nav.Weather
	atis    map[string]*nav.ATISService
	// pads are the de-icing pads picked on the map (#323).
	pads *padStore
}

func (s *state) setLive(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live = v
}

// finish records the outcome of a fetch and wakes everyone waiting for it.
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

// runConnection handles one connection lifecycle. It returns nil when the
// simulator disconnects (so the caller reconnects) and ctx.Err() on shutdown.
func runConnection(ctx context.Context, st *state, requests <-chan string, dumpDir string) error {
	client := simconnect.NewClient("GO Example - airport map", engine.WithContext(ctx))

	fmt.Println("⏳ Waiting for simulator to start...")
	for {
		if err := client.Connect(); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	fmt.Println("✅ Connected to SimConnect")
	defer client.Disconnect()

	for i, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"},
		{"PLANE LONGITUDE", "degrees"},
		{"PLANE HEADING DEGREES TRUE", "degrees"},
		{"GROUND VELOCITY", "knots"},
		{"SIM ON GROUND", "bool"},
	} {
		if err := client.AddToDataDefinition(defAircraft, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
			fmt.Fprintf(os.Stderr, "❌ AddToDataDefinition(%q): %v\n", v.name, err)
		}
	}
	if err := client.RequestDataOnSimObject(reqAircraft, defAircraft, types.SIMCONNECT_OBJECT_ID_USER,
		types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "❌ RequestDataOnSimObject: %v\n", err)
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
	loader := airport.NewLoader(client, airport.LoaderWithCache(st.cache))
	procLoader := airport.NewProcedureLoader(client)
	// Weather at the user aircraft, whenever it changes.
	weather := nav.NewWeatherReader(client, weatherDefID, weatherReqID)
	if err := weather.Subscribe(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  weather: %v\n", err)
	}

	// Traffic control: controllers live in this goroutine; HTTP handlers
	// queue commands to it.
	cc := newControlCenter(client)
	cc.graph = st.cache.Graph
	cc.pads = st.pads.forAirport
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
	// The airports around, for the traffic picture: now and every minute.
	airports := traffic.NewAirportLister(client, 0)
	if err := airports.Request(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  airport list: %v\n", err)
	}
	if err := cc.requestModels(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ model list: %v\n", err)
	}
	// Scheduled traffic: its own goroutine, as it waits for the connection.
	sched := newScheduler(st, cc)
	cc.extra = sched.handle
	seqs := newSequences(cc, sched)
	sep := newSepMonitor()
	tw := newTowers(cc, sched)
	stop := make(chan struct{})
	defer close(stop) // this connection only
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-t.C:
				sched.tick(now)
				seqs.tick(now)
				sep.tick(now, cc.world.Aircraft())
				tw.tick(now)
			}
		}
	}()
	st.mu.Lock()
	st.control, st.schedule, st.sequences, st.separation, st.towers = cc, sched, seqs, sep, tw
	st.mu.Unlock()
	defer func() {
		st.mu.Lock()
		st.control, st.schedule, st.sequences, st.separation, st.towers = nil, nil, nil, nil, nil
		st.mu.Unlock()
	}()

	st.setLive(true)
	defer st.setLive(false)

	stream := client.Stream()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case cmd := <-cc.cmds:
			cmd()

		case icao := <-requests:
			fmt.Printf("🛫 Fetching facility data for %s...\n", icao)
			if err := loader.Request(icao); err != nil {
				st.finish(icao, err)
			}
			if err := procLoader.Request(icao); err != nil {
				fmt.Fprintf(os.Stderr, "❌ procedures of %s: %v\n", icao, err)
			}

		case now := <-tick.C:
			cc.tick()
			if cc.ticks%60 == 0 {
				airports.Request()
			}
			// Every aircraft within TrafficRadius of the user aircraft.
			scan = scan[:0]
			client.RequestDataOnSimObjectType(reqTraffic, defTraffic, trafficRadius, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
			for _, res := range loader.Expire(now) {
				fmt.Fprintf(os.Stderr, "❌ %v\n", res.Err)
				st.finish(res.ICAO, res.Err)
			}

		case msg, ok := <-stream:
			if !ok {
				fmt.Println("📴 Simulator disconnected")
				for _, icao := range loader.Pending() {
					st.finish(icao, errors.New("simulator disconnected"))
				}
				return nil
			}
			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "❌ Stream error: %v\n", msg.Err)
				continue
			}

			if list, ok := airports.Handle(msg); ok {
				cc.world.SetAirports(list)
				continue
			}
			if wx, ok := weather.Handle(msg); ok {
				st.mu.Lock()
				st.weather = &wx
				st.mu.Unlock()
				continue
			}
			if p, done := procLoader.Handle(msg); done {
				fmt.Printf("🧭 %s procedures: %d SIDs, %d STARs, %d approaches\n", p.ICAO, len(p.Departures), len(p.Arrivals), len(p.Approaches))
				st.mu.Lock()
				if st.procedures == nil {
					st.procedures = map[string]airport.Procedures{}
				}
				st.procedures[p.ICAO] = p
				st.mu.Unlock()
				continue
			}
			if res, done := loader.Handle(msg); done {
				if res.Err != nil {
					fmt.Fprintf(os.Stderr, "❌ %v\n", res.Err)
				} else {
					l := res.Layout
					fmt.Printf("🏁 %s %s: %d runways, %d parking, %d taxi points, %d taxi paths, %d names\n",
						l.ICAO, l.Name, len(l.Runways), len(l.Parking), len(l.TaxiPoints), len(l.TaxiPaths), len(l.TaxiNames))
					if dumpDir != "" {
						writeDump(dumpDir, res.Raw)
					}
				}
				st.finish(res.ICAO, res.Err)
				continue
			}
			if cc.handle(msg) {
				continue
			}

			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Fprintf(os.Stderr, "⚠️  SimConnect exception %d (sendID=%d, index=%d)\n", e.DwException, e.DwSendID, e.DwIndex)

			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				if uint32(d.DwRequestID) != reqAircraft {
					continue
				}
				a := engine.CastDataAs[aircraftRaw](&d.DwData)
				userID = uint32(d.DwObjectID) // the user aircraft's real object ID, as by-type scans report it
				st.mu.Lock()
				st.aircraft = &Aircraft{Latitude: a.Latitude, Longitude: a.Longitude, Heading: a.Heading,
					GroundKts: a.GroundKts, OnGround: a.OnGround != 0, Updated: time.Now()}
				st.mu.Unlock()

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
				lastPos[id] = fix{t.Lat, t.Lon, now}
				scan = append(scan, Traffic{
					ObjectID: uint32(d.DwObjectID), Title: engine.BytesToString(t.Title[:]), Tail: engine.BytesToString(t.AtcID[:]),
					State: engine.BytesToString(t.State[:]), Latitude: t.Lat, Longitude: t.Lon, AGL: t.AGL, GroundKts: t.GS,
					Heading: t.Heading, VerticalFpm: t.VS, OnGround: t.OnGround != 0, Gear: t.Gear, Lights: lights(t), Span: t.SpanFt * 0.3048, Alt: t.AltFt, User: uint32(d.DwObjectID) == userID || uint32(d.DwObjectID) == types.SIMCONNECT_OBJECT_ID_USER,
				})
				if uint32(d.DwEntryNumber) >= uint32(d.DwOutOf) {
					st.mu.Lock()
					st.traffic, st.trafficAt = scan, time.Now()
					cc.reportTraffic(scan)
					st.mu.Unlock()
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
		fmt.Fprintf(os.Stderr, "❌ Dump %s: %v\n", path, err)
		return
	}
	fmt.Printf("💾 Wrote %s\n", path)
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

	if first {
		select {
		case requests <- icao:
		case <-ctx.Done():
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
		return nil, ctx.Err()
	}
}

func serve(ctx context.Context, addr string, st *state, requests chan<- string) error {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(web))

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
	registerDeicing(mux, st)
	registerWorld(mux, st)
	registerSchedule(mux, st)
	registerSequence(mux, st)
	registerSeparation(mux, st)
	registerRunways(mux, st)

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

	// GET /api/traffic — every aircraft within 20 km of the user aircraft.
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
		if cc != nil {
			ours := cc.ownIDs()
			t = append([]Traffic(nil), t...)
			for i := range t {
				t[i].Ours = ours[t[i].ObjectID]
			}
		}
		writeJSON(w, t)
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

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	fmt.Printf("🗺️  Map at http://%s\n", addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	icao := flag.String("icao", "LKPR", "airport to open on the map")
	dump := flag.Bool("dump", false, "write each fetched airport's raw facility records to <ICAO>.json")
	dumpDir := flag.String("dump-dir", ".", "directory for -dump files")
	file := flag.String("file", "", "serve airport data from a -dump JSON file instead of the simulator")
	logDir := flag.String("log-dir", ".", "directory for the traffic control log (traffic-*.log)")
	airways := flag.String("airways", "pkg/nav/testdata/LKPR-airways.json", "airway graph for flight plans (see examples/spike-airways); \"\" for direct routes")
	flag.Parse()
	openTrafficLog(*logDir)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	st := &state{cache: airport.NewCache(), fetched: map[string]time.Time{}, waiters: map[string][]chan error{}}
	requests := make(chan string)
	st.requests = requests
	st.pads = loadPadStore(filepath.Join(*dumpDir, "deicing.json"))
	if *airways != "" {
		if g, err := nav.LoadAirwayGraph(*airways); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  airways: %v (flight plans fly direct)\n", err)
		} else {
			st.airways = g
			fmt.Printf("🛣️  airways: %d fixes, %d airways\n", len(g.Fixes), len(g.Airways))
		}
	}

	if *file != "" {
		b, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		var raw airport.RawAirport
		if err := json.Unmarshal(b, &raw); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s: %v\n", *file, err)
			os.Exit(1)
		}
		l, err := airport.BuildLayout(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s: %v\n", *file, err)
			os.Exit(1)
		}
		st.cache.Put(l)
		if fi, err := os.Stat(*file); err == nil {
			st.fetched[l.ICAO] = fi.ModTime().UTC()
		}
		*icao = l.ICAO
		fmt.Printf("📂 Loaded %s (%s) from %s — offline mode\n", l.ICAO, l.Name, *file)
	} else {
		dir := ""
		if *dump {
			dir = *dumpDir
		}
		go func() {
			for {
				if err := runConnection(ctx, st, requests, dir); err != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}()
	}

	fmt.Printf("ℹ️  Open http://%s/?icao=%s (Ctrl+C to exit)\n", *addr, *icao)
	if err := serve(ctx, *addr, st, requests); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
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
