package world

import (
	"slices"
	"strings"

	"fmt"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"math"
	"time"

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

// A real overflight (#845): it crosses the area (overflightRadiusNM round
// the picture's centre) along its route, else straight on along its track,
// and is gone once out.

// realOverflightPath is the way a real overflight at pos (at altFt) crosses
// the area round centre: its route's points ahead up to the first one out
// of the area, else one point straight on along trackDeg where it leaves.
func realOverflightPath(centre, pos airport.LatLon, altFt, trackDeg float64, route []traffic.PathPoint) []traffic.PathPoint {
	out := overflightRadiusNM + 10.0
	var path []traffic.PathPoint
	for _, p := range routeAhead(pos, route) {
		if p.AltFt <= 0 {
			p.AltFt = altFt
		}
		path = append(path, p)
		if calc.HaversineNM(centre.Lat, centre.Lon, p.Lat, p.Lon) > out {
			return path
		}
	}
	if len(path) > 0 {
		return path // the route ends in the area: it goes there
	}
	for d := 5.0; d <= 2*out+5; d += 5 {
		lat, lon := calc.DisplaceByHeading(pos.Lat, pos.Lon, trackDeg, d*1852)
		if calc.HaversineNM(centre.Lat, centre.Lon, lat, lon) > out {
			return []traffic.PathPoint{{Lat: lat, Lon: lon, AltFt: altFt}}
		}
	}
	return nil
}

// pathNM is the length of path from pos.
func pathNM(pos airport.LatLon, path []traffic.PathPoint) float64 {
	nm, at := 0.0, pos
	for _, p := range path {
		nm += calc.HaversineNM(at.Lat, at.Lon, p.Lat, p.Lon)
		at = airport.LatLon{Lat: p.Lat, Lon: p.Lon}
	}
	return nm
}

// spawnRealOverflight puts a real overflight in the air where it is now,
// flown by MSFS AI across the area.
func (s *scheduler) spawnRealOverflight(f traffic.ManagedFlight) error {
	cc := s.cc
	centre, ok := cc.world.Centre()
	if !ok {
		return fmt.Errorf("%w: no centre of the area", traffic.ErrSpawnImpossible)
	}
	o := f.Observed
	pos, alt := o.At(time.Now()) // the feed's clock, not the simulator's
	path := realOverflightPath(centre, pos, alt, o.TrackDeg, o.Route)
	if len(path) == 0 {
		return fmt.Errorf("%w: its way does not cross the area", traffic.ErrSpawnImpossible)
	}
	a := s.airlines[f.Airline]
	models := traffic.ModelsForFlight(cc.modelList(), f.Airline, a.Name, f.Type, f.Callsign, 6)
	if len(models) == 0 {
		return fmt.Errorf("no model of a %s", f.Type)
	}
	model := models[(f.Attempts-1)%len(models)]
	kts := math.Max(o.GroundKts, realMinKts)
	e := &enrouteAC{f: f, model: model, cruiseKts: kts}
	route := []traffic.RoutePoint{{Position: pos, AltFt: alt, Kts: traffic.EnrouteSpeedKts(alt, kts)}}
	for _, p := range path {
		route = append(route, traffic.RoutePoint{Position: airport.LatLon{Lat: p.Lat, Lon: p.Lon}, AltFt: p.AltFt, Kts: traffic.EnrouteSpeedKts(p.AltFt, kts)})
	}
	along := "its track"
	if len(o.Route) > 0 {
		along = fmt.Sprintf("its route (%d points)", len(path))
	}
	return s.spawnEnrouteOn(f, e, model, route, along)
}

// flyGiven has a real departure's plan fly route after its SID (#845):
// the SID's points, then route's from the first one past the SID's end, at
// their levels (else the plan's cruise level), held as planned levels are.
func flyGiven(p *planned, route []traffic.PathPoint) {
	if p == nil || p.plan == nil || len(route) == 0 {
		return
	}
	n := 0 // the route points of the SID
	for _, w := range p.plan.Waypoints {
		if w.Kind == nav.PointRunway || w.Kind == nav.PointAirport || w.Kind == nav.PointProfile {
			continue
		}
		if w.Phase != nav.PhaseSID {
			break
		}
		n++
	}
	n = min(n, len(p.route))
	from := airport.LatLon{Lat: p.plan.Request.Departure.Position.Lat, Lon: p.plan.Request.Departure.Position.Lon}
	if n > 0 {
		from = p.route[n-1].Position
	}
	cruise := float64(p.plan.CruiseFL) * 100
	out := slices.Clone(p.route[:n])
	for _, q := range routeAhead(from, route) {
		alt := q.AltFt
		if alt <= 0 {
			alt = cruise
		}
		out = append(out, airport.NavPoint{Position: airport.LatLon{Lat: q.Lat, Lon: q.Lon}, AltMin: alt * 0.3048, AltMax: alt * 0.3048})
	}
	if len(out) > n {
		p.route = out
	}
}

// routeFromText is o's filed route (RouteText) as points over the airways
// known (#845); nil when none of it is known, or an airway does not join
// its fixes (it then flies without one).
func (s *scheduler) routeFromText(cs string, o traffic.Observed) []traffic.PathPoint {
	st := s.st
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.airways == nil {
		return nil
	}
	steps, skipped, err := st.airways.ExpandRoute(o.RouteText, airport.LatLon{Lat: o.Lat, Lon: o.Lon})
	if err != nil {
		s.cc.log.printf("%-6s real: route %q not flown: %v", cs, o.RouteText, err)
		return nil
	}
	if len(skipped) > 0 {
		s.cc.log.printf("%-6s real: route %q: %s not known, passed over", cs, o.RouteText, strings.Join(skipped, " "))
	}
	out := make([]traffic.PathPoint, 0, len(steps))
	for _, st := range steps {
		out = append(out, traffic.PathPoint{Lat: st.Position.Lat, Lon: st.Position.Lon})
	}
	return out
}
