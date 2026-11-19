package world

import (
	"slices"
	"strconv"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A stand-off: two of ours on the ground, each stopped for the other
// (traffic ahead, holding for it as oncoming) for stoppedLogAfter. Ground
// breaks it: one is given another route round the other, where it stands
// and the way it means to go, when there is one it fits; first the one
// that reported, then the other. It must not last (#1574: XPLY, a single
// apron lane; AFR919 and KLM1817 nose to nose, each "traffic ahead" the
// other, for good). Without a way round it is logged once, for the cause
// (a push or a taxi released into it) to be found.

// standOffAheadM: the other's way ahead kept off in the re-route.
const standOffAheadM = 150.0

// standOff looks, once it has stood stoppedLogAfter for by, whether the
// aircraft by names stands stopped for this one, and breaks it.
func (it *controlled) standOff(by string) {
	m := objectIDs.FindString(by)
	if m == "" {
		return
	}
	id, err := strconv.ParseUint(m[1:], 10, 32)
	if err != nil {
		return
	}
	o := it.cc.byObject(uint32(id))
	me := it.object()
	if o == nil || o == it || me == 0 {
		return
	}
	o.mu.Lock()
	theirs := o.stoppedBy
	o.mu.Unlock()
	if !slices.Contains(objectIDs.FindAllString(theirs, -1), "#"+strconv.FormatUint(uint64(me), 10)) {
		return
	}
	// Once a pair: the reasons each stands for come and go (traffic ahead,
	// holding for it), each logged anew.
	key := "stand-off " + o.Tail
	it.mu.Lock()
	tried := it.spoken[key]
	it.spoken[key] = true
	it.mu.Unlock()
	o.mu.Lock()
	tried = tried || o.spoken["stand-off "+it.Tail]
	o.spoken["stand-off "+it.Tail] = true
	o.mu.Unlock()
	if tried {
		return
	}
	it.cc.log.printf("%-6s ground: stand-off with %s — a way round looked for", it.Tail, o.Tail)
	cc := it.cc
	go cc.reroute(it, []airport.Occupied{o.takenOnGround()}, "a stand-off with "+o.Tail, func(done bool) {
		if !done {
			go cc.reroute(o, []airport.Occupied{it.takenOnGround()}, "a stand-off with "+it.Tail, func(done bool) {
				if !done {
					cc.log.printf("%-6s ground: stand-off with %s — no way round", it.Tail, o.Tail)
				}
			})
		}
	})
}

// takenOnGround is where it stands on the ground and the first
// standOffAheadM of the way it means to go.
func (it *controlled) takenOnGround() airport.Occupied {
	it.mu.Lock()
	pos, model := it.view.Position, it.view.Model
	it.mu.Unlock()
	pts := []airport.LatLon{pos}
	if _, way, _, ok := it.groundWay(); ok {
		pts = append(pts, thinned(firstMeters(way, standOffAheadM))...)
	}
	return airport.Occupied{Points: pts, HalfSpan: traffic.MotionProfileFor(model).SpanMeters / 2}
}
