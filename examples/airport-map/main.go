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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

//go:embed index.html
var web embed.FS

// User aircraft position, polled once per second.
const (
	defAircraft uint32 = 2000
	reqAircraft uint32 = 2001
)

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

	mu       sync.Mutex
	fetched  map[string]time.Time
	waiters  map[string][]chan error
	aircraft *Aircraft
	live     bool
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

	// The loader sends facility requests; this loop hands it every message.
	loader := airport.NewLoader(client, airport.LoaderWithCache(st.cache))

	st.setLive(true)
	defer st.setLive(false)

	stream := client.Stream()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case icao := <-requests:
			fmt.Printf("🛫 Fetching facility data for %s...\n", icao)
			if err := loader.Request(icao); err != nil {
				st.finish(icao, err)
			}

		case now := <-tick.C:
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
				st.mu.Lock()
				st.aircraft = &Aircraft{Latitude: a.Latitude, Longitude: a.Longitude, Heading: a.Heading,
					GroundKts: a.GroundKts, OnGround: a.OnGround != 0, Updated: time.Now()}
				st.mu.Unlock()
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
		route, err := g.RouteToRunway(from, q.Get("to"), airport.RouteOptions{UseRunwayPaths: q.Get("runwayPaths") != ""})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, airport.ErrNoRoute) {
				status = http.StatusUnprocessableEntity
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, route)
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
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	st := &state{cache: airport.NewCache(), fetched: map[string]time.Time{}, waiters: map[string][]chan error{}}
	requests := make(chan string)

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
