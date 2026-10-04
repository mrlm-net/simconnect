//go:build windows
// +build windows

package world

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A host with its own simulator connection runs the World on it (#710):
// it feeds every message of the connection (Feed, never blocking) and runs
// the World on the client (RunOn). The World's SimConnect IDs are listed in
// docs/traffic-world.md; the host keeps its own clear of them.

// DefaultQueueSize is how many fed messages wait for the World before
// Feed drops them.
const DefaultQueueSize = 4096

// Feed hands the World one message of the host's connection without
// blocking: with the queue full it is dropped and counted (Snapshot.
// Dropped). A dropped facility reply is asked again when its loader
// expires. Call it for every message, from the host's dispatch.
func (w *World) Feed(msg engine.Message) {
	q := w.queue()
	select {
	case q <- msg:
	default:
		w.dropped.Add(1)
	}
}

func (w *World) queue() chan engine.Message {
	w.qOnce.Do(func() { w.q = make(chan engine.Message, DefaultQueueSize) })
	return w.q
}

// RunOn runs the traffic on the host's connected client until ctx ends:
// its messages come through Feed. All the World's SimConnect calls are made
// in RunOn's goroutine. Run it again on the next connection.
func (w *World) RunOn(ctx context.Context, client engine.Client) error {
	err := runOn(ctx, w.st, client, w.queue(), w.reqs, w.opts.DumpDir)
	if err == context.Canceled {
		return nil
	}
	return err
}

// Snapshot is the World's picture now, as data.
type Snapshot struct {
	At        time.Time `json:"at"`        // traffic time (the simulation rate, stopped while paused)
	Connected bool      `json:"connected"` // the traffic is running
	// Aircraft are ours: state, ATC position and frequency, routes, the
	// clearances available (Actions) and the ground vehicles.
	Aircraft []ControlView `json:"aircraft"`
	// Dropped is how many fed messages were dropped (Feed).
	Dropped uint64 `json:"dropped"`
}

// Snapshot is the picture now. The rest — an airport's runways in use,
// ATIS, ILS and weather, the landing sequences, the stands, the schedule —
// is in the API: Get("/api/airportinfo?icao=LKPR", &v), as a remote client
// reads it (docs/traffic-world.md).
func (w *World) Snapshot() Snapshot {
	out := Snapshot{At: time.Now(), Aircraft: []ControlView{}, Dropped: w.dropped.Load()}
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc != nil {
		out.Connected, out.At, out.Aircraft = true, cc.clock.Now(), cc.views()
	}
	return out
}

// Do runs one call of the World's HTTP API in process — the same API a
// remote client calls: method, path with its query ("/api/schedule"), and
// a body sent as JSON (nil none). It returns the reply's body; an error
// status is an error with the reply's text.
func (w *World) Do(method, path string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, rd)
	r.RemoteAddr = "127.0.0.1:0" // in process: the host itself
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	w.handler().ServeHTTP(rec, r)
	if rec.Code >= 400 {
		return nil, fmt.Errorf("%s %s: %d %s", method, path, rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	return rec.Body.Bytes(), nil
}

// Get is Do("GET", path) decoded into v.
func (w *World) Get(path string, v any) error {
	b, err := w.Do(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, v)
}

func (w *World) handler() http.Handler {
	w.hOnce.Do(func() {
		mux := http.NewServeMux()
		w.Register(mux)
		w.h = mux
	})
	return w.h
}

// Heard tells the World that someone outside it — the host's own ATC —
// said t on t.Frequency at t.Airport: the World's traffic waits for the
// frequency instead of talking over it. t.At is when it started (zero:
// now); it lasts as long as its text takes to say.
func (w *World) Heard(t traffic.Transmission) {
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil || t.Frequency == "" {
		return
	}
	at := t.At
	if at.IsZero() {
		at = cc.clock.Now()
	}
	cc.radio.Occupy(t.Airport, t.Frequency, at.Add(traffic.SpeakingTime(t.Text)+time.Second))
}

// PlayerPhase is what the host's ATC cleared the user aircraft to do.
type PlayerPhase string

const (
	PlayerLineUp  PlayerPhase = "lineup"  // line up and wait
	PlayerTakeoff PlayerPhase = "takeoff" // cleared for take-off
	PlayerLanding PlayerPhase = "landing" // on approach to (or cleared to land on) the runway
	PlayerVacated PlayerPhase = "vacated" // off the runway: it is free again
)

// PlayerClearance is a clearance the host's ATC gave the user aircraft.
type PlayerClearance struct {
	ICAO   string      `json:"icao"`
	Runway string      `json:"runway"` // runway end ("24")
	Phase  PlayerPhase `json:"phase"`
	// Callsign and Model: the user aircraft as said and its model (for its
	// wake and speed in a landing sequence); optional.
	Callsign string `json:"callsign,omitempty"`
	Model    string `json:"model,omitempty"`
}

// ClearPlayer tells the World what the host cleared the user aircraft to
// (the World never controls nor calls it). While it lines up, takes off or
// lands on a runway, the World clears none of its traffic onto it; landing,
// it is in that runway's landing sequence, as "Player" unless Callsign is
// given, so its traffic fits behind or ahead of it. PlayerVacated ends it.
func (w *World) ClearPlayer(c PlayerClearance) {
	w.st.core.setPlayer(c)
}

// player is the host's clearance of the user aircraft (#710).
type playerState struct {
	mu sync.Mutex
	c  *PlayerClearance
}

func (k *core) setPlayer(c PlayerClearance) {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	if c.Phase == PlayerVacated || c.Phase == "" {
		k.player.c = nil
		return
	}
	c.ICAO = strings.ToUpper(c.ICAO)
	k.player.c = &c
}

// playerOn is the user aircraft's clearance onto icao's runway rwy, if any.
func (k *core) playerOn(icao, rwy string) (PlayerClearance, bool) {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	if c := k.player.c; c != nil && c.ICAO == icao && c.Runway == rwy {
		return *c, true
	}
	return PlayerClearance{}, false
}

// playerLanding is the user aircraft's landing clearance, if it has one.
func (k *core) playerLanding() (PlayerClearance, bool) {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	if c := k.player.c; c != nil && c.Phase == PlayerLanding {
		return *c, true
	}
	return PlayerClearance{}, false
}
