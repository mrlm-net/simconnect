package world

import (
	"math/rand/v2"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The crew decides on its own (#621): our arrivals on final go around
// without being told when the landing clearance has not come by their
// decision point, and very rarely (crewUnstableShare) from an approach not
// stable: at 1 in 100, one with the runway free came too often (live,
// FTHAB; the user: rare).
// The tower acknowledges and the arrival is sequenced again, as after a
// go-around the tower orders. Not with held gates: there the user is the
// tower and gives the landing clearances.
const (
	// crewDecisionNM: not cleared to land this close to the threshold
	// (about 200 ft on a 3° path), the crew goes around.
	crewDecisionNM = 0.6
	// crewUnstableShare: the share of approaches the crew finds not stable
	// and goes around from, judged once per approach: rare.
	crewUnstableShare = 0.0005
)

// crewDecides lets the crews of icao's arrivals in list decide.
func (t *towers) crewDecides(icao string, list []traffic.RunwayUser, ours map[string]*controlled) {
	for _, u := range list {
		if !u.Arrival || u.Other || u.Phase != traffic.RunwayFinal || !u.Established || u.DistanceNM > crewDecisionNM {
			continue
		}
		it := ours[u.Callsign]
		if it == nil || it.arr == nil || it.gates.Load() {
			continue
		}
		cs := u.Callsign
		t.mu.Lock()
		cleared, sent, checked := t.given[cs+" land"], t.given[cs+" goaround"], t.given[cs+" crew"]
		t.given[cs+" crew"] = true // the stability is judged once an approach
		t.mu.Unlock()
		if sent {
			continue
		}
		why := ""
		switch {
		case !cleared:
			why = "no landing clearance"
		case !checked && rand.Float64() < crewUnstableShare:
			why = "approach not stable"
		}
		if why == "" {
			continue
		}
		t.mu.Lock()
		t.given[cs+" goaround"] = true
		t.mu.Unlock()
		t.cc.log.printf("%-6s crew: going around — %s", cs, why)
		it.say(traffic.GoingAround(cs))
		go func() {
			if err := t.cc.do(func() error { return it.act("goaround", 0) }); err != nil {
				t.cc.log.printf("%-6s crew go-around: %v", cs, err)
			}
		}()
		it.call(traffic.PosTower, prioUrgent, func() { it.say(it.goAroundAck()) })
	}
}

// crewRejectShare: the share of take-offs the crew rejects on its own (an
// engine warning, a bird), decided once on each roll between
// crewRejectFromKts and crewRejectToKts, below the speed it decides by
// (V1: past it the take-off goes on, AbortTakeoff says too late).
const (
	crewRejectShare   = 0.0005 // rare (the user)
	crewRejectFromKts = 40.0
	crewRejectToKts   = 100.0
)

// crewRejects lets the crews of our departures rolling at icao reject the
// take-off, rarely: they stop on the runway, vacate and taxi back to the
// holding point for a new clearance; the tower acknowledges (#621).
func (t *towers) crewRejects(ours map[string]*controlled) {
	for cs, it := range ours {
		if it.dep == nil || it.gates.Load() {
			continue
		}
		it.mu.Lock()
		state, ground, kts := it.view.State, it.view.OnGround, it.view.GroundSpeed
		it.mu.Unlock()
		if state != traffic.TaxiDeparting.String() || !ground || kts < crewRejectFromKts || kts > crewRejectToKts {
			continue
		}
		t.mu.Lock()
		judged := t.rtoJudged[cs]
		t.rtoJudged[cs] = true
		t.mu.Unlock()
		if judged || rand.Float64() >= crewRejectShare {
			continue
		}
		cs, it := cs, it
		go func() {
			if err := t.cc.do(func() error { return it.act("abort", 0) }); err != nil {
				t.cc.log.printf("%-6s crew reject: %v", cs, err) // past V1: it goes on
				return
			}
			t.cc.log.printf("%-6s crew: take-off rejected at %.0f kt", cs, kts)
			it.say(traffic.RejectingTakeoff(cs))
			it.call(traffic.PosTower, prioUrgent, func() { it.say(traffic.Acknowledge(traffic.PosTower, cs)) })
		}()
	}
}

// crewDirectShare: the share of our departures whose crew, with the
// departure radar, asks to fly direct to a fix further along its route;
// the fix is crewDirectMinNM to crewDirectMaxNM away, past the next one.
const (
	crewDirectShare = 0.3
	crewDirectMinNM = 8.0
	crewDirectMaxNM = 60.0
)

// crewRequests lets the crews of our departures with the departure radar
// ask for direct to a fix ahead, once a flight (#621). The radar clears it
// unless the aircraft is in a predicted conflict or flying a resolution;
// then it is "unable".
func (w *conflictWatch) crewRequests(now time.Time, aircraft []traffic.TrackedAircraft, opts traffic.ConflictOptions) {
	for _, a := range aircraft {
		if !a.Ours || a.OnGround {
			continue
		}
		it := w.s.cc.byTail(a.Tail)
		if it == nil || it.dep == nil || it.gates.Load() {
			continue
		}
		it.mu.Lock()
		radar, id, fixes := it.atc == traffic.PosDeparture, it.objectID, it.fixes
		it.mu.Unlock()
		if id != a.ObjectID || !radar || len(it.dep.ClimbPlan(a.Position)) == 0 {
			continue
		}
		w.mu.Lock()
		asked := w.asked[a.Tail]
		w.asked[a.Tail] = true
		w.mu.Unlock()
		if asked || rand.Float64() >= crewDirectShare {
			continue
		}
		ahead := fixesAhead(fixes, it.dep.ClimbRoute(a.Position))
		var fix *airFix
		for i := len(ahead) - 1; i >= 1; i-- { // the furthest, past the next
			// Worth asking for: it saves a real part of the way (#670).
			along, d := traffic.AlongTo(a.Position, it.dep.ClimbRoute(a.Position), ahead[i].LatLon)
			if d >= crewDirectMinNM && d <= crewDirectMaxNM && traffic.DirectWorthIt(along, d) {
				fix = &ahead[i]
				break
			}
		}
		if fix == nil {
			continue
		}
		cs, a, f := a.Tail, a, *fix
		w.s.cc.log.printf("%-6s crew: request direct %s", cs, f.Ident)
		it.say(traffic.RequestDirect(traffic.PosDeparture, cs, f.Ident))
		answer := func() (traffic.Transmission, bool) {
			w.mu.Lock()
			busy := now.Before(w.busy[cs])
			w.mu.Unlock()
			// The direct itself must stay clear of everyone, not only the
			// aircraft be out of conflict now (live, PHGVV cleared direct
			// DONAD, stopped at 4000 ft for TVS440 eleven seconds later).
			path := append([]airport.LatLon{f.LatLon}, traffic.RouteAhead(f.LatLon, it.dep.ClimbRoute(a.Position))...)
			if busy || w.inConflictAny(cs) || !traffic.PathClear(a, path, aircraft, opts) {
				w.s.cc.log.printf("%-6s direct %s: unable, traffic", cs, f.Ident)
				return traffic.UnableDirect(traffic.PosDeparture, cs), true
			}
			if err := w.s.cc.do(func() error { return it.dep.DirectTo(a.Position, a.AltFt, a.GroundKts, f.LatLon) }); err != nil {
				w.s.cc.log.printf("%-6s direct %s refused: %v", cs, f.Ident, err)
				return traffic.Transmission{}, false
			}
			return traffic.ClearedDirectTo(traffic.PosDeparture, cs, f.Ident), true
		}
		// Not identified yet: answered with "identified, climb", one call,
		// not two in a row (live, LOT924).
		it.mu.Lock()
		identified := it.identified
		if !identified {
			it.directAnswer = answer
		}
		it.mu.Unlock()
		if identified {
			it.call(traffic.PosDeparture, prioApproach, func() {
				if tx, ok := answer(); ok {
					it.say(tx)
				}
			})
		}
	}
}

// inConflictAny reports that cs is in any predicted conflict (the latest
// look).
func (w *conflictWatch) inConflictAny(cs string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.now {
		if c.A == cs || c.B == cs {
			return true
		}
	}
	return false
}

// crewIntersectionShare: the share of departures planned for the full
// length whose crew asks to take the runway from an intersection (#621),
// by wake category: light aircraft mostly do (they need little runway),
// heavies do not.
func crewIntersectionShare(model string) float64 {
	switch traffic.WakeFor(model).ICAO {
	case traffic.WakeLight:
		return 0.85
	case traffic.WakeHeavy, traffic.WakeSuper:
		return 0
	}
	return 0.15
}

// crewEntry is the intersection a departure's crew asks for with its taxi
// request: the first its class may take (firstEntry, #1030), "" when it
// does not ask (most), departs from an intersection already or the runway
// has none. it.mu is held.
func (it *controlled) crewEntry() string {
	if it.dep == nil || it.view.Entry != "" || rand.Float64() >= crewIntersectionShare(it.view.Model) {
		return ""
	}
	return firstEntry(it.graph, it.ICAO, it.view.Runway, it.view.Model, it.view.Position)
}

// grantEntry answers the crew's intersection request before the taxi
// clearance: the departure re-planned from it when the runway left is long
// enough for the type (ChangeEntry), the clearance then naming it; else
// the clearance is for the full length, the answer as given.
func (it *controlled) grantEntry() {
	it.mu.Lock()
	e := it.askedEntry
	it.askedEntry = ""
	it.mu.Unlock()
	if e == "" || it.dep == nil {
		return
	}
	if err := it.cc.do(func() error { return it.dep.ChangeEntry(e) }); err != nil {
		it.cc.log.printf("%-6s crew: intersection %s not given: %v", it.Tail, e, err)
		return
	}
	// The route re-planned to the intersection, drawn so (live, BAW1367
	// cleared at B, its route still to A on the map).
	it.mu.Lock()
	it.setRoute()
	it.view.Entry, it.entryRemaining = e, entryLeft(it.graph, it.view.Runway, e)
	it.mu.Unlock()
	it.cc.changed("control")
	it.cc.log.printf("%-6s crew: intersection %s given", it.Tail, e)
}
