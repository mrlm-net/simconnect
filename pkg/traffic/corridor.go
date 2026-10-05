package traffic

import (
	"math"
	"math/rand/v2"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Traffic along a route (#740): aircraft placed around the user's flight
// in cruise, so the sky on its way is not empty — one ahead going the
// same way, one coming the other way, one crossing. Each is a RoutePoint
// chain for EnrouteStart: created airborne, flown by MSFS AI.

// CorridorKind is how an aircraft flies against the route.
type CorridorKind string

const (
	CorridorSame     CorridorKind = "same"     // ahead, the same way, 2000 ft above or below
	CorridorOpposite CorridorKind = "opposite" // ahead, coming the other way, 1000 ft above or below
	CorridorCrossing CorridorKind = "crossing" // across the route ahead, 1000 or 2000 ft above or below
)

// CorridorOptions are the user's flight: its route ahead (two or more
// points, in its direction), where it is, its level and speed.
type CorridorOptions struct {
	Route   []airport.LatLon
	At      airport.LatLon
	LevelFt float64
	Kts     float64 // 0: 450
}

// Corridor distances (NM along the route from the user): the same-way
// aircraft appears CorridorAheadNM ahead (and up to 20 more), the
// opposite one CorridorOppositeNM, the crossing point CorridorCrossNM,
// its aircraft CorridorCrossInNM before it on its own track.
const (
	CorridorAheadNM    = 25.0
	CorridorOppositeNM = 70.0
	CorridorCrossNM    = 45.0
	CorridorCrossInNM  = 35.0
)

// CorridorRoute is the way of an aircraft of kind around the user's
// flight: where it appears (the first point) and on; false when the route
// ahead is too short for it.
func CorridorRoute(kind CorridorKind, o CorridorOptions, rng *rand.Rand) ([]RoutePoint, bool) {
	if len(o.Route) < 2 {
		return nil, false
	}
	kts := o.Kts
	if kts <= 0 {
		kts = 450
	}
	line := newPolyline(o.Route)
	s0 := line.project(o.At)
	side := float64(1 - 2*rng.IntN(2)) // above or below
	switch kind {
	case CorridorSame:
		start := s0 + CorridorAheadNM + 20*rng.Float64()
		if start+30 > line.length {
			return nil, false
		}
		alt := o.LevelFt + side*2000
		speed := kts + 30*(rng.Float64()-0.5)
		var out []RoutePoint
		for _, s := range line.from(start) {
			out = append(out, RoutePoint{Position: line.at(s), AltFt: alt, Kts: speed})
		}
		return out, len(out) >= 2
	case CorridorOpposite:
		start := math.Min(s0+CorridorOppositeNM+30*rng.Float64(), line.length)
		if start-s0 < 40 {
			return nil, false
		}
		alt := o.LevelFt + side*1000
		speed := kts + 30*(rng.Float64()-0.5)
		// Back along the route, past the user and on.
		var out []RoutePoint
		for _, s := range line.back(start) {
			out = append(out, RoutePoint{Position: line.at(s), AltFt: alt, Kts: speed})
		}
		return out, len(out) >= 2
	case CorridorCrossing:
		cross := s0 + CorridorCrossNM + 25*rng.Float64()
		if cross > line.length {
			return nil, false
		}
		p, track := line.at(cross), line.bearing(cross)
		// Across at 60–120° to the route, from either side.
		hdg := math.Mod(track+side*(60+60*rng.Float64())+360, 360)
		lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, hdg+180, CorridorCrossInNM*1852)
		alt := o.LevelFt + float64(1-2*rng.IntN(2))*float64(1000+1000*rng.IntN(2))
		speed := kts + 40*(rng.Float64()-0.5)
		return []RoutePoint{{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: alt, Kts: speed}, {Position: p, AltFt: alt, Kts: speed}}, true
	}
	return nil, false
}

// polyline is a route measured along its length (NM).
type polyline struct {
	pts    []airport.LatLon
	cum    []float64
	length float64
}

func newPolyline(pts []airport.LatLon) polyline {
	l := polyline{pts: pts, cum: make([]float64, len(pts))}
	for i := 1; i < len(pts); i++ {
		l.cum[i] = l.cum[i-1] + calc.HaversineNM(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
	}
	l.length = l.cum[len(pts)-1]
	return l
}

// at is the point s NM along the line (clamped to it).
func (l polyline) at(s float64) airport.LatLon {
	s = math.Max(0, math.Min(s, l.length))
	for i := 1; i < len(l.pts); i++ {
		if s <= l.cum[i] || i == len(l.pts)-1 {
			seg := l.cum[i] - l.cum[i-1]
			f := 0.0
			if seg > 0 {
				f = (s - l.cum[i-1]) / seg
			}
			a, b := l.pts[i-1], l.pts[i]
			return airport.LatLon{Lat: a.Lat + f*(b.Lat-a.Lat), Lon: a.Lon + f*(b.Lon-a.Lon)}
		}
	}
	return l.pts[len(l.pts)-1]
}

// bearing is the line's track at s.
func (l polyline) bearing(s float64) float64 {
	a, b := l.at(s-1), l.at(s+1)
	return calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
}

// project is how far along the line the point nearest p is (sampled every
// NM).
func (l polyline) project(p airport.LatLon) float64 {
	best, bestD := 0.0, math.Inf(1)
	for s := 0.0; s <= l.length; s++ {
		q := l.at(s)
		if d := calc.HaversineNM(p.Lat, p.Lon, q.Lat, q.Lon); d < bestD {
			best, bestD = s, d
		}
	}
	return best
}

// from is s, then the line's corners after it and its end.
func (l polyline) from(s float64) []float64 {
	out := []float64{s}
	for _, c := range l.cum {
		if c > s+1 {
			out = append(out, c)
		}
	}
	return out
}

// back is s, then the corners before it back to the start.
func (l polyline) back(s float64) []float64 {
	out := []float64{s}
	for i := len(l.cum) - 1; i >= 0; i-- {
		if l.cum[i] < s-1 {
			out = append(out, l.cum[i])
		}
	}
	return out
}
