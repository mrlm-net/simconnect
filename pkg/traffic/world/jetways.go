package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/dict"
	"github.com/mrlm-net/simconnect/pkg/jetway"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Jetways for our traffic (#572): switched on or off (off by default),
// docked by a jetway.Service. With them off, or the service not available
// (no helper package), nothing is spawned and traffic parks and pushes as
// without them. On: our airports' jetway gates are prepared, an arrival
// parked on a gate (not a type with its own airstairs) and a departure
// spawned on one get the jetway, and ground holds a push until it is home.

var jetwaysOn struct {
	mu    sync.Mutex
	on    bool
	since time.Time // when switched on (jetwaysReady)
}

// SetJetways switches jetways for our traffic on or off. Off, every helper
// and placeholder is removed (the jetways go home).
func (w *World) SetJetways(on bool) {
	jetwaysOn.mu.Lock()
	was := jetwaysOn.on
	jetwaysOn.on = on
	if on && !was {
		jetwaysOn.since = traffic.Now()
	}
	jetwaysOn.mu.Unlock()
	if was == on {
		return
	}
	w.st.core.log.printf("jetways for our traffic %s", map[bool]string{true: "on", false: "off"}[on])
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc != nil && !on {
		cc.releaseJetways()
	}
}

func jetwaysEnabled() bool {
	jetwaysOn.mu.Lock()
	defer jetwaysOn.mu.Unlock()
	return jetwaysOn.on
}

// JetwayView is the jetways' state: switched on, whether the backend can
// work and why not, and each gate's jetway by airport and stand.
type JetwayView struct {
	On        bool                             `json:"on"`
	Available bool                             `json:"available"`
	Reason    string                           `json:"reason,omitempty"`
	Gates     map[string]map[int]jetway.Status `json:"gates,omitempty"`
}

// Jetways is the jetways' state now.
func (w *World) Jetways() JetwayView {
	v := JetwayView{On: jetwaysEnabled(), Reason: "not connected"}
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil {
		return v
	}
	svc := cc.jetwayService()
	if svc == nil {
		v.Reason = "jetway helper package missing"
		return v
	}
	v.Available, v.Reason = svc.Available()
	cc.jw.mu.Lock()
	defer cc.jw.mu.Unlock()
	v.Gates = map[string]map[int]jetway.Status{}
	for k := range cc.jw.docked {
		icao, stand := splitGateKey(k)
		if v.Gates[icao] == nil {
			v.Gates[icao] = map[int]jetway.Status{}
		}
		v.Gates[icao][stand] = svc.Status(icao, stand)
	}
	return v
}

// jetwayState is a connection's jetway backend and what it has asked of
// it.
type jetwayState struct {
	mu       sync.Mutex
	svc      *jetway.Helpers
	prepared map[string]bool
	docked   map[string]bool // "ICAO stand" asked to dock
	askedAt  map[string]time.Time
	// refused: why a gate's last dock was refused, logged once each reason
	// (a dock asked every second, refused, left no trace: live, LKPR B14).
	refused map[string]string
	// The gate's aircraft's own (reset when another takes the gate: own):
	// owner its entry (controlled.ID); tries its docks asked (a refused one
	// none), askedAt the last; refusedSince when its docks began to be
	// refused (zero: the last asked); said its rest logged. A turnaround's
	// departure is another entry: asked afresh (live, LKPR 2026-11-08: the
	// arrival's tries used up, the departure never asked).
	owner        map[string]int
	tries        map[string]int
	refusedSince map[string]time.Time
	said         map[string]bool
	// noJet: the gate's aircraft has no jetway coming (refused, out of
	// tries): stairs, GPU and buses instead (gateWithoutJetway); kept for
	// the same aircraft (object) when a turnaround's departure takes it.
	noJet map[string]uint32
	// placed: where the simulator parked each gate's placeholders and
	// helpers, the latest last.
	placed map[string][]parkedAt
	// fails: a gate's docks failed in a row, whatever the aircraft (reset
	// by a dock), failAt the last; lastStatus its jetway's status at the
	// last tick, for a dock going from docking to failed.
	fails      map[string]int
	failAt     map[string]time.Time
	lastStatus map[string]jetway.Status
}

// init makes the maps. cc.jw.mu held.
func (s *jetwayState) init() {
	if s.owner != nil {
		return
	}
	s.owner, s.tries, s.askedAt, s.refusedSince = map[string]int{}, map[string]int{}, map[string]time.Time{}, map[string]time.Time{}
	s.said, s.noJet, s.refused = map[string]bool{}, map[string]uint32{}, map[string]string{}
	if s.docked == nil {
		s.docked = map[string]bool{}
	}
}

// own makes aircraft id gate k's: another aircraft's asks, tries,
// refusals and no-jetway there forgotten (a turnaround's departure, made
// while its arrival still owned the gate, got its stairs then). cc.jw.mu
// held.
func (s *jetwayState) own(k string, id int) {
	s.init()
	if o, ok := s.owner[k]; ok && o == id {
		return
	}
	s.owner[k] = id
	delete(s.tries, k)
	delete(s.askedAt, k)
	delete(s.refused, k)
	delete(s.refusedSince, k)
	delete(s.said, k)
	delete(s.docked, k)
	delete(s.noJet, k)
}

// leave forgets gate k's aircraft, gone from it. cc.jw.mu held.
func (s *jetwayState) leave(k string) {
	s.init()
	for _, m := range []map[string]time.Time{s.askedAt, s.refusedSince} {
		delete(m, k)
	}
	delete(s.owner, k)
	delete(s.tries, k)
	delete(s.refused, k)
	delete(s.said, k)
	delete(s.noJet, k)
}

// A gate whose dock failed JetwayGateFails times in a row, for any
// aircraft, rests JetwayGateRest: its helper went to another gate each
// time, taking a docked gate's jetway (live, LKPR B15's helpers given A1,
// B9, C19, C23 … again and again; the sim counts docked gates free).
const (
	JetwayGateFails = 3
	JetwayGateRest  = time.Hour
)

// JetwayRefusedMax: a dock refused this long (the airport refilling,
// other gates docking) leaves the aircraft without a jetway: stairs, GPU
// and buses instead.
const JetwayRefusedMax = time.Minute

// gateResting reports whether gate k rests after failed docks, counting a
// dock that went from docking (the last tick) to failed (now) once; a
// refused dock, which never docked, never counts (live, LKPR 2026-11-08:
// three refused seconds counted as three failed docks). cc.jw.mu held.
func (s *jetwayState) gateResting(k string, status jetway.Status, now time.Time) bool {
	if s.fails == nil {
		s.fails, s.failAt, s.lastStatus = map[string]int{}, map[string]time.Time{}, map[string]jetway.Status{}
	}
	prev := s.lastStatus[k]
	s.lastStatus[k] = status
	switch status {
	case jetway.StatusDocked:
		delete(s.fails, k)
	case jetway.StatusFailed:
		if prev == jetway.StatusDocking {
			s.failedDock(k, now)
		}
	}
	if s.fails[k] < JetwayGateFails {
		return false
	}
	if now.Sub(s.failAt[k]) >= JetwayGateRest {
		delete(s.fails, k) // rested: tried again
		return false
	}
	return true
}

// failedDock counts a failed dock at gate k. cc.jw.mu held.
func (s *jetwayState) failedDock(k string, now time.Time) {
	if s.fails == nil {
		s.fails, s.failAt, s.lastStatus = map[string]int{}, map[string]time.Time{}, map[string]jetway.Status{}
	}
	s.fails[k]++
	s.failAt[k] = now
}

// restingNow reports whether gate k rests now (gateResting), changing
// nothing. cc.jw.mu held.
func (s *jetwayState) restingNow(k string, now time.Time) bool {
	return s.fails[k] >= JetwayGateFails && now.Sub(s.failAt[k]) < JetwayGateRest
}

// A gate whose jetway never came for an aircraft type (its dock timed out,
// or the gate rested after failed docks with one there) has no jetway for
// that type: stairs, GPU and buses as on a remote stand, kept over restarts
// (jetway-gates.json) until one of the type docks there. It is tried again
// once JetwayTypeRetest after the last failure (live, LKPR C21 2026-11-08:
// an A321, a BCS3 and an A20N each waited for a jetway that never came).
const JetwayTypeRetest = 6 * time.Hour

// JetwayTypeTimeouts: timed-out docks of one type at a gate, each within
// JetwayTypeRetest of the last, before the gate has no jetway for that
// type. One taught it before: a sim session whose jetways all failed for
// a while left seven gates without a jetway for the B738, the A320 and
// the BCS3 for six hours, over restarts (live, LKPR 2026-11-10 16:26 to
// 17:03; at 19:49 TVS682 got stairs at C22).
const JetwayTypeTimeouts = 2

// jetwayType is the type a gate's jetway lessons are kept by: model's
// ICAO type when known, else model as given.
func jetwayType(model string) string {
	m := strings.ToUpper(strings.TrimSpace(model))
	if t := strings.ToUpper(traffic.ProfileFor(m).Type); t != "" && strings.Contains(m, t) {
		return t
	}
	return m
}

// dockResult hears how a dock for model at icao's stand ended
// (jetway.HelperOptions.OnDock): its jetway never came, the gate has none
// for that type from now on; docked, it has again. Under the backend's
// lock: it takes none of ours but the lessons'.
func (cc *controlCenter) dockResult(icao string, stand int, model string, docked bool) {
	typ := jetwayType(model)
	if typ == "" {
		return
	}
	counts := gateLessons.setNoJetway(icao, stand, typ, traffic.Now(), !docked)
	if !counts && !docked && cc.core != nil {
		cc.core.log.printf("jetway %s stand %d: no jetway came for %s (one of %d before none for that type here)", icao, stand, typ, JetwayTypeTimeouts)
	}
	if counts && cc.core != nil {
		if docked {
			cc.core.log.printf("jetway %s stand %d: docked for %s again: its jetway counts for that type", icao, stand, typ)
		} else {
			cc.core.log.printf("jetway %s stand %d: no jetway came for %s: none for that type here (stairs and buses), tried again in %s", icao, stand, typ, JetwayTypeRetest)
		}
	}
}

// gateWithoutJetway reports that icao's stand has no jetway for model now:
// learned so for its type (dockResult), the gate resting after failed
// docks, or none coming for the aircraft on it (noJet: its docks refused
// or out of tries; a turnaround's departure is made with stairs).
func (cc *controlCenter) gateWithoutJetway(icao string, stand int, model string) bool {
	now := traffic.Now()
	if gateLessons.noJetway(icao, stand, jetwayType(model), now) {
		return true
	}
	k := gateKey(icao, stand)
	cc.jw.mu.Lock()
	defer cc.jw.mu.Unlock()
	if _, ok := cc.jw.noJet[k]; ok {
		return true
	}
	return cc.jw.restingNow(k, now)
}

// markNoJetway notes that it has no jetway coming at its gate (noJet):
// stairs instead. Reports whether it is new. cc.jw.mu held.
func (s *jetwayState) markNoJetway(k string, obj uint32) bool {
	s.init()
	if _, ok := s.noJet[k]; ok {
		return false
	}
	s.noJet[k] = obj
	return true
}

// jetwayAt reports that model at g's stand boards by a jetway: a gate,
// unless its jetway does not come for model (gateWithoutJetway).
func (cc *controlCenter) jetwayAt(g *airport.Graph, stand int, model string) bool {
	if g == nil || stand < 0 || stand >= len(g.Layout.Parking) || !g.Layout.Parking[stand].IsGate() {
		return false
	}
	return !cc.gateWithoutJetway(g.Layout.ICAO, stand, model)
}

// standServicesAdder is a departure controller that takes stand services
// after its start (traffic.TaxiController).
type standServicesAdder interface {
	AddStandServices(stairs, gpu traffic.FuelService, buses []traffic.FuelService) bool
}

// lateStandServices gives a departure waiting on a gate whose jetway does
// not come the stairs, GPU and buses of a remote stand, once; none to be
// had yet, asked again at the next tick (queued, not given up). An
// arrival (no stand services of its own) is told once: its turnaround's
// departure is made with them (gateWithoutJetway).
func (cc *controlCenter) lateStandServices(it *controlled) {
	it.mu.Lock()
	if it.standLate || it.jetwayGone {
		it.mu.Unlock()
		return
	}
	dep, mk, tail := it.dep, it.standServices, it.Tail
	label := fmt.Sprint(it.stand)
	if g := it.graph; g != nil && it.stand >= 0 && it.stand < len(g.Layout.Parking) {
		label = g.Layout.Parking[it.stand].Label()
	}
	icao := it.ICAO
	if dep == nil || mk == nil {
		it.standLate = true // an arrival, or a departure with its stairs from the start
		arrival := dep == nil && it.arr != nil
		it.mu.Unlock()
		if arrival && cc.core != nil {
			cc.core.log.printf("jetway %s %s: no jetway for %s: stairs and buses for its turnaround", icao, label, tail)
		}
		return
	}
	it.mu.Unlock()
	a, ok := dep.(standServicesAdder)
	if !ok {
		it.mu.Lock()
		it.standLate = true // on a remote actuator: not given late
		it.mu.Unlock()
		return
	}
	stairs, gpu, buses := mk()
	if stairs == nil {
		it.mu.Lock()
		first := !it.stairsAsked
		it.stairsAsked = true
		it.mu.Unlock()
		if first {
			cc.core.log.printf("jetway %s %s: no jetway for %s, and no stairs yet: waiting for a set", icao, label, tail)
		}
		return
	}
	given := a.AddStandServices(stairs, gpu, buses)
	it.mu.Lock()
	it.standLate = true
	it.mu.Unlock()
	if given {
		cc.core.log.printf("jetway %s %s: no jetway for %s: stairs and %d buses instead", icao, label, tail, len(buses))
	}
}

func gateKey(icao string, stand int) string { return fmt.Sprintf("%s %d", icao, stand) }

func splitGateKey(k string) (string, int) {
	var icao string
	var stand int
	fmt.Sscanf(k, "%s %d", &icao, &stand)
	return icao, stand
}

// helperTypes are the types of the helper models the simulator lists.
func (cc *controlCenter) helperTypes() []string {
	var out []string
	for _, t := range cc.modelList() {
		if !jetway.IsHelper(t) || strings.HasSuffix(t, " ALL") || jetway.IsHelperVariant(t) {
			continue
		}
		out = append(out, strings.TrimPrefix(t, jetway.HelperTitlePrefix))
	}
	return out
}

// jetwayBuilt is the connection's helper backend if made, nil else (no
// model list scan: every message asks).
func (cc *controlCenter) jetwayBuilt() *jetway.Helpers {
	cc.jw.mu.Lock()
	defer cc.jw.mu.Unlock()
	return cc.jw.svc
}

// jetwayService is the connection's helper backend, made once the helper
// models are listed; nil before, or without them.
func (cc *controlCenter) jetwayService() *jetway.Helpers {
	cc.jw.mu.Lock()
	defer cc.jw.mu.Unlock()
	if cc.jw.svc != nil {
		return cc.jw.svc
	}
	types := cc.helperTypes()
	if len(types) == 0 {
		return nil
	}
	cc.jw.svc = jetway.NewHelpers(cc.client, jetway.HelperOptions{Types: types, RetractPlan: cc.retractPlan,
		RequestBase: cc.core.libIDs().puppetReq + 1000, Log: cc.core.log.printf, AircraftAt: cc.aircraftAt, Offset: jetwayDoorOffset,
		OnPlaceholder: cc.placeholderParked, OnLearn: func(icao string, stand int, helper string, docked bool) {
			go gateLessons.add(gateLesson{ICAO: icao, Stand: stand, Helper: helper, Docked: docked})
		}, OnGateResult: func(icao string, stand int, docked bool) {
			go gateLessons.count(icao, stand, docked)
		}, OnDock: cc.dockResult})
	gateLessons.teach(cc.jw.svc)
	cc.jw.prepared, cc.jw.docked = map[string]bool{}, map[string]bool{}
	return cc.jw.svc
}

// retractPlans: each airport's retract plan file once written. The jetway
// backend asks for it under its own lock, so after the first time (made in
// jetwayTick, outside any lock) it is read from here, never from the
// state (st.mu), which other locks are taken under.
var retractPlans sync.Map // ICAO → path

// retractPlan is a flight plan file out of icao to the nearest other
// airport known, for the placeholders (written once per airport).
func (cc *controlCenter) retractPlan(icao string) (string, error) {
	if p, ok := retractPlans.Load(icao); ok {
		return p.(string), nil
	}
	p, err := cc.writeRetractPlan(icao)
	if err == nil {
		retractPlans.Store(icao, p)
	}
	return p, err
}

func (cc *controlCenter) writeRetractPlan(icao string) (string, error) {
	if cc.airportRefs == nil {
		return "", fmt.Errorf("no airports known")
	}
	refs := cc.airportRefs()
	from, ok := refs[icao]
	best, bestD := traffic.AirportRef{}, math.Inf(1)
	for code, r := range refs {
		if code == icao || len(code) != 4 {
			continue
		}
		if d := calc.HaversineNM(from.Position.Lat, from.Position.Lon, r.Position.Lat, r.Position.Lon); d >= 20 && d < bestD {
			best, bestD = r, d
		}
	}
	dir := cc.core.hooks.DataDir
	if !ok || best.ICAO == "" {
		return "", fmt.Errorf("no airport known near %s", icao)
	}
	pt := func(r traffic.AirportRef) string { return nav.FormatLLA(r.Position, r.AltM*3.28084) }
	pln := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SimBase.Document Type="AceXML" version="1,1">
    <Descr>AceXML Document</Descr>
    <FlightPlan.FlightPlan>
        <Title>%[1]s to %[2]s</Title>
        <FPType>IFR</FPType>
        <RouteType>HighAlt</RouteType>
        <CruisingAlt>15000</CruisingAlt>
        <DepartureID>%[1]s</DepartureID>
        <DepartureLLA>%[3]s</DepartureLLA>
        <DestinationID>%[2]s</DestinationID>
        <DestinationLLA>%[4]s</DestinationLLA>
        <ATCWaypoint id="%[1]s">
            <ATCWaypointType>Airport</ATCWaypointType>
            <WorldPosition>%[3]s</WorldPosition>
            <ICAO><ICAOIdent>%[1]s</ICAOIdent></ICAO>
        </ATCWaypoint>
        <ATCWaypoint id="%[2]s">
            <ATCWaypointType>Airport</ATCWaypointType>
            <WorldPosition>%[4]s</WorldPosition>
            <ICAO><ICAOIdent>%[2]s</ICAOIdent></ICAO>
        </ATCWaypoint>
    </FlightPlan.FlightPlan>
</SimBase.Document>
`, icao, best.ICAO, pt(from), pt(best))
	if dir == "" {
		dir = "."
	}
	path, err := filepath.Abs(filepath.Join(dir, "jetways", icao+"-retract.pln"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(pln), 0o644)
}

// jetwayTick runs every second: our airports prepared, gates docked for
// parked aircraft, the backend's timers.
func (cc *controlCenter) jetwayTick(now time.Time) {
	if !jetwaysEnabled() {
		return
	}
	svc := cc.jetwayService()
	if svc == nil {
		return
	}
	if ok, _ := svc.Available(); !ok {
		return
	}
	cc.mu.Lock()
	airports := make([]string, 0, len(cc.stands))
	for icao := range cc.stands {
		airports = append(airports, icao)
	}
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, icao := range airports {
		cc.retractPlan(icao) // made here, outside the backend's lock
	}
	cc.jw.mu.Lock()
	for _, icao := range airports {
		if !cc.jw.prepared[icao] {
			cc.jw.prepared[icao] = true
			if err := svc.Prepare(icao); err != nil {
				cc.core.log.printf("jetways %s: %v", icao, err)
			}
		}
	}
	cc.jw.mu.Unlock()
	parked := map[string]bool{}
	var late []*controlled // no jetway coming: stand services instead
	for _, it := range items {
		it.mu.Lock()
		icao, stand, model, gone := it.ICAO, it.stand, it.view.Model, it.jetwayGone
		pos, tail, g, id, obj := it.view.Position, it.Tail, it.graph, it.ID, it.objectID
		// A departure made with stairs at a gate (no jetway for it then:
		// its arrival's, a lesson, a rest) asks for none.
		stairs := it.dep != nil && it.standServices == nil
		it.mu.Unlock()
		if stand < 0 || gone || !cc.jetwayParked(it) || traffic.NoJetway(model) || g == nil || stand >= len(g.Layout.Parking) || !svc.HasJetway(icao, stand) {
			continue
		}
		k := gateKey(icao, stand)
		parked[k] = true
		label := g.Layout.Parking[stand].Label()
		noJetType := gateLessons.noJetway(icao, stand, jetwayType(model), now)
		status := svc.Status(icao, stand)
		cc.jw.mu.Lock()
		cc.jw.own(k, id)
		resting := cc.jw.gateResting(k, status, now)
		if status == jetway.StatusDocked {
			delete(cc.jw.tries, k) // docked: a jetway taken later is asked afresh
		}
		_, noJet := cc.jw.noJet[k]
		if (resting || noJetType || noJet || stairs) && status != jetway.StatusDocking && status != jetway.StatusDocked {
			// No jetway for it: stairs instead (lateStandServices, asked each
			// tick until given). A rest is the gate's, never saved as the
			// type's (live, LKPR 2026-11-08: ten false lessons).
			if resting && !cc.jw.said[k] {
				cc.jw.said[k] = true
				cc.core.log.printf("jetway %s %s: resting after %d failed docks, no jetway for %s", icao, label, JetwayGateFails, tail)
			}
			cc.jw.docked[k] = true
			cc.jw.mu.Unlock()
			late = append(late, it)
			continue
		}
		// A dock not done (failed, its jetway taken, the gate refilled:
		// whatever the status) is asked again a minute after, up to
		// JetwayDockTries times for the aircraft there: some gates the
		// simulator gives no helper of its type (live, LKPR B14: a 787's
		// helper put 308 m away, again and again); then none for it.
		asked := cc.jw.docked[k]
		home := status != jetway.StatusDocking && status != jetway.StatusDocked && status != jetway.StatusUndocking
		if asked && home && now.Sub(cc.jw.askedAt[k]) > time.Minute {
			if cc.jw.tries[k] < JetwayDockTries {
				asked = false
			} else {
				if cc.jw.markNoJetway(k, obj) {
					cc.core.log.printf("jetway %s %s: no jetway came for %s after %d docks: stairs instead", icao, label, tail, JetwayDockTries)
				}
				cc.jw.mu.Unlock()
				late = append(late, it)
				continue
			}
		}
		if asked || status == jetway.StatusUndocking {
			cc.jw.mu.Unlock() // asked, or the last aircraft's jetway still going home
			continue
		}
		cc.jw.mu.Unlock()
		variant, ok := cc.gateVariant(g, stand, model)
		if !ok {
			variant = gateHelper(svc, g, stand, model, cc.hasModel) // the stand's own helper type, if it has one
		}
		var err error
		if q, blocked := wideHelperBlocked(svc, g, stand, model, svc.TitleFor(icao, stand, model, variant)); blocked {
			err = fmt.Errorf("its helper %s is wider than the aircraft's and fits %s, docked: the sim would give it that gate", svc.TitleFor(icao, stand, model, variant), g.Layout.Parking[q].Label())
		} else {
			err = svc.DockAs(icao, stand, model, variant)
		}
		cc.jw.mu.Lock()
		if err == nil {
			// Asked: counted, timed. A refused one is neither (live, LKPR
			// 2026-11-08: each refused second written as a new ask).
			cc.jw.docked[k], cc.jw.askedAt[k] = true, now
			cc.jw.tries[k]++
			delete(cc.jw.refused, k)
			delete(cc.jw.refusedSince, k)
			cc.jw.mu.Unlock()
			cc.logStandFit(g, stand, tail, pos) // once a dock is asked, not each tick a gate has none
			continue
		}
		said := cc.jw.refused[k] == err.Error()
		cc.jw.refused[k] = err.Error()
		if cc.jw.refusedSince[k].IsZero() {
			cc.jw.refusedSince[k] = now
		}
		refused := now.Sub(cc.jw.refusedSince[k])
		none := (errors.Is(err, jetway.ErrNoGate) || refused >= JetwayRefusedMax) && cc.jw.markNoJetway(k, obj)
		cc.jw.mu.Unlock()
		if !said {
			cc.core.log.printf("jetway %s %s: no dock for %s yet: %v", icao, label, tail, err)
		}
		if none {
			cc.core.log.printf("jetway %s %s: no jetway for %s (refused %s): stairs instead", icao, label, tail, refused.Round(time.Second))
			late = append(late, it)
		}
	}
	// No jetway coming: stairs and buses instead.
	for _, it := range late {
		cc.lateStandServices(it)
	}
	// A gate whose aircraft has gone (pushed, removed): its jetway home,
	// the aircraft's asks forgotten.
	cc.jw.mu.Lock()
	var away []string
	for k := range cc.jw.docked {
		if !parked[k] {
			away = append(away, k)
		}
	}
	for k := range cc.jw.owner {
		if !parked[k] {
			cc.jw.leave(k)
		}
	}
	cc.jw.mu.Unlock()
	for _, k := range away {
		icao, stand := splitGateKey(k)
		switch svc.Status(icao, stand) {
		case jetway.StatusDocking, jetway.StatusDocked:
			svc.Undock(icao, stand)
		case jetway.StatusUndocking:
		default:
			cc.jw.mu.Lock()
			delete(cc.jw.docked, k)
			cc.jw.mu.Unlock()
		}
	}
	svc.Update(now)
}

// parkedAt is where the simulator parked an aircraft (title) on a gate.
type parkedAt struct {
	title         string
	lat, lon, hdg float64
}

// placeholderParked keeps where the simulator parked a placeholder or
// helper on icao's stand (jetway.HelperOptions.OnPlaceholder), the last
// few: one put on another gate is no measure of this one (gatePlace).
func (cc *controlCenter) placeholderParked(icao string, stand int, title string, lat, lon, hdg float64) {
	cc.jw.mu.Lock()
	if cc.jw.placed == nil {
		cc.jw.placed = map[string][]parkedAt{}
	}
	k := gateKey(icao, stand)
	l := append(cc.jw.placed[k], parkedAt{title, lat, lon, hdg})
	if len(l) > 4 {
		l = l[len(l)-4:]
	}
	cc.jw.placed[k] = l
	cc.jw.mu.Unlock()
}

// gatePlace is how the simulator parks on g's stand: its nose gear inset
// meters inside the parking circle, side meters right of the parking line.
// Measured on the gate's placeholder (live, LKPR: inset 5.2 to 6.1 m, side
// up to ±0.55 m, each the gate's own), else SimNoseGearInsetM on the line.
func (cc *controlCenter) gatePlace(g *airport.Graph, stand int) (inset, side float64) {
	p := g.Layout.Parking[stand]
	cc.jw.mu.Lock()
	l := slices.Clone(cc.jw.placed[gateKey(g.Layout.ICAO, stand)])
	cc.jw.mu.Unlock()
	h := p.Heading * math.Pi / 180
	for i := len(l) - 1; i >= 0; i-- { // the latest that is this gate's
		at := l[i]
		n, ok := noseGearM(at.title)
		if !ok {
			continue
		}
		north := (at.lat - p.Position.Lat) * 111_320
		east := (at.lon - p.Position.Lon) * 111_320 * math.Cos(p.Position.Lat*math.Pi/180)
		ahead := north*math.Cos(h) + east*math.Sin(h)
		right := east*math.Cos(h) - north*math.Sin(h)
		if in := p.Radius - ahead - n; in >= 0 && in <= 15 && math.Abs(right) <= 3 {
			return in, right
		}
	}
	return SimNoseGearInsetM, 0
}

// aircraftAt is our aircraft on icao's stand (its object), 0 none.
func (cc *controlCenter) aircraftAt(icao string, stand int) uint32 {
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		at, obj := it.ICAO == icao && it.stand == stand && !it.left, it.objectID // not one gone from it
		it.mu.Unlock()
		if at && obj != 0 {
			return obj
		}
	}
	return 0
}

// doorOffsetStore keeps the local jetway door offsets
// (DataDir/jetway-door-offsets.json, over the library's: what is there
// wins, per type), set with POST /api/jetways/offset while calibrating.
type doorOffsetStore struct {
	mu    sync.Mutex
	file  string
	items []jetway.DoorOffset
}

func loadDoorOffsets(file string) *doorOffsetStore {
	s := &doorOffsetStore{file: file}
	if b, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(b, &s.items); err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		}
	}
	if len(s.items) > 0 {
		if err := s.apply(); err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		}
	}
	return s
}

func (s *doorOffsetStore) apply() error {
	b, err := json.Marshal(s.items)
	if err != nil {
		return err
	}
	return dict.Use("jetway.doorOffsets", b)
}

// set adds or replaces type o.Type's offset, puts it in use and saves.
func (s *doorOffsetStore) set(o jetway.DoorOffset) error {
	o.Type = strings.ToUpper(strings.TrimSpace(o.Type))
	if o.Type == "" {
		return fmt.Errorf("a door offset needs a type")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = slices.DeleteFunc(s.items, func(i jetway.DoorOffset) bool { return strings.EqualFold(i.Type, o.Type) })
	s.items = append(s.items, o)
	if err := s.apply(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	if s.file == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.file, b, 0o644)
}

// jetwayGates are icao's gates whose jetway can dock to our traffic now,
// for the stand allocator to take first; none with jetways off.
func (cc *controlCenter) jetwayGates(icao string) []int {
	if !jetwaysEnabled() {
		return nil
	}
	svc := cc.jetwayBuilt()
	if svc == nil {
		return nil
	}
	return svc.Dockable(icao)
}

// jetwayStands are icao's stands with a jetway, from the simulator's
// jetway list (the stand allocator's Bridged); none before it is read.
func (cc *controlCenter) jetwayStands(icao string) []int {
	svc := cc.jetwayBuilt()
	if svc == nil {
		return nil
	}
	return svc.Jetways(icao)
}

// jetwayParked reports that it stands still on its gate for a jetway: an
// arrival parked, a departure waiting for its push.
func (cc *controlCenter) jetwayParked(it *controlled) bool {
	switch {
	case it.arr != nil:
		return it.arr.State() == traffic.ArrivalParked
	case it.dep != nil:
		return it.dep.State() == traffic.TaxiAwaitingPushback
	}
	return false
}

// JetwayDockTries: how many times a gate's dock is asked for the same
// aircraft model before it is left without a jetway.
const JetwayDockTries = 2

// JetwayBoarding is how long a departure's jetway stays on before its crew
// asks for the push.
const JetwayBoarding = 3 * time.Minute

// StairsWaitMax: the longest a departure whose jetway never came waits for
// the stairs it is given instead (none to be had: it goes without).
const StairsWaitMax = 3 * time.Minute

// boardingLeft is how long it still boards through its jetway before
// asking for the push: 0 none (no jetway, or long enough). While the
// jetway is still on its way, 15 s at a time; a jetway that never came,
// until its stairs are given (lateStandServices), not with nothing at the
// door (live, LKPR 2026-11-08). it.mu held.
func (it *controlled) boardingLeft() time.Duration {
	svc := it.cc.jetwayBuilt()
	if svc == nil || !jetwaysEnabled() || it.jetwayGone || it.stand < 0 || traffic.NoJetway(it.view.Model) || !svc.HasJetway(it.ICAO, it.stand) {
		return 0
	}
	now := it.cc.clock.Now()
	stairsDue := func() time.Duration {
		if it.standServices == nil || it.standLate {
			return 0 // stairs from the start, or given: the controller boards
		}
		if it.boardWaitSince.IsZero() {
			it.boardWaitSince = now
		}
		if now.Sub(it.boardWaitSince) >= JetwayWaitMax+StairsWaitMax {
			return 0
		}
		return 15 * time.Second
	}
	if it.cc.gateWithoutJetway(it.ICAO, it.stand, it.view.Model) {
		return stairsDue()
	}
	switch svc.Status(it.ICAO, it.stand) {
	case jetway.StatusDocked:
		if left := JetwayBoarding - now.Sub(svc.Since(it.ICAO, it.stand)); left > 0 {
			return left
		}
		return 0
	case jetway.StatusUndocking:
		return 0 // going home for the push already
	}
	// Its jetway still to come (docking, asked again, docks going one at a
	// time): boarding has not begun, so no push call yet (live, LKPR:
	// departures due at once pushed before theirs docked). Not for ever: a
	// jetway that never comes leaves it stairs after JetwayWaitMax (live,
	// LKPR 2026-11-08: pushed with nothing at the door).
	if it.boardWaitSince.IsZero() {
		it.boardWaitSince = now
	}
	if now.Sub(it.boardWaitSince) < JetwayWaitMax {
		return 15 * time.Second
	}
	if it.standServices == nil || it.standLate {
		return 0
	}
	cc, k := it.cc, gateKey(it.ICAO, it.stand)
	cc.jw.mu.Lock()
	fresh := cc.jw.markNoJetway(k, it.objectID)
	cc.jw.mu.Unlock()
	if fresh {
		cc.core.log.printf("jetway %s: none came for %s in %s: stairs instead", it.ICAO, it.Tail, JetwayWaitMax)
	}
	return stairsDue()
}

// tugWaitsForJetway is TaxiRequest.TugWait: for about half the
// departures (TugAfterJetwayShare, drawn at the spawn) the tug sets off
// only once the jetway is home; the others' tug hooks up during boarding.
// It runs inside the controller's frame, under its lock, so it takes no
// lock of the aircraft (it.mu): the aircraft's update holds it.mu and
// calls the controller, and the two waited for each other for good (live,
// LKPR 01:05: the whole map stood still).
func (it *controlled) tugWaitsForJetway() bool {
	if !it.tugAfter.Load() {
		return false
	}
	icao, stand := it.ICAO, int(it.tugStand.Load())
	svc := it.cc.jetwayBuilt()
	after := true
	if !after || svc == nil || stand < 0 {
		return false
	}
	switch svc.Status(icao, stand) {
	case jetway.StatusDocking, jetway.StatusDocked, jetway.StatusUndocking:
		return true
	}
	return false
}

// TugAfterJetwayShare is the share of departures at a jetway whose tug
// comes only once the jetway is home.
const TugAfterJetwayShare = 0.5

// JetwayWaitMax: the longest a departure waits for its jetway to dock
// before it boards without it.
const JetwayWaitMax = 3 * time.Minute

// jetwayHolds undocks its jetway before its push: true while ground must
// wait (the jetway still on its way home). Called with it.mu held; from
// then on the jetway stays home (jetwayGone).
func (it *controlled) jetwayHolds() bool {
	cc := it.cc
	svc := cc.jetwayBuilt()
	if svc == nil {
		return false
	}
	it.jetwayGone = true
	k := gateKey(it.ICAO, it.stand)
	cc.jw.mu.Lock()
	asked := cc.jw.docked[k]
	cc.jw.mu.Unlock()
	if !asked {
		return false
	}
	switch svc.Status(it.ICAO, it.stand) {
	case jetway.StatusDocking, jetway.StatusDocked:
		svc.Undock(it.ICAO, it.stand)
		return true
	case jetway.StatusUndocking:
		return true
	}
	cc.jw.mu.Lock()
	delete(cc.jw.docked, k)
	cc.jw.mu.Unlock()
	return false
}

// releaseJetways removes every helper and placeholder of the connection.
func (cc *controlCenter) releaseJetways() {
	cc.jw.mu.Lock()
	svc := cc.jw.svc
	prepared := cc.jw.prepared
	cc.jw.prepared, cc.jw.docked = map[string]bool{}, map[string]bool{}
	cc.jw.mu.Unlock()
	if svc == nil {
		return
	}
	for icao := range prepared {
		svc.Release(icao)
	}
}

// registerJetways serves GET /api/jetways and POST /api/jetways {"on": true}.
func registerJetways(mux *http.ServeMux, w *World) {
	mux.HandleFunc("GET /api/jetways", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, w.Jetways())
	})
	mux.HandleFunc("POST /api/jetways", func(rw http.ResponseWriter, r *http.Request) {
		var req struct {
			On bool `json:"on"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		w.SetJetways(req.On)
		writeJSON(rw, w.Jetways())
	})
	// The door offsets by type (jetway.DoorOffset): GET all in use, POST
	// one ({"type": "B738", "aheadM": 1.5, "rightM": 0}), kept locally;
	// the next helper of that type is put there.
	mux.HandleFunc("GET /api/jetways/offsets", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, jetway.DoorOffsets())
	})
	mux.HandleFunc("POST /api/jetways/offset", func(rw http.ResponseWriter, r *http.Request) {
		var o jetway.DoorOffset
		if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if err := w.st.doorOffsets.set(o); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		w.st.core.log.printf("jetway door offset %s: %.2f m ahead, %.2f m right", strings.ToUpper(o.Type), o.AheadM, o.RightM)
		writeJSON(rw, jetway.DoorOffsets())
	})
}

// logStandFit logs where tail stands on its gate: ahead of the parking
// point and right of its line, and what the gate's measure asked for
// (gatePlace); off the line by more than a metre, flagged.
func (cc *controlCenter) logStandFit(g *airport.Graph, stand int, tail string, at airport.LatLon) {
	if g == nil || stand < 0 || stand >= len(g.Layout.Parking) {
		return
	}
	if at.Lat == 0 && at.Lon == 0 {
		return // no position yet: a departure docked as it spawns (live, LKPR: "5228941.69 m ahead")
	}
	p := g.Layout.Parking[stand]
	h := p.Heading * math.Pi / 180
	north := (at.Lat - p.Position.Lat) * 111_320
	east := (at.Lon - p.Position.Lon) * 111_320 * math.Cos(p.Position.Lat*math.Pi/180)
	ahead, right := north*math.Cos(h)+east*math.Sin(h), east*math.Cos(h)-north*math.Sin(h)
	inset, side := cc.gatePlace(g, stand)
	flag := ""
	if math.Abs(right) > 1 {
		flag = " — OFF THE LINE"
	}
	cc.core.log.printf("%-6s stand %s: %.2f m ahead of the parking point, %.2f m right of its line (gate measure: nose gear %.2f m in, %.2f m right)%s", tail, p.Label(), ahead, right, inset, side, flag)
}

// Helper variants (jetway.HelperVariantTitle): each type's helper with its
// door JetwayVariantStepM × k to the side, k up to ±JetwayVariantSteps.
const (
	JetwayVariantStepM = 0.1
	JetwayVariantSteps = 6
)

// gateVariant is the helper variant whose door makes up for the side the
// simulator parks off g's stand's line (gatePlace): the helper parked
// there, its jetway meets the door of model standing on the line. ok
// false: none installed (or nothing measured), our aircraft parks where
// the simulator does instead (standPlace).
func (cc *controlCenter) gateVariant(g *airport.Graph, stand int, model string) (string, bool) {
	if cc == nil || g == nil || !jetwaysEnabled() {
		return "", false
	}
	svc := cc.jetwayBuilt()
	if svc == nil {
		return "", false
	}
	_, side := cc.gatePlace(g, stand)
	if side == 0 {
		return "", false
	}
	base := gateHelper(svc, g, stand, model, cc.hasModel)
	if jetway.IsHelperVariant(base) {
		return "", false // a door variant (HelperDoorTitle): no side steps of it
	}
	right := 0.0
	if _, r, ok := jetwayDoorOffset(model, base); ok {
		right = r
	}
	k := int(math.Round((right - side) / JetwayVariantStepM))
	if k < -JetwayVariantSteps || k > JetwayVariantSteps {
		return "", false
	}
	title := jetway.HelperVariantTitle(strings.TrimPrefix(base, jetway.HelperTitlePrefix), k)
	if k != 0 && !cc.hasModel(title) {
		return "", false
	}
	return title, true
}

// jetwaysWaitMax: departures on jetway gates wait at most this long after
// the start for the jetways to be ready, then come whatever.
const jetwaysWaitMax = 6 * time.Minute

// jetwaysReady reports whether a departure may appear on a jetway gate at
// icao: the jetways off, no helpers installed, icao's gates filled with
// placeholders, or jetwaysWaitMax gone since the jetways were switched on
// for this connection (live: counted from the traffic's start, the cap
// had run out before the jetways were on; TVS1779 spawned unwaited).
func (cc *controlCenter) jetwaysReady(icao string) bool {
	jetwaysOn.mu.Lock()
	on, since := jetwaysOn.on, jetwaysOn.since
	jetwaysOn.mu.Unlock()
	if cc.startedAt.After(since) {
		since = cc.startedAt // the jetways on before this connection
	}
	if !on || traffic.Since(since) > jetwaysWaitMax {
		return true
	}
	cc.mu.Lock()
	listed := len(cc.models) > 0
	cc.mu.Unlock()
	if !listed {
		return false // the models not listed yet: helpers or not unknown
	}
	svc := cc.jetwayService()
	if svc == nil {
		return true // no helpers installed: no jetways for our traffic
	}
	cc.jw.mu.Lock()
	prepared := cc.jw.prepared[icao]
	cc.jw.mu.Unlock()
	return prepared && !svc.Filling(icao)
}

// gateHelperTypes are stands whose helper is always of the type given
// (none now: the size rule in gateHelper chooses).
var gateHelperTypes = map[string]string{}

// gateFitMarginM: a helper for a bigger gate is wider than a smaller gate
// by this much, so the simulator cannot fit it there.
const gateFitMarginM = 2.0

// gateHelper is the helper that docks for model at g's stand: the stand's
// own type (gateHelperTypes); at a gate bigger than others of the airport,
// one too wide for the smaller ones and fitting this one, when model's
// would fit a smaller one; else model's. The simulator gives a ParkedATC
// the best-fitting gate it counts free, a docked one among them: an A320
// helper for C24 (28 m) or B14 (35 m) went to a docked 22 m gate and took
// its jetway (live, LKPR A1, C30, C23), a 22 m gate's never. Ours stands
// with its door where the helper's is (gateNoseOffset, gateSide). For a
// code C aircraft or smaller, the wider helper with model's own door,
// else an A320's, when has (the simulator lists it) says it is installed
// (jetway.HelperDoorTitle): its jetway meets a smaller aircraft's door,
// not one higher and further back (live, LKPR B14: a B748 helper's jetway
// above a B738's door).
func gateHelper(svc *jetway.Helpers, g *airport.Graph, stand int, model string, has func(string) bool) string {
	own := svc.HelperFor(model)
	if g == nil || stand < 0 || stand >= len(g.Layout.Parking) {
		return own
	}
	p := g.Layout.Parking[stand]
	if t, ok := gateHelperTypes[strings.ToUpper(g.Layout.ICAO+" "+p.Label())]; ok {
		return svc.HelperFor(t)
	}
	smaller := 0.0 // the biggest gate radius below this one's
	for _, q := range g.Layout.Parking {
		if q.IsGate() && q.Radius < p.Radius-0.5 && q.Radius > smaller {
			smaller = q.Radius
		}
	}
	if smaller == 0 {
		return own
	}
	span := func(title string) float64 {
		t := strings.TrimPrefix(title, jetway.HelperTitlePrefix)
		if s := typeSpanM(t); s > 0 {
			return s
		}
		return traffic.ProfileFor(t).WingspanM // an A320's when not known
	}
	if span(own) > 2*smaller+gateFitMarginM {
		return own // too wide for the smaller gates already
	}
	best, bestSpan := "", math.Inf(1)
	for _, t := range svc.Types() {
		if s := typeSpanM(t); s > 2*smaller+gateFitMarginM && s <= 2*p.Radius && s < bestSpan {
			best, bestSpan = t, s
		}
	}
	if best == "" {
		return own
	}
	// The door variants carry a type's door as far behind the nose gear as
	// on that type (jetway.WritePackage), so ours stops where the sim parks
	// its own type: its own type's variant first, any size (live, LKPR B14:
	// an Emirates 777 by the B748's own door, the jetway 15 m aft of L1).
	// An A320's door only for code C or below by its published span: a
	// widebody's door is far behind its nose gear, so by an A320's it stood
	// its nose gear up to 20 m past the circle (live, LKPR B14: QTR337's
	// B789 by the "B748 D-A320" helper, its engine in the jetway).
	// A widebody without its own type's helper (own the fallback's) none.
	prof := traffic.ProfileFor(model)
	var doors []string
	if d := strings.TrimPrefix(own, jetway.HelperTitlePrefix); traffic.ICAOCodeFor(prof.WingspanM) <= 'C' || strings.EqualFold(d, prof.Type) {
		doors = append(doors, d)
	}
	if traffic.ICAOCodeFor(prof.WingspanM) <= 'C' {
		doors = append(doors, "A320")
	}
	for _, d := range doors {
		if t := jetway.HelperDoorTitle(best, d); has != nil && has(t) {
			return t
		}
	}
	return svc.HelperFor(best)
}

// helperType is the ICAO type of helper title (its variants' too).
func helperType(title string) string {
	f := strings.Fields(strings.TrimPrefix(title, jetway.HelperTitlePrefix))
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// wideHelperBlocked reports another gate of g, docked, that helper (the
// title the dock would make for model at stand, jetway.Helpers.TitleFor)
// fits when it is a wider type than model's own helper: the simulator
// gives a ParkedATC the first gate it counts free, a docked one among
// them, and a wide helper fits few gates (live, LKPR 2026-11-08: C24's
// A332 helper for a B738 went to docked widebody gate C22, taking its
// jetway). Asked again once that gate is no longer docked.
func wideHelperBlocked(svc *jetway.Helpers, g *airport.Graph, stand int, model, helper string) (int, bool) {
	if svc == nil || g == nil {
		return -1, false
	}
	t, own := helperType(helper), helperType(svc.HelperFor(model))
	if t == "" || strings.EqualFold(t, own) {
		return -1, false
	}
	span := typeSpanM(t)
	if span == 0 || span <= typeSpanM(own) {
		return -1, false
	}
	for q, p := range g.Layout.Parking {
		if q == stand || !p.IsGate() || span > 2*p.Radius {
			continue
		}
		if svc.Status(g.Layout.ICAO, q) == jetway.StatusDocked {
			return q, true
		}
	}
	return -1, false
}

// hasModel reports whether the simulator lists title.
func (cc *controlCenter) hasModel(title string) bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	_, ok := cc.models[title]
	return ok
}

// gateLesson is what a jetway gate taught (jetway.HelperOptions.OnLearn):
// a helper docked there, or the sim put it on another gate instead.
type gateLesson struct {
	ICAO   string `json:"icao"`
	Stand  int    `json:"stand"`
	Helper string `json:"helper,omitempty"`
	Docked bool   `json:"docked,omitempty"`
	// Docks and Aways: the gate's docks done and its helpers put
	// elsewhere, of any type (Helper "": jetway.GateRecord).
	Docks int `json:"docks,omitempty"`
	Aways int `json:"aways,omitempty"`
	// Type and NoJetwayAt: no jetway came at the gate for aircraft of
	// Type (dockResult), last at NoJetwayAt (Unix seconds); kept until one
	// of the type docks there.
	Type       string `json:"type,omitempty"`
	NoJetwayAt int64  `json:"noJetwayAt,omitempty"`
	// Why: what taught it ("timeout": a dock timed out, dockResult). One
	// without was saved by a gate's rest, from refused docks counted as
	// failed (live, LKPR 2026-11-08: ten false lessons): dropped at load.
	Why string `json:"why,omitempty"`
	// Timeouts: the type's docks timed out there in a row, each within
	// JetwayTypeRetest of the last; the lesson counts from
	// JetwayTypeTimeouts (one saved without it counts as one).
	Timeouts int `json:"timeouts,omitempty"`
}

// timeouts is l's timed-out docks in a row (one without the count).
func (l gateLesson) timeouts() int {
	if l.Timeouts == 0 && l.NoJetwayAt != 0 {
		return 1
	}
	return l.Timeouts
}

// gateLessonStore keeps the gates' lessons in a local file
// (jetway-gates.json beside the other local files), given back to the
// helpers at the next start: a gate that refuses a helper type is not
// tried with it again after a restart.
type gateLessonStore struct {
	mu     sync.Mutex
	file   string
	items  []gateLesson
	saving sync.WaitGroup // saveLater's writes
}

var gateLessons = &gateLessonStore{}

func (s *gateLessonStore) load(file string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file, s.items = file, nil
	if b, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(b, &s.items); err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		}
	}
	n := len(s.items)
	s.items = slices.DeleteFunc(s.items, func(l gateLesson) bool { return l.Type != "" && l.Why == "" })
	if len(s.items) != n {
		fmt.Fprintf(stdout, "jetway lessons: %d 'no jetway for this type' saved by a gate's rest dropped (%s)\n", n-len(s.items), file)
		s.saveLater()
	}
}

// add notes l (replacing the same gate and helper's) and saves.
func (s *gateLessonStore) add(l gateLesson) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.put(l)
}

// put is add with s.mu held.
func (s *gateLessonStore) put(l gateLesson) {
	s.items = slices.DeleteFunc(s.items, func(i gateLesson) bool {
		return i.ICAO == l.ICAO && i.Stand == l.Stand && i.Helper == l.Helper && i.Type == l.Type
	})
	s.items = append(s.items, l)
	if s.file == "" {
		return
	}
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err == nil {
		err = os.WriteFile(s.file, b, 0o644)
	}
	if err != nil {
		fmt.Fprintf(stdout, "⚠️  %s: %v\n", s.file, err)
	}
}

// count adds a dock (or a miss) to icao's stand's record and saves.
func (s *gateLessonStore) count(icao string, stand int, docked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var l gateLesson
	for _, i := range s.items {
		if i.ICAO == icao && i.Stand == stand && i.Helper == "" && i.Type == "" {
			l = i
		}
	}
	l.ICAO, l.Stand = icao, stand
	if docked {
		l.Docks++
	} else {
		l.Aways++
	}
	s.put(l)
}

// teach gives svc the gates' lessons.
func (s *gateLessonStore) teach(svc *jetway.Helpers) {
	s.mu.Lock()
	items := slices.Clone(s.items)
	s.mu.Unlock()
	for _, l := range items {
		if l.Type != "" {
			continue // the world's own (noJetway)
		}
		if l.Helper == "" {
			svc.GateRecord(l.ICAO, l.Stand, l.Docks, l.Aways)
			continue
		}
		svc.Learn(l.ICAO, l.Stand, l.Helper, l.Docked)
	}
}

// setNoJetway notes (on) a timed-out dock of typ at icao's stand, at at,
// or clears the gate's lesson for typ (one docked); saved in the
// background (called under the jetway backend's lock). Reports whether
// the lesson began to count (JetwayTypeTimeouts reached) or was cleared.
func (s *gateLessonStore) setNoJetway(icao string, stand int, typ string, at time.Time, on bool) bool {
	icao, typ = strings.ToUpper(icao), strings.ToUpper(typ)
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.items, func(l gateLesson) bool {
		return l.ICAO == icao && l.Stand == stand && l.Type == typ
	})
	switch {
	case on && i >= 0:
		l := &s.items[i]
		n := l.timeouts()
		if at.Sub(time.Unix(l.NoJetwayAt, 0)) >= JetwayTypeRetest {
			n = 0 // the last too long ago: counted afresh
		}
		l.NoJetwayAt, l.Why, l.Timeouts = at.Unix(), "timeout", n+1
		s.saveLater()
		return l.Timeouts == JetwayTypeTimeouts
	case on:
		s.items = append(s.items, gateLesson{ICAO: icao, Stand: stand, Type: typ, NoJetwayAt: at.Unix(), Why: "timeout", Timeouts: 1})
		s.saveLater()
		return JetwayTypeTimeouts <= 1
	case i >= 0:
		counted := s.items[i].timeouts() >= JetwayTypeTimeouts
		s.items = slices.Delete(s.items, i, i+1)
		s.saveLater()
		return counted
	default:
		return false
	}
}

// noJetway reports that icao's stand gives typ no jetway now: its docks
// timed out there JetwayTypeTimeouts times in a row, the last less than
// JetwayTypeRetest ago.
func (s *gateLessonStore) noJetway(icao string, stand int, typ string, now time.Time) bool {
	icao, typ = strings.ToUpper(icao), strings.ToUpper(typ)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.items {
		if l.ICAO == icao && l.Stand == stand && l.Type == typ && typ != "" {
			return l.timeouts() >= JetwayTypeTimeouts && now.Sub(time.Unix(l.NoJetwayAt, 0)) < JetwayTypeRetest
		}
	}
	return false
}

// saveLater writes the lessons in the background. s.mu held.
func (s *gateLessonStore) saveLater() {
	if s.file == "" {
		return
	}
	s.saving.Add(1)
	go func() {
		defer s.saving.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		b, err := json.MarshalIndent(s.items, "", "  ")
		if err == nil {
			err = os.WriteFile(s.file, b, 0o644)
		}
		if err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", s.file, err)
		}
	}()
}
