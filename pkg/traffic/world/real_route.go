package world

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A real arrival given its route (#845): it flies the route from where it
// is, and joins a STAR of the runway in use where the route meets it
// (passes within routeMeetNM of one of its points up to the initial
// approach fix), at the point with the shortest way in. A route that meets
// none is left for the direct join, as without one.

// routeMeetNM: a route passing this close to a STAR point meets it there.
const routeMeetNM = 5.0

// starJoin is a point an arrival may join a STAR at: the STAR from there,
// its name, the approach expected, and the way along it to its end.
type starJoin struct {
	pts          []airport.NavPoint
	name, expect string
	restNM       float64
}

// routeJoin is where a route meets a STAR: the route points flown before
// leaving it for the join.
type routeJoin struct {
	flown []traffic.PathPoint
	join  starJoin
}

// joinAlong picks the join of joins that route (from pos) meets with the
// shortest way in: along the route to its point nearest the join, across,
// then along the STAR. False when it meets none.
func joinAlong(pos airport.LatLon, route []traffic.PathPoint, joins []starJoin) (routeJoin, bool) {
	path := routeAhead(pos, route)
	if len(path) == 0 || len(joins) == 0 {
		return routeJoin{}, false
	}
	pt := func(i int) airport.LatLon {
		if i == 0 {
			return pos
		}
		return airport.LatLon{Lat: path[i-1].Lat, Lon: path[i-1].Lon}
	}
	n := len(path) + 1 // pos, then the route
	best, bestNM, at := routeJoin{}, math.Inf(1), -1
	along := 0.0
	for i := 0; i+1 < n; i++ {
		a, b := pt(i), pt(i+1)
		seg := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
		for _, j := range joins {
			p := j.pts[0].Position
			x := math.Max(0, math.Min(seg*1852, calc.AlongTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon))) / 1852
			lat, lon := calc.DisplaceByHeading(a.Lat, a.Lon, calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon), x*1852)
			off := calc.HaversineNM(lat, lon, p.Lat, p.Lon)
			if off > routeMeetNM {
				continue
			}
			if nm := along + x + off + j.restNM; nm < bestNM {
				best, bestNM, at = routeJoin{join: j}, nm, i
			}
		}
		along += seg
	}
	if at < 0 {
		return routeJoin{}, false
	}
	best.flown = path[:at] // the points up to the segment it leaves from
	return best, true
}

// routeAhead is route from the point after the one nearest pos: the part
// still to fly.
func routeAhead(pos airport.LatLon, route []traffic.PathPoint) []traffic.PathPoint {
	near, nearNM := -1, math.Inf(1)
	for i, p := range route {
		if d := calc.HaversineNM(pos.Lat, pos.Lon, p.Lat, p.Lon); d < nearNM {
			near, nearNM = i, d
		}
	}
	if near < 0 {
		return nil
	}
	// The nearest point still ahead (within 90° of the way on) is kept.
	if near+1 < len(route) {
		p, q := route[near], route[near+1]
		toP := calc.BearingDegrees(pos.Lat, pos.Lon, p.Lat, p.Lon)
		on := calc.BearingDegrees(p.Lat, p.Lon, q.Lat, q.Lon)
		if math.Abs(math.Mod(toP-on+540, 360)-180) < 90 && nearNM > 0.5 {
			return route[near:]
		}
	}
	return route[near+1:]
}
