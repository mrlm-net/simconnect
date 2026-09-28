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
		pos: pos, hdg: hdg, at: now, ahead: p.aircraft[id].ahead, half: p.aircraft[id].half,
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
		e.ahead, e.half = ahead, half
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
	type other struct {
		id uint32
		e  groundEntry
	}
	p.mu.Lock()
	var others []other
	for oid, e := range p.aircraft {
		if oid != id && len(e.ahead) > 0 && now.Sub(e.at) <= TrafficStaleAfter {
			others = append(others, other{oid, e})
		}
	}
	p.mu.Unlock()
	best := math.Inf(1)
	if len(others) == 0 {
		return best
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
		if mineTo < 0 || mineTo < half {
			continue // no conflict, or already in it: go on through
		}
		theirsTo := first(o.e.ahead, mine, reach)
		if theirsTo < mineTo || (theirsTo == mineTo && o.id < id) {
			best = math.Min(best, from+mineTo)
		}
	}
	return best
}

// Forget drops aircraft id (airborne, parked for good, removed).
func (p *GroundPicture) Forget(id uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.aircraft, id)
}

// blocking is how far along path (between from and from+look) the nearest
// point of another aircraft's body lies within half meters of it: the
// aircraft to stop behind. +Inf when the way is free. Reports older than
// TrafficStaleAfter are ignored.
func (p *GroundPicture) blocking(id uint32, path *GroundPath, from, look, half float64, now time.Time) float64 {
	p.mu.Lock()
	others := make([]groundEntry, 0, len(p.aircraft))
	for oid, e := range p.aircraft {
		if oid != id && now.Sub(e.at) <= TrafficStaleAfter {
			others = append(others, e)
		}
	}
	p.mu.Unlock()
	if len(others) == 0 {
		return math.Inf(1)
	}
	// The path between from and from+look, as sample points with distances.
	pts, cum := path.pts, path.cum
	lo := 0
	for lo < len(cum)-1 && cum[lo+1] < from {
		lo++
	}
	best := math.Inf(1)
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
					best = s
				}
			}
		}
	}
	return best
}
