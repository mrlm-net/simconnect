package world

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Intersection departures for the queue (traffic ideas, #1030): with
// departures queuing for a runway's full length, ground gives one that
// needs less runway an intersection with its taxi clearance, so it does not
// join the queue (LKPR 24 at B or F). The rules are the class's
// (traffic.IntersectionRule: the runway it must leave, widebodies the full
// length), the airport's own entries where it lists them
// (traffic.IntersectionEntries), the first intersection after the full
// length first, deeper only while the runway left is enough (live at EDDM:
// AFR1910, an A320, at B10 with 2,250 m left; jets get B12, 2,808 m), and
// only one whose own queue is empty. Some crews decline it and ask for the
// full length (traffic.DeclineChance).

// IntersectionQueue: with this many departures already queuing for the
// full length of a runway (holding short, lining up, or taxiing there), the
// next one is offered an intersection.
const IntersectionQueue = 3

// entryGroupMeters: entries this close along the runway are one place (the
// taxiways either side of it: EDDM B11 and B12).
const entryGroupMeters = 60.0

// intersectionInMeters: an intersection given is this far in from the
// threshold at least; nearer it is the full length as near as matters
// (EDDM B13, 156 m in from 26L, beside B14 and B15).
const intersectionInMeters = 300.0

// offersIntersection: a departure of model is offered an intersection with
// queue departures ahead of it for the full length.
func offersIntersection(model string, queue int) bool {
	if queue < IntersectionQueue {
		return false
	}
	_, full := traffic.IntersectionRule(model)
	return !full
}

// fullLengthQueue counts our departures for icao's runway rwy from its full
// length, on the ground and not yet rolling, but for except.
func (cc *controlCenter) fullLengthQueue(icao, rwy string, except *controlled) int {
	return cc.entryQueue(icao, rwy, except, func(entry string) bool { return entry == "" })
}

// entryQueue counts our departures for icao's runway rwy from an entry
// that is, on the ground and not yet rolling, but for except.
func (cc *controlCenter) entryQueue(icao, rwy string, except *controlled, is func(entry string) bool) int {
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	n := 0
	for _, it := range items {
		if it == except || it.dep == nil {
			continue
		}
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		if v.Done || !v.OnGround || !is(v.Entry) || v.Runway != rwy || !strings.EqualFold(v.ICAO, icao) {
			continue
		}
		switch v.State {
		case "taxiing", "holding short", "lining up":
			n++
		}
	}
	return n
}

// entryChoices are the intersections of runway end rwy at icao a departure
// of model may be given, grouped by place along the runway, the first after
// the full length first; within a place the entry nearest p first. Each
// leaves the class's runway (IntersectionRule) and, where the airport lists
// its entries, is one of them.
func entryChoices(g *airport.Graph, icao, rwy, model string, p airport.LatLon) [][]airport.RunwayEntry {
	minLeft, full := traffic.IntersectionRule(model)
	if full {
		return nil
	}
	entries, err := g.RunwayEntries(rwy)
	if err != nil {
		return nil
	}
	allowed, listed := traffic.IntersectionEntries(icao, rwy, model)
	var ok []airport.RunwayEntry
	for _, e := range entries {
		if e.Taxiway == "" || e.FromThreshold < math.Max(airport.FullLengthMeters, intersectionInMeters) || e.Remaining < minLeft {
			continue
		}
		if listed && !slices.ContainsFunc(allowed, func(a string) bool { return strings.EqualFold(a, e.Taxiway) }) {
			continue
		}
		ok = append(ok, e)
	}
	slices.SortStableFunc(ok, func(a, b airport.RunwayEntry) int { return cmp.Compare(a.FromThreshold, b.FromThreshold) })
	var groups [][]airport.RunwayEntry
	for _, e := range ok {
		if n := len(groups); n > 0 && e.FromThreshold-groups[n-1][0].FromThreshold <= entryGroupMeters {
			groups[n-1] = append(groups[n-1], e)
			continue
		}
		groups = append(groups, []airport.RunwayEntry{e})
	}
	for _, grp := range groups {
		slices.SortStableFunc(grp, func(a, b airport.RunwayEntry) int {
			return cmp.Compare(distTo(g, a, p), distTo(g, b, p))
		})
	}
	return groups
}

func distTo(g *airport.Graph, e airport.RunwayEntry, p airport.LatLon) float64 {
	q := g.Nodes[e.Node].Position
	return calc.HaversineMeters(p.Lat, p.Lon, q.Lat, q.Lon)
}

// firstEntry is the intersection of runway end rwy a departure of model at
// p would be given: the first place after the full length (entryChoices),
// the entry there nearest p; "" none.
func firstEntry(g *airport.Graph, icao, rwy, model string, p airport.LatLon) string {
	if groups := entryChoices(g, icao, rwy, model, p); len(groups) > 0 {
		return groups[0][0].Taxiway
	}
	return ""
}

// entryLeft is the runway left from entry of runway end rwy (meters), 0
// unknown.
func entryLeft(g *airport.Graph, rwy, entry string) float64 {
	entries, err := g.RunwayEntries(rwy)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if strings.EqualFold(e.Taxiway, entry) {
			return e.Remaining
		}
	}
	return 0
}

// offerEntry gives a departure planned for the full length an intersection
// when the queue for the full length is long enough (offersIntersection):
// the first place along the runway whose own queue is empty, re-planned
// there before its taxi clearance, which then names it. Its crew may
// decline it (entryDeclined, answered with the clearance).
func (it *controlled) offerEntry() {
	if it.dep == nil || it.graph == nil {
		return
	}
	it.mu.Lock()
	v := it.view
	it.mu.Unlock()
	if v.Entry != "" || v.Manual {
		return
	}
	queue := it.cc.fullLengthQueue(it.ICAO, v.Runway, it)
	if !offersIntersection(v.Model, queue) {
		return
	}
	for _, grp := range entryChoices(it.graph, it.ICAO, v.Runway, v.Model, v.Position) {
		busy := it.cc.entryQueue(it.ICAO, v.Runway, it, func(entry string) bool {
			return slices.ContainsFunc(grp, func(e airport.RunwayEntry) bool { return strings.EqualFold(e.Taxiway, entry) })
		})
		if busy > 0 {
			continue // its own queue: no gain
		}
		for _, e := range grp {
			if err := it.cc.do(func() error { return it.dep.ChangeEntry(e.Taxiway) }); err != nil {
				continue // too short for it, or no route
			}
			declined := rand.Float64() < traffic.DeclineChance(v.Model, it.Tail)
			it.mu.Lock()
			it.setRoute()
			it.view.Entry, it.entryRemaining, it.entryDeclined = e.Taxiway, e.Remaining, declined
			it.mu.Unlock()
			it.cc.changed("control")
			it.cc.log.printf("%-6s ground: intersection %s for the queue (%d for the full length of %s, %.0f m left)", it.Tail, e.Taxiway, queue, v.Runway, e.Remaining)
			return
		}
	}
}

// declineEntry has the crew decline the intersection it was just cleared
// to (offerEntry drew it): "unable intersection, request full length", and
// the departure re-planned to the full length. It reports whether it did.
func (it *controlled) declineEntry() bool {
	it.mu.Lock()
	declined := it.entryDeclined
	it.entryDeclined = false
	it.mu.Unlock()
	if !declined || it.dep == nil {
		return false
	}
	it.say(traffic.UnableIntersection(it.Tail))
	if err := it.cc.do(func() error { return it.dep.ChangeEntry("") }); err != nil {
		it.cc.log.printf("%-6s crew: unable intersection, full length refused: %v", it.Tail, err)
		return false
	}
	it.mu.Lock()
	it.setRoute()
	it.view.Entry, it.entryRemaining = "", 0
	it.mu.Unlock()
	it.cc.changed("control")
	it.cc.log.printf("%-6s crew: unable intersection, full length", it.Tail)
	return true
}

// withRemaining has a taxi clearance to an intersection say the runway left
// from it where the airport says so (traffic.SayRemaining), in metres
// rounded down to 50.
func (it *controlled) withRemaining(tx traffic.Transmission) traffic.Transmission {
	it.mu.Lock()
	entry, left := it.view.Entry, it.entryRemaining
	it.mu.Unlock()
	if entry == "" || left <= 0 || tx.Intent != traffic.IntentTaxi || tx.Params[traffic.ParamEntry] == "" || !traffic.SayRemaining(it.ICAO) {
		return tx
	}
	p := make(map[string]string, len(tx.Params)+1)
	for k, v := range tx.Params {
		p[k] = v
	}
	p[traffic.ParamRemaining] = strconv.Itoa(int(math.Floor(left/50) * 50))
	tx.Params = p
	return traffic.Say(tx)
}
