//go:build windows
// +build windows

package main

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
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
	// slowed: arrivals told to lose time for a conflict; still in one when
	// that has had time to work, they hold (#455).
	slowed map[string]bool
	seen   map[string]bool // conflicts logged, by pair
	// informed: when each pair not separated here was last told of each
	// other (traffic information, #570).
	informed map[string]time.Time
	asked    map[string]bool // crews that have made their request (#621)
	leveled  map[string]bool // arrivals told to stop descent for a conflict
	now    []traffic.Conflict
	done   []resolutionView // the latest last (at most 50)
}

type resolutionView struct {
	At time.Time `json:"at"`
	traffic.Resolution
	Said string `json:"said"`
}

func newConflictWatch(s *scheduler) *conflictWatch {
	return &conflictWatch{s: s, busy: map[string]time.Time{}, seen: map[string]bool{}, slowed: map[string]bool{}, informed: map[string]time.Time{}, asked: map[string]bool{}, leveled: map[string]bool{}}
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
	// Ours departed and handed to MSFS AI: their climb waypoints can be
	// changed as well (#639; live, RYR1527 flew through OKCVY ahead on the
	// same SID, both handed the same climb).
	departed := func(a traffic.TrackedAircraft) *controlled {
		it := w.s.cc.byTail(a.Tail)
		if it == nil || it.dep == nil || it.objectID != a.ObjectID || len(it.dep.ClimbPlan(a.Position)) == 0 {
			return nil
		}
		return it
	}
	canSteer := func(a traffic.TrackedAircraft, _ traffic.ResolutionKind) bool {
		w.mu.Lock()
		busy := now.Before(w.busy[a.Tail])
		w.mu.Unlock()
		return a.Ours && !busy && (enroute(a) != nil || departed(a) != nil)
	}
	pairs := map[string]bool{}
	needed := w.s.cc.separationNeeded(aircraft, w.s.airports())
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
		// Not separated in this airspace (VFR in D, E, G): told of each
		// other instead (#570).
		if !needed(c.A, c.B) {
			w.tellTraffic(now, c, aircraft)
			continue
		}
		r, ok := traffic.ResolveConflict(c, aircraft, canSteer, conflictOpts)
		if !ok {
			w.resolveArrivals(now, c) // ours on their STARs (#455)
			continue
		}
		var a traffic.TrackedAircraft
		for _, x := range aircraft {
			if x.ObjectID == r.ObjectID {
				a = x
			}
		}
		var err error
		pos, icao := traffic.PosCenter, "" // our en route aircraft: the centre (#415)
		if e := enroute(a); e != nil {
			icao = e.f.Airport
			var wps []types.SIMCONNECT_DATA_WAYPOINT
			_, wps, err = traffic.EnrouteStart(traffic.ResolvedRoute(e.route, a, r, conflictLookAhead))
			if err == nil {
				err = w.s.cc.do(func() error { return w.s.cc.fleet.SetWaypoints(e.objectID, enrouteDefWaypoints, wps) })
			}
		} else if it := departed(a); it != nil {
			// A departure: the departure radar that has it.
			pos, icao = traffic.PosDeparture, it.ICAO
			route := traffic.ResolvedRoute(it.dep.ClimbPlan(a.Position), a, r, conflictLookAhead)
			err = w.s.cc.do(func() error { return it.dep.Reroute(route) })
		} else {
			continue
		}
		if err != nil {
			tlog.printf("%-6s conflict: %s refused: %v", r.Callsign, r.Kind, err)
			continue
		}
		tx := traffic.Resolved(pos, r, a.AltFt, a.Heading, a.GroundKts)
		w.s.cc.radio.Transmit(icao, tx)
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
	involved := map[string]bool{}
	for _, c := range cs {
		involved[c.A], involved[c.B] = true, true
	}
	for cs := range w.slowed {
		if !involved[cs] {
			delete(w.slowed, cs) // out of conflict: a new one starts with the level again
		}
	}
	for cs := range w.leveled {
		if !involved[cs] {
			delete(w.leveled, cs)
		}
	}
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
	w.crewRequests(now, aircraft) // after the look: a crew in a conflict is told "unable"
}

// inConflict reports that a and b are predicted to lose separation (the
// latest look).
func (w *conflictWatch) inConflict(a, b string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.now {
		if c.A == a && c.B == b || c.A == b && c.B == a {
			return true
		}
	}
	return false
}

// view is what the separation API shows of the watch.
func (w *conflictWatch) view() ([]traffic.Conflict, []resolutionView) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]traffic.Conflict{}, w.now...), append([]resolutionView{}, w.done...)
}

// arrivalConflictRecheck is how long an arrival told to lose time for a
// conflict flies it before the conflict is looked at again (#455).
const arrivalConflictRecheck = 90 * time.Second

// resolveArrivals resolves a conflict between our arrivals on their STARs,
// which the en route resolver does not steer (#455; LKPR, live: DLH1675 on
// APRA2S and QTR1489 on VLM6T, both to 06, merging at the same level, were
// predicted 1.8 NM apart and nothing acted), as a radar controller does:
// the one landing later first stops its descent 1000 ft above the other;
// then it loses time as the Approach tab's 🐢 does (speed, then vectors: a
// dog-leg), again while that helps; it holds only once nothing more can be
// absorbed. Holds are the exception (live, AUA529 held five minutes for a
// conflict at the merge with a delay of two).
func (w *conflictWatch) resolveArrivals(now time.Time, c traffic.Conflict) {
	w.s.st.mu.Lock()
	q := w.s.st.sequences
	w.s.st.mu.Unlock()
	if q == nil {
		return
	}
	type cand struct {
		it *controlled
		e  traffic.SequenceEntry
	}
	var trailer *cand
	for _, cs := range []string{c.A, c.B} {
		it := w.s.cc.byTail(cs)
		if it == nil || it.arr == nil || it.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		if _, e, ok := q.entryOf(it.ICAO, cs); ok && (trailer == nil || e.Landing.After(trailer.e.Landing)) {
			trailer = &cand{it, e}
		}
	}
	if trailer == nil {
		return
	}
	cs, oth := trailer.it.Tail, other(c, trailer.it.Tail)
	w.mu.Lock()
	busy, leveled := now.Before(w.busy[cs]), w.leveled[cs]
	w.mu.Unlock()
	if _, _, holding := trailer.it.arr.Holding(); busy || holding {
		return // a change is flown already: see it work
	}
	recheck := func() {
		w.mu.Lock()
		w.busy[cs] = now.Add(arrivalConflictRecheck)
		w.mu.Unlock()
	}
	// Level first: above the other, still descending to it.
	if !leveled {
		w.mu.Lock()
		w.leveled[cs] = true
		w.mu.Unlock()
		var me, them *traffic.TrackedAircraft
		for _, a := range w.s.cc.world.Aircraft() {
			a := a
			switch a.Tail {
			case cs:
				me = &a
			case oth:
				them = &a
			}
		}
		if me != nil && them != nil {
			level := math.Ceil((them.AltFt+arrivalLevelAboveFt)/500) * 500
			if me.AltFt >= level-200 {
				err := w.s.cc.do(func() error { return trailer.it.arr.StopDescent(level, arrivalLevelForNM) })
				if err == nil {
					tlog.printf("%-6s conflict with %s: stop descent at %.0f ft (arrival on its STAR)", cs, oth, level)
					trailer.it.say(traffic.StopDescent(traffic.PosApproach, cs, level, oth))
					recheck()
					return
				}
				tlog.printf("%-6s conflict with %s: stop descent refused: %v", cs, oth, err)
			}
		}
	}
	action := "slow"
	err := q.approachAction(trailer.it.ICAO, cs, action)
	if errors.Is(err, errNothingToSlow) {
		// Slowed and stretched as far as it goes already: it holds now
		// (live, CSA1257 and AFR1552 merging on GOLO4S and LOMK8S: "number
		// 3" changed nothing, and they met at 0.5 NM).
		action = "hold"
		err = q.approachAction(trailer.it.ICAO, cs, action)
	}
	if err != nil {
		tlog.printf("%-6s conflict with %s: %s refused: %v", cs, oth, action, err)
		return
	}
	if action == "hold" {
		// Held until the conflict is over, not released by the sequence's
		// small delay a second later (live, CSA1257 at ERASU).
		q.mu.Lock()
		q.conflictHeld[cs] = conflictHold{other: oth, at: now}
		q.mu.Unlock()
	}
	tlog.printf("%-6s conflict with %s: %s (arrival on its STAR)", cs, oth, map[string]string{"slow": "loses time", "hold": "holds"}[action])
	recheck()
	w.mu.Lock()
	w.slowed[cs] = true
	w.mu.Unlock()
}

// An arrival in conflict with another below it stops its descent this far
// above it, for this far along its STAR.
const (
	arrivalLevelAboveFt = 1000.0
	arrivalLevelForNM   = 20.0
)

// other is the other aircraft of conflict c.
func other(c traffic.Conflict, cs string) string {
	if c.A == cs {
		return c.B
	}
	return c.A
}
