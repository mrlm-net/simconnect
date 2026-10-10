package world

import (
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Intersection departures for the queue (traffic ideas): with departures
// queuing for a runway's full length, ground gives one that needs less
// runway an intersection nearer it with its taxi clearance, so it does not
// join the queue (LKPR 24 at B or F). Heavies keep the full length; the
// runway left must be long enough for the type (ChangeEntry checks).

// IntersectionQueue: with this many departures already queuing for the
// full length of a runway (holding short, lining up, or taxiing there), the
// next one is offered an intersection.
const IntersectionQueue = 2

// offersIntersection: a departure of model is offered an intersection with
// queue departures ahead of it for the full length.
func offersIntersection(model string, queue int) bool {
	if queue < IntersectionQueue {
		return false
	}
	switch traffic.WakeFor(model).ICAO {
	case traffic.WakeHeavy, traffic.WakeSuper:
		return false
	}
	return true
}

// fullLengthQueue counts our departures for icao's runway rwy from its full
// length, on the ground and not yet rolling, but for except.
func (cc *controlCenter) fullLengthQueue(icao, rwy string, except *controlled) int {
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
		if v.Done || !v.OnGround || v.Entry != "" || v.Runway != rwy || !strings.EqualFold(v.ICAO, icao) {
			continue
		}
		switch v.State {
		case "taxiing", "holding short", "lining up":
			n++
		}
	}
	return n
}

// nearestEntry is the named intersection of runway end rwy nearest p (not
// the full length); "" none.
func nearestEntry(g *airport.Graph, rwy string, p airport.LatLon) string {
	entries, err := g.RunwayEntries(rwy)
	if err != nil {
		return ""
	}
	best, bestD := "", math.Inf(1)
	for _, e := range entries {
		if e.Taxiway == "" || e.FromThreshold < airport.FullLengthMeters {
			continue
		}
		q := g.Nodes[e.Node].Position
		if d := calc.HaversineMeters(p.Lat, p.Lon, q.Lat, q.Lon); d < bestD {
			best, bestD = e.Taxiway, d
		}
	}
	return best
}

// offerEntry gives a departure planned for the full length an intersection
// when the queue for the full length is long enough (offersIntersection):
// re-planned there before its taxi clearance, which then names it.
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
	e := nearestEntry(it.graph, v.Runway, v.Position)
	if e == "" {
		return
	}
	if err := it.cc.do(func() error { return it.dep.ChangeEntry(e) }); err != nil {
		return // too short for it, or no route: the full length
	}
	it.mu.Lock()
	it.setRoute()
	it.view.Entry = e
	it.mu.Unlock()
	it.cc.changed("control")
	it.cc.log.printf("%-6s ground: intersection %s for the queue (%d for the full length of %s)", it.Tail, e, queue, v.Runway)
}
