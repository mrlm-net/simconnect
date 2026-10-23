package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// OpposedStretch finds where two taxi routes (their ways ahead, in the
// order each taxis them) run along the same pavement in opposite
// directions: within reach metres of each other (both half-spans and a
// margin), facing at least oncomingDeg apart. fromA and fromB are how far
// along each its stretch begins, length how long it is along b, and
// alongB the points of b on it, for a re-plan to keep off
// (airport.Occupied). ok false when they do not meet so. Generic, any
// airport: what a ground controller sees before clearing two aircraft
// nose to nose along one taxiway (live, LKPR: TVS681 in along F to C24,
// CSA398 out along F from C28).
func OpposedStretch(a, b []airport.LatLon, reach float64) (fromA, fromB, length float64, alongB []airport.LatLon, ok bool) {
	if len(a) < 2 || len(b) < 2 {
		return 0, 0, 0, nil, false
	}
	f := newFlat(a[0])
	type sample struct{ s, x, y, hdg float64 }
	sampled := func(way []airport.LatLon) []sample {
		var out []sample
		cum := 0.0
		for i := 0; i+1 < len(way); i++ {
			ax, ay := f.xy(way[i])
			bx, by := f.xy(way[i+1])
			l := math.Hypot(bx-ax, by-ay)
			if l < 1e-6 {
				continue
			}
			hdg := math.Mod(math.Atan2(bx-ax, by-ay)*180/math.Pi+360, 360)
			for t := 0.0; t < l; t += oncomingStep {
				out = append(out, sample{cum + t, ax + (bx-ax)*t/l, ay + (by-ay)*t/l, hdg})
			}
			cum += l
		}
		return out
	}
	sa, sb := sampled(a), sampled(b)
	cell := func(x, y float64) [2]int32 {
		return [2]int32{int32(math.Floor(x / oncomingCell)), int32(math.Floor(y / oncomingCell))}
	}
	grid := map[[2]int32][]int32{}
	for k, m := range sa {
		c := cell(m.x, m.y)
		grid[c] = append(grid[c], int32(k))
	}
	reach2 := reach * reach
	n := int32(math.Ceil(reach / oncomingCell))
	fromA, fromB = math.Inf(1), math.Inf(1)
	last := math.Inf(-1)
	for _, q := range sb {
		c := cell(q.x, q.y)
		hit := false
		for dx := -n; dx <= n && !hit; dx++ {
			for dy := -n; dy <= n && !hit; dy++ {
				for _, k := range grid[[2]int32{c[0] + dx, c[1] + dy}] {
					m := sa[k]
					ex, ey := m.x-q.x, m.y-q.y
					if ex*ex+ey*ey <= reach2 && math.Abs(headingDiff(m.hdg, q.hdg)) >= oncomingDeg {
						fromA = math.Min(fromA, m.s)
						hit = true
						break
					}
				}
			}
		}
		if !hit {
			continue
		}
		if math.IsInf(fromB, 1) {
			fromB = q.s
		}
		if q.s-last > 2*oncomingStep && len(alongB) > 0 {
			length += oncomingStep // a gap: counted as one step, the rest stays apart
		} else if len(alongB) > 0 {
			length += q.s - last
		}
		last = q.s
		alongB = append(alongB, f.at(q.x, q.y))
	}
	if len(alongB) == 0 {
		return 0, 0, 0, nil, false
	}
	return fromA, fromB, length, alongB, true
}
