package world

import (
	"net/http"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TCAS for our airborne traffic (#450): each of ours sees every aircraft
// around (ours, the user's, other traffic) as TCAS II does
// (traffic.Evaluate), gets traffic and resolution advisories, flies an RA
// after the crew's reaction time, reports it on the frequency ("TCAS RA")
// and, the RA over, "clear of conflict, returning to assigned altitude"
// (FAA JO 7110.65 2-1-28); meanwhile approach gives it no instruction.
// Between two of ours the RAs are coordinated: the second takes the
// complement of the first's sense.

const (
	// tcasRangeNM: intruders farther than this are not looked at.
	tcasRangeNM = 12.0
	// tcasClearAfter: an RA is clear of conflict once no RA has held this
	// long and the range opens.
	tcasClearAfter = 3 * time.Second
	// tcasRAFt: a climb or descend RA is flown this far from where it began.
	tcasRAFt = 1000.0
	// tcasRAForNM: an arrival's RA level holds this far along its STAR.
	tcasRAForNM = 8.0
	// tcasEvents: how many advisories GET /api/tcas keeps.
	tcasEvents = 200
)

// tcasState is one of ours' TCAS: its advisory and RA.
type tcasState struct {
	adv      traffic.Advisory
	intruder string
	intrID   uint32
	ra       traffic.RA
	since    time.Time // the RA began
	flown    bool      // the response is under way
	reported bool      // "TCAS RA" said
	noneAt   time.Time // no RA since
	altFt    float64   // where it was when the RA began
}

// TCASEvent is an advisory as it happened.
type TCASEvent struct {
	At       time.Time `json:"at"`
	Callsign string    `json:"callsign"`
	Intruder string    `json:"intruder"`
	Advisory string    `json:"advisory"` // TA, RA, clear
	Aural    string    `json:"aural,omitempty"`
	RangeNM  float64   `json:"rangeNM"`
	DZFt     float64   `json:"dzFt"`
}

// TCASView is one of ours' TCAS now, on its ControlView.
type TCASView struct {
	Advisory string `json:"advisory"` // TA, RA
	Intruder string `json:"intruder"`
	Aural    string `json:"aural,omitempty"`
	Sense    int    `json:"sense,omitempty"` // RA: +1 up, -1 down
}

type tcasWatch struct {
	s      *scheduler
	mu     sync.Mutex
	st     map[uint32]*tcasState
	events []TCASEvent
	ta, ra int
}

func newTCASWatch(s *scheduler) *tcasWatch {
	return &tcasWatch{s: s, st: map[uint32]*tcasState{}}
}

func tcasTrack(a traffic.TrackedAircraft) traffic.TCASTrack {
	return traffic.TCASTrack{ID: a.ObjectID, Callsign: a.Tail, Lat: a.Position.Lat, Lon: a.Position.Lon, AltFt: a.AltFt, AGLFt: a.AGLFt,
		GroundKts: a.GroundKts, TrackDeg: a.Heading, VSFpm: a.VSFpm, OnGround: a.OnGround}
}

// tick looks at every one of ours in the air against the traffic around.
func (w *tcasWatch) tick(now time.Time, air []traffic.TrackedAircraft) {
	seen := map[uint32]bool{}
	for _, a := range air {
		if !a.Ours || a.OnGround {
			continue
		}
		seen[a.ObjectID] = true
		own := tcasTrack(a)
		// The intruder that matters most: an RA before a TA, the nearest.
		adv, best, bestG := traffic.AdvisoryNone, traffic.TrackedAircraft{}, traffic.TCASGeometry{}
		var bestLv traffic.TCASLevel
		for _, b := range air {
			if b.ObjectID == a.ObjectID || b.OnGround || calc.HaversineNM(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon) > tcasRangeNM {
				continue
			}
			ad, lv, g := traffic.Evaluate(own, tcasTrack(b))
			if ad > adv || ad == adv && ad != traffic.AdvisoryNone && g.RangeNM < bestG.RangeNM {
				adv, best, bestG, bestLv = ad, b, g, lv
			}
		}
		w.update(now, a, adv, best, bestG, bestLv)
	}
	// Gone (landed, removed): forgotten, an RA ended with it (#77: it stayed
	// in "TCAS RA" and refused every instruction).
	var ended []uint32
	w.mu.Lock()
	for id, st := range w.st {
		if !seen[id] {
			if st.adv == traffic.AdvisoryRA {
				ended = append(ended, id)
			}
			delete(w.st, id)
		}
	}
	w.mu.Unlock()
	for _, id := range ended {
		w.s.cc.clearTCAS(id)
	}
}

func (w *tcasWatch) update(now time.Time, a traffic.TrackedAircraft, adv traffic.Advisory, b traffic.TrackedAircraft, g traffic.TCASGeometry, lv traffic.TCASLevel) {
	w.mu.Lock()
	st := w.st[a.ObjectID]
	if st == nil {
		st = &tcasState{}
		w.st[a.ObjectID] = st
	}
	log := w.s.cc.log
	switch {
	case adv == traffic.AdvisoryRA && st.adv != traffic.AdvisoryRA:
		// Coordinated with an intruder of ours already in an RA against it.
		forced := 0
		if o := w.st[b.ObjectID]; o != nil && o.adv == traffic.AdvisoryRA && o.intrID == a.ObjectID {
			forced = -o.ra.Sense
		}
		st.adv, st.intruder, st.intrID = adv, b.Tail, b.ObjectID
		st.ra = traffic.SelectRA(tcasTrack(a), tcasTrack(b), lv, forced)
		st.since, st.flown, st.reported, st.noneAt, st.altFt = now, false, false, time.Time{}, a.AltFt
		w.ra++
		w.event(TCASEvent{At: now, Callsign: a.Tail, Intruder: b.Tail, Advisory: "RA", Aural: st.ra.Aural(), RangeNM: g.RangeNM, DZFt: g.DZFt})
		log.printf("%-6s TCAS RA: %s (traffic %s, %.1f NM, %+.0f ft)", a.Tail, st.ra.Aural(), b.Tail, g.RangeNM, g.DZFt)
	case adv == traffic.AdvisoryTA && st.adv == traffic.AdvisoryNone:
		st.adv, st.intruder, st.intrID = adv, b.Tail, b.ObjectID
		w.ta++
		w.event(TCASEvent{At: now, Callsign: a.Tail, Intruder: b.Tail, Advisory: "TA", Aural: "Traffic, Traffic", RangeNM: g.RangeNM, DZFt: g.DZFt})
		log.printf("%-6s TCAS TA: traffic %s, %.1f NM, %+.0f ft", a.Tail, b.Tail, g.RangeNM, g.DZFt)
	case st.adv == traffic.AdvisoryRA && adv != traffic.AdvisoryRA:
		if st.noneAt.IsZero() {
			st.noneAt = now
		}
		if now.Sub(st.noneAt) < tcasClearAfter || g.RangeRateKts < 0 && adv != traffic.AdvisoryNone {
			break // not clear yet
		}
		w.event(TCASEvent{At: now, Callsign: a.Tail, Intruder: st.intruder, Advisory: "clear", Aural: "Clear of Conflict"})
		log.printf("%-6s TCAS: clear of conflict with %s", a.Tail, st.intruder)
		reported := st.reported
		*st = tcasState{adv: adv}
		w.mu.Unlock()
		if reported {
			w.report(a, false)
		}
		w.release(a)
		return
	case adv == traffic.AdvisoryNone && st.adv == traffic.AdvisoryTA:
		st.adv, st.intruder = adv, ""
	case adv == traffic.AdvisoryRA:
		st.noneAt = time.Time{}
	}
	// The crew flies the RA after its reaction time, and reports it.
	respond := st.adv == traffic.AdvisoryRA && !st.flown && now.Sub(st.since) >= time.Duration(traffic.TCASResponseDelaySec*float64(time.Second))
	ra, alt := st.ra, st.altFt
	if respond {
		st.flown = true
		st.reported = ra.Kind != traffic.RAMonitor // a preventive RA changes nothing: nothing to report
	}
	w.mu.Unlock()
	if respond && ra.Kind != traffic.RAMonitor {
		w.fly(a, ra, alt)
		w.report(a, true)
	}
}

func (w *tcasWatch) event(e TCASEvent) {
	w.events = append(w.events, e)
	if len(w.events) > tcasEvents {
		w.events = w.events[len(w.events)-tcasEvents:]
	}
}

// fly has a of ours fly the RA: a level to hold (level off), or one
// tcasRAFt up or down from where it began; on its injected final a climb
// RA is a go-around.
func (w *tcasWatch) fly(a traffic.TrackedAircraft, ra traffic.RA, altFt float64) {
	cc := w.s.cc
	target := a.AltFt
	if ra.Kind != traffic.RALevelOff {
		target = altFt + float64(ra.Sense)*tcasRAFt
	}
	var err error
	w.s.mu.Lock()
	e := w.s.enroute[a.Tail]
	w.s.mu.Unlock()
	it := cc.byTail(a.Tail)
	switch {
	case e != nil && e.objectID == a.ObjectID && len(e.route) > 1:
		r := traffic.Resolution{Callsign: a.Tail, ObjectID: a.ObjectID, Kind: traffic.ResolveLevel, AltFt: target}
		route := traffic.ResolvedRoute(e.route, a, r, conflictLookAhead)
		var wps []types.SIMCONNECT_DATA_WAYPOINT
		if _, wps, err = traffic.EnrouteStart(route); err == nil {
			err = cc.do(func() error { return cc.sim.SetRoute(e.objectID, wps) })
		}
	case it != nil && it.dep != nil && len(it.dep.ClimbPlan(a.Position)) > 0:
		r := traffic.Resolution{Callsign: a.Tail, ObjectID: a.ObjectID, Kind: traffic.ResolveLevel, AltFt: target}
		route := traffic.ResolvedRoute(it.dep.ClimbPlan(a.Position), a, r, conflictLookAhead)
		err = cc.do(func() error { return it.dep.Reroute(route) })
	case it != nil && it.arr != nil:
		it.tcasRA.Store(true)
		if ra.Sense > 0 || ra.Kind == traffic.RALevelOff {
			err = cc.do(func() error { return it.arr.StopDescent(target, tcasRAForNM) })
			if err != nil && ra.Sense > 0 {
				// On its injected final: a climb RA takes it around.
				err = cc.do(func() error { return it.act("goaround", 0) })
			}
		}
	}
	if it != nil {
		it.tcasRA.Store(true)
	}
	if err != nil {
		cc.log.printf("%-6s TCAS RA: not flown: %v", a.Tail, err)
	}
}

// release ends an RA: approach works the aircraft again; the route
// changed for it goes on as planned (an arrival's level hold runs out by
// itself).
func (w *tcasWatch) release(a traffic.TrackedAircraft) {
	cc := w.s.cc
	it := cc.byTail(a.Tail)
	if it == nil {
		return
	}
	it.tcasRA.Store(false)
	if it.dep != nil {
		if plan := it.dep.ClimbPlan(a.Position); len(plan) > 0 {
			_ = cc.do(func() error { return it.dep.Reroute(plan) })
		}
	}
}

// report has the crew say "TCAS RA" or, over, "clear of conflict,
// returning to (assigned clearance)" on the frequency it is on: the level
// or approach it was last cleared to (Doc 4444 12.3.1.2 r, t; CAP 413
// 5.32, 5.33), "assigned altitude" the FAA way. The controller answers
// "roger" (12.3.1.2 s, u).
func (w *tcasWatch) report(a traffic.TrackedAircraft, ra bool) {
	cc := w.s.cc
	it := cc.byTail(a.Tail)
	pos, icao := traffic.PosCenter, ""
	if it != nil {
		it.mu.Lock()
		pos, icao = it.atc, it.ICAO
		it.mu.Unlock()
	}
	station, _ := cc.stationOf(icao, pos)
	clearance := ""
	if traffic.PhraseologyFor(icao) != traffic.PhraseologyFAA {
		cc.assignedMu.Lock()
		clearance = cc.assigned[a.Tail]
		cc.assignedMu.Unlock()
	}
	tx := traffic.ClearOfConflictTo(pos, station, a.Tail, clearance)
	if ra {
		tx = traffic.TCASRAReport(pos, station, a.Tail)
	}
	if it == nil {
		cc.radio.Transmit(icao, tx)
		return
	}
	it.say(tx)
	p := cc.pending
	p.later(it.clearAt(pos).Add(atcAnswerDelay+p.jitter(atcAnswerJitter)), func() {
		it.say(traffic.Acknowledge(pos, a.Tail))
	})
}

// view is one of ours' TCAS for its ControlView; nil when quiet.
func (w *tcasWatch) view(objectID uint32) *TCASView {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.st[objectID]
	if st == nil || st.adv == traffic.AdvisoryNone {
		return nil
	}
	v := &TCASView{Advisory: "TA", Intruder: st.intruder}
	if st.adv == traffic.AdvisoryRA {
		v.Advisory, v.Aural, v.Sense = "RA", st.ra.Aural(), st.ra.Sense
	}
	return v
}

func registerTCAS(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/tcas", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		t := st.tcas
		st.mu.Unlock()
		if t == nil {
			http.Error(w, "not connected", http.StatusServiceUnavailable)
			return
		}
		t.mu.Lock()
		out := map[string]any{"ta": t.ta, "ra": t.ra, "events": append([]TCASEvent(nil), t.events...)}
		t.mu.Unlock()
		writeJSON(w, out)
	})
}

// clearTCAS ends the RA of the controlled aircraft that was objectID.
func (cc *controlCenter) clearTCAS(objectID uint32) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for _, it := range cc.items {
		if it.object() == objectID {
			it.tcasRA.Store(false)
		}
	}
}
