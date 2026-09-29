//go:build windows
// +build windows

package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
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

	mu  sync.Mutex
	seq map[string]*traffic.ApproachSequencer // by "ICAO runway"
}

func newSequences(cc *controlCenter, s *scheduler) *sequences {
	return &sequences{cc: cc, s: s, seq: map[string]*traffic.ApproachSequencer{}}
}

// sequencer is the sequencer of an airport's runway, created on first use.
func (q *sequences) sequencer(icao, runway string) *traffic.ApproachSequencer {
	q.mu.Lock()
	defer q.mu.Unlock()
	k := icao + " " + runway
	if s := q.seq[k]; s != nil {
		return s
	}
	s := traffic.NewApproachSequencer(runway, traffic.SequencerOptions{OnChange: func(c traffic.SequenceChange) {
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
	return fmt.Sprintf(" behind %s, %g NM", e.Leader, e.SpacingNM)
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
	for k, list := range feed {
		if k.rwy == "" {
			continue
		}
		q.sequencer(k.icao, k.rwy).Update(now, list)
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
	Runway string                  `json:"runway"`
	Arrive []traffic.SequenceEntry `json:"sequence"`
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
					out = append(out, sequenceView{Runway: rwy, Arrive: seq})
				}
			}
		}
		q.mu.Unlock()
		sort.Slice(out, func(i, j int) bool { return out[i].Runway < out[j].Runway })
		writeJSON(w, out)
	})
}
