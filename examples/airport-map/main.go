//go:build windows
// +build windows

// Command airport-map fetches the complete ground layout of an airport from
// the simulator (runways, taxi paths, taxi points, taxi names and parking
// spots) and serves it on an interactive Leaflet map at http://127.0.0.1:8080.
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
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

//go:embed index.html
var web embed.FS

// ── Facility definitions ───────────────────────────────────────────────────

// Facility definition IDs — one per query shape.
const (
	defAirport uint32 = iota + 1000
	defRunway
	defParking
	defTaxiPoint
	defTaxiPath
	defTaxiName
)

// Aircraft position data definition and request.
const (
	defAircraft uint32 = 2000
	reqAircraft uint32 = 2001
)

// Each airport fetch uses request IDs reqBase+seq*reqStride+def offset, so
// late messages from an abandoned (timed out) fetch are recognised and ignored.
const (
	reqBase      uint32 = 10000
	reqStride    uint32 = 10
	fetchParts          = 6
	fetchTimeout        = 30 * time.Second
)

// Field order in every raw struct must match its AddToFacilityDefinition calls.

type airportRaw struct {
	Latitude  float64
	Longitude float64
	Altitude  float64 // meters MSL
	ICAO      [8]byte
	Name      [32]byte
	Name64    [64]byte
}

type parkingRaw struct {
	Name    int32
	Number  uint32
	Type    int32
	Heading float32 // degrees true
	Radius  float32 // meters
	BiasX   float32 // meters east of airport reference
	BiasZ   float32 // meters north of airport reference
}

type taxiPointRaw struct {
	Type        int32
	Orientation int32
	BiasX       float32
	BiasZ       float32
}

type taxiPathRaw struct {
	Type             int32
	Width            float32
	RunwayNumber     int32
	RunwayDesignator int32
	Start            int32
	End              int32
	NameIndex        uint32
}

type taxiNameRaw struct {
	Name [32]byte
}

// runwayWireSize is the packed size of the runway definition:
// lat, lon, alt (3×f64) + heading, length, width (3×f32) + 4×i32.
const runwayWireSize = 52

// parseRunway decodes the runway buffer field by field. The record is packed
// (52 bytes), so casting it to a Go struct would read past its end.
func parseRunway(data *types.DWORD) Runway {
	raw := (*[runwayWireSize]byte)(unsafe.Pointer(data))
	f64 := func(o int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(raw[o:])) }
	f32 := func(o int) float64 { return float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[o:]))) }
	i32 := func(o int) int32 { return int32(binary.LittleEndian.Uint32(raw[o:])) }
	return Runway{
		Latitude:            f64(0),
		Longitude:           f64(8),
		Altitude:            f64(16),
		Heading:             f32(24),
		Length:              f32(28),
		Width:               f32(32),
		PrimaryNumber:       i32(36),
		PrimaryDesignator:   i32(40),
		SecondaryNumber:     i32(44),
		SecondaryDesignator: i32(48),
	}
}

func registerDefinitions(client engine.Client) {
	add := func(def uint32, fields ...string) {
		for _, f := range fields {
			if err := client.AddToFacilityDefinition(def, f); err != nil {
				fmt.Fprintf(os.Stderr, "❌ AddToFacilityDefinition(%d, %q): %v\n", def, f, err)
			}
		}
	}
	add(defAirport, "OPEN AIRPORT", "LATITUDE", "LONGITUDE", "ALTITUDE", "ICAO", "NAME", "NAME64", "CLOSE AIRPORT")
	add(defRunway, "OPEN AIRPORT", "OPEN RUNWAY",
		"LATITUDE", "LONGITUDE", "ALTITUDE", "HEADING", "LENGTH", "WIDTH",
		"PRIMARY_NUMBER", "PRIMARY_DESIGNATOR", "SECONDARY_NUMBER", "SECONDARY_DESIGNATOR",
		"CLOSE RUNWAY", "CLOSE AIRPORT")
	add(defParking, "OPEN AIRPORT", "OPEN TAXI_PARKING",
		"NAME", "NUMBER", "TYPE", "HEADING", "RADIUS", "BIAS_X", "BIAS_Z",
		"CLOSE TAXI_PARKING", "CLOSE AIRPORT")
	add(defTaxiPoint, "OPEN AIRPORT", "OPEN TAXI_POINT",
		"TYPE", "ORIENTATION", "BIAS_X", "BIAS_Z",
		"CLOSE TAXI_POINT", "CLOSE AIRPORT")
	add(defTaxiPath, "OPEN AIRPORT", "OPEN TAXI_PATH",
		"TYPE", "WIDTH", "RUNWAY_NUMBER", "RUNWAY_DESIGNATOR", "START", "END", "NAME_INDEX",
		"CLOSE TAXI_PATH", "CLOSE AIRPORT")
	add(defTaxiName, "OPEN AIRPORT", "OPEN TAXI_NAME", "NAME", "CLOSE TAXI_NAME", "CLOSE AIRPORT")

	// User aircraft position, polled once per second.
	for _, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"},
		{"PLANE LONGITUDE", "degrees"},
		{"PLANE HEADING DEGREES TRUE", "degrees"},
		{"GROUND VELOCITY", "knots"},
		{"SIM ON GROUND", "bool"},
	} {
		if err := client.AddToDataDefinition(defAircraft, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
			fmt.Fprintf(os.Stderr, "❌ AddToDataDefinition(%q): %v\n", v.name, err)
		}
	}
}

type aircraftRaw struct {
	Latitude  float64
	Longitude float64
	Heading   float64
	GroundKts float64
	OnGround  float64
}

// ── Output model (served as JSON and written by -dump) ─────────────────────

// AirportData is the raw facility data of one airport. Every list keeps the
// SimConnect item index as Index; Lat/Lon are derived from BiasX/BiasZ.
type AirportData struct {
	ICAO       string      `json:"icao"`
	Name       string      `json:"name"`
	Latitude   float64     `json:"lat"`
	Longitude  float64     `json:"lon"`
	Altitude   float64     `json:"alt"`
	FetchedAt  time.Time   `json:"fetchedAt"`
	Runways    []Runway    `json:"runways"`
	Parking    []Parking   `json:"parking"`
	TaxiPoints []TaxiPoint `json:"taxiPoints"`
	TaxiPaths  []TaxiPath  `json:"taxiPaths"`
	TaxiNames  []string    `json:"taxiNames"`
}

type Runway struct {
	Index               int     `json:"index"`
	Latitude            float64 `json:"lat"`
	Longitude           float64 `json:"lon"`
	Altitude            float64 `json:"alt"`
	Heading             float64 `json:"heading"`
	Length              float64 `json:"length"`
	Width               float64 `json:"width"`
	PrimaryNumber       int32   `json:"primaryNumber"`
	PrimaryDesignator   int32   `json:"primaryDesignator"`
	SecondaryNumber     int32   `json:"secondaryNumber"`
	SecondaryDesignator int32   `json:"secondaryDesignator"`
}

type Parking struct {
	Index   int     `json:"index"`
	Name    int32   `json:"name"`
	Number  uint32  `json:"number"`
	Type    int32   `json:"type"`
	Heading float64 `json:"heading"`
	Radius  float64 `json:"radius"`
	BiasX   float64 `json:"biasX"`
	BiasZ   float64 `json:"biasZ"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

type TaxiPoint struct {
	Index       int     `json:"index"`
	Type        int32   `json:"type"`
	Orientation int32   `json:"orientation"`
	BiasX       float64 `json:"biasX"`
	BiasZ       float64 `json:"biasZ"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

type TaxiPath struct {
	Index            int     `json:"index"`
	Type             int32   `json:"type"`
	Width            float64 `json:"width"`
	RunwayNumber     int32   `json:"runwayNumber"`
	RunwayDesignator int32   `json:"runwayDesignator"`
	Start            int32   `json:"start"`
	End              int32   `json:"end"`
	NameIndex        uint32  `json:"nameIndex"`
}

type Aircraft struct {
	Latitude  float64   `json:"lat"`
	Longitude float64   `json:"lon"`
	Heading   float64   `json:"heading"`
	GroundKts float64   `json:"groundKts"`
	OnGround  bool      `json:"onGround"`
	Updated   time.Time `json:"updated"`
}

// setAt stores v at index i, growing the slice as needed. SimConnect reports
// each list item's index explicitly; storing by it keeps Index == position.
func setAt[T any](s []T, i int, v T) []T {
	for len(s) <= i {
		var zero T
		s = append(s, zero)
	}
	s[i] = v
	return s
}

// ── Simulator side ─────────────────────────────────────────────────────────

type fetchResult struct {
	data *AirportData
	err  error
}

type fetchRequest struct {
	icao  string
	reply chan fetchResult
}

// fetch is the in-flight airport request, owned by the SimConnect loop.
type fetch struct {
	fetchRequest
	base     uint32
	pending  int
	deadline time.Time
	data     AirportData
	airport  airportRaw
	parking  []parkingRaw
	points   []taxiPointRaw
}

type state struct {
	mu       sync.RWMutex
	cache    map[string]*AirportData
	aircraft *Aircraft
	live     bool
}

func (s *state) cached(icao string) *AirportData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[icao]
}

func (s *state) store(d *AirportData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[d.ICAO] = d
}

// runConnection handles one connection lifecycle. It returns nil when the
// simulator disconnects (so the caller reconnects) and ctx.Err() on shutdown.
func runConnection(ctx context.Context, st *state, requests <-chan fetchRequest, dumpDir string) error {
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

	registerDefinitions(client)
	if err := client.RequestDataOnSimObject(reqAircraft, defAircraft, types.SIMCONNECT_OBJECT_ID_USER,
		types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "❌ RequestDataOnSimObject: %v\n", err)
	}

	st.mu.Lock()
	st.live = true
	st.mu.Unlock()
	defer func() {
		st.mu.Lock()
		st.live = false
		st.mu.Unlock()
	}()

	var (
		cur   *fetch
		queue []fetchRequest
		seq   uint32
	)

	start := func(r fetchRequest) {
		seq++
		cur = &fetch{fetchRequest: r, base: reqBase + seq*reqStride, pending: fetchParts, deadline: time.Now().Add(fetchTimeout)}
		fmt.Printf("🛫 Fetching facility data for %s...\n", r.icao)
		for i, def := range []uint32{defAirport, defRunway, defParking, defTaxiPoint, defTaxiPath, defTaxiName} {
			if err := client.RequestFacilityData(def, cur.base+uint32(i), r.icao, ""); err != nil {
				fmt.Fprintf(os.Stderr, "❌ RequestFacilityData(%s, def %d): %v\n", r.icao, def, err)
			}
		}
	}
	finish := func(err error) {
		if err == nil {
			d := cur.finalize()
			if d.Latitude == 0 && d.Longitude == 0 && len(d.Runways) == 0 {
				err = fmt.Errorf("no facility data for %q (unknown ICAO?)", cur.icao)
			} else {
				st.store(d)
				fmt.Printf("🏁 %s %s: %d runways, %d parking, %d taxi points, %d taxi paths, %d names\n",
					d.ICAO, d.Name, len(d.Runways), len(d.Parking), len(d.TaxiPoints), len(d.TaxiPaths), len(d.TaxiNames))
				if dumpDir != "" {
					writeDump(dumpDir, d)
				}
				cur.reply <- fetchResult{data: d}
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			cur.reply <- fetchResult{err: err}
		}
		cur = nil
		if len(queue) > 0 {
			next := queue[0]
			queue = queue[1:]
			start(next)
		}
	}

	stream := client.Stream()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case r := <-requests:
			if cur == nil {
				start(r)
			} else {
				queue = append(queue, r)
			}

		case <-tick.C:
			if cur != nil && time.Now().After(cur.deadline) {
				finish(fmt.Errorf("timed out fetching %s (%d of %d parts pending)", cur.icao, cur.pending, fetchParts))
			}

		case msg, ok := <-stream:
			if !ok {
				fmt.Println("📴 Simulator disconnected")
				if cur != nil {
					finish(errors.New("simulator disconnected"))
				}
				for _, r := range queue {
					r.reply <- fetchResult{err: errors.New("simulator disconnected")}
				}
				return nil
			}
			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "❌ Stream error: %v\n", msg.Err)
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

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
				if cur != nil {
					cur.add(msg.AsFacilityData())
				}

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
				end := msg.AsFacilityDataEnd()
				if cur != nil && uint32(end.RequestId) >= cur.base && uint32(end.RequestId) < cur.base+fetchParts {
					cur.pending--
					if cur.pending == 0 {
						finish(nil)
					}
				}
			}
		}
	}
}

// add stores one FACILITY_DATA message. Each request first delivers the
// AIRPORT record it was opened with; only the airport request carries its
// fields, so for the list requests only list items of the expected type are kept.
func (f *fetch) add(m *types.SIMCONNECT_RECV_FACILITY_DATA) {
	req := uint32(m.UserRequestId)
	if req < f.base || req >= f.base+fetchParts {
		return // stale message from an abandoned fetch
	}
	i := int(m.ItemIndex)
	switch req - f.base {
	case 0:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_AIRPORT {
			f.airport = *engine.CastDataAs[airportRaw](&m.Data)
		}
	case 1:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_RUNWAY {
			rw := parseRunway(&m.Data)
			rw.Index = i
			f.data.Runways = setAt(f.data.Runways, i, rw)
		}
	case 2:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_PARKING {
			f.parking = setAt(f.parking, i, *engine.CastDataAs[parkingRaw](&m.Data))
		}
	case 3:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_POINT {
			f.points = setAt(f.points, i, *engine.CastDataAs[taxiPointRaw](&m.Data))
		}
	case 4:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_PATH {
			p := engine.CastDataAs[taxiPathRaw](&m.Data)
			f.data.TaxiPaths = setAt(f.data.TaxiPaths, i, TaxiPath{
				Index: i, Type: p.Type, Width: float64(p.Width),
				RunwayNumber: p.RunwayNumber, RunwayDesignator: p.RunwayDesignator,
				Start: p.Start, End: p.End, NameIndex: p.NameIndex,
			})
		}
	case 5:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_NAME {
			n := engine.CastDataAs[taxiNameRaw](&m.Data)
			f.data.TaxiNames = setAt(f.data.TaxiNames, i, engine.BytesToString(n.Name[:]))
		}
	}
}

// finalize converts the raw records, resolving BiasX/BiasZ to lat/lon.
func (f *fetch) finalize() *AirportData {
	d := f.data
	a := f.airport
	d.ICAO = engine.BytesToString(a.ICAO[:])
	if d.ICAO == "" {
		d.ICAO = f.icao
	}
	d.Name = engine.BytesToString(a.Name64[:])
	if d.Name == "" {
		d.Name = engine.BytesToString(a.Name[:])
	}
	d.Latitude, d.Longitude, d.Altitude = a.Latitude, a.Longitude, a.Altitude
	d.FetchedAt = time.Now().UTC()

	for i, p := range f.parking {
		lat, lon := convert.OffsetToLatLon(a.Latitude, a.Longitude, float64(p.BiasX), float64(p.BiasZ))
		d.Parking = append(d.Parking, Parking{
			Index: i, Name: p.Name, Number: p.Number, Type: p.Type,
			Heading: float64(p.Heading), Radius: float64(p.Radius),
			BiasX: float64(p.BiasX), BiasZ: float64(p.BiasZ), Lat: lat, Lon: lon,
		})
	}
	for i, p := range f.points {
		lat, lon := convert.OffsetToLatLon(a.Latitude, a.Longitude, float64(p.BiasX), float64(p.BiasZ))
		d.TaxiPoints = append(d.TaxiPoints, TaxiPoint{
			Index: i, Type: p.Type, Orientation: p.Orientation,
			BiasX: float64(p.BiasX), BiasZ: float64(p.BiasZ), Lat: lat, Lon: lon,
		})
	}
	return &d
}

func writeDump(dir string, d *AirportData) {
	path := filepath.Join(dir, d.ICAO+".json")
	b, err := json.MarshalIndent(d, "", "  ")
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

func serve(ctx context.Context, addr string, st *state, requests chan<- fetchRequest) error {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(web))

	mux.HandleFunc("GET /api/airport", func(w http.ResponseWriter, r *http.Request) {
		icao := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("icao")))
		if icao == "" || len(icao) > 8 {
			http.Error(w, "icao query parameter required", http.StatusBadRequest)
			return
		}
		if d := st.cached(icao); d != nil && r.URL.Query().Get("refresh") == "" {
			writeJSON(w, d)
			return
		}
		st.mu.RLock()
		live := st.live
		st.mu.RUnlock()
		if !live {
			http.Error(w, "simulator not connected and "+icao+" not loaded from file", http.StatusServiceUnavailable)
			return
		}
		reply := make(chan fetchResult, 1)
		select {
		case requests <- fetchRequest{icao: icao, reply: reply}:
		case <-r.Context().Done():
			return
		}
		select {
		case res := <-reply:
			if res.err != nil {
				http.Error(w, res.err.Error(), http.StatusBadGateway)
				return
			}
			writeJSON(w, res.data)
		case <-r.Context().Done():
		}
	})

	mux.HandleFunc("GET /api/aircraft", func(w http.ResponseWriter, r *http.Request) {
		st.mu.RLock()
		a := st.aircraft
		st.mu.RUnlock()
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	icao := flag.String("icao", "LKPR", "airport to open on the map")
	dump := flag.Bool("dump", false, "write each fetched airport to <ICAO>.json")
	dumpDir := flag.String("dump-dir", ".", "directory for -dump files")
	file := flag.String("file", "", "serve airport data from a -dump JSON file instead of the simulator")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	st := &state{cache: map[string]*AirportData{}}
	requests := make(chan fetchRequest)

	if *file != "" {
		b, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		var d AirportData
		if err := json.Unmarshal(b, &d); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s: %v\n", *file, err)
			os.Exit(1)
		}
		st.store(&d)
		*icao = d.ICAO
		fmt.Printf("📂 Loaded %s (%s) from %s — offline mode\n", d.ICAO, d.Name, *file)
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
