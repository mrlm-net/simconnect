//go:build windows
// +build windows

package traffic

import (
	"math"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// GroundPicture is what the injected aircraft on the ground know of each
// other (#334). Each controller reports its aircraft every frame; while
// taxiing, each looks ahead along its own path for another aircraft's body
// in its way — one ahead on the same taxiway, one waiting at the holding
// point, one crossing — and stops behind it at a safe gap, moving on when
// it moves. One picture serves all the controllers at an airport
// (TaxiWithGroundPicture, ArrivalWithGroundPicture).
type GroundPicture struct {
	mu       sync.Mutex
	aircraft map[uint32]groundEntry
}

type groundEntry struct {
	pos        airport.LatLon
	hdg        float64
	nose, tail float64 // meters ahead of and behind the reference point
	at         time.Time
	// Where it will drive next, up to its next stop, every trafficBodyStep
	// meters (ReportPath), and its half-span.
	ahead []airport.LatLon
	half  float64
	// pushing: ahead is a pushback under way, which taxiing traffic gives
	// way to whatever the distances.
	pushing bool
	// waiting: ahead is the way it will taxi once cleared (#452): pushes
	// do not start into it, but it has no priority over moving traffic.
	waiting bool
}

// NewGroundPicture creates an empty picture.
func NewGroundPicture() *GroundPicture {
	return &GroundPicture{aircraft: map[uint32]groundEntry{}}
}

// Report records aircraft id at its reference point pos, heading hdg (true
// degrees), with the airframe of prof.
func (p *GroundPicture) Report(id uint32, pos airport.LatLon, hdg float64, prof MotionProfile, now time.Time) {
	if prof == (MotionProfile{}) {
		prof = DefaultMotionProfile()
	}
	tail := prof.TailMeters
	if tail <= 0 {
		tail = 20.5
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.aircraft[id] = groundEntry{
		pos: pos, hdg: hdg, at: now, ahead: p.aircraft[id].ahead, half: p.aircraft[id].half, pushing: p.aircraft[id].pushing, waiting: p.aircraft[id].waiting,
		nose: prof.WheelbaseMeters*pushNoseFactor - prof.RefAheadMeters,
		tail: tail + prof.RefAheadMeters,
	}
}

// ReportPath records where aircraft id will drive next: its path ahead up
// to its next stop, sampled every trafficBodyStep meters (nil when it is
// not taxiing), and its half-span, for giving way (#334).
func (p *GroundPicture) ReportPath(id uint32, ahead []airport.LatLon, half float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.aircraft[id]; ok {
		e.ahead, e.half, e.pushing, e.waiting = ahead, half, false, false
		p.aircraft[id] = e
	}
}

// ReportPlanned records the way aircraft id will taxi once cleared, while
// it waits for the clearance after its push (#452; LKPR, live: TVS706 was
// cleared to push onto A1 where TVS795, pushed there a moment before, was
// about to taxi; TVS795 then drove through the push). A push does not
// start across it; taxiing traffic does not give way to it.
func (p *GroundPicture) ReportPlanned(id uint32, ahead []airport.LatLon, half float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.aircraft[id]; ok {
		e.ahead, e.half, e.pushing, e.waiting = ahead, half, false, len(ahead) > 0
		p.aircraft[id] = e
	}
}

// ReportPush records the corridor a pushback under way still sweeps
// (nil once done): taxiing traffic whose path crosses it gives way.
func (p *GroundPicture) ReportPush(id uint32, corridor []airport.LatLon, half float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.aircraft[id]; ok {
		e.ahead, e.half, e.pushing, e.waiting = corridor, half, len(corridor) > 0, false
		p.aircraft[id] = e
	}
}

// giveWay is where along path (after from) this aircraft stops to give
// way: the first point within look where its path comes within both
// half-spans (plus GiveWayMarginMeters) of another aircraft's path ahead,
// when the other is closer to that point (a tie goes to the lower ID).
// The one further away waits, the other goes, so exactly one of them
// stops; an aircraft stopping before the point (holding short, at its
// clearance limit, giving way itself) reports no path there and takes no
// priority. +Inf when there is nobody to give way to.
func (p *GroundPicture) giveWay(id uint32, path *GroundPath, from, look, half float64, now time.Time) float64 {
	at, _ := p.giveWayTo(id, path, from, look, half, now)
	return at
}

// pushingNow reports whether aircraft id is pushing back (taxiing traffic
// gives way to it whatever happens).
func (p *GroundPicture) pushingNow(id uint32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.aircraft[id]
	return ok && e.pushing
}

// giveWayTo is giveWay and the aircraft given way to (0 for none).
func (p *GroundPicture) giveWayTo(id uint32, path *GroundPath, from, look, half float64, now time.Time) (float64, uint32) {
	type other struct {
		id uint32
		e  groundEntry
	}
	p.mu.Lock()
	var others []other
	for oid, e := range p.aircraft {
		if oid != id && len(e.ahead) > 0 && !e.waiting && now.Sub(e.at) <= TrafficStaleAfter {
			others = append(others, other{oid, e})
		}
	}
	p.mu.Unlock()
	best, to := math.Inf(1), uint32(0)
	if len(others) == 0 {
		return best, 0
	}
	end := math.Min(path.Length(), from+look)
	var mine []airport.LatLon // this aircraft's path ahead
	for s := from; s <= end; s += trafficBodyStep {
		mine = append(mine, path.PointAt(s))
	}
	// first is how far along a the first point within reach of b lies
	// (-1 for none): measured the same way for both aircraft, so both
	// come to the same decision.
	first := func(a, b []airport.LatLon, reach float64) float64 {
		for i, q := range a {
			for _, r := range b {
				if localDist(q, r) <= reach {
					return float64(i) * trafficBodyStep
				}
			}
		}
		return -1
	}
	for _, o := range others {
		reach := half + o.e.half + GiveWayMarginMeters
		mineTo := first(mine, o.e.ahead, reach)
		// Beside a push under way only an aircraft already close enough for
		// the push to stop for it (corridorBlocked: both half-spans and
		// PushClearMarginMeters) goes on through, or each would wait for the
		// other; one merely within the margin waits where it is (#452:
		// TVS795, waiting at the end of TVS706's corridor, drove through it).
		inIt := mineTo == 0 && (!o.e.pushing || first(mine[:1], o.e.ahead, half+o.e.half+PushClearMarginMeters) == 0)
		if mineTo < 0 || mineTo < half && (!o.e.pushing || inIt) {
			continue // no conflict, or already in it: go on through
		}
		// (Beside a push under way it waits where it is unless already in its
		// corridor: at LKPR DLH977, waiting at the edge of TVS1960's push,
		// drove into it; each then stopped for the other for 8 minutes.)
		theirsTo := first(o.e.ahead, mine, reach)
		if !o.e.pushing {
			theirsTo += trafficBodyStep // their path starts a step ahead of them (ReportPath)
		}
		if o.e.pushing || theirsTo < mineTo || (theirsTo == mineTo && o.id < id) {
			if from+mineTo < best {
				best, to = from+mineTo, o.id
			}
		}
	}
	return best, to
}

// corridorBlocked reports another aircraft in the way of a pushback along
// corridor (points the pushing aircraft sweeps, with half its span): its
// fuselage within half+PushClearMarginMeters of the corridor, or its path
// ahead within both half-spans plus GiveWayMarginMeters — traffic taxiing
// behind the stand. Parked neighbours are a stand spacing away and do not
// count. It returns the first such aircraft's ID.
func (p *GroundPicture) corridorBlocked(id uint32, corridor []airport.LatLon, half float64, withPaths bool, now time.Time) (uint32, bool) {
	p.mu.Lock()
	type other struct {
		id uint32
		e  groundEntry
	}
	var others []other
	for oid, e := range p.aircraft {
		if oid != id && now.Sub(e.at) <= TrafficStaleAfter {
			others = append(others, other{oid, e})
		}
	}
	p.mu.Unlock()
	near := func(q airport.LatLon, reach float64) bool {
		for _, c := range corridor {
			if localDist(q, c) <= reach {
				return true
			}
		}
		return false
	}
	for _, o := range others {
		oh := o.e.half
		if oh <= 0 {
			oh = DefaultHalfSpanMeters
		}
		// A moving aircraft (pushing, taxiing) keeps its wings clear too: at
		// LKPR DLH1740 pushed from A3 while TVS1823, pushed up A1, was a
		// fuselage and 3 m from its corridor, a wing inside it (#446). A
		// parked neighbour is a stand spacing away, wing to wing.
		// Only before a push starts (withPaths): under way a push stops for a
		// body in the way, and an aircraft giving way to it, stopped short
		// with a little path left, would hold it for ever while it waits for
		// the push (#466; LKPR, live: AFR657 and AFR1246).
		reach := half + PushClearMarginMeters
		if withPaths && len(o.e.ahead) > 0 {
			reach += oh
		}
		for d := -o.e.tail; d <= o.e.nose+0.01; d += trafficBodyStep {
			if near(offsetHeading(o.e.pos, o.e.hdg, d), reach) {
				return o.id, true
			}
		}
		if !withPaths {
			continue // a push under way stops for bodies only; others give way to it
		}
		// Not started yet: it waits for the way ahead of taxiing traffic and
		// for the rest of a neighbour's push — two pushes into the same
		// corridor would each stop for the other's body and stay there.
		for _, r := range o.e.ahead {
			if near(r, half+oh+GiveWayMarginMeters) {
				return o.id, true
			}
		}
	}
	return 0, false
}

// Forget drops aircraft id (airborne, parked for good, removed).
func (p *GroundPicture) Forget(id uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.aircraft, id)
}

// blocking is how far along path (between from and from+look) the nearest
// point of another aircraft's body lies within half meters of it: the
// aircraft to stop behind, returned too. +Inf when the way is free. Reports
// older than TrafficStaleAfter are ignored.
func (p *GroundPicture) blocking(id uint32, path *GroundPath, from, look, half float64, now time.Time) (float64, groundEntry) {
	p.mu.Lock()
	others := make([]groundEntry, 0, len(p.aircraft))
	for oid, e := range p.aircraft {
		if oid != id && now.Sub(e.at) <= TrafficStaleAfter {
			others = append(others, e)
		}
	}
	p.mu.Unlock()
	if len(others) == 0 {
		return math.Inf(1), groundEntry{}
	}
	// The path between from and from+look, as sample points with distances.
	pts, cum := path.pts, path.cum
	lo := 0
	for lo < len(cum)-1 && cum[lo+1] < from {
		lo++
	}
	best, who := math.Inf(1), groundEntry{}
	for _, o := range others {
		// Points along the other aircraft's axis, nose to tail.
		for d := -o.tail; d <= o.nose+0.01; d += trafficBodyStep {
			q := offsetHeading(o.pos, o.hdg, d)
			for i := lo; i+1 < len(pts) && cum[i] <= from+look; i++ {
				h := localBearing(pts[i], pts[i+1])
				seg := cum[i+1] - cum[i]
				along := math.Max(0, math.Min(seg, alongHeading(pts[i], h, q)))
				s := cum[i] + along
				if s < from || s >= best {
					continue
				}
				if localDist(q, offsetHeading(pts[i], h, along)) <= half {
					best, who = s, o
				}
			}
		}
	}
	return best, who
}
