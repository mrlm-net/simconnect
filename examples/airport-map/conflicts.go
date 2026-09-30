//go:build windows
// +build windows

package main

import (
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The conflict watch (#395): every few seconds every airborne pair is flown
// on for the look-ahead; a pair predicted to come within 5 NM and 1000 ft
// gets the least disturbing change to one of our en route aircraft (a speed,
// a level or a heading off its route for the look-ahead, then back on it),
// said as ATC would. Other traffic is avoided, never steered; our aircraft
// in the terminal area are spaced by the sequencer and the tower.

const (
	conflictEvery     = 5 * time.Second
	conflictLookAhead = 5 * time.Minute
)

var conflictOpts = traffic.ConflictOptions{LookAhead: conflictLookAhead, MinNM: sepMinNM, TerminalNM: sepMinNM}

type conflictWatch struct {
	s *scheduler

	mu   sync.Mutex
	at   time.Time
	busy map[string]time.Time // resolved: the aircraft flies its change until then
	seen map[string]bool      // conflicts logged, by pair
	now  []traffic.Conflict
	done []resolutionView // the latest last (at most 50)
}

type resolutionView struct {
	At time.Time `json:"at"`
	traffic.Resolution
	Said string `json:"said"`
}

func newConflictWatch(s *scheduler) *conflictWatch {
	return &conflictWatch{s: s, busy: map[string]time.Time{}, seen: map[string]bool{}}
}

func (w *conflictWatch) tick(now time.Time, aircraft []traffic.TrackedAircraft) {
	w.mu.Lock()
	if now.Sub(w.at) < conflictEvery {
		w.mu.Unlock()
		return
	}
	w.at = now
	w.mu.Unlock()
	cs := traffic.PredictConflicts(aircraft, conflictOpts)
	// Ours en route can be steered, any way, unless already flying a change.
	enroute := func(a traffic.TrackedAircraft) *enrouteAC {
		w.s.mu.Lock()
		defer w.s.mu.Unlock()
		if e := w.s.enroute[a.Tail]; e != nil && e.objectID == a.ObjectID && !e.handing && len(e.route) > 1 {
			return e
		}
		return nil
	}
	canSteer := func(a traffic.TrackedAircraft, _ traffic.ResolutionKind) bool {
		w.mu.Lock()
		busy := now.Before(w.busy[a.Tail])
		w.mu.Unlock()
		return a.Ours && !busy && enroute(a) != nil
	}
	pairs := map[string]bool{}
	for _, c := range cs {
		pair := c.A + "/" + c.B
		pairs[pair] = true
		w.mu.Lock()
		first := !w.seen[pair]
		w.seen[pair] = true
		busy := now.Before(w.busy[c.A]) || now.Before(w.busy[c.B])
		w.mu.Unlock()
		if first {
			tlog.printf("conflict: %s and %s lose separation in %s, closest %.1f NM, %.0f ft in %s", c.A, c.B, c.In.Round(time.Second), c.ClosestNM, c.VerticalFt, c.ClosestIn.Round(time.Second))
		}
		if busy {
			continue // a change is flown already: see it work
		}
		r, ok := traffic.ResolveConflict(c, aircraft, canSteer, conflictOpts)
		if !ok {
			continue
		}
		var a traffic.TrackedAircraft
		for _, x := range aircraft {
			if x.ObjectID == r.ObjectID {
				a = x
			}
		}
		e := enroute(a)
		if e == nil {
			continue
		}
		_, wps, err := traffic.EnrouteStart(traffic.ResolvedRoute(e.route, a, r, conflictLookAhead))
		if err == nil {
			err = w.s.cc.do(func() error { return w.s.cc.fleet.SetWaypoints(e.objectID, enrouteDefWaypoints, wps) })
		}
		if err != nil {
			tlog.printf("%-6s conflict: %s refused: %v", r.Callsign, r.Kind, err)
			continue
		}
		// Said by the centre: our en route aircraft (#415).
		tx := traffic.Resolved(traffic.PosCenter, r, a.AltFt, a.Heading, a.GroundKts)
		w.s.cc.radio.Transmit(e.f.Airport, tx)
		said := tx.Text
		w.mu.Lock()
		w.busy[r.Callsign] = now.Add(conflictLookAhead)
		w.done = append(w.done, resolutionView{At: now, Resolution: r, Said: said})
		if len(w.done) > 50 {
			w.done = w.done[len(w.done)-50:]
		}
		w.mu.Unlock()
	}
	w.mu.Lock()
	w.now = cs
	for p := range w.seen {
		if !pairs[p] {
			delete(w.seen, p) // over: a new one is logged again
		}
	}
	for cs, until := range w.busy {
		if now.After(until) {
			delete(w.busy, cs)
		}
	}
	w.mu.Unlock()
}

// view is what the separation API shows of the watch.
func (w *conflictWatch) view() ([]traffic.Conflict, []resolutionView) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]traffic.Conflict{}, w.now...), append([]resolutionView{}, w.done...)
}
