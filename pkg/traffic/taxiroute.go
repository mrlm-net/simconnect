//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ErrShortRoute is returned for a route that does not start at a parking spot
// or has no taxi segment after it.
var ErrShortRoute = errors.New("traffic: route must start at a parking spot and continue onto a taxiway")

// TaxiWaypoints converts a departure route from airport.Graph.RouteToRunway
// into an AI waypoint chain:
//
//   - one REVERSE leg from the stand straight back to the taxiway junction
//     (MSFS AI cannot steer while reversing);
//   - forward taxi legs along the route, starting TurnInMeters or more from
//     the junction so the AI can turn onto the taxiway, split to at most
//     MaxWaypointSpacingMeters, slowing to TurnSpeedKts / SharpTurnSpeedKts
//     before turns and to HoldShortApproachSpeedKts near the end;
//   - the last waypoint is the route's final point (the hold-short).
//
// The route must start at a parking node followed by at least two taxi points.
func TaxiWaypoints(g *airport.Graph, r *airport.Route) ([]types.SIMCONNECT_DATA_WAYPOINT, error) {
	if len(r.Nodes) < 3 || g.Nodes[r.Nodes[0]].Kind != airport.NodeParking {
		return nil, ErrShortRoute
	}
	alt := convert.MetersToFeet(g.Layout.Altitude)
	pts := r.Points

	// MSFS AI cannot steer while reversing: any bend in a reverse leg makes it
	// spin or turn round and drive forward. So the pushback is one straight
	// leg along the stand axis (the PARKING path) to the taxiway junction, and
	// the aircraft turns onto the taxiway going forward.
	junction := pts[1]
	wps := []types.SIMCONNECT_DATA_WAYPOINT{PushbackWaypoint(junction.Lat, junction.Lon, alt, PushbackSpeedKts)}

	// Forward taxi from the junction onwards, over the simplified route. Speed
	// at each waypoint is the speed wanted when reaching it, so it reflects
	// the turn at that point.
	fwd := turnIn(simplify(pts[1:]))
	total := routeLength(fwd)
	done := 0.0
	for i := 0; i < len(fwd)-1; i++ {
		a, b := fwd[i], fwd[i+1]
		seg := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
		n := int(math.Ceil(seg / MaxWaypointSpacingMeters))
		if i == 0 {
			n = 1 // the turn-in leg: no waypoint inside the turn
		}
		if n < 1 {
			n = 1
		}
		turn := 0.0
		if i+2 < len(fwd) {
			turn = turnAngle(a, b, fwd[i+2])
		}
		for k := 1; k <= n; k++ {
			f := float64(k) / float64(n)
			p := airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: a.Lon + (b.Lon-a.Lon)*f}
			speed := TaxiSpeedKts
			if k == n {
				speed = turnSpeed(turn)
			}
			if total-(done+seg*f) <= HoldShortApproachMeters {
				speed = math.Min(speed, HoldShortApproachSpeedKts)
			}
			wps = append(wps, TaxiWaypoint(p.Lat, p.Lon, alt, speed))
		}
		done += seg
	}
	return wps, nil
}

// turnIn drops the forward route points closer than TurnInMeters to the
// junction (fwd[0]), so the first forward waypoint is far enough away for the
// AI to turn from its pushback heading onto the taxiway without circling.
func turnIn(fwd []airport.LatLon) []airport.LatLon {
	j := fwd[0]
	for i := 1; i < len(fwd)-1; i++ {
		if calc.HaversineMeters(j.Lat, j.Lon, fwd[i].Lat, fwd[i].Lon) >= TurnInMeters {
			return append([]airport.LatLon{j}, fwd[i:]...)
		}
	}
	return []airport.LatLon{j, fwd[len(fwd)-1]}
}

// simplify drops route points that lie on a straight line: a point is kept
// where the route bends by at least SimplifyAngleDeg, where the distance from
// the last kept point would otherwise exceed MaxWaypointSpacingMeters, and at
// both ends. Taxi networks place points every few meters; a waypoint at each
// makes MSFS AI crawl.
func simplify(pts []airport.LatLon) []airport.LatLon {
	if len(pts) <= 2 {
		return pts
	}
	out := []airport.LatLon{pts[0]}
	for i := 1; i < len(pts)-1; i++ {
		last := out[len(out)-1]
		next := pts[i+1]
		bend := turnAngle(pts[i-1], pts[i], next)
		if bend >= SimplifyAngleDeg || calc.HaversineMeters(last.Lat, last.Lon, next.Lat, next.Lon) > MaxWaypointSpacingMeters ||
			turnAngle(last, pts[i], next) >= SimplifyAngleDeg {
			out = append(out, pts[i])
		}
	}
	return append(out, pts[len(pts)-1])
}

// LineUpWaypoints returns the waypoints that take an aircraft holding short at
// the end of r onto the runway and down it for take-off: onto the centreline
// abeam the hold-short, LineUpAlignMeters along the runway heading, then the
// TakeoffClimb chain. The first non-ground waypoint starts the take-off roll.
//
// The aircraft departs from the point abeam its hold-short (an intersection
// departure when the hold-short is down the runway from the threshold).
func LineUpWaypoints(g *airport.Graph, r *airport.Route) ([]types.SIMCONNECT_DATA_WAYPOINT, error) {
	rwy, end, ok := g.Layout.RunwayEnd(r.RunwayEnd)
	if !ok || len(r.Points) == 0 {
		return nil, errors.New("traffic: route has no runway end")
	}
	alt := convert.MetersToFeet(g.Layout.Altitude)
	hold := r.Points[len(r.Points)-1]

	// Project the hold-short onto the runway centreline, measured from this
	// end's threshold along its heading.
	t := end.Threshold
	dist := calc.HaversineMeters(t.Lat, t.Lon, hold.Lat, hold.Lon)
	brg := calc.BearingDegrees(t.Lat, t.Lon, hold.Lat, hold.Lon)
	along := dist * math.Cos((brg-end.Heading)*math.Pi/180)
	along = math.Max(0, math.Min(along, rwy.Length-LineUpAlignMeters))

	entryLat, entryLon := calc.DisplaceByHeading(t.Lat, t.Lon, end.Heading, along)
	alignLat, alignLon := calc.DisplaceByHeading(t.Lat, t.Lon, end.Heading, along+LineUpAlignMeters)
	wps := []types.SIMCONNECT_DATA_WAYPOINT{
		TaxiWaypoint(entryLat, entryLon, alt, LineUpSpeedKts),
		LineupWaypoint(alignLat, alignLon, alt),
	}
	return append(wps, TakeoffClimb(alignLat, alignLon, end.Heading)...), nil
}

// turnSpeed returns the speed for reaching a point where the route turns by
// angle degrees.
func turnSpeed(angle float64) float64 {
	switch {
	case angle >= SharpTurnAngleDeg:
		return SharpTurnSpeedKts
	case angle >= TurnAngleDeg:
		return TurnSpeedKts
	}
	return TaxiSpeedKts
}

// turnAngle is the heading change at b when travelling a → b → c, 0–180°.
func turnAngle(a, b, c airport.LatLon) float64 {
	h1 := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
	h2 := calc.BearingDegrees(b.Lat, b.Lon, c.Lat, c.Lon)
	d := math.Abs(math.Mod(h2-h1+540, 360) - 180)
	return d
}

func routeLength(pts []airport.LatLon) float64 {
	sum := 0.0
	for i := 1; i < len(pts); i++ {
		sum += calc.HaversineMeters(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
	}
	return sum
}
