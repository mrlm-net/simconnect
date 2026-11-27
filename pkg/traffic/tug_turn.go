package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// tugMinTurnMeters is the tightest turn a tug drives (#1693: driving home
// it turned almost on the spot, a 0.5–1.5 m radius at walking pace). A TLD
// TPX-200 turns between an outside radius of 7,830 mm and an inner one of
// 2,150 mm (the TPX-200 listing of Aero Specialties, read 2026-11-26):
// about 5 m at its middle. Every tug model is held to it.
const tugMinTurnMeters = 5.0

// tugWayMeters is the radius a tug's own way turns on (steerWay): its front
// axle on the way, its rear axle a wheelbase behind turns inside it, and it
// is shown VehicleLaneMeters right of it, inside again on a right turn. So
// that turns on tugMinTurnMeters at the least: √((5 + 2)² + 3²) ≈ 7.6 m,
// and a step more for the way's sampling.
var tugWayMeters = math.Hypot(tugMinTurnMeters+VehicleLaneMeters, tugProfile().WheelbaseMeters) + steerStepMeters

// tugBackMoreMeters is how much further a tug backs off from the nose, at
// most tugBackMoreTimes, where its turns onto the way home take it under
// the aircraft.
const (
	tugBackMoreMeters = 8.0
	tugBackMoreTimes  = 2
)

// steerStepMeters is how far apart steerWay samples its way;
// steerLookMeters how far ahead on the route it steers for (further, it
// cuts inside a turn by about look²/2r: 3 m inside an 11 m one at 8 m).
const (
	steerStepMeters = 0.5
	steerLookMeters = 2.0
)

// steerWay is the way a vehicle at from, heading h, drives along route
// turning on no less than radius r: it steers for the point
// steerLookMeters ahead on route, its corners rounded on r first (filletXY:
// it turns before a corner, not past it), past the end on along its last
// leg, and turns at most a step's worth of r each step, so the way starts
// along h and has no corner sharper than r (fillet rounds a corner between
// short legs tighter, and a route set off across the heading turned it in
// one point). Exact, it ends on route's last point along its last leg,
// onto the leg's start on a Dubins way of radius r kept clear by clear (a
// tug comes onto the nose in line, not still turning), or on the shortest
// loop kept clear; not exact, level with the end, beside it by what it
// misses. False when it does not get there (a route too short, or, exact,
// no clear way onto its end).
func steerWay(from airport.LatLon, h float64, route []airport.LatLon, r float64, exact bool, clear func(a, b airport.LatLon) bool) ([]airport.LatLon, bool) {
	if len(route) < 2 || r <= 0 {
		return nil, false
	}
	o := from
	kx := metersPerDegree * math.Cos(o.Lat*math.Pi/180)
	xy := func(q airport.LatLon) (float64, float64) {
		return (q.Lon - o.Lon) * kx, (q.Lat - o.Lat) * metersPerDegree
	}
	ll := func(x, y float64) airport.LatLon {
		return airport.LatLon{Lat: o.Lat + y/metersPerDegree, Lon: o.Lon + x/kx}
	}
	n := len(route)
	rx, ry := make([]float64, n), make([]float64, n)
	for i, q := range route {
		rx[i], ry[i] = xy(q)
	}
	// Exact, the last corner stays: the way comes onto the last leg whole.
	last := n - 1
	if exact {
		last = n - 2
	}
	rx, ry = filletXY(rx, ry, r, last, !exact)
	endHdg := localBearing(route[n-2], route[n-1])
	in := route[n-2]
	end := route[n-1]
	n = len(rx)
	cum := make([]float64, n)
	for i := 1; i < n; i++ {
		cum[i] = cum[i-1] + math.Hypot(rx[i]-rx[i-1], ry[i]-ry[i-1])
	}
	total := cum[n-1]
	if total < 2*steerStepMeters || cum[n-1]-cum[n-2] < 1e-6 {
		return nil, false
	}
	// at is the point s along route, beyond its end on along its last leg.
	at := func(s float64) (float64, float64) {
		s = math.Max(0, s)
		i := 1
		for i < n-1 && cum[i] < s {
			i++
		}
		l := math.Max(cum[i]-cum[i-1], 1e-9)
		f := (s - cum[i-1]) / l
		return rx[i-1] + (rx[i]-rx[i-1])*f, ry[i-1] + (ry[i]-ry[i-1])*f
	}
	// project is the distance along route nearest (x, y), searched from s0
	// to upTo only (a way doubling back is not cut short); on the last leg
	// it runs on past the end.
	project := func(x, y, s0, upTo float64) float64 {
		best, bestD := s0, math.Inf(1)
		for i := 1; i < n; i++ {
			if cum[i] < s0 && i < n-1 || cum[i-1] > upTo {
				continue
			}
			ax, ay, bx, by := rx[i-1], ry[i-1], rx[i], ry[i]
			l2 := (bx-ax)*(bx-ax) + (by-ay)*(by-ay)
			if l2 < 1e-12 {
				continue
			}
			f := math.Max(0, ((x-ax)*(bx-ax)+(y-ay)*(by-ay))/l2)
			if i < n-1 {
				f = math.Min(1, f)
			}
			s := cum[i-1] + f*(cum[i]-cum[i-1])
			if s < s0 || s > upTo {
				continue
			}
			if d := math.Hypot(x-(ax+(bx-ax)*f), y-(ay+(by-ay)*f)); d < bestD {
				best, bestD = s, d
			}
		}
		return best
	}
	look := math.Min(r, steerLookMeters)
	most := steerStepMeters / r // radians a step at most
	x, y, hd := 0.0, 0.0, h*math.Pi/180
	px, py := []float64{0}, []float64{0}
	straightIn := cum[n-1] - cum[n-2]
	var finish []airport.LatLon // exact: the way onto the end
	// best: the shortest way onto the end found with a loop, kept clear,
	// and how much of the way before it (bestAt points).
	var best []airport.LatLon
	bestAt, bestLen := 0, math.Inf(1)
	s, done := 0.0, false
	for k, limit := 0, int((total+20*r)/steerStepMeters)*2; k < limit && !done; k++ {
		gx, gy := at(s + look)
		want := math.Atan2(gx-x, gy-y)
		turn := math.Max(-most, math.Min(most, math.Remainder(want-hd, 2*math.Pi)))
		hd += turn
		x, y = x+math.Sin(hd)*steerStepMeters, y+math.Cos(hd)*steerStepMeters
		s = project(x, y, s, s+look+2*steerStepMeters)
		if s >= total {
			if exact {
				break // past the end, never onto it in line
			}
			// Level with the end: it stops there, beside it by the miss.
			back := s - total
			x, y = x-math.Sin(hd)*back, y-math.Cos(hd)*back
			done = true
		}
		px, py = append(px, x), append(py, y)
		if exact && !done && s >= cum[n-2]-3*r && s < total-straightIn {
			// Near the last leg: onto its start along it on a Dubins way of
			// radius r (where that is no loop), and straight on from there:
			// its rear axle in line by the end.
			here := ll(x, y)
			d := calc.Dubins(here.Lat, here.Lon, hd*180/math.Pi, in.Lat, in.Lon, endHdg, r, steerStepMeters, 0)
			if len(d) >= 2 {
				way := make([]airport.LatLon, len(d))
				for i, q := range d {
					way[i] = airport.LatLon{Lat: q[0], Lon: q[1]}
				}
				switch l := pathLen(way); {
				case l <= 1.2*localDist(here, in)+steerStepMeters && clearWay(way, clear):
					finish, done = append(way[1:], end), true
				case l < bestLen && clearWay(way, clear):
					best, bestAt, bestLen = way[1:], len(px), l
				}
			}
		}
	}
	// No way onto the end in line: round once on the shortest loop kept
	// clear (a tug lining up on the nose from a road too near it).
	if exact && !done && best != nil {
		px, py = px[:bestAt], py[:bestAt]
		finish, done = append(best, end), true
	}
	if !done {
		return nil, false
	}
	// Straight runs as one leg.
	m := len(px)
	out := []airport.LatLon{from}
	for j := 1; j < m; j++ {
		if j < m-1 && len(out) >= 2 {
			a := math.Atan2(px[j]-px[j-1], py[j]-py[j-1])
			b := math.Atan2(px[j+1]-px[j], py[j+1]-py[j])
			if math.Abs(math.Remainder(b-a, 2*math.Pi)) < 1e-4 {
				continue
			}
		}
		out = append(out, ll(px[j], py[j]))
	}
	if exact {
		out = append(out, finish...)
		out[len(out)-1] = end
	}
	return out, true
}

// filletXY is the polyline x, y with its corners 1 to upTo-1 rounded on
// radius r, sampled every steerStepMeters: a tangent of r·tan(θ/2), at most
// half of a leg it shares with another corner, all of the first and last
// (a shorter radius there, which steerWay widens). Merging, a corner under
// r/2 before the next one is dropped first: two corners that close are one
// turn, and rounded apart each was too tight (LKPR N54, home through the
// stand: it swung 8 m wide of the second).
func filletXY(x, y []float64, r float64, upTo int, merge bool) ([]float64, []float64) {
	if len(x) < 3 {
		return x, y
	}
	kx, ky := []float64{x[0]}, []float64{y[0]}
	keepTo := 1
	for i := 1; i < len(x); i++ {
		if merge && i < upTo && i+1 < len(x)-1 && math.Hypot(x[i+1]-x[i], y[i+1]-y[i]) < r/2 {
			continue
		}
		if i == upTo {
			keepTo = len(kx)
		}
		kx, ky = append(kx, x[i]), append(ky, y[i])
	}
	x, y, upTo = kx, ky, keepTo
	n := len(x)
	if n < 3 {
		return x, y
	}
	ox, oy := []float64{x[0]}, []float64{y[0]}
	for i := 1; i < n-1; i++ {
		ax, ay := x[i]-x[i-1], y[i]-y[i-1]
		bx, by := x[i+1]-x[i], y[i+1]-y[i]
		la, lb := math.Hypot(ax, ay), math.Hypot(bx, by)
		th := 0.0
		if la > 1e-6 && lb > 1e-6 {
			th = math.Abs(math.Remainder(math.Atan2(bx, by)-math.Atan2(ax, ay), 2*math.Pi))
		}
		if i >= upTo || th < 1e-3 || th > math.Pi-1e-3 {
			ox, oy = append(ox, x[i]), append(oy, y[i])
			continue
		}
		ca, cb := la, lb
		if i > 1 {
			ca /= 2
		}
		if i < n-2 {
			cb /= 2
		}
		tl := math.Min(r*math.Tan(th/2), math.Min(ca, cb))
		// The arc from the tangent point on the leg in to the one on the
		// leg out: a quadratic Bézier through the corner is near enough at
		// this sampling and turns the same way.
		sx, sy := x[i]-ax/la*tl, y[i]-ay/la*tl
		ex, ey := x[i]+bx/lb*tl, y[i]+by/lb*tl
		k := int(2*tl/steerStepMeters) + 1
		for j := 0; j <= k; j++ {
			f := float64(j) / float64(k)
			u := 1 - f
			ox = append(ox, u*u*sx+2*u*f*x[i]+f*f*ex)
			oy = append(oy, u*u*sy+2*u*f*y[i]+f*f*ey)
		}
	}
	return append(ox, x[n-1]), append(oy, y[n-1])
}

// clearWay reports whether way keeps clear by clear (nil: any way does).
func clearWay(way []airport.LatLon, clear func(a, b airport.LatLon) bool) bool {
	if clear == nil {
		return true
	}
	// The rear axle too, which cuts inside the turns: a wheelbase back
	// along the chord from the point a wheelbase before. And where it is
	// shown, VehicleLaneMeters right of its way (lane).
	wb := tugProfile().WheelbaseMeters
	var rear, shown []airport.LatLon
	back := 0
	for i := 1; i < len(way); i++ {
		if !clear(way[i-1], way[i]) {
			return false
		}
		h := localBearing(way[i-1], way[i])
		if len(shown) == 0 {
			shown = append(shown, offsetHeading(way[0], h+90, VehicleLaneMeters))
		}
		shown = append(shown, offsetHeading(way[i], h+90, VehicleLaneMeters))
		for back < i && localDist(way[back+1], way[i]) >= wb {
			back++
		}
		if d := localDist(way[back], way[i]); d > 0.1 {
			rear = append(rear, offsetHeading(way[i], localBearing(way[i], way[back]), wb))
		}
	}
	for i := 1; i < len(rear); i++ {
		if !clear(rear[i-1], rear[i]) {
			return false
		}
	}
	for i := 1; i < len(shown); i++ {
		if !clear(shown[i-1], shown[i]) {
			return false
		}
	}
	return true
}

// steerPath is the GroundPath of steerWay from (from, h) along route at
// radius r (exact: onto its end, a loop onto it kept clear by clear),
// profile p; the arc path of route (NewArcPath) where steerWay finds none.
func steerPath(from airport.LatLon, h float64, route []airport.LatLon, p MotionProfile, r float64, exact bool, clear func(a, b airport.LatLon) bool) (*GroundPath, error) {
	if way, ok := steerWay(from, h, route, r, exact, clear); ok {
		return NewSmoothPath(way, p)
	}
	return NewArcPath(route, p, r)
}
