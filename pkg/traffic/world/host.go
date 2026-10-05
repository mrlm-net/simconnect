package world

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
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
	// Player is the user aircraft's place in a landing sequence while the
	// host has cleared it to land (ClearPlayer); nil otherwise.
	Player *PlayerPlace `json:"player,omitempty"`
	// Dropped is how many fed messages were dropped (Feed).
	Dropped uint64 `json:"dropped"`
}

// PlayerPlace is where the user aircraft is in its runway's landing
// sequence: "number 2, follow the Airbus A320".
type PlayerPlace struct {
	ICAO   string `json:"icao"`
	Runway string `json:"runway"`
	Number int    `json:"number"` // 1 lands first
	// Leader is the call sign landing before it ("" none); LeaderType its
	// type as said ("Airbus A320"), "" when not one of ours.
	Leader     string  `json:"leader,omitempty"`
	LeaderType string  `json:"leaderType,omitempty"`
	SpacingNM  float64 `json:"spacingNM,omitempty"`
	// DistanceToGoNM: the player's, and the leader's (-1 none).
	DistanceToGoNM       float64 `json:"distanceToGoNM"`
	LeaderDistanceToGoNM float64 `json:"leaderDistanceToGoNM"`
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
		out.Player = cc.playerPlace()
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
	// On the ground (#739): pushing back (ours near its stand wait), taxiing
	// to Runway, holding short of it (its place in the departure queue).
	PlayerPushback     PlayerPhase = "pushback"
	PlayerTaxi         PlayerPhase = "taxi"
	PlayerHoldingShort PlayerPhase = "holding_short"
	PlayerLineUp       PlayerPhase = "lineup"  // line up and wait
	PlayerTakeoff      PlayerPhase = "takeoff" // cleared for take-off
	PlayerLanding      PlayerPhase = "landing" // on approach to (or cleared to land on) the runway
	PlayerVacated      PlayerPhase = "vacated" // off the runway: it is free again
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
// (the World never controls nor calls it). Pushing back, ours on stands
// near it wait to push; holding short of Runway, it takes its place in the
// departure queue (ours behind it wait for it). While it lines up, takes off or
// lands on a runway, the World clears none of its traffic onto it; landing,
// it is in that runway's landing sequence, as "Player" unless Callsign is
// given, so its traffic fits behind or ahead of it. PlayerVacated ends it.
func (w *World) ClearPlayer(c PlayerClearance) {
	w.st.core.setPlayer(c)
}

// player is the host's clearance of the user aircraft (#710), and where
// the user aircraft is (the feed).
type playerState struct {
	mu sync.Mutex
	c  *PlayerClearance
	at airport.LatLon
	// localSec: the sim's local time at the user aircraft, seconds of the
	// day (0 unknown).
	localSec float64
}

// setLocalSec keeps the sim's local time of day.
func (k *core) setLocalSec(s float64) {
	k.player.mu.Lock()
	k.player.localSec = s
	k.player.mu.Unlock()
}

// localHour is the sim's local hour at the user aircraft; false unknown.
func (k *core) localHour() (int, bool) {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	if k.player.localSec <= 0 {
		return 0, false
	}
	return int(k.player.localSec/3600) % 24, true
}

// userAt is where the user aircraft is (zero: not known yet).
func (k *core) userAt() airport.LatLon {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	return k.player.at
}

// setUserAt keeps where the user aircraft is.
func (k *core) setUserAt(p airport.LatLon) {
	k.player.mu.Lock()
	k.player.at = p
	k.player.mu.Unlock()
}

// playerPushingNear reports whether the user aircraft pushes back at icao
// within m meters of p (#739).
func (k *core) playerPushingNear(icao string, p airport.LatLon, m float64) bool {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	c := k.player.c
	return c != nil && c.Phase == PlayerPushback && c.ICAO == icao && calc.HaversineMeters(k.player.at.Lat, k.player.at.Lon, p.Lat, p.Lon) < m
}

// playerClearance is the host's clearance of the user aircraft, if any.
func (k *core) playerClearance() (PlayerClearance, bool) {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	if c := k.player.c; c != nil {
		return *c, true
	}
	return PlayerClearance{}, false
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

// playerOn is the user aircraft's clearance onto icao's runway rwy (a
// runway end "24" or the runway "06/24": the host gives an end), if it
// lines up, takes off or lands there.
func (k *core) playerOn(icao, rwy string) (PlayerClearance, bool) {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	onIt := func(c *PlayerClearance) bool {
		return c.Runway == rwy || slices.Contains(strings.Split(rwy, "/"), c.Runway)
	}
	if c := k.player.c; c != nil && c.ICAO == icao && onIt(c) && (c.Phase == PlayerLineUp || c.Phase == PlayerTakeoff || c.Phase == PlayerLanding) {
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

// playerPlace is the user aircraft's place in its landing sequence.
func (cc *controlCenter) playerPlace() *PlayerPlace {
	p, ok := cc.core.playerLanding()
	if !ok || cc.sequencesAt == nil {
		return nil
	}
	cs := p.Callsign
	if cs == "" {
		cs = "Player"
	}
	seq := cc.sequencesAt(p.ICAO)[p.Runway]
	for _, e := range seq {
		if e.Callsign != cs {
			continue
		}
		out := &PlayerPlace{ICAO: p.ICAO, Runway: p.Runway, Number: e.Number, Leader: e.Leader, SpacingNM: e.SpacingNM, DistanceToGoNM: e.DistanceToGoNM, LeaderDistanceToGoNM: -1}
		for _, l := range seq {
			if l.Callsign == e.Leader && e.Leader != "" {
				out.LeaderDistanceToGoNM = l.DistanceToGoNM
			}
		}
		if it := cc.byTail(e.Leader); it != nil && e.Leader != "" {
			it.mu.Lock()
			out.LeaderType = typeSaid(traffic.ProfileFor(it.view.Model).Type)
			it.mu.Unlock()
		}
		return out
	}
	return nil
}

// ScheduleSettings start or stop the scheduled traffic (POST /api/schedule).
type ScheduleSettings struct {
	Enabled     bool    `json:"enabled"`
	ICAO        string  `json:"icao"`
	Density     float64 `json:"density,omitempty"`     // 1: the timetable as it is
	MaxAircraft int     `json:"maxAircraft,omitempty"` // 0: no limit
	Seed        uint64  `json:"seed,omitempty"`
	// Airports are several scheduled airports (ICAO, with ICAO).
	Airports []string `json:"airports,omitempty"`
	// IFR, VFR: the airline and the light aircraft flights (nil: as
	// they are). Generator false: only flights added (AddFlights, #738).
	IFR       *bool `json:"ifr,omitempty"`
	VFR       *bool `json:"vfr,omitempty"`
	Generator *bool `json:"generator,omitempty"`
	// OffsetMin: the airline timetable this many minutes later (or
	// earlier) is flown now (#738); VFR flights keep the daylight of now.
	OffsetMin *float64 `json:"offsetMin,omitempty"`
	// Others: "respect" or "ignore" the traffic not ours.
	Others string `json:"others,omitempty"`
}

// SetSchedule starts or stops the scheduled traffic.
func (w *World) SetSchedule(s ScheduleSettings) error {
	_, err := w.Do(http.MethodPost, "/api/schedule", s)
	return err
}

// AddFlights adds flights at chosen times (#737): each with a callsign,
// origin and destination (one a scheduled airport, SetSchedule), STD and
// STA in traffic time, and an airline or type (default A320). The manager
// spawns them as the timetable's.
func (w *World) AddFlights(flights []traffic.Flight) error {
	_, err := w.Do(http.MethodPost, "/api/flights", flights)
	return err
}

// Clear gives one of ours (ControlView.ID) a clearance from its Actions:
// pushback, startup, taxi, lineup, takeoff, land, goaround, remove, …
func (w *World) Clear(id int, action string) error {
	_, err := w.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/%s", id, action), nil)
	return err
}

// Approach acts on an arrival in icao's sequence: slow, direct, goaround.
func (w *World) Approach(icao, callsign, action string) error {
	_, err := w.Do(http.MethodPost, fmt.Sprintf("/api/approach/%s/%s/%s", icao, callsign, action), nil)
	return err
}

// registerPlayer serves the host's clearance of the user aircraft (#739):
// POST /api/player a PlayerClearance (as ClearPlayer; phase "vacated"
// ends it), GET /api/player the clearance now ({} none).
func registerPlayer(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/player", func(w http.ResponseWriter, r *http.Request) {
		c, _ := st.core.playerClearance()
		writeJSON(w, c)
	})
	mux.HandleFunc("POST /api/player", func(w http.ResponseWriter, r *http.Request) {
		var c PlayerClearance
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		st.core.setPlayer(c)
		st.core.log.printf("player: %s %s %s", c.Phase, strings.ToUpper(c.ICAO), c.Runway)
		w.WriteHeader(http.StatusNoContent)
	})
}
