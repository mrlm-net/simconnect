package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A tug's straight leg between the vehicle roads and an aircraft's nose
// (driving in, and home after the push) runs round the aircraft instead of
// through it (live, LROP: from a road behind the tails a tug drove through
// the fuselage to the nose). The aircraft is kept clear as its fuselage, nose
// to tail, and its wing with the engines, each with a margin; a leg into it
// goes round the wingtip.

const (
	keepFuselageHalfM = 4.0  // the fuselage's half width and margin
	keepWingChordM    = 7.0  // the wing band either side of the main gear
	keepMarginM       = 3.0  // beyond the wingtips and the tail
	roundWideM        = 8.0  // a detour's lateral clearance off the wingtip
	roundAheadM       = 20.0 // and ahead of the nose
)

// aircraftFrame is an aircraft standing at pose: points in metres right of
// its centre line and forward of its nose gear.
type aircraftFrame struct {
	nose airport.LatLon
	hdg  float64
	prof MotionProfile
}

func (f aircraftFrame) to(p airport.LatLon) (right, fwd float64) {
	kx := metersPerDegree * math.Cos(f.nose.Lat*math.Pi/180)
	x, y := (p.Lon-f.nose.Lon)*kx, (p.Lat-f.nose.Lat)*metersPerDegree
	h := f.hdg * math.Pi / 180
	return x*math.Cos(h) - y*math.Sin(h), x*math.Sin(h) + y*math.Cos(h)
}

func (f aircraftFrame) from(right, fwd float64) airport.LatLon {
	return offsetHeading(offsetHeading(f.nose, f.hdg, fwd), f.hdg+90, right)
}

// inside reports whether p is on the aircraft or within its margin.
func (f aircraftFrame) inside(p airport.LatLon) bool {
	x, y := f.to(p)
	wb, tail := f.prof.WheelbaseMeters, f.prof.TailMeters
	if wb <= 0 {
		wb = 13
	}
	if tail <= 0 {
		tail = 2 * wb
	}
	if math.Abs(x) <= keepFuselageHalfM && y <= 6 && y >= -(wb+tail+keepMarginM) {
		return true
	}
	half := f.prof.SpanMeters/2 + keepMarginM
	return math.Abs(x) <= half && y <= -(wb-keepWingChordM) && y >= -(wb+keepWingChordM)
}

// clear reports whether the leg a → b keeps off the aircraft (checked
// every 2 m). A leg starting within the margin (a road just behind the
// tail) may leave it, not come back in.
func (f aircraftFrame) clear(a, b airport.LatLon) bool {
	d := localDist(a, b)
	n := int(d/2) + 1
	leaving := f.inside(a)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		p := airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*t, Lon: a.Lon + (b.Lon-a.Lon)*t}
		in := f.inside(p)
		if in && !leaving {
			return false
		}
		leaving = leaving && in
	}
	return true
}

// roundRoute is route kept off the aircraft at pose: the points of it under the
// aircraft dropped (a vehicle road can run under a parked aircraft: LKPR N51,
// LOWW F21) and a way round it wherever a leg crosses it. The ends stay.
func roundRoute(route []airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	return roundRouteOn(nil, route, pose, prof)
}

// serviceWay is a service vehicle's way route cleaned (cleanWay), round
// the aircraft at pose off the other stands of l (roundRouteOn) and
// without knots (nearWay).
func serviceWay(l *airport.Layout, route []airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	return nearWay(roundRouteOn(l, cleanWay(route), pose, prof))
}

// roundRouteOn is roundRoute with its corners off the other stands of l
// (nil: any), where a way round keeps off them (#1630: LROP 120A's tail
// corner fell on stand 120B's parking point).
func roundRouteOn(l *airport.Layout, route []airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
	kept := make([]airport.LatLon, 0, len(route))
	for i, p := range route {
		if i == 0 || i == len(route)-1 || !f.inside(p) {
			kept = append(kept, p)
		}
	}
	out := kept[:1:1]
	for i := 1; i < len(kept); i++ {
		// Weighing the turns onto and off the way round with the legs
		// either side (#1630: the shortest went to a corner and straight
		// back, a hairpin).
		var prev, next *airport.LatLon
		if len(out) >= 2 {
			prev = &out[len(out)-2]
		}
		if i+1 < len(kept) {
			next = &kept[i+1]
		}
		via := roundAircraftOn(l, prev, kept[i-1], kept[i], next, pose, prof)
		out = append(append(out, via...), kept[i])
		// Off a corner of the way round onto a point it turns back from
		// (LKPR C19's stairs: round the front corner, on to the road's
		// join ahead of the nose and back west along the road): straight
		// on to the point after it where that is clear.
		if len(via) > 0 && next != nil {
			c := via[len(via)-1]
			turn := math.Abs(headingDiff(localBearing(c, kept[i]), localBearing(kept[i], *next)))
			if turn > wayTurnBackDeg && f.clear(c, *next) {
				out = out[:len(out)-1]
			}
		}
	}
	return out
}

// roundAircraft is the way from a to b round the aircraft at pose: the
// points to pass between them (none when the straight leg is clear), the
// shortest of going by either wingtip, abeam the nose and the tail.
func roundAircraft(a, b airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	return roundAircraftTurns(nil, a, b, nil, pose, prof)
}

// roundAircraftTurns is roundAircraft choosing by the way's length with
// its turns priced (wayCost), from prev (nil: none) before a to next after
// b.
func roundAircraftTurns(prev *airport.LatLon, a, b airport.LatLon, next *airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	return roundAircraftOn(nil, prev, a, b, next, pose, prof)
}

// roundAircraftOn is roundAircraftTurns keeping its corners off the other
// stands of l where some way round does (nil: any).
func roundAircraftOn(l *airport.Layout, prev *airport.LatLon, a, b airport.LatLon, next *airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	if l != nil {
		if via, ok := roundAircraftPick(l, prev, a, b, next, pose, prof); ok {
			return via
		}
	}
	via, _ := roundAircraftPick(nil, prev, a, b, next, pose, prof)
	return via
}

// roundAircraftPick is the way round of roundAircraftOn, false when none
// keeps off the aircraft (and the stands, l not nil).
func roundAircraftPick(l *airport.Layout, prev *airport.LatLon, a, b airport.LatLon, next *airport.LatLon, pose GroundPose, prof MotionProfile) ([]airport.LatLon, bool) {
	f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
	if f.clear(a, b) {
		return nil, true
	}
	wb, tail := prof.WheelbaseMeters, prof.TailMeters
	if wb <= 0 {
		wb = 13
	}
	if tail <= 0 {
		tail = 2 * wb
	}
	wide := prof.SpanMeters/2 + roundWideM
	var best []airport.LatLon
	bestD := math.Inf(1)
	_, yb := f.to(b)
	for _, side := range []float64{-1, 1} {
		front := f.from(side*wide, roundAheadM)
		back := f.from(side*wide, -(wb + tail + roundWideM))
		vias := [][]airport.LatLon{{front}, {back}, {back, front}, {front, back}}
		// Abeam an end beside the aircraft (#1630, LROP 119K's stairs: in
		// from behind to a door, round the front corner and back, a hairpin):
		// along the side to level with it, then across.
		// Across, round the nose or the tail, to the other side (LROP
		// 107R's tug, backed off to the nose's right, its road on the
		// left: under the fuselage).
		front2, back2 := f.from(-side*wide, roundAheadM), f.from(-side*wide, -(wb+tail+roundWideM))
		vias = append(vias, []airport.LatLon{front, front2}, []airport.LatLon{back, back2})
		if abeam := f.from(side*wide, yb); yb > -(wb+tail+roundWideM) && yb < roundAheadM {
			vias = append(vias, []airport.LatLon{back, abeam}, []airport.LatLon{front, abeam}, []airport.LatLon{abeam})
			vias = append(vias, []airport.LatLon{front2, front, abeam}, []airport.LatLon{back2, back, abeam})
		}
		for _, via := range vias {
			pts := append(append([]airport.LatLon{a}, via...), b)
			ok := true
			for i := 1; i < len(pts) && ok; i++ {
				ok = f.clear(pts[i-1], pts[i]) && (l == nil || !l.VehicleAcrossStand(pose.Position, pts[i-1], pts[i], standOnWayMeters))
			}
			if prev != nil {
				pts = append([]airport.LatLon{*prev}, pts...)
			}
			if next != nil {
				pts = append(pts, *next)
			}
			if d := wayCost(pts, noHeading); ok && d < bestD {
				best, bestD = via, d
			}
		}
	}
	return best, best != nil
}
