package world

import (
	"cmp"
	"errors"
	"math"
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
		// Still on a closed runway (on its short final at the change, it went
		// around): moved now.
		if shut := (nav.RunwayLimits{Closed: cc.core.closed(icao)}); known && len(shut.Closed) > 0 {
			for _, it := range items {
				it.mu.Lock()
				v, stand := it.view, it.stand
				it.mu.Unlock()
				if it.arr != nil && !v.Done && shut.ClosedEnd(v.Runway) && !slices.Contains(arrs, v.Runway) {
					cc.changeArrivalRunway(g, it, cc.runwayFor(g, true, stand))
				}
			}
		}
		cc.changeOverdue(g, items, deps, arrs, now)
		if !known || before == use {
			continue
		}
		keep := cc.lastOnOldRunway(g, items, arrs)
		var rejoined []*controlled // joined their new STAR part-way: out of position
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
				if cc.changeArrivalRunway(g, it, cc.runwayFor(g, true, stand)) {
					rejoined = append(rejoined, it)
				}
			}
		}
		cc.behindTheFlow(g, items, rejoined)
	}
}

// behindTheFlow sequences the arrivals that joined their new STAR
// part-way on a runway change (out of position, near the field on the
// old side) behind those already coming in on the new runway's flow
// within flowNM of it, nearest first, each behind the one before: they
// are vectored in behind, not turned across the stream (the user, live:
// TVS555 east of the field and OKJAF on LOMK 8S, "TVS555 should be
// vectored behind").
func (cc *controlCenter) behindTheFlow(g *airport.Graph, items, rejoined []*controlled) {
	if len(rejoined) == 0 || cc.followed == nil {
		return
	}
	out := map[*controlled]bool{}
	for _, it := range rejoined {
		out[it] = true
	}
	far := func(it *controlled) (float64, string, bool) {
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		_, end, ok := g.Layout.RunwayEnd(v.Runway)
		if !ok || v.Done || v.State != "approaching" {
			return 0, "", false
		}
		return calc.HaversineNM(v.Position.Lat, v.Position.Lon, end.Threshold.Lat, end.Threshold.Lon), v.Runway, true
	}
	type arr struct {
		it  *controlled
		nm  float64
		rwy string
	}
	var flow, late []arr
	for _, it := range items {
		if it.arr == nil {
			continue
		}
		nm, rwy, ok := far(it)
		if !ok {
			continue
		}
		if out[it] {
			late = append(late, arr{it, nm, rwy})
		} else if nm <= flowNM {
			flow = append(flow, arr{it, nm, rwy})
		}
	}
	slices.SortFunc(late, func(a, b arr) int { return cmp.Compare(a.nm, b.nm) })
	last := map[string]string{} // by runway: the one the next goes behind
	lastNM := map[string]float64{}
	for _, f := range flow {
		if f.nm > lastNM[f.rwy] {
			last[f.rwy], lastNM[f.rwy] = f.it.Tail, f.nm
		}
	}
	for _, l := range late {
		if lead := last[l.rwy]; lead != "" {
			cc.followed(l.it.ICAO, l.it.Tail, lead)
			cc.log.printf("%-6s runway change: out of position, behind %s on the new flow", l.it.Tail, lead)
		}
		last[l.rwy] = l.it.Tail
	}
}

// flowNM: arrivals on the new runway's flow this close are ahead of one
// out of position after a runway change.
const flowNM = 40.0

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
func (cc *controlCenter) changeArrivalRunway(g *airport.Graph, it *controlled, runway string) (rejoined bool) {
	if cc.procedures == nil {
		return false
	}
	p, ok := cc.procedures(g.Layout.ICAO)
	if !ok {
		return false
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
		return false
	}
	it.mu.Lock()
	at, hdg := it.view.Position, it.view.Heading
	it.mu.Unlock()
	full := len(pts)
	pts = joinNewSTAR(at, hdg, pts)
	rejoined = len(pts) < full
	err = cc.do(func() error { return it.arr.ChangeRunway(runway, pts, cc.missedFor(g, runway)) })
	if errors.Is(err, traffic.ErrTooLate) {
		return false // established: it lands on the runway it flies to
	}
	if err != nil {
		cc.log.printf("%-6s runway change to %s refused: %v", it.Tail, runway, err)
		return false
	}
	it.mu.Lock()
	it.view.Runway, it.view.Procedure = runway, star
	it.keepSpot()
	if expect != "" {
		it.view.Procedure += " → " + expect
	}
	it.procSaid = procedureSaid(star, nil)
	// Its approach cleared again for the new runway — once told of it, not
	// before (live, CSA1583 "cleared ILS approach runway 30" 26 s before
	// "runway change, runway 30 in use").
	it.approachSaid = true
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
		it.mu.Lock()
		it.approachSaid = false
		it.mu.Unlock()
	})
	return rejoined
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
			// Under way to its runway or on it: it finishes there (the user:
			// departures leave; the arrivals are given the gap,
			// runwayChangeGap). Re-planned mid-taxi it met the traffic coming
			// the other way (live, OKJQW down F against BAW450). On its stand
			// or about to taxi: the new runway.
			case traffic.TaxiDeparting:
				keep[it.Tail] = true // rolling or climbing out: it goes
			case traffic.TaxiTaxiing, traffic.TaxiHoldingShort, traffic.TaxiLiningUp, traffic.TaxiLinedUp:
				// Not for a runway that is closed: moved with the others.
				if !(nav.RunwayLimits{Closed: cc.core.closed(g.Layout.ICAO)}).ClosedEnd(v.Runway) {
					keep[it.Tail] = true
				}
			}
		case it.arr != nil && !slices.Contains(arrs, v.Runway) && it.arr.State() == traffic.ArrivalApproaching:
			_, end, ok := g.Layout.RunwayEnd(v.Runway)
			if !ok {
				continue
			}
			// Only established on its final (its STAR and approach flown): one
			// still vectored is moved to the new runway (live, PHDYS left on
			// the 06 approach, vectored 10 NM out, as 24 departed toward it).
			if len(it.arr.ProcedureRoute()) > 0 {
				continue
			}
			// Never kept for a runway that is closed: moved with the others
			// (on a short final it goes around, closures.go).
			if (nav.RunwayLimits{Closed: cc.core.closed(g.Layout.ICAO)}).ClosedEnd(v.Runway) {
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

// joinNewSTAR is where an arrival already on its way joins the new
// runway's STAR on a runway change: from its entry fix while that is ahead
// of it (within 90° of hdg) and no farther out than it is (joinOutNM to
// spare); else from the first point of it no farther from the end of the
// procedure than the arrival is — never sent back out against the
// arrivals coming in on the same STAR (live, TVS356 back to VLM after the
// change to 06, against THY940; TVS555, number 2 east of the field, sent
// back to LOMKI head-on with OKJAF, number 3, on LOMK 8S), nor joined so
// near the final that it cannot be spaced (live, WZZ1209 and QTR879, 0.1
// NM): the approach's initial fix at the nearest (IAF; else two points
// before the final approach fix).
func joinNewSTAR(pos airport.LatLon, hdg float64, pts []airport.NavPoint) []airport.NavPoint {
	if pos == (airport.LatLon{}) || len(pts) < 3 {
		return pts
	}
	ref := pts[len(pts)-1].Position // the procedure's end, by the final
	out := func(p airport.LatLon) float64 { return calc.HaversineNM(p.Lat, p.Lon, ref.Lat, ref.Lon) }
	ahead := func(p airport.LatLon) bool {
		return math.Abs(math.Mod(calc.BearingDegrees(pos.Lat, pos.Lon, p.Lat, p.Lon)-hdg+540, 360)-180) <= 90
	}
	mine := out(pos)
	if ahead(pts[0].Position) && out(pts[0].Position) <= mine+joinOutNM {
		return pts
	}
	last := -1 // the latest it may join at: the IAF, else two before the FAF
	for i, n := range pts {
		if n.IAF {
			last = i
			break
		}
		if n.FAF {
			last = max(i-2, 0)
			break
		}
	}
	if last < 0 {
		last = len(pts) - 2
	}
	for i := 0; i <= last; i++ {
		if out(pts[i].Position) <= mine {
			return pts[i:]
		}
	}
	if last <= 0 || len(pts)-last < 2 {
		return pts
	}
	return pts[last:]
}

// joinOutNM: a runway change's new STAR is joined at its entry while that
// is no more than this farther out than the arrival.
const joinOutNM = 5.0

// changeOverdue re-clears to the new runway the departures still at the
// old end's holding point once the changeover's deadline has passed
// (sequences.runwayChangeGap): the arrivals were planned for the runway
// being the new direction's by then (deps, arrs: the runways in use now).
func (cc *controlCenter) changeOverdue(g *airport.Graph, items []*controlled, deps, arrs []string, now time.Time) {
	if cc.changeDeadline == nil {
		return
	}
	for _, it := range items {
		if it.dep == nil {
			continue
		}
		it.mu.Lock()
		v, stand := it.view, it.stand
		it.mu.Unlock()
		if v.Done || slices.Contains(deps, v.Runway) {
			continue
		}
		switch it.dep.State() {
		case traffic.TaxiTaxiing, traffic.TaxiHoldingShort:
		default:
			continue // lining up or gone: it goes
		}
		own, ok := runwayOf(g.Layout, v.Runway)
		if !ok {
			continue
		}
		for _, a := range arrs {
			r, ok := runwayOf(g.Layout, a)
			if !ok || a == v.Runway || r.Name() != own.Name() {
				continue
			}
			if by, ok := cc.changeDeadline(g.Layout.ICAO + " " + a); ok && now.After(by) {
				cc.log.printf("%-6s runway change: past the changeover, to the new runway", it.Tail)
				cc.changeDepartureRunway(g, it, cc.runwayFor(g, false, stand))
			}
			break
		}
	}
}
