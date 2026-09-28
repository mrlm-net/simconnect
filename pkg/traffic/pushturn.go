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
// (pushing towards pushDir, true degrees) to face routeDir with the nose
// gear on the route line through jp, short of it — the widest radius from
// PushbackArcMeters down to PushbackMinArcMeters, stopping as close to the
// junction as the neighbouring stands allow; nil if none is found.
func pushTurnPlan(g *airport.Graph, own int, gear airport.LatLon, pushDir float64, jp airport.LatLon, routeDir float64, prof MotionProfile) []airport.LatLon {
	base := standIntrusion(g, own, []airport.LatLon{offsetHeading(gear, pushDir+180, 1), gear}, prof)
	var best, clear []airport.LatLon
	bestIn, bestCost := math.Inf(1), math.Inf(1)
	for r := PushbackArcMeters; r >= PushbackMinArcMeters-0.01; r -= 4 {
		for d := 0.0; d <= pushTurnBackMeters; d += 5 {
			end := offsetHeading(jp, routeDir+180, d+prof.WheelbaseMeters)
			// Straight back PushStraightMeters first, clear of the stand.
			turn := dubins(offsetHeading(gear, pushDir, PushStraightMeters), pushDir, end, routeDir+180, r, 0.5)
			if turn == nil {
				continue
			}
			pts := append([]airport.LatLon{gear}, turn...)
			if pathLen(pts) > pushTurnMaxMeters {
				continue
			}
			// Clear of the neighbours: the shortest push, with a tighter turn
			// costing pushTurnRadiusCost per meter of radius given up.
			in := standIntrusion(g, own, pts, prof)
			if in <= base+pushClearanceSlackMeters {
				if cost := pathLen(pts) + pushTurnRadiusCost*(PushbackArcMeters-r); cost < bestCost {
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
