package world

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Taxiing round an aircraft in the way: a departure cleared to push, pushing
// or pushed back waiting for its taxi takes its taxiway (and the push path)
// for a while. One taxiing on a route that passes it too near for the two
// wings to clear is given another route that keeps clear, when there is one
// it fits (any airport: the span check of the route search; at LKPR a
// pushback onto JB leaves J beside it too narrow, a CRJ goes by JO). Without
// one it keeps its route and gives way as before.

// keepClearEvery: how often the places taken are looked at.
const keepClearEvery = time.Second

// keepClear re-plans the taxiing aircraft round the places taken when they
// change. On the connection's goroutine (tick).
func (cc *controlCenter) keepClear() {
	if now := cc.clock.Now(); now.Sub(cc.keepClearAt) < keepClearEvery {
		return
	} else {
		cc.keepClearAt = now
	}
	items := cc.snapshotItems()
	type taken struct {
		it *controlled
		o  airport.Occupied
	}
	var occ []taken
	var key strings.Builder
	for _, it := range items {
		if it.dep == nil {
			continue
		}
		if o, ok := it.dep.Occupies(); ok {
			occ = append(occ, taken{it, o})
			fmt.Fprintf(&key, "%s/%d/%d;", it.Tail, it.dep.State(), len(o.Points))
		}
	}
	if key.String() == cc.keepClearKey {
		return
	}
	cc.keepClearKey = key.String()
	if len(occ) == 0 {
		return
	}
	for _, it := range items {
		if it.dep == nil && it.arr == nil {
			continue
		}
		it.mu.Lock()
		manual := it.view.Manual
		it.mu.Unlock()
		if manual {
			continue // the user's routes stay as given
		}
		var mine []airport.Occupied
		for _, t := range occ {
			if t.it != it && t.it.ICAO == it.ICAO {
				mine = append(mine, t.o)
			}
		}
		if len(mine) == 0 {
			continue
		}
		it.mu.Lock()
		before := it.rerouteSaid().Text
		it.mu.Unlock()
		if !it.avoidOccupied(mine) {
			continue
		}
		it.mu.Lock()
		it.setRoute()
		tx := it.rerouteSaid()
		it.view.Instruction, it.view.InstructionAt = "taxi round traffic", nil
		it.mu.Unlock()
		tlog.printf("%-6s ground: new route round traffic in the way: %s", it.Tail, tx.Text)
		// Said only when the taxiways changed: the same "via J, H, A" again
		// is no new clearance (live, SWR1216 heard it twice in 19 s).
		if tx.Text != "" && tx.Text != before {
			it.say(tx)
		}
	}
}

// occupiedFor is the places other departures at its airport take now.
func (cc *controlCenter) occupiedFor(it *controlled) []airport.Occupied {
	var out []airport.Occupied
	for _, o := range cc.snapshotItems() {
		if o == it || o.dep == nil || o.ICAO != it.ICAO {
			continue
		}
		if p, ok := o.dep.Occupies(); ok {
			out = append(out, p)
		}
	}
	return out
}

// avoidOccupied is the controller's AvoidOccupied.
func (it *controlled) avoidOccupied(occ []airport.Occupied) bool {
	if it.dep != nil {
		return it.dep.AvoidOccupied(occ)
	}
	return it.arr.AvoidOccupied(occ)
}

// rerouteSaid is ground's new taxi instruction for the route from where the
// aircraft is (it.mu held).
func (it *controlled) rerouteSaid() traffic.Transmission {
	var r *airport.Route
	if it.dep != nil {
		r = it.dep.Route()
	} else if p := it.arr.Plan(); p != nil {
		r = p.Route
	}
	if r == nil || len(r.Points) == 0 {
		return traffic.Transmission{}
	}
	near, best := 0, math.Inf(1)
	for i, p := range r.Points {
		if d := (p.Lat-it.view.Position.Lat)*(p.Lat-it.view.Position.Lat) + (p.Lon-it.view.Position.Lon)*(p.Lon-it.view.Position.Lon); d < best {
			near, best = i, d
		}
	}
	ahead, err := it.graph.RouteFromNodes(r.Nodes[near:])
	if err != nil {
		return traffic.Transmission{}
	}
	via := ahead.SpokenTaxiways(-1)
	if it.dep != nil {
		return traffic.ClearedTaxiToRunway(it.Tail, it.view.Runway, r.Entry, via)
	}
	return traffic.ClearedTaxiToStand(it.Tail, it.view.Stand, via)
}

// snapshotItems is the controlled aircraft now.
func (cc *controlCenter) snapshotItems() []*controlled {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	out := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		out = append(out, it)
	}
	return out
}

// climbOn has a departure handed to MSFS AI climb on to the level
// departure cleared it to (radarFt), past the top of its SID: said alone,
// the climb waypoints still ended there (live, EZY516 level at FL100).
func (it *controlled) climbOn() {
	it.mu.Lock()
	ft, pos, alt, id := it.radarFt, it.view.Position, it.heightFt, it.objectID
	it.mu.Unlock()
	if ft <= 0 {
		ft = departureClimbFt
	}
	if it.graph != nil {
		alt += it.graph.Layout.Altitude / 0.3048
	}
	for _, a := range it.cc.world.Aircraft() { // where it is now, if seen
		if a.ObjectID == id && id != 0 {
			pos, alt = a.Position, a.AltFt
		}
	}
	if err := it.cc.do(func() error { return it.dep.ClimbTo(pos, alt, ft) }); err != nil {
		it.cc.log.printf("%-6s climb to %.0f ft not applied: %v", it.Tail, ft, err)
	}
}
