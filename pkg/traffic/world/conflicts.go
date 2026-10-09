package world

import (
	"errors"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
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

var conflictOpts = traffic.ConflictOptions{LookAhead: conflictLookAhead, MinNM: sepMinNM, TerminalNM: sepTerminalNM}

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
	// stopped: ours told to stop a climb or descent for traffic, cleared
	// on once clear of it.
	stopped map[string]stoppedLevel
	now     []traffic.Conflict
	done    []resolutionView // the latest last (at most 50)
}

// stoppedLevel is a climb or descent stopped for traffic: who says the
// clearance on (pos at icao) and to what level.
type stoppedLevel struct {
	icao  string
	pos   traffic.Position
	altFt float64
	climb bool
	// other is the traffic it was stopped for; lastNM their distance at
	// the last look: moving apart, the stop is over (live, TVS161 held at
	// 8000 ft 1.5 min past the closest point).
	other  string
	lastNM float64
	// planned: the route before the change, flown again when the stop ends
	// before the change does (the change holds the level LookAhead long).
	planned []traffic.RoutePoint
}

// routeClimbs reports whether planned's next altitude off altFt (by more
// than 300 ft) is above it.
func routeClimbs(planned []traffic.RoutePoint, altFt float64) bool {
	for _, p := range planned {
		if p.AltFt > 0 && math.Abs(p.AltFt-altFt) > 300 {
			return p.AltFt > altFt
		}
	}
	return false
}

// levelOn is the level the planned route climbs (or descends) on to
// beyond stopFt: its highest (lowest) point; ok false when none is.
func levelOn(planned []traffic.RoutePoint, stopFt float64, climb bool) (float64, bool) {
	best, ok := stopFt, false
	for _, p := range planned {
		if climb && p.AltFt > best+200 || !climb && p.AltFt > 0 && p.AltFt < best-200 {
			best, ok = p.AltFt, true
		}
	}
	return math.Round(best/1000) * 1000, ok // a level: whole thousands
}

type resolutionView struct {
	At time.Time `json:"at"`
	traffic.Resolution
	Said string `json:"said"`
}

func newConflictWatch(s *scheduler) *conflictWatch {
	return &conflictWatch{s: s, busy: map[string]time.Time{}, seen: map[string]bool{}, slowed: map[string]bool{}, informed: map[string]time.Time{}, asked: map[string]bool{}, leveled: map[string]bool{}, stopped: map[string]stoppedLevel{}}
}

func (w *conflictWatch) tick(now time.Time, aircraft []traffic.TrackedAircraft) {
	w.mu.Lock()
	if now.Sub(w.at) < conflictEvery {
		w.mu.Unlock()
		return
	}
	w.at = now
	w.mu.Unlock()
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
		if it == nil || it.dep == nil || it.object() != a.ObjectID || len(it.dep.ClimbPlan(a.Position)) == 0 {
			return nil
		}
		return it
	}
	// A departure is never slowed: it climbs at its climb speed and is
	// turned or levelled instead (live, AUA818 told "reduce speed to 200
	// knots" passing 2400 ft).
	canSteer := func(a traffic.TrackedAircraft, kind traffic.ResolutionKind) bool {
		w.mu.Lock()
		busy := now.Before(w.busy[a.Tail])
		w.mu.Unlock()
		if !a.Ours || busy {
			return false
		}
		// Flying a TCAS RA: nothing contrary to it (FAA JO 7110.65 2-1-28).
		if it := w.s.cc.byTail(a.Tail); it != nil && it.tcasRA.Load() {
			return false
		}
		if enroute(a) != nil {
			return true
		}
		return departed(a) != nil && kind != traffic.ResolveSpeed
	}
	// The named fixes ahead of ours, for shortcuts.
	opts := conflictOpts
	opts.DirectFixes = func(a traffic.TrackedAircraft) []traffic.DirectFix {
		var fixes []airFix
		if e := enroute(a); e != nil {
			fixes = e.fixes
		} else if it := departed(a); it != nil {
			it.mu.Lock()
			own := it.fixes
			it.mu.Unlock()
			fixes = fixesAhead(own, it.dep.ClimbRoute(a.Position))
		}
		var out []traffic.DirectFix
		for _, f := range fixes {
			brg := calc.BearingDegrees(a.Position.Lat, a.Position.Lon, f.Lat, f.Lon)
			if math.Abs(math.Mod(brg-a.Heading+540, 360)-180) < 90 {
				out = append(out, traffic.DirectFix{Ident: f.Ident, Position: f.LatLon})
			}
		}
		return out
	}
	// Ours are predicted along their routes, turning where they turn:
	// arrivals on their STAR and approach, departures on their climb, en
	// route on their plan (live, CSA786 stopped at 8000 ft for KLM130
	// predicted straight on where its STAR turned away).
	opts.Route = func(a traffic.TrackedAircraft) []airport.LatLon {
		if !a.Ours {
			return nil
		}
		if e := enroute(a); e != nil {
			var pts []airport.LatLon
			for _, p := range e.route {
				pts = append(pts, p.Position)
			}
			return traffic.RouteAhead(a.Position, pts)
		}
		it := w.s.cc.byTail(a.Tail)
		if it == nil || it.object() != a.ObjectID {
			return nil
		}
		if it.dep != nil {
			return it.dep.ClimbRoute(a.Position) // from its next waypoint already
		}
		if it.arr != nil && it.arr.State() == traffic.ArrivalApproaching {
			it.mu.Lock()
			route := it.approach
			it.mu.Unlock()
			return traffic.RouteAhead(a.Position, route)
		}
		return nil
	}
	// And vertically along their profile: climbing and descending as the
	// SID, STAR and plan have them, not at the vertical speed now for
	// ever (live, TVS524 and BAW1413 on their SID and STAR, #657).
	opts.Profile = func(a traffic.TrackedAircraft) []traffic.RoutePoint {
		if !a.Ours {
			return nil
		}
		if e := enroute(a); e != nil {
			return traffic.ProfileAhead(a.Position, e.route)
		}
		it := w.s.cc.byTail(a.Tail)
		if it == nil || it.object() != a.ObjectID {
			return nil
		}
		if it.dep != nil {
			return it.dep.ClimbPlan(a.Position)
		}
		if it.arr != nil && it.arr.State() == traffic.ArrivalApproaching {
			return it.arr.ProcedurePlan()
		}
		return nil
	}
	cs := traffic.PredictConflicts(aircraft, opts)
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
			w.s.cc.log.printf("conflict: %s (%.0f ft) and %s (%.0f ft) lose separation in %s at %.1f NM, %.0f ft; closest %.1f NM, %.0f ft in %s",
				c.A, c.AAltFt, c.B, c.BAltFt, c.In.Round(time.Second), c.LossNM, c.LossFt, c.ClosestNM, c.VerticalFt, c.ClosestIn.Round(time.Second))
		}
		if busy {
			continue // a change is flown already: see it work
		}
		// Not separated in this airspace (VFR in D, E, G): told of each
		// other instead (#570).
		if !needed(c.A, c.B) {
			// Only while they still close on each other: at the closest
			// point already they move apart, nothing to look out for (live,
			// TVS1324 told of an SR22 at 3 o'clock moving away, #704),
			// unless that closest point is very close.
			if c.ClosestIn <= 0 && c.ClosestNM >= trafficInfoNearNM {
				continue
			}
			w.tellTraffic(now, c, aircraft)
			continue
		}
		r, ok := traffic.ResolveConflict(c, aircraft, canSteer, opts)
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
		radarFt := departureClimbFt        // a departure: the level departure cleared it to
		var planned []traffic.RoutePoint   // the route before the change
		pos, icao := traffic.PosCenter, "" // our en route aircraft: the centre (#415)
		if e := enroute(a); e != nil {
			icao, planned = e.f.Airport, e.route
			resolved := traffic.ResolvedRoute(e.route, a, r, conflictLookAhead)
			var wps []types.SIMCONNECT_DATA_WAYPOINT
			_, wps, err = traffic.EnrouteStart(resolved)
			if err == nil {
				err = w.s.cc.do(func() error { return w.s.cc.sim.SetRoute(e.objectID, wps) })
			}
			if err == nil {
				// Predicted on the route it flies now, the change in it
				// (#657 review: a stop predicted climbing on through).
				w.s.mu.Lock()
				e.route = resolved
				w.s.mu.Unlock()
			}
		} else if it := departed(a); it != nil {
			// A departure: the departure radar that has it.
			pos, icao = traffic.PosDeparture, it.ICAO
			planned = it.dep.ClimbPlan(a.Position)
			if it.radarFt > 0 {
				radarFt = it.radarFt
			}
			route := traffic.ResolvedRoute(planned, a, r, conflictLookAhead)
			err = w.s.cc.do(func() error { return it.dep.Reroute(route) })
		} else {
			continue
		}
		if err != nil {
			w.s.cc.log.printf("%-6s conflict: %s refused: %v", r.Callsign, r.Kind, err)
			continue
		}
		tx := traffic.Resolved(pos, r, a.AltFt, a.Heading, a.GroundKts, traffic.SaidWhere{TAFt: w.s.cc.taOf(icao), MagVar: w.s.cc.magVar(icao)})
		w.s.cc.log.printf("%-6s conflict: %s at %.0f ft, keeps %.0f ft from the traffic within the lateral minimum (%s)", r.Callsign, r.Kind, a.AltFt, r.KeepsFt, r.Why)
		w.s.cc.radio.Transmit(icao, tx)
		said := tx.Text
		w.mu.Lock()
		w.busy[r.Callsign] = now.Add(conflictLookAhead)
		if r.Kind == traffic.ResolveLevel {
			delete(w.stopped, r.Callsign) // a new level replaces the stop; another change keeps it to be cleared on
		}
		// A departure capped below the level departure cleared it to counts
		// as stopped: cleared on once clear (live, EZY516 "climb to flight
		// level 110, due traffic" after FL240, never cleared higher).
		capped := r.Kind == traffic.ResolveLevel && pos == traffic.PosDeparture && r.AltFt < radarFt
		if r.Kind == traffic.ResolveLevel && (r.Stop || r.Maintain || capped) {
			up := r.AltFt > a.AltFt
			if r.Maintain { // level now: the way its route was going (#697)
				up = routeClimbs(planned, r.AltFt)
			}
			on, ok := levelOn(planned, r.AltFt, up)
			if pos == traffic.PosDeparture && up {
				// On to the level departure cleared it to, not the top of
				// its climb waypoints (live, KLM704 "climb to flight level
				// 192" after "climb to flight level 240").
				on, ok = radarFt, true
			}
			if ok {
				w.stopped[r.Callsign] = stoppedLevel{icao: icao, pos: pos, altFt: on, climb: up, other: otherOf(c, r.Callsign), planned: planned}
			}
		}
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
	for p := range w.leveled { // by pair "cs/other"
		a, b, _ := strings.Cut(p, "/")
		if !pairs[a+"/"+b] && !pairs[b+"/"+a] {
			delete(w.leveled, p)
		}
	}
	w.now = cs
	for p := range w.seen {
		if !pairs[p] {
			delete(w.seen, p) // over: a new one is logged again
		}
	}
	for cs, until := range w.busy {
		// Kept through engagedAfter: engaged says "just out of a resolution"
		// from it (#76: deleted at expiry, the grace never applied).
		if now.After(until.Add(engagedAfter)) {
			delete(w.busy, cs)
		}
	}
	// Stopped and clear now: cleared on at any look, not only the one its
	// change ends on (#657 review: involved then, stopped for good); gone
	// from the sky, forgotten.
	airborne := map[string]bool{}
	at := map[string]airport.LatLon{}
	for _, a := range aircraft {
		if !a.OnGround && a.Tail != "" {
			airborne[a.Tail], at[a.Tail] = true, a.Position
		}
	}
	var cleared []string
	for cs, st := range w.stopped {
		// Past the traffic it was stopped for, moving apart and clear of
		// the terminal minimum: over, whatever the change was to last.
		// With a margin: cleared on at the minimum itself, its climb took
		// it straight into the traffic again (live, DLH1245 under OKOXX at
		// 3.4 NM: stopped, cleared, stopped, cleared within 50 s).
		apart, clear := false, true
		if p, ok := at[cs]; ok {
			if q, ok := at[st.other]; ok {
				d := calc.HaversineMeters(p.Lat, p.Lon, q.Lat, q.Lon) / 1852
				apart = st.lastNM > 0 && d > st.lastNM && d >= sepTerminalNM+resumeApartNM
				clear = d >= sepTerminalNM+resumeTimedNM
				st.lastNM = d
				w.stopped[cs] = st
			} else if st.other != "" {
				apart = true // the traffic is gone
			}
		}
		switch {
		case !airborne[cs]:
			delete(w.stopped, cs)
		case !involved[cs] && (apart || !now.Before(w.busy[cs]) && clear):
			cleared = append(cleared, cs)
		}
	}
	resume := map[string]stoppedLevel{}
	early := map[string]bool{}
	for _, cs := range cleared {
		resume[cs] = w.stopped[cs]
		early[cs] = now.Before(w.busy[cs])
		delete(w.stopped, cs)
		delete(w.busy, cs)
	}
	w.mu.Unlock()
	// Stopped for traffic and clear of it now: on to the level planned
	// (the route resumes it as the change ends).
	for cs, st := range resume {
		w.s.cc.log.printf("%-6s conflict over: %s to %.0f ft", cs, map[bool]string{true: "climb", false: "descend"}[st.climb], st.altFt)
		// A departure not identified yet (still with the tower, or its
		// check-in not answered): its identification clears the climb, the
		// stop being over; said now as well, it heard it twice (#698).
		if it := w.s.cc.byTail(cs); it != nil && it.dep != nil && st.climb {
			it.mu.Lock()
			identified := it.identified
			it.mu.Unlock()
			if !identified {
				w.s.cc.log.printf("%-6s conflict over: the climb comes with its identification", cs)
				continue
			}
		}
		// Before the change ends: its level still holds in the route, so the
		// route planned before it is flown again from here.
		if early[cs] && len(st.planned) > 0 {
			if err := w.restoreRoute(cs, st, aircraft); err != nil {
				w.s.cc.log.printf("%-6s conflict over: route not restored: %v", cs, err)
				continue
			}
		}
		ta := 0.0 // the airport's transition altitude: "flight level 100" at LKPR (#686)
		if g, err := w.s.st.cache.Graph(st.icao); err == nil {
			ta = w.s.cc.limitsOf(g).TransitionAltitudeFt
		}
		w.s.cc.radio.Transmit(st.icao, traffic.ContinueLevelAbove(st.pos, cs, st.altFt, st.climb, ta))
		if it := w.s.cc.byTail(cs); it != nil && it.dep != nil && st.climb && st.pos == traffic.PosDeparture {
			it.climbOn() // its route on to the cleared level, past its SID
		}
	}
	w.crewRequests(now, aircraft, opts) // after the look: a crew in a conflict is told "unable"
}

// isStopped reports cs told to stop its climb or descent for traffic and
// not yet cleared on.
func (w *conflictWatch) isStopped(cs string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.stopped[cs]
	return ok
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

// arrivalConflictSoon: a conflict closer than this is acted on even right
// after the sequence slowed the arrival.
const arrivalConflictSoon = 3 * time.Minute

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
	// Levelled for this pair: one for an earlier conflict does not count
	// (live, EZY131 levelled for KLM185, then only slowed for OKGOZ).
	busy, leveled := now.Before(w.busy[cs]), w.leveled[cs+"/"+oth]
	w.mu.Unlock()
	if _, _, holding := trailer.it.arr.Holding(); busy || holding {
		return // a change is flown already: see it work
	}
	recheck := func() {
		w.mu.Lock()
		w.busy[cs] = now.Add(arrivalConflictRecheck)
		w.mu.Unlock()
	}
	// Level first: above the other, still descending to it — only well out:
	// closer in both are bound for the same final and must descend to it, so
	// height parts them for a moment only; there the trailer is slowed and
	// vectored instead (live, TVS251 told to stop descent behind a PC-24 at
	// 108 kt on the same ILS, then sent around on the final).
	if !leveled && trailer.e.DistanceToGoNM > arrivalLevelFromNM {
		w.mu.Lock()
		w.leveled[cs+"/"+oth] = true
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
					w.s.cc.log.printf("%-6s conflict with %s: stop descent at %.0f ft (arrival on its STAR)", cs, oth, level)
					trailer.it.say(traffic.StopDescent(traffic.PosApproach, cs, level, oth, w.s.cc.taOf(trailer.it.ICAO)))
					recheck()
					return
				}
				w.s.cc.log.printf("%-6s conflict with %s: stop descent refused: %v", cs, oth, err)
			} else if below := math.Floor((them.AltFt-arrivalLevelAboveFt)/500) * 500; below >= arrivalDescendMinFt {
				// Level with it or below: down to 1000 ft under it, never a
				// climb for an arrival (live, EZY131 200 ft under OKGOZ,
				// only slowed, met at 0.4 NM).
				err := w.s.cc.do(func() error { return trailer.it.arr.DescendTo(below) })
				if err == nil {
					w.s.cc.log.printf("%-6s conflict with %s: descend to %.0f ft (arrival on its STAR)", cs, oth, below)
					trailer.it.say(traffic.Descend(traffic.PosApproach, cs, below, w.s.cc.taOf(trailer.it.ICAO)))
					recheck()
					return
				}
				w.s.cc.log.printf("%-6s conflict with %s: descent refused: %v", cs, oth, err)
			}
		}
	}
	// Just slowed by the sequence and the conflict still minutes off: let
	// that speed work first (live, ENT1816 told 220 kt, then 210 kt three
	// seconds later).
	q.mu.Lock()
	absorbed := q.absorbed[cs]
	q.mu.Unlock()
	if now.Sub(absorbed) < arrivalConflictRecheck && c.In > arrivalConflictSoon {
		recheck()
		return
	}
	q.mu.Lock()
	refused := q.refused[cs]
	q.mu.Unlock()
	if now.Sub(refused) < conflictRefusedWait {
		return // nothing approach can do for it yet (live: "slow refused" every 5 s)
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
		w.s.cc.log.printf("%-6s conflict with %s: %s refused: %v", cs, oth, action, err)
		q.mu.Lock()
		q.refused[cs] = now
		q.mu.Unlock()
		return
	}
	if action == "hold" {
		// Held until the conflict is over, not released by the sequence's
		// small delay a second later (live, CSA1257 at ERASU).
		q.mu.Lock()
		q.conflictHeld[cs] = conflictHold{other: oth, at: now}
		q.mu.Unlock()
	}
	w.s.cc.log.printf("%-6s conflict with %s: %s (arrival on its STAR)", cs, oth, map[string]string{"slow": "loses time", "hold": "holds"}[action])
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
	// arrivalLevelFromNM: the level is given only this far or more to go.
	arrivalLevelFromNM = 20.0
	// arrivalDescendMinFt: an arrival is sent down under another no lower
	// than this on its STAR.
	arrivalDescendMinFt = 4000.0
)

// other is the other aircraft of conflict c.
func other(c traffic.Conflict, cs string) string {
	if c.A == cs {
		return c.B
	}
	return c.A
}

// restoreRoute sends cs on the route it had before it was stopped, from
// where it is now.
func (w *conflictWatch) restoreRoute(cs string, st stoppedLevel, aircraft []traffic.TrackedAircraft) error {
	var a traffic.TrackedAircraft
	for _, x := range aircraft {
		if x.Tail == cs {
			a = x
		}
	}
	if a.ObjectID == 0 {
		return errors.New("not seen")
	}
	route := append([]traffic.RoutePoint{{Position: a.Position, AltFt: a.AltFt, Kts: a.GroundKts}}, traffic.ProfileAhead(a.Position, st.planned)...)
	if it := w.s.cc.byTail(cs); it != nil && it.dep != nil && it.object() == a.ObjectID {
		return w.s.cc.do(func() error { return it.dep.Reroute(route) })
	}
	w.s.mu.Lock()
	var e *enrouteAC
	for _, x := range w.s.enroute {
		if x.objectID == a.ObjectID {
			e = x
		}
	}
	w.s.mu.Unlock()
	if e == nil {
		return errors.New("not ours")
	}
	_, wps, err := traffic.EnrouteStart(route)
	if err != nil {
		return err
	}
	if err := w.s.cc.do(func() error { return w.s.cc.sim.SetRoute(e.objectID, wps) }); err != nil {
		return err
	}
	w.s.mu.Lock()
	e.route = route
	w.s.mu.Unlock()
	return nil
}

// engaged reports that cs has to do with a conflict (#785): predicted in
// one now, flying a resolution, slowed, held level or stopped for traffic,
// or resolved within engagedAfter. A sequencer shortcut is not for it:
// shortening its way undoes the resolution (live, TVS979 sent direct
// RATEV 30 s after it was slowed for THY319; they closed to 0.7 NM).
func (w *conflictWatch) engaged(cs string, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.now {
		if c.A == cs || c.B == cs {
			return true
		}
	}
	if until, ok := w.busy[cs]; ok && now.Before(until.Add(engagedAfter)) {
		return true
	}
	_, stopped := w.stopped[cs]
	for p := range w.leveled {
		if strings.HasPrefix(p, cs+"/") {
			return true
		}
	}
	return w.slowed[cs] || stopped
}

// engagedAfter: an aircraft counts as engaged this long after its
// resolution is flown.
const engagedAfter = 3 * time.Minute

// conflictRefusedWait: an arrival approach could do nothing for in a
// conflict is not asked again for this long.
const conflictRefusedWait = 2 * time.Minute

// forget drops what the watch remembers of a call sign, spawned again as a
// new flight (#75, #85).
func (w *conflictWatch) forget(tail string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.busy, tail)
	delete(w.slowed, tail)
	delete(w.asked, tail)
	for p := range w.leveled {
		if a, b, _ := strings.Cut(p, "/"); a == tail || b == tail {
			delete(w.leveled, p)
		}
	}
	delete(w.stopped, tail)
	for k := range w.informed {
		if strings.Contains(k, tail) {
			delete(w.informed, k) // by pair
		}
	}
}

// A climb or descent stopped for traffic resumes once moving apart and
// resumeApartNM beyond the terminal minimum, or, its change run out,
// resumeTimedNM beyond it (or the traffic gone): at the minimum itself it
// met the traffic again at once.
const (
	resumeApartNM = 2.0
	resumeTimedNM = 1.0
)
