package traffic

import (
	"math"
	"slices"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A push keeps its wingtips, tail and stabiliser clear of the aircraft
// parked on the stands beside it, not only of the stands' circles: an
// aircraft overhangs its circle at the tail and the push's wing sweeps
// between the sampled points (live LKPR: TVS139 pushed from B9 for 06
// ended with its wing over A7's stand line, 1.8 m from an A321 parked
// there; B10 beside it). Every stand counts, taken or not: one empty when
// the push is planned can be taken while it is flown.
const (
	// pushWingtipClearanceMeters: the clearance a push keeps from an
	// aircraft parked beside it (ICAO Annex 14 3.13.6, code C: 4.5 m), or
	// what the stand already gives at its start, if less.
	pushWingtipClearanceMeters = 4.5
	// pushCloseCost: meters of push a meter of clearance short of it costs:
	// a push with clearance wins over a shorter one without (LKPR B9 for
	// 06: onto B1 facing north-west), one without is still taken where
	// nothing else is.
	pushCloseCost = 150.0
	// parkedSpanFactor, parkedWingAft, parkedStabSpan: a parked aircraft
	// sized by its stand (radius r): fuselage across the circle, 2r long,
	// span parkedSpanFactor·r, wingtips parkedWingAft of its length behind
	// the nose, stabiliser parkedStabSpan of the span (an A321 on a 22 m
	// stand: 44 m, 36 m).
	parkedSpanFactor = 1.64
	parkedWingAft    = 0.55
	parkedStabSpan   = 0.36
)

// outlineSeg is a segment of an aircraft's outline, flat meters.
type outlineSeg struct{ ax, ay, bx, by float64 }

// pushOutlines are the outlines of the aircraft that may park beside the
// push from own, in f's flat meters; nor own, nor the stands overlapping
// it (nobody parks there while own is taken).
func pushOutlines(g *airport.Graph, own int, f *flatPave) [][3]outlineSeg {
	skip := g.Layout.ParkingConflicts(own)
	var out [][3]outlineSeg
	for _, p := range g.Layout.Parking {
		if p.Index == own || slices.Contains(skip, p.Index) || p.Radius <= 0 || localDist(p.Position, f.c) >= 200 {
			continue
		}
		l, span := 2*p.Radius, parkedSpanFactor*p.Radius
		nose := offsetHeading(p.Position, p.Heading, p.Radius)
		wing := offsetHeading(nose, p.Heading+180, parkedWingAft*l)
		stab := offsetHeading(nose, p.Heading+180, 0.92*l)
		out = append(out, [3]outlineSeg{
			f.seg(nose, offsetHeading(nose, p.Heading+180, l)),
			f.seg(offsetHeading(wing, p.Heading-90, span/2), offsetHeading(wing, p.Heading+90, span/2)),
			f.seg(offsetHeading(stab, p.Heading-90, parkedStabSpan*span/2), offsetHeading(stab, p.Heading+90, parkedStabSpan*span/2)),
		})
	}
	return out
}

func (f *flatPave) seg(a, b airport.LatLon) outlineSeg {
	ax, ay := f.xy(a)
	bx, by := f.xy(b)
	return outlineSeg{ax, ay, bx, by}
}

// moverOutline is the pushed aircraft's outline, its main gear at gear
// facing hdg: fuselage, wing and stabiliser.
func moverOutline(f *flatPave, gear airport.LatLon, hdg float64, prof MotionProfile) [3]outlineSeg {
	span, tail := prof.SpanMeters, prof.TailMeters
	if span <= 0 {
		span = 35.8
	}
	if tail <= 0 {
		tail = 20.5
	}
	nose := offsetHeading(gear, hdg, prof.WheelbaseMeters*pushNoseFactor)
	wing := offsetHeading(gear, hdg, 2)
	stab := offsetHeading(gear, hdg+180, tail-3)
	return [3]outlineSeg{
		f.seg(nose, offsetHeading(gear, hdg+180, tail)),
		f.seg(offsetHeading(wing, hdg-90, span/2), offsetHeading(wing, hdg+90, span/2)),
		f.seg(offsetHeading(stab, hdg-90, parkedStabSpan*span/2), offsetHeading(stab, hdg+90, parkedStabSpan*span/2)),
	}
}

// outlineClearance is the least distance (meters) between the aircraft at
// gear facing hdg and the parked ones.
func outlineClearance(f *flatPave, parked [][3]outlineSeg, gear airport.LatLon, hdg float64, prof MotionProfile) float64 {
	mv := moverOutline(f, gear, hdg, prof)
	best := math.Inf(1)
	for _, p := range parked {
		for _, a := range mv {
			for _, b := range p {
				best = math.Min(best, segDist(a, b))
			}
		}
	}
	return best
}

// pushClearance is the least clearance (meters) from the parked aircraft
// along the push pts (main gear points, the stand first).
func pushClearance(f *flatPave, parked [][3]outlineSeg, pts []airport.LatLon, prof MotionProfile) float64 {
	best := math.Inf(1)
	for i := 2; i < len(pts); i += 2 {
		best = math.Min(best, outlineClearance(f, parked, pts[i], localBearing(pts[i], pts[i-2]), prof))
	}
	return best
}

// segDist is the least distance between two segments; 0 if they cross.
func segDist(p, q outlineSeg) float64 {
	cross := func(ox, oy, ax, ay, bx, by float64) float64 { return (ax-ox)*(by-oy) - (ay-oy)*(bx-ox) }
	d1, d2 := cross(q.ax, q.ay, q.bx, q.by, p.ax, p.ay), cross(q.ax, q.ay, q.bx, q.by, p.bx, p.by)
	d3, d4 := cross(p.ax, p.ay, p.bx, p.by, q.ax, q.ay), cross(p.ax, p.ay, p.bx, p.by, q.bx, q.by)
	if d1*d2 < 0 && d3*d4 < 0 {
		return 0
	}
	pt := func(x, y float64, s outlineSeg) float64 {
		dx, dy := s.bx-s.ax, s.by-s.ay
		t := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			t = math.Max(0, math.Min(1, ((x-s.ax)*dx+(y-s.ay)*dy)/l))
		}
		return math.Hypot(x-s.ax-t*dx, y-s.ay-t*dy)
	}
	return math.Min(math.Min(pt(p.ax, p.ay, q), pt(p.bx, p.by, q)), math.Min(pt(q.ax, q.ay, p), pt(q.bx, q.by, p)))
}

// closePenalty is what the push pts costs for coming closer than want to
// the parked aircraft (pushCloseCost a meter).
func closePenalty(f *flatPave, parked [][3]outlineSeg, pts []airport.LatLon, prof MotionProfile, want float64) float64 {
	if len(parked) == 0 {
		return 0
	}
	return math.Max(0, want-pushClearance(f, parked, pts, prof)) * pushCloseCost
}
