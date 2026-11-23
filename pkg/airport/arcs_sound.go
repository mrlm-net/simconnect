package airport

import (
	"math"
	"slices"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// A DME arc is drawn and flown only where that makes the way no worse than
// the straight leg does (#1646): an arc that would make the procedure cross
// itself, reverse or go round once more (its join to the next leg a loop)
// stays straight.
const (
	// arcReversalDeg: a corner of more between a procedure's points is a
	// reversal.
	arcReversalDeg = 135.0
	// arcLoopDeg: a path that turns this much more in all has gained a loop.
	arcLoopDeg = 300.0
	// arcCornerMeters: a step shorter than this has no heading to judge a
	// corner by.
	arcCornerMeters = 200.0
)

// wayShape is what an arc mustn't make worse.
type wayShape struct {
	crossings [5]int // of the points, and of the drawn path as it is and smoothed at both radii
	reversals int    // of the points
	turn      [2]float64
}

func shapeOf(legs []Leg, start LatLon, alt, magVar float64) wayShape {
	var s wayShape
	var pts []LatLon
	if start != (LatLon{}) {
		pts = append(pts, start)
	}
	for _, n := range resolveLegsWith(legs, start, alt, magVar) {
		pts = append(pts, n.Position)
	}
	s.crossings[0], s.reversals = selfCrossings(pts), reversals(pts)
	for i, r := range [2]float64{TurnRadiusEnroute, TurnRadiusApproach} {
		drawn := procedurePath(legs, start, alt, magVar, r)
		var xy [][2]float64
		for _, p := range drawn {
			xy = append(xy, [2]float64{p.Lat, p.Lon})
		}
		s.turn[i] = arcTurn(xy)
		s.crossings[1+2*i] = selfCrossings(drawn)
		s.crossings[2+2*i] = selfCrossings(SmoothPath(drawn, r, nil))
	}
	return s
}

func (s wayShape) worseThan(o wayShape) bool {
	for i := range s.crossings {
		if s.crossings[i] > o.crossings[i] {
			return true
		}
	}
	for i := range s.turn {
		if s.turn[i] > o.turn[i]+arcLoopDeg {
			return true
		}
	}
	return s.reversals > o.reversals
}

// soundArcs is legs with each DME arc (an AF leg without ArcCenter) that
// would make the way worse than the straight legs kept straight: its Rho
// cleared. legs itself when there are none or all are sound. Drawn
// (ProcedurePath) and flown (resolveLegs) take the same ones.
func soundArcs(legs []Leg, start LatLon, alt, magVar float64) []Leg {
	var arcs []int
	for i, l := range legs {
		if l.Type == types.SIMCONNECT_FACILITY_LEG_TYPE_AF && l.ArcCenter == (LatLon{}) && l.Origin != (LatLon{}) && l.Rho > 0 {
			arcs = append(arcs, i)
		}
	}
	if len(arcs) == 0 {
		return legs
	}
	out := slices.Clone(legs)
	for _, i := range arcs {
		out[i].Rho = 0
	}
	straight := shapeOf(out, start, alt, magVar)
	if !shapeOf(legs, start, alt, magVar).worseThan(straight) {
		return legs
	}
	if len(arcs) == 1 {
		return out
	}
	for _, i := range arcs {
		out[i].Rho = legs[i].Rho
		if shapeOf(out, start, alt, magVar).worseThan(straight) {
			out[i].Rho = 0
		}
	}
	return out
}

// selfCrossings is how often pts cross themselves (flat about the first).
func selfCrossings(pts []LatLon) int {
	type xy struct{ x, y float64 }
	var q []xy
	for i, p := range pts {
		if i > 0 && calc.HaversineMeters(pts[i-1].Lat, pts[i-1].Lon, p.Lat, p.Lon) < 1 {
			continue
		}
		q = append(q, xy{math.Mod(p.Lon-pts[0].Lon+540, 360) - 180, p.Lat - pts[0].Lat})
	}
	side := func(a, b, c xy) float64 { return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x) }
	n := 0
	for i := 0; i+3 < len(q); i++ {
		a, b := q[i], q[i+1]
		x0, x1 := min(a.x, b.x), max(a.x, b.x)
		for j := i + 2; j+1 < len(q); j++ {
			c, d := q[j], q[j+1]
			if c.x < x0 && d.x < x0 || c.x > x1 && d.x > x1 {
				continue
			}
			if side(a, b, c)*side(a, b, d) < 0 && side(c, d, a)*side(c, d, b) < 0 {
				n++
			}
		}
	}
	return n
}

// reversals is how many of pts' corners are reversals (arcReversalDeg).
func reversals(pts []LatLon) int {
	n := 0
	var last float64
	have := false
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		if calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon) < arcCornerMeters {
			continue
		}
		h := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
		if have && math.Abs(math.Mod(h-last+540, 360)-180) > arcReversalDeg {
			n++
		}
		last, have = h, true
	}
	return n
}
