// Package world is the traffic engine of the airport map (#710): scheduled
// traffic around the focus airports with its ATC — spawning, pushback,
// taxi, runways, sequencing, separation, conflicts, holds, approaches,
// circuits, enroute, crews, fuel and de-icing, the phrases per position —
// with the map's HTTP API (Register) and its camera.
//
// The airport map (cmd/airport-map) is a front end over it: its page, its
// voice. A host runs a World, gives it hooks (Options) for what it does
// with the radio, and serves or calls its API.
package world

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Options configure a World. The zero value is the airport map's defaults.
type Options struct {
	// LogDir is where the traffic log is written (traffic-*.log); "": no
	// file (console and the API's recent lines only).
	LogDir string
	// GroundVehicles turns kinds of ground vehicles off from the start
	// (false; a kind not given is on): see SetGroundVehicles.
	GroundVehicles map[traffic.VehicleKind]bool
	// Airways is an airway graph for flight plans; the airways around
	// every airport loaded are read from the sim and added (#799).
	Airways *nav.AirwayGraph
	// AirwaysMaxAge: an airport's airways read from the sim are kept in
	// DataDir/airways and read again once older (0: DefaultAirwaysMaxAge).
	AirwaysMaxAge time.Duration
	// Airspace is the managed airports' control zone class for VFR rules
	// (#570); zero: class D.
	Airspace traffic.AirspaceClass
	// DataDir keeps the de-icing pads picked on the map (deicing.json) and
	// the review overlays (review/*.geojson); "": the working directory.
	DataDir string
	// DumpDir, when set, gets each fetched airport's raw facility records
	// (<ICAO>.json).
	DumpDir string
	// IDBase moves the library helpers the World creates off their default
	// SimConnect IDs, for a host using the same helpers on its connection:
	// airport loader at IDBase (definitions) and +100 (requests), procedure
	// loader +200/+300, nav loader +400/+500, airport list +600, injector
	// +700/+800/+900 (events), airway crawl +1000/+1010, enroute creations
	// +1100–+2099 (requests), late followers' puppets +2100 (a recorder)
	// and +2200–+2499 (creations). 0: the defaults
	// (docs/traffic-world.md).
	IDBase uint32
	// QueueSize is how many fed messages (Feed) wait for the World; 0:
	// DefaultQueueSize. A layout loaded while any was dropped is not kept.
	QueueSize int
	// Output is where the World writes its console lines (the traffic log
	// echoed, what it loads); nil: stdout. A host speaking a protocol on
	// stdout (an MCP server on stdio) gives another (stderr, or io.Discard).
	Output io.Writer
	// KeepOnStop leaves our aircraft in the simulator when RunOn's ctx
	// ends; by default they are removed (a host restarting its traffic on
	// the same connection found them frozen).
	KeepOnStop bool
	// Cache, when set, is the airport cache the World loads layouts into
	// and reads them from, shared with the host (which loads its own beside
	// the World's: two loads of one airport at once left the World's
	// layout without runways); nil: its own.
	Cache *airport.Cache
	// Scenes is a directory of camera scenes (*.json); "": the built-in.
	Scenes string
	// Schedule times the scheduled traffic (#741); zero values keep the
	// manager's defaults.
	Schedule ScheduleTiming

	// OnTransmission hears every transmission, once logged: the host says
	// it (its voice) or shows it. Called on the engine's goroutines; it
	// must not block.
	OnTransmission func(traffic.Transmission)
	// OnChange is told a part of the picture changed ("control": our
	// aircraft, "radio": a transmission): fetch it again now. It must not
	// block.
	OnChange func(topic string)
	// OnCom1 is told the user aircraft's COM1 active frequency each
	// second ("118.100").
	OnCom1 func(freq string)
	// OnTune is given the COM1 tuner of each connection (tunes the user
	// aircraft's COM1 to mhz), and nil once it ends.
	OnTune func(tune func(mhz float64) error)
	// SceneFrequency is told the frequency a camera scene's radio is on,
	// for a voice to follow it.
	SceneFrequency func(freq string)
}

// World is the traffic engine: one per process (its traffic log and camera
// are the process's).
type World struct {
	st   *state
	opts Options
	reqs chan string

	// A host's connection (host.go): its messages, the in-process API.
	qOnce sync.Once
	q     chan engine.Message
	hOnce sync.Once
	h     http.Handler
}

// New makes a World; it connects with Run.
func New(o Options) *World {
	if o.Output != nil {
		stdout = o.Output
	}
	if o.LogDir != "" {
		openTrafficLog(o.LogDir)
	}
	if o.Scenes != "" {
		scenesDir = o.Scenes
	}
	if o.DataDir == "" {
		o.DataDir = "."
	}
	cache := o.Cache
	if cache == nil {
		cache = airport.NewCache()
	}
	st := &state{core: newCore(tlog), cache: cache, fetched: map[string]time.Time{}, waiters: map[string][]chan error{}, keepOnStop: o.KeepOnStop}
	if o.Airspace != "" {
		st.core.zone = o.Airspace
	}
	st.core.hooks = o
	reqs := make(chan string)
	st.requests = reqs
	st.airwayRoutes = make(chan airwayRoute, 4)
	st.procRequests = make(chan string, 16)
	st.pads = loadPadStore(filepath.Join(o.DataDir, "deicing.json"))
	st.pushes = loadPushStore(filepath.Join(o.DataDir, "custom-pushes.json"))
	st.stations = loadStationStore(filepath.Join(o.DataDir, "stations.json"))
	st.reviewDir = filepath.Join(o.DataDir, "review")
	st.airways, st.airwaysGiven = o.Airways, o.Airways
	w := &World{st: st, opts: o, reqs: reqs}
	if len(o.GroundVehicles) > 0 {
		w.SetGroundVehicles(o.GroundVehicles)
	}
	return w
}

// Run connects to the simulator and runs the traffic until ctx ends,
// connecting again whenever the simulator goes away.
func (w *World) Run(ctx context.Context) {
	for {
		if err := runConnection(ctx, w.st, w.reqs, w.opts.DumpDir); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// LoadFile serves an airport from a raw facility dump (-dump) instead of
// the simulator: offline. It returns the airport's ICAO.
func (w *World) LoadFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	w.st.cache.Put(l)
	if fi, err := os.Stat(path); err == nil {
		w.st.mu.Lock()
		w.st.fetched[l.ICAO] = fi.ModTime().UTC()
		w.st.mu.Unlock()
	}
	return l.ICAO, nil
}

// Connected reports a live simulator connection with the traffic running.
func (w *World) Connected() bool {
	w.st.mu.Lock()
	defer w.st.mu.Unlock()
	return w.st.control != nil
}

// Recent is the latest n transmissions at icao ("" all), oldest first.
func (w *World) Recent(icao string, n int) []traffic.Transmission {
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil {
		return nil
	}
	return cc.radio.Recent(icao, n)
}

// ATISOn is the ATIS broadcast on freq now: its airport and text.
func (w *World) ATISOn(freq string) (icao, text string, ok bool) { return w.st.atisOn(freq) }

// ATISText is icao's current ATIS, as written.
func (w *World) ATISText(icao string) (string, bool) {
	w.st.mu.Lock()
	svc := w.st.atis[icao]
	w.st.mu.Unlock()
	if svc == nil {
		return "", false
	}
	a, ok := svc.Current()
	if !ok {
		return "", false
	}
	return a.Text(), true
}

// HeardOnCamera cuts the camera to t's aircraft as it is heard (a voice
// calls it when it says t; without one the World does as it is said).
func (w *World) HeardOnCamera(t traffic.Transmission) { heardOnCamera(t) }

// ShareLinksHost is the address the HTTP API is served on, for the camera
// and links that name it ("127.0.0.1:8080").
func (w *World) SetListenAddr(addr string) { listenAddr = addr }

// ParseAirspaceClass reads a control zone class: C, D, E or G.
func ParseAirspaceClass(s string) (traffic.AirspaceClass, error) { return parseAirspaceClass(s) }

// systemEventStater turns a subscribed system event on or off (the
// engine's SetSystemEventState), asserted on a client.
type systemEventStater interface {
	SetSystemEventState(eventID uint32, state types.SIMCONNECT_STATE) error
}
