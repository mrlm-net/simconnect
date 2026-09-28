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
		pos: pos, hdg: hdg, at: now,
		nose: prof.WheelbaseMeters*pushNoseFactor - prof.RefAheadMeters,
		tail: tail + prof.RefAheadMeters,
	}
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
