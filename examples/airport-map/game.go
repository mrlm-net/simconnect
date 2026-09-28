//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The ATC game (v0.10, #272): the player works ground and tower. Traffic
// arrives on STARs and departs on SIDs, every aircraft holding at each
// clearance; the player gives them all (pushback, taxi, line up, take-off,
// crossings, hold position, go around, abort take-off). Handling a flight
// scores, waiting at a clearance, lost separation on the ground and two
// aircraft on a runway cost.

// Scoring.
const (
	gameFlightPoints    = 10               // a departure airborne, an arrival parked
	gameWaitGrace       = 60 * time.Second // waiting for a clearance is free this long…
	gameWaitEvery       = 30 * time.Second // …then costs gameWaitPenalty this often
	gameWaitPenalty     = 1
	gameSeparation      = 50 // wingtip clearance lost on the ground (safe zones overlap)
	gameIncursion       = 100
	gameConflictRepeat  = time.Minute // the same conflict counts again after this
	gameSafeMarginM     = 3.0
	gameMaxActive       = 8
	gameDefaultInterval = 3 * time.Minute
)

var gameAirlines = []string{"CSA", "AUA", "DLH", "BAW", "AFR", "KLM", "WZZ", "RYR", "LOT", "SWR", "EZY", "UAE"}

type game struct {
	mu       sync.Mutex
	On       bool
	ICAO     string
	Runway   string
	Interval float64 // seconds between new flights (±30 %)
	Score    int
	Handled  int
	Spawned  int
	Started  time.Time
	NextIn   float64
	Events   []string

	next      time.Time
	arrival   bool                 // the next spawn is an arrival
	done      map[int]bool         // flights scored
	waitFrom  map[int]time.Time    // waiting for a clearance since
	waitPaid  map[int]time.Time    // last wait penalty
	conflicts map[string]time.Time // pair → last counted
}

func (g *game) event(points int, format string, args ...any) {
	g.Score += points
	msg := fmt.Sprintf(format, args...)
	if points != 0 {
		msg = fmt.Sprintf("%+d  %s", points, msg)
	}
	g.Events = append(g.Events, time.Now().Format("15:04:05")+"  "+msg)
	if len(g.Events) > 200 {
		g.Events = g.Events[len(g.Events)-200:]
	}
	tlog.printf("GAME %s (score %d)", msg, g.Score)
}

// gameTick runs the game once a second, in the connection goroutine.
func (cc *controlCenter) gameTick(now time.Time) {
	g := cc.game
	if g == nil {
		return
	}
	g.mu.Lock()
	on, icao := g.On, g.ICAO
	g.mu.Unlock()
	if !on || cc.graph == nil {
		return
	}
	graph, err := cc.graph(icao)
	if err != nil {
		return
	}
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		if it.ICAO == icao {
			items = append(items, it)
		}
	}
	scan := cc.scan
	cc.mu.Unlock()
	sort.Slice(items, func(a, b int) bool { return items[a].ID < items[b].ID })

	g.mu.Lock()
	defer g.mu.Unlock()
	// New traffic.
	active := 0
	for _, it := range items {
		it.mu.Lock()
		if !it.view.Done {
			active++
		}
		it.mu.Unlock()
	}
	if !now.Before(g.next) && active < gameMaxActive {
		g.next = now.Add(time.Duration(g.Interval * float64(time.Second) * (0.7 + 0.6*rand.Float64())))
		kind := "departure"
		if g.arrival {
			kind = "arrival"
		}
		g.arrival = !g.arrival
		tail := fmt.Sprintf("%s%d", gameAirlines[rand.IntN(len(gameAirlines))], 100+rand.IntN(900))
		r := SpawnRequest{Kind: kind, ICAO: icao, Stand: -1, Runway: g.Runway, Model: "FSLTL A320 Air France SL", Tail: tail,
			Gates: true, Tug: kind == "departure", Procedure: true, Deice: "auto"}
		if it, err := cc.spawn(graph, r); err != nil {
			g.event(0, "%s %s could not be spawned: %v", tail, kind, err)
		} else {
			g.Spawned++
			what := "ready for pushback at " + it.view.Stand
			if kind == "arrival" {
				what = "inbound on " + it.view.Procedure
			}
			g.event(0, "%s %s", tail, what)
		}
	}
	g.NextIn = g.next.Sub(now).Seconds()

	// Flights handled, and waiting for clearances.
	type onRunway struct {
		tail string
		rwy  int
	}
	var occupying []onRunway
	for _, it := range items {
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		switch {
		case !g.done[v.ID] && ((v.Kind == "departure" && v.State == traffic.TaxiComplete.String()) ||
			(v.Kind == "arrival" && v.State == traffic.ArrivalParked.String())):
			g.done[v.ID] = true
			g.Handled++
			verb := "airborne"
			if v.Kind == "arrival" {
				verb = "on stand " + v.Stand
			}
			g.event(gameFlightPoints, "%s %s", v.Tail, verb)
		}
		if waitingForATC(v) {
			if g.waitFrom[v.ID].IsZero() {
				g.waitFrom[v.ID] = now
			}
			if now.Sub(g.waitFrom[v.ID]) > gameWaitGrace && now.Sub(g.waitPaid[v.ID]) >= gameWaitEvery {
				g.waitPaid[v.ID] = now
				g.event(-gameWaitPenalty, "%s waiting %s (%s)", v.Tail, now.Sub(g.waitFrom[v.ID]).Round(time.Second), v.State)
			}
		} else {
			delete(g.waitFrom, v.ID)
		}
		if rwy, ok := runwayOccupied(graph, v); ok {
			occupying = append(occupying, onRunway{v.Tail, rwy})
		}
	}
	// Two aircraft on one runway.
	for i := range occupying {
		for j := i + 1; j < len(occupying); j++ {
			if occupying[i].rwy == occupying[j].rwy && g.newConflict("rwy", occupying[i].tail, occupying[j].tail, now) {
				g.event(-gameIncursion, "runway incursion: %s and %s on runway %s", occupying[i].tail, occupying[j].tail, graph.Layout.Runways[occupying[i].rwy].Name())
			}
		}
	}
	// Wingtips on the ground.
	var ground []Traffic
	for _, t := range scan {
		if t.OnGround && t.Span > 0 && !t.User {
			ground = append(ground, t)
		}
	}
	for i := range ground {
		for j := i + 1; j < len(ground); j++ {
			a, b := ground[i], ground[j]
			d := calc.HaversineMeters(a.Latitude, a.Longitude, b.Latitude, b.Longitude)
			if d < a.Span/2+b.Span/2+gameSafeMarginM && (a.GroundKts > 1 || b.GroundKts > 1) &&
				g.newConflict("sep", a.Tail, b.Tail, now) {
				g.event(-gameSeparation, "separation lost on the ground: %s and %s %.0f m apart", a.Tail, b.Tail, d)
			}
		}
	}
}

// newConflict reports a conflict between a and b not counted within
// gameConflictRepeat, and records it.
func (g *game) newConflict(kind, a, b string, now time.Time) bool {
	if a > b {
		a, b = b, a
	}
	k := kind + "|" + a + "|" + b
	if last, ok := g.conflicts[k]; ok && now.Sub(last) < gameConflictRepeat {
		return false
	}
	g.conflicts[k] = now
	return true
}

// waitingForATC reports an aircraft stopped for a clearance only the
// player can give.
func waitingForATC(v ControlView) bool {
	switch v.State {
	case traffic.TaxiAwaitingPushback.String(), traffic.TaxiAwaitingTaxi.String(), traffic.TaxiHoldingShort.String(), traffic.TaxiLinedUp.String(),
		traffic.ArrivalAwaitingTaxi.String(), traffic.ArrivalHoldingShort.String():
		return true
	}
	return v.AtLimit
}

// runwayOccupied is the runway an aircraft is on: lining up, lined up or
// rolling for take-off; landing, rolling out or vacating.
func runwayOccupied(g *airport.Graph, v ControlView) (int, bool) {
	switch v.State {
	case traffic.TaxiLiningUp.String(), traffic.TaxiLinedUp.String(), traffic.TaxiDeparting.String(),
		traffic.ArrivalLanding.String(), traffic.ArrivalRollout.String():
	default:
		return 0, false
	}
	if !v.OnGround {
		return 0, false // airborne: above the runway, not on it
	}
	r := g.RunwayAt(v.Position, 50)
	return r, r >= 0
}

// registerGame adds the game API: GET /api/game, POST /api/game
// {"on":true,"icao":"LKPR","runway":"24","intervalSec":180}.
func registerGame(mux *http.ServeMux, st *state) {
	get := func(w http.ResponseWriter) *controlCenter {
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		if cc == nil {
			http.Error(w, "simulator not connected", http.StatusServiceUnavailable)
		}
		return cc
	}
	mux.HandleFunc("GET /api/game", func(w http.ResponseWriter, r *http.Request) {
		cc := get(w)
		if cc == nil {
			return
		}
		g := cc.game
		g.mu.Lock()
		defer g.mu.Unlock()
		writeJSON(w, map[string]any{
			"on": g.On, "icao": g.ICAO, "runway": g.Runway, "intervalSec": g.Interval, "score": g.Score,
			"handled": g.Handled, "spawned": g.Spawned, "started": g.Started, "nextInSec": g.NextIn,
			"events": append([]string(nil), g.Events[max(0, len(g.Events)-40):]...),
		})
	})
	mux.HandleFunc("POST /api/game", func(w http.ResponseWriter, r *http.Request) {
		cc := get(w)
		if cc == nil {
			return
		}
		var req struct {
			On       bool    `json:"on"`
			ICAO     string  `json:"icao"`
			Runway   string  `json:"runway"`
			Interval float64 `json:"intervalSec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		g := cc.game
		g.mu.Lock()
		defer g.mu.Unlock()
		if req.On && !g.On {
			g.ICAO, g.Runway = strings.ToUpper(req.ICAO), req.Runway
			g.Interval = req.Interval
			if g.Interval <= 0 {
				g.Interval = gameDefaultInterval.Seconds()
			}
			g.Score, g.Handled, g.Spawned, g.Events = 0, 0, 0, nil
			g.done, g.waitFrom, g.waitPaid, g.conflicts = map[int]bool{}, map[int]time.Time{}, map[int]time.Time{}, map[string]time.Time{}
			g.Started, g.next = time.Now(), time.Now()
			g.On = true
			g.event(0, "game started at %s, runway %s, traffic every ~%s", g.ICAO, g.Runway, time.Duration(g.Interval*float64(time.Second)))
		} else if !req.On && g.On {
			g.On = false
			g.event(0, "game over: %d points, %d flights handled in %s", g.Score, g.Handled, time.Since(g.Started).Round(time.Second))
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
