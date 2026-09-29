//go:build windows
// +build windows

package main

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The landing sequence on the map (#390): an approach sequencer per airport
// and arrival runway, fed every second with our arrivals (controlled on
// their STAR and approach, or en route to it) and, respected, the other
// traffic arriving there.
//
//	GET /api/sequence?icao=LKPR — the runways and their sequences

type sequences struct {
	cc *controlCenter
	s  *scheduler

	mu   sync.Mutex
	seq  map[string]*traffic.ApproachSequencer // by "ICAO runway"
	cond map[string]traffic.ApproachConditions // the conditions last logged, by sequencer
	// absorbed: when each arrival last got a delay to absorb (#391).
	absorbed map[string]time.Time
	// stacks: the holding stacks by airport and fix (#392).
	stacks map[string]*traffic.HoldStack
}

func newSequences(cc *controlCenter, s *scheduler) *sequences {
	return &sequences{cc: cc, s: s, seq: map[string]*traffic.ApproachSequencer{}, cond: map[string]traffic.ApproachConditions{}, absorbed: map[string]time.Time{}, stacks: map[string]*traffic.HoldStack{}}
}

// sequencer is the sequencer of an airport's runway, created on first use.
func (q *sequences) sequencer(icao, runway string) *traffic.ApproachSequencer {
	q.mu.Lock()
	defer q.mu.Unlock()
	k := icao + " " + runway
	if s := q.seq[k]; s != nil {
		return s
	}
	s := traffic.NewApproachSequencer(runway, traffic.SequencerOptions{MinSpacingNM: sepMinNM, OnChange: func(c traffic.SequenceChange) {
		e := c.Entry
		switch {
		case c.Gone:
			tlog.printf("%-6s sequence %s %s: out of the sequence", e.Callsign, icao, c.Runway)
		case c.Previous == 0:
			tlog.printf("%-6s sequence %s %s: number %d%s, %.0f NM to go, delay %s", e.Callsign, icao, c.Runway, e.Number, behind(e), e.DistanceToGoNM, e.Delay.Round(time.Second))
		default:
			tlog.printf("%-6s sequence %s %s: number %d (was %d)%s, delay %s", e.Callsign, icao, c.Runway, e.Number, c.Previous, behind(e), e.Delay.Round(time.Second))
		}
	}})
	q.seq[k] = s
	return s
}

func behind(e traffic.SequenceEntry) string {
	if e.Leader == "" {
		return ""
	}
	why := ""
	if e.SpacingWhy != "" {
		why = " (" + e.SpacingWhy + ")"
	}
	return fmt.Sprintf(" behind %s, %g NM%s", e.Leader, e.SpacingNM, why)
}

// Delays: absorbed from 30 s on, at most every absorbEvery per arrival so
// it has slowed before its delay is looked at again.
const (
	absorbFrom  = 30 * time.Second
	absorbEvery = 90 * time.Second
)

// absorb has the arrivals of a sequence on their STAR lose their delay:
// speed, then a dog-leg; what is left waits for the hold (#392).
func (q *sequences) absorb(now time.Time, icao string, seq []traffic.SequenceEntry, items []*controlled) {
	for _, e := range seq {
		var it *controlled
		for _, x := range items {
			if x.arr != nil && x.Tail == e.Callsign && x.ICAO == icao {
				it = x
				break
			}
		}
		if it == nil || e.Fixed {
			continue
		}
		// In a hold: released once its delay is down to holdRelease.
		if h, _, holding := it.arr.Holding(); holding {
			if e.Delay <= holdRelease {
				q.leaveHold(icao, it, h, e)
			}
			continue
		}
		if e.Delay < absorbFrom {
			continue
		}
		q.mu.Lock()
		recent := now.Sub(q.absorbed[e.Callsign]) < absorbEvery
		q.mu.Unlock()
		if recent {
			continue
		}
		var a traffic.Absorption
		err := q.cc.do(func() (err error) { a, err = it.arr.AbsorbDelay(e.Delay); return err })
		if errors.Is(err, traffic.ErrNotOnProcedure) || errors.Is(err, traffic.ErrHolding) {
			continue // on the final, not flying a STAR, or holding
		}
		q.mu.Lock()
		q.absorbed[e.Callsign] = now
		q.mu.Unlock()
		if err != nil {
			tlog.printf("%-6s sequence: absorbing %s failed: %v", e.Callsign, e.Delay.Round(time.Second), err)
			continue
		}
		if r := it.arr.ProcedureRoute(); len(r) > 0 {
			it.mu.Lock()
			it.approach = r // the route with its dog-leg: the distance to go
			it.mu.Unlock()
		}
		tlog.printf("%-6s ATC: %s, number %d, delay %s: %s", e.Callsign, e.Callsign, e.Number, e.Delay.Round(time.Second), a)
		// Too much for speed and a dog-leg: the rest in the hold.
		if a.Left >= holdFrom {
			q.enterHold(now, icao, it, e, a.Left)
		}
	}
}

// Holding (#392): an arrival with holdFrom or more left after speed and
// path stretching holds at the first STAR point holdFixNM or more from the
// threshold, in that fix's stack from holdBaseFt; it leaves once its delay
// is down to holdRelease, and the ones above step down.
const (
	holdFrom    = time.Minute
	holdRelease = time.Minute
	holdFixNM   = 15.0
	holdBaseFt  = 6000.0
)

// stack is the stack at a fix, created on first use.
func (q *sequences) stack(icao string, h traffic.Hold) *traffic.HoldStack {
	q.mu.Lock()
	defer q.mu.Unlock()
	k := fmt.Sprintf("%s %.3f %.3f", icao, h.Fix.Lat, h.Fix.Lon)
	if st := q.stacks[k]; st != nil {
		return st
	}
	st := &traffic.HoldStack{Hold: h, BaseFt: holdBaseFt}
	q.stacks[k] = st
	return st
}

func (q *sequences) enterHold(now time.Time, icao string, it *controlled, e traffic.SequenceEntry, left time.Duration) {
	h, ok := it.arr.HoldFix(holdFixNM)
	if !ok {
		tlog.printf("%-6s sequence: %s to lose, no fix to hold at", e.Callsign, left.Round(time.Second))
		return
	}
	st := q.stack(icao, h)
	h = st.Hold // the stack's: the same racetrack for all
	alt := st.Assign(e.Callsign)
	var entry traffic.HoldEntry
	if err := q.cc.do(func() (err error) { entry, err = it.arr.EnterHold(h, alt); return err }); err != nil {
		st.Release(e.Callsign)
		tlog.printf("%-6s sequence: hold failed: %v", e.Callsign, err)
		return
	}
	if r := it.arr.ProcedureRoute(); len(r) > 0 {
		it.mu.Lock()
		it.approach = r
		it.mu.Unlock()
	}
	tlog.printf("%-6s ATC: %s, hold at %s, %s entry, maintain %.0f ft, expect further clearance %s", e.Callsign, e.Callsign,
		fixName(h), entry, alt, now.Add(left).Format("15:04"))
}

func (q *sequences) leaveHold(icao string, it *controlled, h traffic.Hold, e traffic.SequenceEntry) {
	if err := q.cc.do(it.arr.LeaveHold); err != nil {
		tlog.printf("%-6s sequence: leaving the hold failed: %v", e.Callsign, err)
		return
	}
	tlog.printf("%-6s ATC: %s, leave the hold at %s, number %d, continue the arrival", e.Callsign, e.Callsign, fixName(h), e.Number)
	if r := it.arr.ProcedureRoute(); len(r) > 0 {
		it.mu.Lock()
		it.approach = r
		it.mu.Unlock()
	}
	// The ones above step down.
	for cs, alt := range q.stack(icao, h).Release(e.Callsign) {
		if above := q.cc.byTail(cs); above != nil && above.arr != nil {
			if err := q.cc.do(func() error { return above.arr.HoldAltitude(alt) }); err == nil {
				tlog.printf("%-6s ATC: %s, descend %.0f ft, hold as published", cs, cs, alt)
			}
		}
	}
}

// fixName is a hold's name: the STAR fix ident when it has one.
func fixName(h traffic.Hold) string {
	if h.Ident != "" {
		return h.Ident
	}
	return fmt.Sprintf("%.3f %.3f", h.Fix.Lat, h.Fix.Lon)
}

// tick feeds every sequencer with the arrivals now.
func (q *sequences) tick(now time.Time) {
	type key struct{ icao, rwy string }
	feed := map[key][]traffic.ApproachAircraft{}
	thresholds := map[key]airport.LatLon{}
	pos := map[uint32]traffic.TrackedAircraft{}
	for _, a := range q.cc.world.Aircraft() {
		pos[a.ObjectID] = a
	}
	threshold := func(icao, rwy string) (airport.LatLon, bool) {
		k := key{icao, rwy}
		if t, ok := thresholds[k]; ok {
			return t, true
		}
		g, err := q.cc.graph(icao)
		if err != nil {
			return airport.LatLon{}, false
		}
		_, end, ok := g.Layout.RunwayEnd(rwy)
		if ok {
			thresholds[k] = end.Threshold
		}
		return end.Threshold, ok
	}
	add := func(icao, rwy, cs, model string, p airport.LatLon, kts float64, route []airport.LatLon, fixed bool) {
		t, ok := threshold(icao, rwy)
		if !ok {
			return
		}
		prof := traffic.ProfileFor(model)
		feed[key{icao, rwy}] = append(feed[key{icao, rwy}], traffic.ApproachAircraft{Callsign: cs, Wake: traffic.WakeFor(model),
			DistanceToGoNM: traffic.DistanceToGo(p, route, t), GroundKts: kts, FinalKts: prof.Approach.ApproachKts, Fixed: fixed})
	}
	// Controlled arrivals, airborne.
	q.cc.mu.Lock()
	items := make([]*controlled, 0, len(q.cc.items))
	for _, it := range q.cc.items {
		items = append(items, it)
	}
	q.cc.mu.Unlock()
	for _, it := range items {
		if it.arr == nil {
			continue
		}
		it.mu.Lock()
		v, route := it.view, it.approach
		it.mu.Unlock()
		if v.OnGround || v.Done || (v.State != "approaching" && v.State != "landing" && v.State != "spawning") {
			continue
		}
		p, kts := v.Position, v.GroundSpeed
		if a, ok := pos[it.objectID]; ok {
			p, kts = a.Position, a.GroundKts
		}
		if p == (airport.LatLon{}) {
			continue
		}
		add(it.ICAO, v.Runway, v.Tail, v.Model, p, kts, remaining(p, route), false)
	}
	// Enroute arrivals, on their way to the STAR entry.
	q.s.mu.Lock()
	var enroute []*enrouteAC
	for _, e := range q.s.enroute {
		if e.arrive != nil && !e.handing {
			enroute = append(enroute, e)
		}
	}
	q.s.mu.Unlock()
	for _, e := range enroute {
		a, ok := pos[e.objectID]
		if !ok {
			continue
		}
		var route []airport.LatLon
		for _, n := range e.arrive.route {
			route = append(route, n.Position)
		}
		rwy := ""
		if e.arrive.plan != nil {
			rwy = e.arrive.plan.ArrivalRunway
		}
		add(e.f.Airport, rwy, e.f.Callsign, e.model, a.Position, a.GroundKts, route, false)
	}
	// Other traffic arriving, respected: it keeps its slot.
	if q.s.mgr.Options().Others == traffic.OtherRespect {
		for _, icao := range q.s.mgr.Airports() {
			g, err := q.cc.graph(icao)
			if err != nil {
				continue
			}
			rwy := q.cc.activeRunway(g, true)
			for _, a := range q.s.mgr.Others(icao) {
				if a.Phase == traffic.PhaseArriving {
					add(icao, rwy, nameOf(a), a.Title, a.Position, a.GroundKts, nil, true)
				}
			}
		}
	}
	// Every sequencer gets its arrivals, also none (they leave).
	q.mu.Lock()
	for k, s := range q.seq {
		i, r, _ := strings.Cut(k, " ")
		if _, ok := feed[key{i, r}]; !ok && len(s.Sequence()) > 0 {
			feed[key{i, r}] = nil
		}
	}
	q.mu.Unlock()
	var wx *nav.Weather
	if q.cc.weather != nil {
		wx = q.cc.weather()
	}
	for k, list := range feed {
		if k.rwy == "" {
			continue
		}
		s := q.sequencer(k.icao, k.rwy)
		if wx != nil {
			if g, err := q.cc.graph(k.icao); err == nil {
				if _, end, ok := g.Layout.RunwayEnd(k.rwy); ok {
					c := traffic.ConditionsFrom(*wx, end.Heading)
					s.SetConditions(c)
					// Logged when what matters for spacing changes.
					q.mu.Lock()
					last, seen := q.cond[k.icao+" "+k.rwy]
					changed := !seen || last.LowVisibility() != c.LowVisibility() || last.ReducedAllowed() != c.ReducedAllowed() ||
						last.Surface != c.Surface || math.Abs(last.HeadwindKts-c.HeadwindKts) >= 5
					if changed {
						q.cond[k.icao+" "+k.rwy] = c
					}
					q.mu.Unlock()
					if changed {
						lvp := ""
						if c.LowVisibility() {
							lvp = ", low visibility procedures"
						}
						tlog.printf("sequence %s %s: %s%s", k.icao, k.rwy, c, lvp)
					}
				}
			}
		}
		q.absorb(now, k.icao, s.Update(now, list), items)
	}
}

func nameOf(a traffic.TrackedAircraft) string {
	if a.Tail != "" {
		return a.Tail
	}
	return a.Title
}

// remaining is the part of route still ahead of p: from the point after
// the nearest one.
func remaining(p airport.LatLon, route []airport.LatLon) []airport.LatLon {
	if len(route) == 0 {
		return nil
	}
	best, at := -1.0, 0
	for i, r := range route {
		if d := dist(p, r); best < 0 || d < best {
			best, at = d, i
		}
	}
	return route[at:]
}

func dist(a, b airport.LatLon) float64 {
	dx, dy := a.Lat-b.Lat, a.Lon-b.Lon
	return dx*dx + dy*dy
}

type sequenceView struct {
	Runway     string                     `json:"runway"`
	Conditions traffic.ApproachConditions `json:"conditions"`
	LVP        bool                       `json:"lvp"`
	Arrive     []traffic.SequenceEntry    `json:"sequence"`
}

func registerSequence(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/sequence", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		q := st.sequences
		st.mu.Unlock()
		out := []sequenceView{}
		if q == nil {
			writeJSON(w, out)
			return
		}
		icao := strings.ToUpper(r.URL.Query().Get("icao"))
		q.mu.Lock()
		for k, s := range q.seq {
			if i, rwy, _ := strings.Cut(k, " "); i == icao {
				if seq := s.Sequence(); len(seq) > 0 {
					c := s.Conditions()
					out = append(out, sequenceView{Runway: rwy, Conditions: c, LVP: c.LowVisibility(), Arrive: seq})
				}
			}
		}
		q.mu.Unlock()
		sort.Slice(out, func(i, j int) bool { return out[i].Runway < out[j].Runway })
		writeJSON(w, out)
	})
}
