//go:build windows
// +build windows

package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Push-and-turn (#341): where the only taxiway at a stand's junction is
// the one the aircraft taxis out on (LKPR A7, B9), there is no branch to
// push the tail onto. The tug turns the aircraft on the apron instead: the
// main gear follows the shortest path of bounded curvature (a Dubins path)
// from the stand to a pose short of the junction facing along the taxi-out.

// pushTurnPlan returns the main gear points of a push-and-turn from gear
// (pushing towards pushDir, true degrees) to a pose facing along the
// taxi-out ahead (the route from the junction on): preferably past the
// junction on the taxiway itself (LKPR B9: up B2, looping through the open
// apron beside it), otherwise short of it on the taxi-out line. Radii from
// PushbackArcMeters down to PushbackMinArcMeters are tried; of the pushes
// clear of the neighbouring stands and the terminal the cheapest wins;
// nil if none is found.
func pushTurnPlan(g *airport.Graph, own int, gear airport.LatLon, pushDir float64, ahead []airport.LatLon, prof MotionProfile) []airport.LatLon {
	if len(ahead) < 2 {
		return nil
	}
	cum := make([]float64, len(ahead))
	for i := 1; i < len(ahead); i++ {
		cum[i] = cum[i-1] + localDist(ahead[i-1], ahead[i])
	}
	type goal = pushTurnGoal
	var goals []goal
	for x := 20.0; x <= math.Min(pushTurnPastMeters, cum[len(cum)-1]-5); x += 10 {
		dir := localBearing(pointAlong(ahead, cum, x-5), pointAlong(ahead, cum, x+5))
		goals = append(goals, goal{pointAlong(ahead, cum, x), dir, 0})
	}
	jp, out := ahead[0], localBearing(ahead[0], pointAlong(ahead, cum, math.Min(pushRouteLookMeters, cum[len(cum)-1])))
	for d := 0.0; d <= pushTurnBackMeters; d += 5 {
		goals = append(goals, goal{offsetHeading(jp, out+180, d), out, pushTurnShortPenalty})
	}
	base := standIntrusion(g, own, []airport.LatLon{offsetHeading(gear, pushDir+180, 1), gear}, prof)
	pv := pavementAround(g, gear, pushTurnMaxMeters)
	// Within the pavement as modelled (stand circles and taxiway strips); if
	// no push-and-turn fits, a wider tolerance: the apron is wider than the
	// model, and without a push-and-turn the aircraft is pushed straight and
	// left across its way out (LKPR A1, EDDF A16).
	for _, tol := range []float64{pushOffPavementMeters, pushOffPavementWideMeters} {
		if pts := pushTurnSearch(g, own, gear, pushDir, goals, prof, base, pv, tol); pts != nil {
			return pts
		}
	}
	return nil
}

// pushTurnSearch is pushTurnPlan's search with the main gear within tol of
// the pavement pv.
func pushTurnSearch(g *airport.Graph, own int, gear airport.LatLon, pushDir float64, goals []pushTurnGoal, prof MotionProfile, base float64, pv pavement, tol float64) []airport.LatLon {
	var best, clear []airport.LatLon
	bestIn, bestCost := math.Inf(1), math.Inf(1)
	for r := PushbackArcMeters; r >= PushbackMinArcMeters-0.01; r -= 4 {
		for _, gl := range goals {
			end := offsetHeading(gl.nose, gl.dir+180, prof.WheelbaseMeters)
			// Straight back PushStraightMeters first, clear of the stand.
			turn := dubins(offsetHeading(gear, pushDir, PushStraightMeters), pushDir, end, gl.dir+180, r, 0.5)
			if turn == nil {
				continue
			}
			pts := append([]airport.LatLon{gear}, turn...)
			if pathLen(pts) > pushTurnMaxMeters {
				continue
			}
			// Clear of the neighbours: the cheapest push, with a tighter turn
			// costing pushTurnRadiusCost per meter of radius given up.
			if offPavement(pv, pts) > tol {
				continue // off the stands and taxiways: a building or grass
			}
			in := standIntrusion(g, own, pts, prof)
			if in <= base+pushClearanceSlackMeters {
				if cost := pathLen(pts) + pushTurnRadiusCost*(PushbackArcMeters-r) + gl.pen; cost < bestCost {
					clear, bestCost = pts, cost
				}
				continue
			}
			if clear == nil && in < bestIn {
				best, bestIn = pts, in
			}
		}
	}
	if clear != nil {
		return clear
	}
	return best
}

// pushTurnGoal is where a push-and-turn may end: the nose there, facing
// dir, at a cost pen.
type pushTurnGoal struct {
	nose     airport.LatLon
	dir, pen float64
}

// dubins samples the shortest path of turn radius r from a (travelling
// along headA, true degrees) to b (travelling along headB) every step
// meters; nil if none exists.
func dubins(a airport.LatLon, headA float64, b airport.LatLon, headB, r, step float64) []airport.LatLon {
	// Local metres around a, math angles (counter-clockwise from east).
	bx, by := alongHeading(a, 90, b), alongHeading(a, 0, b)
	th0, th1 := (90-headA)*math.Pi/180, (90-headB)*math.Pi/180
	dist := math.Hypot(bx, by) / r
	theta := mod2pi(math.Atan2(by, bx))
	alpha, beta := mod2pi(th0-theta), mod2pi(th1-theta)
	sa, sb, ca, cb := math.Sin(alpha), math.Sin(beta), math.Cos(alpha), math.Cos(beta)
	cab := math.Cos(alpha - beta)
	type word struct {
		kinds   [3]float64 // curvature sign per segment: +1 left, 0 straight, -1 right
		t, p, q float64
		ok      bool
	}
	var words []word
	add := func(k [3]float64, t, p, q float64, ok bool) {
		words = append(words, word{k, t, p, q, ok})
	}
	// LSL
	if tmp := 2 + dist*dist - 2*cab + 2*dist*(sa-sb); tmp >= 0 {
		at := math.Atan2(cb-ca, dist+sa-sb)
		add([3]float64{1, 0, 1}, mod2pi(-alpha+at), math.Sqrt(tmp), mod2pi(beta-at), true)
	}
	// RSR
	if tmp := 2 + dist*dist - 2*cab + 2*dist*(sb-sa); tmp >= 0 {
		at := math.Atan2(ca-cb, dist-sa+sb)
		add([3]float64{-1, 0, -1}, mod2pi(alpha-at), math.Sqrt(tmp), mod2pi(-beta+at), true)
	}
	// LSR
	if tmp := -2 + dist*dist + 2*cab + 2*dist*(sa+sb); tmp >= 0 {
		p := math.Sqrt(tmp)
		at := math.Atan2(-ca-cb, dist+sa+sb) - math.Atan2(-2, p)
		add([3]float64{1, 0, -1}, mod2pi(-alpha+at), p, mod2pi(-mod2pi(beta)+at), true)
	}
	// RSL
	if tmp := dist*dist - 2 + 2*cab - 2*dist*(sa+sb); tmp >= 0 {
		p := math.Sqrt(tmp)
		at := math.Atan2(ca+cb, dist-sa-sb) - math.Atan2(2, p)
		add([3]float64{-1, 0, 1}, mod2pi(alpha-at), p, mod2pi(beta-at), true)
	}
	// RLR
	if tmp := (6 - dist*dist + 2*cab + 2*dist*(sa-sb)) / 8; math.Abs(tmp) <= 1 {
		p := mod2pi(2*math.Pi - math.Acos(tmp))
		t := mod2pi(alpha - math.Atan2(ca-cb, dist-sa+sb) + p/2)
		add([3]float64{-1, 1, -1}, t, p, mod2pi(alpha-beta-t+p), true)
	}
	// LRL
	if tmp := (6 - dist*dist + 2*cab + 2*dist*(sb-sa)) / 8; math.Abs(tmp) <= 1 {
		p := mod2pi(2*math.Pi - math.Acos(tmp))
		t := mod2pi(-alpha - math.Atan2(ca-cb, dist+sa-sb) + p/2)
		add([3]float64{1, -1, 1}, t, p, mod2pi(mod2pi(beta)-alpha-t+p), true)
	}
	bestLen, bi := math.Inf(1), -1
	for i, w := range words {
		if l := w.t + w.p + w.q; w.ok && l < bestLen {
			bestLen, bi = l, i
		}
	}
	if bi < 0 {
		return nil
	}
	w := words[bi]
	// Integrate the path (midpoint rule), in metres.
	x, y, th := 0.0, 0.0, th0
	out := []airport.LatLon{a}
	for i, seg := range []float64{w.t, w.p, w.q} {
		k := w.kinds[i] / r
		length := seg * r
		for s := 0.0; s < length-1e-9; {
			ds := math.Min(step, length-s)
			mid := th + k*ds/2
			x, y = x+math.Cos(mid)*ds, y+math.Sin(mid)*ds
			th += k * ds
			s += ds
			out = append(out, offsetHeading(offsetHeading(a, 90, x), 0, y))
		}
	}
	return out
}

func mod2pi(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a
}

// simplifyLine drops the points of a polyline that lie within tol meters of the
// line through their kept neighbours (Douglas–Peucker); the ends stay.
func simplifyLine(pts []airport.LatLon, tol float64) []airport.LatLon {
	if len(pts) < 3 {
		return pts
	}
	keep := make([]bool, len(pts))
	keep[0], keep[len(pts)-1] = true, true
	var dp func(a, b int)
	dp = func(a, b int) {
		far, at := 0.0, -1
		h := localBearing(pts[a], pts[b])
		for i := a + 1; i < b; i++ {
			if d := math.Abs(alongHeading(pts[a], h+90, pts[i])); d > far {
				far, at = d, i
			}
		}
		if at >= 0 && far > tol {
			keep[at] = true
			dp(a, at)
			dp(at, b)
		}
	}
	dp(0, len(pts)-1)
	var out []airport.LatLon
	for i, p := range pts {
		if keep[i] {
			out = append(out, p)
		}
	}
	return out
}

// pavement is the paved area around a point, from the scenery data: stand
// circles and taxi path strips (half their WIDTH either side). Buildings and
// grass are not in the data; whatever is off this pavement is treated as
// such.
type pavement struct {
	stands []airport.Parking
	segs   []paveSeg
}

type paveSeg struct {
	a, b airport.LatLon
	half float64
}

// pavementAround collects the pavement within radius meters of center.
func pavementAround(g *airport.Graph, center airport.LatLon, radius float64) pavement {
	var pv pavement
	for _, p := range g.Layout.Parking {
		if localDist(p.Position, center) < radius+p.Radius {
			pv.stands = append(pv.stands, p)
		}
	}
	for a := range g.Adj {
		pa := g.Nodes[a].Position
		if localDist(pa, center) > radius+200 {
			continue
		}
		for _, e := range g.Adj[a] {
			if e.To < airport.NodeID(a) {
				continue // each segment once
			}
			half := 12.5
			if e.Path >= 0 && e.Path < len(g.Layout.TaxiPaths) && g.Layout.TaxiPaths[e.Path].Width > 0 {
				half = g.Layout.TaxiPaths[e.Path].Width / 2
			}
			pv.segs = append(pv.segs, paveSeg{pa, g.Nodes[e.To].Position, half})
		}
	}
	return pv
}

// off is how far (meters) q lies off the pavement; 0 on it.
func (pv pavement) off(q airport.LatLon) float64 {
	best := math.Inf(1)
	for _, p := range pv.stands {
		if d := localDist(q, p.Position) - p.Radius; d < best {
			if d <= 0 {
				return 0
			}
			best = d
		}
	}
	for _, s := range pv.segs {
		h := localBearing(s.a, s.b)
		l := localDist(s.a, s.b)
		along := math.Max(0, math.Min(l, alongHeading(s.a, h, q)))
		d := localDist(q, offsetHeading(s.a, h, along)) - s.half
		if d <= 0 {
			return 0
		}
		best = math.Min(best, d)
	}
	return best
}

// offPavement is the furthest any main gear point of pts lies off the
// pavement.
func offPavement(pv pavement, pts []airport.LatLon) float64 {
	worst := 0.0
	for i := 0; i < len(pts); i += 2 {
		worst = math.Max(worst, pv.off(pts[i]))
	}
	return worst
}
