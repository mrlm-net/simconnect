package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Most DME arcs meet the final 8-10 NM out, inside the point where an
// arrival lines up (ProcedureAlignNm before the join): flown to its end,
// such an arc went back out and round 150° (CYPG VOR/DME 13R via SETPI,
// #1646). An arrival flies an arc's points only where they make its way no
// worse than the straight leg does: no more reversals, crossings of itself
// or climbs.
const (
	planReversalDeg  = 135.0
	planCornerMeters = 200.0
	planClimbFt      = 1.0
)

// planShape is what an arc mustn't make worse in an arrival's plan.
type planShape struct{ reversals, crossings, climbs int }

func planShapeOf(p *ArrivalProcedure) planShape {
	pts := []airport.LatLon{{Lat: p.Spawn.Latitude, Lon: p.Spawn.Longitude}}
	var s planShape
	alt := p.Spawn.Altitude
	for _, w := range p.Waypoints {
		pts = append(pts, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude})
		if w.Altitude > alt+planClimbFt {
			s.climbs++
		}
		alt = w.Altitude
	}
	last, have := 0.0, false
	for i := 1; i < len(pts); i++ {
		for j := 1; j+1 < i; j++ {
			if segmentsCross(pts[j-1], pts[j], pts[i-1], pts[i]) {
				s.crossings++
			}
		}
		a, b := pts[i-1], pts[i]
		if calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon) < planCornerMeters {
			continue
		}
		h := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
		if have && math.Abs(headingDiff(last, h)) > planReversalDeg {
			s.reversals++
		}
		last, have = h, true
	}
	return s
}

func (s planShape) worseThan(o planShape) bool {
	return s.reversals > o.reversals || s.crossings > o.crossings || s.climbs > o.climbs
}

// withoutArcs is route with the DME arcs keep doesn't hold left straight,
// as if there were none: their points dropped, the intercept before one
// back at its Straight. An arc is numbered from 0 in route's order; keep
// nil holds none.
func withoutArcs(route []airport.NavPoint, keep []bool) []airport.NavPoint {
	var out []airport.NavPoint
	k := -1
	for i, n := range route {
		if n.DMEArc {
			if i == 0 || !route[i-1].DMEArc {
				k++
			}
			if k >= len(keep) || !keep[k] {
				if n.Straight == (airport.LatLon{}) {
					continue
				}
				n.Position, n.DMEArc, n.Straight = n.Straight, false, airport.LatLon{}
			}
		}
		out = append(out, n)
	}
	return out
}

// soundArcRoute is route as an arrival to end flies it: each DME arc that
// would make its plan (planArrival) worse than with the arcs straight is
// left straight (withoutArcs). route itself when all are sound.
func soundArcRoute(route []airport.NavPoint, end airport.RunwayEnd, joinMeters, joinFt float64) []airport.NavPoint {
	arcs := 0
	for i, n := range route {
		if n.DMEArc && (i == 0 || !route[i-1].DMEArc) {
			arcs++
		}
	}
	if arcs == 0 {
		return route
	}
	out := withoutArcs(route, nil)
	straight, err := planArrival(out, end, joinMeters, joinFt)
	if err != nil {
		return out
	}
	most := planShapeOf(straight)
	sound := func(r []airport.NavPoint) bool {
		p, err := planArrival(r, end, joinMeters, joinFt)
		return err == nil && !planShapeOf(p).worseThan(most)
	}
	if sound(route) {
		return route
	}
	if arcs == 1 {
		return out
	}
	keep := make([]bool, arcs)
	for k := range keep {
		keep[k] = true
		if r := withoutArcs(route, keep); sound(r) {
			out = r
		} else {
			keep[k] = false
		}
	}
	return out
}
