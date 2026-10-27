package airspace

import (
	"errors"
	"math"
)

// ErrNoOutline: the record has no outline to build (no start or circle
// vector, Airspace.Broken).
var ErrNoOutline = errors.New("airspace: no outline")

// metersPerDeg is a degree of latitude in metres.
const metersPerDeg = 111320.0

// arcStepRad and circleSteps: arcs are flattened every 2°, circles into 72
// sides (a 3 NM circle's sides are 0.2 NM).
const (
	arcStepRad  = 2 * math.Pi / 180
	circleSteps = 72
)

// Point is a position in degrees. In a Shape's rings the longitude runs on
// past ±180 where an outline crosses the antimeridian.
type Point struct{ Lat, Lon float64 }

// Shape is an airspace's outline flattened into rings: lines as they
// are, arcs every 2°, circles as 72-sided polygons. A point inside an odd
// number of rings is inside.
type Shape struct {
	Airspace *Airspace
	Rings    [][]Point
	// per ring: its longitude range, for the point test across ±180
	lonMin, lonMax []float64
}

// Shape builds the outline of a.
func (a *Airspace) Shape() (*Shape, error) {
	s := &Shape{Airspace: a}
	var (
		ring    []Point
		origins = map[int]Point{}
		origin  Point
		haveOrg bool
	)
	add := func(p Point) {
		if n := len(ring); n > 0 {
			p.Lon = near(p.Lon, ring[n-1].Lon)
		}
		ring = append(ring, p)
	}
	flush := func() {
		if len(ring) >= 3 {
			s.addRing(ring)
		}
		ring = nil
	}
	for _, v := range a.Vectors {
		p := Point{Lat: v.Lat, Lon: v.Lon}
		switch v.Type {
		case VectorStart:
			flush()
			add(p)
		case VectorLine:
			add(p)
		case VectorOrigin:
			origins[v.OriginID], origin, haveOrg = p, p, true
		case VectorArcCW, VectorArcCCW:
			c, ok := origins[v.OriginID]
			if !ok {
				c, ok = origin, haveOrg
			}
			if !ok || len(ring) == 0 {
				add(p)
				continue
			}
			for _, q := range arc(c, ring[len(ring)-1], p, v.Type == VectorArcCW) {
				add(q)
			}
		case VectorCircle:
			if !haveOrg || v.Radius <= 0 {
				continue
			}
			flush()
			for _, q := range circle(origin, v.Radius) {
				add(q)
			}
			flush()
		}
	}
	flush()
	if len(s.Rings) == 0 {
		return nil, ErrNoOutline
	}
	return s, nil
}

func (s *Shape) addRing(r []Point) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range r {
		lo, hi = math.Min(lo, p.Lon), math.Max(hi, p.Lon)
	}
	s.Rings = append(s.Rings, r)
	s.lonMin, s.lonMax = append(s.lonMin, lo), append(s.lonMax, hi)
}

// Inside reports that lat/lon is inside the outline, at any altitude.
func (s *Shape) Inside(lat, lon float64) bool {
	in := false
	for i, r := range s.Rings {
		for _, l := range [3]float64{lon, lon + 360, lon - 360} {
			if l >= s.lonMin[i] && l <= s.lonMax[i] {
				if inRing(r, lat, l) {
					in = !in
				}
				break
			}
		}
	}
	return in
}

// Contains reports that a point at altM metres MSL, over ground at groundM
// metres MSL, is inside the outline and from the floor to the ceiling
// (AGL limits over that ground).
func (s *Shape) Contains(lat, lon, altM, groundM float64) bool {
	a := s.Airspace
	return altM >= a.Floor(groundM) && altM <= a.Ceiling(groundM) && s.Inside(lat, lon)
}

// inRing is the even-odd ray test in degrees.
func inRing(r []Point, lat, lon float64) bool {
	in := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		a, b := r[i], r[j]
		if (a.Lat > lat) != (b.Lat > lat) && lon < (b.Lon-a.Lon)*(lat-a.Lat)/(b.Lat-a.Lat)+a.Lon {
			in = !in
		}
	}
	return in
}

// arc runs round c from p to e (clockwise or not), every arcStepRad; the
// radius goes from p's distance to e's, so it ends on e.
func arc(c, p, e Point, cw bool) []Point {
	kx := math.Cos(c.Lat * math.Pi / 180)
	pl, el := near(p.Lon, c.Lon), near(e.Lon, c.Lon)
	r0 := math.Hypot((pl-c.Lon)*kx, p.Lat-c.Lat)
	r1 := math.Hypot((el-c.Lon)*kx, e.Lat-c.Lat)
	a0 := math.Atan2((pl-c.Lon)*kx, p.Lat-c.Lat) // compass: from north, clockwise
	a1 := math.Atan2((el-c.Lon)*kx, e.Lat-c.Lat)
	var d float64
	if cw {
		d = mod2pi(a1 - a0)
	} else {
		d = -mod2pi(a0 - a1)
	}
	n := int(math.Ceil(math.Abs(d) / arcStepRad))
	if n < 1 || kx < 1e-9 {
		return []Point{e}
	}
	out := make([]Point, 0, n)
	for i := 1; i < n; i++ {
		f := float64(i) / float64(n)
		a, r := a0+d*f, r0+(r1-r0)*f
		out = append(out, Point{Lat: c.Lat + r*math.Cos(a), Lon: c.Lon + r*math.Sin(a)/kx})
	}
	return append(out, e)
}

// circle is a circleSteps-sided polygon of radiusM round c.
func circle(c Point, radiusM float64) []Point {
	kx := math.Cos(c.Lat * math.Pi / 180)
	r := radiusM / metersPerDeg
	out := make([]Point, circleSteps)
	for i := range out {
		a := 2 * math.Pi * float64(i) / circleSteps
		out[i] = Point{Lat: c.Lat + r*math.Cos(a), Lon: c.Lon + r*math.Sin(a)/math.Max(kx, 1e-9)}
	}
	return out
}

// near is lon moved by whole turns to within 180° of ref.
func near(lon, ref float64) float64 {
	for lon-ref > 180 {
		lon -= 360
	}
	for lon-ref < -180 {
		lon += 360
	}
	return lon
}

func mod2pi(a float64) float64 {
	return math.Mod(math.Mod(a, 2*math.Pi)+2*math.Pi, 2*math.Pi)
}
