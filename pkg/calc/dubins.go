package calc

import "math"

// Dubins returns the shortest path of turn radius r (meters) from
// (lat1, lon1) travelling along heading1 to (lat2, lon2) travelling along
// heading2 (true degrees), sampled every step meters, as [lat, lon] points
// starting with the first position. first restricts the direction of the
// first turn: +1 right (clockwise), -1 left, 0 either (a charted turn
// direction). It returns nil when no such path exists.
func Dubins(lat1, lon1, heading1, lat2, lon2, heading2, r, step float64, first int) [][2]float64 {
	// Local metres around the start, math angles (counter-clockwise from
	// east): a left turn has curvature +1/r.
	north := HaversineMeters(lat1, lon1, lat2, lon1)
	if lat2 < lat1 {
		north = -north
	}
	east := HaversineMeters(lat1, lon1, lat1, lon2)
	if lon2 < lon1 {
		east = -east
	}
	th0, th1 := (90-heading1)*math.Pi/180, (90-heading2)*math.Pi/180
	dist := math.Hypot(east, north) / r
	theta := mod2pi(math.Atan2(north, east))
	alpha, beta := mod2pi(th0-theta), mod2pi(th1-theta)
	sa, sb, ca, cb := math.Sin(alpha), math.Sin(beta), math.Cos(alpha), math.Cos(beta)
	cab := math.Cos(alpha - beta)
	type word struct {
		kinds   [3]float64 // curvature sign per segment: +1 left, 0 straight, -1 right
		t, p, q float64
	}
	var words []word
	if tmp := 2 + dist*dist - 2*cab + 2*dist*(sa-sb); tmp >= 0 { // LSL
		at := math.Atan2(cb-ca, dist+sa-sb)
		words = append(words, word{[3]float64{1, 0, 1}, mod2pi(-alpha + at), math.Sqrt(tmp), mod2pi(beta - at)})
	}
	if tmp := 2 + dist*dist - 2*cab + 2*dist*(sb-sa); tmp >= 0 { // RSR
		at := math.Atan2(ca-cb, dist-sa+sb)
		words = append(words, word{[3]float64{-1, 0, -1}, mod2pi(alpha - at), math.Sqrt(tmp), mod2pi(-beta + at)})
	}
	if tmp := -2 + dist*dist + 2*cab + 2*dist*(sa+sb); tmp >= 0 { // LSR
		p := math.Sqrt(tmp)
		at := math.Atan2(-ca-cb, dist+sa+sb) - math.Atan2(-2, p)
		words = append(words, word{[3]float64{1, 0, -1}, mod2pi(-alpha + at), p, mod2pi(-mod2pi(beta) + at)})
	}
	if tmp := dist*dist - 2 + 2*cab - 2*dist*(sa+sb); tmp >= 0 { // RSL
		p := math.Sqrt(tmp)
		at := math.Atan2(ca+cb, dist-sa-sb) - math.Atan2(2, p)
		words = append(words, word{[3]float64{-1, 0, 1}, mod2pi(alpha - at), p, mod2pi(beta - at)})
	}
	if tmp := (6 - dist*dist + 2*cab + 2*dist*(sa-sb)) / 8; math.Abs(tmp) <= 1 { // RLR
		p := mod2pi(2*math.Pi - math.Acos(tmp))
		t := mod2pi(alpha - math.Atan2(ca-cb, dist-sa+sb) + p/2)
		words = append(words, word{[3]float64{-1, 1, -1}, t, p, mod2pi(alpha - beta - t + p)})
	}
	if tmp := (6 - dist*dist + 2*cab + 2*dist*(sb-sa)) / 8; math.Abs(tmp) <= 1 { // LRL
		p := mod2pi(2*math.Pi - math.Acos(tmp))
		t := mod2pi(-alpha - math.Atan2(ca-cb, dist+sa-sb) + p/2)
		words = append(words, word{[3]float64{1, -1, 1}, t, p, mod2pi(mod2pi(beta) - alpha - t + p)})
	}
	bestLen, bi := math.Inf(1), -1
	for i, w := range words {
		// A charted turn direction: the first turn (when it turns) must go
		// that way. Right is clockwise: curvature -1.
		if first != 0 && w.t > 1e-6 && w.kinds[0] != -float64(first) {
			continue
		}
		if l := w.t + w.p + w.q; l < bestLen {
			bestLen, bi = l, i
		}
	}
	if bi < 0 {
		return nil
	}
	w := words[bi]
	x, y, th := 0.0, 0.0, th0
	out := [][2]float64{{lat1, lon1}}
	for i, seg := range []float64{w.t, w.p, w.q} {
		k := w.kinds[i] / r
		length := seg * r
		for s := 0.0; s < length-1e-9; {
			ds := math.Min(step, length-s)
			mid := th + k*ds/2
			x, y = x+math.Cos(mid)*ds, y+math.Sin(mid)*ds
			th += k * ds
			s += ds
			la, lo := DisplaceByHeading(lat1, lon1, 0, y)
			la, lo = DisplaceByHeading(la, lo, 90, x)
			out = append(out, [2]float64{la, lo})
		}
	}
	// The flat local frame drifts a few meters over kilometers: end exactly
	// on the target.
	out[len(out)-1] = [2]float64{lat2, lon2}
	return out
}

func mod2pi(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a
}
