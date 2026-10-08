package world

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A change of the runway in use (#456): departures not yet lining up are
// re-cleared to the new runway (a new SID named after the same fix, a new
// taxi route from where they are), arrivals not yet on the final get the
// new runway's STAR from the same entry fix and its approach. Who is lined
// up, departing or established goes on to the old runway: one runway
// controller per physical runway keeps both directions apart.

// runwayCheckEvery is how often the runway in use is looked at.
const runwayCheckEvery = 5 * time.Second

// checkRunways re-plans the traffic of every airport whose runway in use
// changed since the last look.
func (cc *controlCenter) checkRunways(now time.Time) {
	if now.Sub(cc.runwayCheckAt) < runwayCheckEvery {
		return
	}
	cc.runwayCheckAt = now
	cc.mu.Lock()
	byICAO := map[string][]*controlled{}
	for _, it := range cc.items {
		byICAO[it.ICAO] = append(byICAO[it.ICAO], it)
	}
	if cc.runwaysNow == nil {
		cc.runwaysNow = map[string]string{}
	}
	cc.mu.Unlock()
	for icao, items := range byICAO {
		g, err := cc.graph(icao)
		if err != nil {
			continue
		}
		deps, arrs := nav.Names(cc.runwaysInUse(g, false)), nav.Names(cc.runwaysInUse(g, true))
		use := strings.Join(deps, ",") + "/" + strings.Join(arrs, ",")
		cc.mu.Lock()
		before, known := cc.runwaysNow[icao]
		cc.runwaysNow[icao] = use
		cc.mu.Unlock()
		if !known || before == use {
			continue
		}
		keep := cc.lastOnOldRunway(g, items, arrs)
		for _, it := range items {
			it.mu.Lock()
			v, stand := it.view, it.stand
			it.mu.Unlock()
			if keep[it.Tail] {
				cc.log.printf("%-6s runway change: finishes on %s", it.Tail, v.Runway)
				continue
			}
			// Only who is on a runway no longer in use moves: with parallels,
			// to the one nearest its stand.
			switch {
			case v.Done:
			case it.dep != nil && !slices.Contains(deps, v.Runway):
				cc.changeDepartureRunway(g, it, cc.runwayFor(g, false, stand))
			case it.arr != nil && !slices.Contains(arrs, v.Runway):
				cc.changeArrivalRunway(g, it, cc.runwayFor(g, true, stand))
			}
		}
	}
}

// sameFix picks the procedure named after the same fix as old ("BALT7D"
// → "BALT2A"), else nil.
func sameFix(list []airport.Procedure, old string) *airport.Procedure {
	prefix := old
	if i := strings.IndexAny(old, "0123456789"); i > 0 {
		prefix = old[:i]
	}
	for i := range list {
		if old != "" && strings.HasPrefix(list[i].Name, prefix) {
			return &list[i]
		}
	}
	return nil
}

// changeDepartureRunway re-clears a departure to runway: the SID, the
// taxi route from where it is, said by the position working it.
func (cc *controlCenter) changeDepartureRunway(g *airport.Graph, it *controlled, runway string) {
	it.mu.Lock()
	oldSID, pos := it.view.Procedure, it.atc
	it.mu.Unlock()
	var pts []airport.NavPoint
	sid := ""
	if oldSID != "" && cc.procedures != nil {
		if p, ok := cc.procedures(g.Layout.ICAO); ok {
			name := ""
			if s := sameFix(p.SIDsFor(runway), oldSID); s != nil {
				name = s.Name
			}
			r := SpawnRequest{Kind: "departure", Runway: runway, ProcName: name}
			if got, n, _, err := cc.procedureFor(g, r); err == nil {
				pts, sid = got, n
			}
		}
	}
	err := cc.do(func() error { return it.dep.ChangeRunway(runway, "", pts) })
	if errors.Is(err, traffic.ErrTooLate) {
		return // lined up: it goes from the runway it is on
	}
	if err != nil {
		cc.log.printf("%-6s runway change to %s refused: %v", it.Tail, runway, err)
		return
	}
	if pos == "" {
		pos = traffic.PosDelivery
	}
	it.mu.Lock()
	it.view.Runway, it.view.Procedure = runway, sid
	it.keepSpot()
	it.procSaid = ""
	it.fixes = nil
	if sid != "" {
		it.procSaid = procedureSaid(sid, nil)
		for _, p := range pts {
			if p.Ident != "" {
				it.fixes = append(it.fixes, airFix{Ident: p.Ident, LatLon: p.Position})
			}
		}
	}
	it.setRoute()
	said, state := it.procSaid, it.view.State
	r := it.dep.Route()
	it.mu.Unlock()
	cc.log.printf("%-6s runway change: runway %s, SID %s", it.Tail, runway, orNone(sid))
	p := cc.pending
	p.later(it.clearAt(pos).Add(atcAnswerDelay+p.jitter(atcAnswerJitter)), func() {
		it.say(traffic.RunwayChange(pos, it.Tail, runway, said, "", ""))
		// Taxiing: the new route at once, it goes on along it.
		if state == "taxiing" && r != nil {
			it.say(traffic.ClearedTaxiToRunway(it.Tail, runway, r.Entry, r.SpokenTaxiways(len(r.Edges))))
		}
	})
}

// changeArrivalRunway gives an arrival not yet on the final the new
// runway's STAR from the same entry fix and its approach, sequenced afresh.
func (cc *controlCenter) changeArrivalRunway(g *airport.Graph, it *controlled, runway string) {
	if cc.procedures == nil {
		return
	}
	p, ok := cc.procedures(g.Layout.ICAO)
	if !ok {
		return
	}
	it.mu.Lock()
	old, _, _ := strings.Cut(it.view.Procedure, " → ")
	it.mu.Unlock()
	name := ""
	if s := sameFix(p.STARsFor(runway), old); s != nil {
		name = s.Name
	}
	pts, star, expect, err := cc.procedureFor(g, SpawnRequest{Kind: "arrival", Runway: runway, ProcName: name})
	if err != nil {
		cc.log.printf("%-6s runway change to %s: %v", it.Tail, runway, err)
		return
	}
	err = cc.do(func() error { return it.arr.ChangeRunway(runway, pts, cc.missedFor(g, runway)) })
	if errors.Is(err, traffic.ErrTooLate) {
		return // established: it lands on the runway it flies to
	}
	if err != nil {
		cc.log.printf("%-6s runway change to %s refused: %v", it.Tail, runway, err)
		return
	}
	it.mu.Lock()
	it.view.Runway, it.view.Procedure = runway, star
	it.keepSpot()
	if expect != "" {
		it.view.Procedure += " → " + expect
	}
	it.procSaid = procedureSaid(star, nil)
	it.approach, it.fixes = nil, nil
	for _, n := range pts {
		it.approach = append(it.approach, n.Position)
		if n.Ident != "" {
			it.fixes = append(it.fixes, airFix{Ident: n.Ident, LatLon: n.Position})
		}
	}
	it.setRoute()
	said, pos := it.procSaid, it.atc
	it.mu.Unlock()
	if pos == "" {
		pos = traffic.PosApproach
	}
	if cc.rejoin != nil {
		cc.rejoin(it.ICAO, it.Tail)
	}
	cc.log.printf("%-6s runway change: runway %s, STAR %s", it.Tail, runway, star)
	pq := cc.pending
	pq.later(it.clearAt(pos).Add(atcAnswerDelay+pq.jitter(atcAnswerJitter)), func() {
		it.say(traffic.RunwayChange(pos, it.Tail, runway, "", said, expect))
	})
}

// A runway change is a transition, as a tower runs it: the new runway is
// prepared for everyone at once, while the take-offs under way and the last
// landings on the final finish on the old one. runwayChangeKeepArrivals at
// most (the nearest, within runwayChangeKeepNM to go) land on it; every
// other arrival is rerouted, as far as it makes sense.
const (
	runwayChangeKeepArrivals = 2
	runwayChangeKeepNM       = 15.0
)

// lastOnOldRunway is who finishes on a runway no longer in use (arrs: the
// arrival runways now): departures holding short of their runway, lining
// up, lined up or rolling; and the runwayChangeKeepArrivals arrivals
// nearest its threshold, approaching within runwayChangeKeepNM.
func (cc *controlCenter) lastOnOldRunway(g *airport.Graph, items []*controlled, arrs []string) map[string]bool {
	keep := map[string]bool{}
	type near struct {
		tail string
		nm   float64
	}
	var landing []near
	for _, it := range items {
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		if v.Done {
			continue
		}
		switch {
		case it.dep != nil:
			switch it.dep.State() {
			case traffic.TaxiLiningUp, traffic.TaxiLinedUp, traffic.TaxiDeparting:
				keep[it.Tail] = true
			case traffic.TaxiHoldingShort:
				if rwy, ok := runwayOf(g.Layout, v.Runway); ok && v.HoldingShortOf == rwy.Name() {
					keep[it.Tail] = true // at its own runway: the take-off goes on
				}
			}
		case it.arr != nil && !slices.Contains(arrs, v.Runway) && it.arr.State() == traffic.ArrivalApproaching:
			_, end, ok := g.Layout.RunwayEnd(v.Runway)
			if !ok {
				continue
			}
			if nm := calc.HaversineNM(v.Position.Lat, v.Position.Lon, end.Threshold.Lat, end.Threshold.Lon); nm <= runwayChangeKeepNM {
				landing = append(landing, near{it.Tail, nm})
			}
		}
	}
	slices.SortFunc(landing, func(a, b near) int { return cmp.Compare(a.nm, b.nm) })
	for i, l := range landing {
		if i >= runwayChangeKeepArrivals {
			break
		}
		keep[l.tail] = true
	}
	return keep
}
