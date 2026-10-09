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

// roundAircraft is the way from a to b round the aircraft at pose: the
// points to pass between them (none when the straight leg is clear), the
// shortest of going by either wingtip, abeam the nose and the tail.
func roundAircraft(a, b airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
	if f.clear(a, b) {
		return nil
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
	for _, side := range []float64{-1, 1} {
		front := f.from(side*wide, roundAheadM)
		back := f.from(side*wide, -(wb + tail + roundWideM))
		for _, via := range [][]airport.LatLon{{front}, {back}, {back, front}, {front, back}} {
			pts := append(append([]airport.LatLon{a}, via...), b)
			d, ok := 0.0, true
			for i := 1; i < len(pts) && ok; i++ {
				ok = f.clear(pts[i-1], pts[i])
				d += localDist(pts[i-1], pts[i])
			}
			if ok && d < bestD {
				best, bestD = via, d
			}
		}
	}
	return best
}
