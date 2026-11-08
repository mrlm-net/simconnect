package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Stand turn-in (live LKPR 2026-11-07, the user: big planes turned onto
// their stands on too short a radius and it looked like drifting).
//
// Three causes, all fixed for the taxi-in:
//
//   - The nose gear's way off the taxilane onto the stand was the taxi
//     path's usual corner rounding, the same for every type: 26-36 m at the
//     nose gear for a B77W, A359, B789 or A333 at LKPR E6, E7 and B14, about
//     full nose wheel lock for their wheelbases, the main gear on 10-16 m.
//   - With the nose gear on the lead-in line, the main gear had not caught
//     up by the stop: widebodies parked 3-6° off the stand heading.
//   - The aircraft turned about the motion profile's main gear, 1 m behind
//     the reference point for every type, not the model's: the real main
//     gear slid sideways through every turn (offline: B789 up to 14° of
//     slip at 0.3 m/s, B738 8°). moverGear pivots the mover on the model's
//     main gear (gearFor).
//
// So the turn onto the stand is planned for the main gear, as a pilot of a
// long aircraft steers it (judgemental oversteer): the main gear rounds the
// corner on StandTurnWheelbases and runs straight up the stand centreline,
// and the nose gear's path is one wheelbase ahead of it along its track,
// swinging out past the line through the turn.

// StandTurnWheelbases: the main gear turns onto a stand on a radius of at
// least this many wheelbases; the nose gear then turns on about 1.4 of
// them, some 45° of nose wheel steering.
const StandTurnWheelbases = 1.0

// standTurnTighterWheelbases: a stand turn whose nose gear turns tighter
// than this many wheelbases, or that leaves the aircraft more than
// standTurnOffDeg off the stand heading at the stop, is planned anew.
const (
	standTurnTighterWheelbases = 1.3
	standTurnOffDeg            = 1.5
)

// standTurnCornerDeg: the turn onto a stand is at the last route point
// reached this far off the stand heading.
const standTurnCornerDeg = 30.0

// widenStandTurn is pts (a taxi-in ground path's points for the nose gear:
// ..., the stand's junction at index join, the stand axis start, the stop)
// with the way onto the stand planned for the main gear of mover profile m:
// it leaves the route on a radius of StandTurnWheelbases·m's wheelbase and
// stops lined up with heading. Only a turn tighter than
// standTurnTighterWheelbases at the nose gear, or one that stops off the
// stand heading, changes; a stand too close for the main gear to start its
// turn from the route, or with no room for half that radius, stays as it was.
func widenStandTurn(pts []airport.LatLon, join int, m MotionProfile, heading float64, p MotionProfile) []airport.LatLon {
	n, wb := len(pts), m.WheelbaseMeters
	if wb <= 0 || join < 1 || join != n-3 {
		return pts
	}
	rm := StandTurnWheelbases * wb
	cum := make([]float64, n)
	for i := 1; i < n; i++ {
		cum[i] = cum[i-1] + localDist(pts[i-1], pts[i])
	}
	// The turn onto the stand: at the last route point reached off the stand
	// heading by more than standTurnCornerDeg (a lead-in may leave the
	// taxilane before the junction kept), else at the junction.
	c := join
	for i := join; i >= 1; i-- {
		if math.Abs(headingDiff(localBearing(pts[i-1], pts[i]), heading)) > standTurnCornerDeg {
			c = i
			break
		}
	}
	turn := math.Abs(headingDiff(localBearing(pts[c-1], pts[c]), heading)) * math.Pi / 180
	// The new way starts at route point k, the nose gear there and the main
	// gear a wheelbase behind it, where the main gear is still the turn's
	// tangent length before the corner.
	tangent := math.Max(rm*math.Tan(math.Min(turn, 120*math.Pi/180)/2), CornerMeters)
	k := c
	for k > 0 && cum[k] > cum[c]+wb-tangent-2 {
		k--
	}
	if k < 1 {
		return pts
	}
	if !standTurnPoor(pts, cum[k], wb, heading, p) {
		return pts
	}
	// Where the main gear is with the nose gear at k, as GroundMover trails it.
	pre, err := newGroundPath(pts[:k+1], p, firmZone{})
	if err != nil {
		return pts
	}
	gear := trail(pre, wb, nil)
	toNose := localBearing(gear, pts[k])
	// The main gear's line: on from where it is towards the route ahead, the
	// route to the junction, then to its stop a wheelbase behind the nose
	// gear's, the last 10 m along the stand heading.
	stopMain := offsetHeading(pts[n-1], heading, -wb)
	next := pts[min(k+1, join)]
	if k == join {
		next = pts[n-2]
	}
	line := []airport.LatLon{gear, offsetHeading(gear, toNose, math.Max(1, alongHeading(gear, toNose, next)))}
	line = append(line, pts[k+1:join+1]...)
	if axis := offsetHeading(stopMain, heading, -10); alongHeading(stopMain, heading, line[len(line)-1]) < -15 {
		line = append(line, axis)
	}
	// Taxilane nodes a few metres apart round a junction left no room.
	line = mergeClose(append(simplifyLine(line, 1.0), stopMain), 5)
	main, tightest := filletAtLeast(line, rm)
	if tightest < 0.5*wb {
		return pts // no room for it: as before
	}
	main = resampleLine(main, 1)
	out := append([]airport.LatLon(nil), pts[:k]...)
	for i, q := range main {
		a, b := main[max(i-1, 0)], main[min(i+1, len(main)-1)]
		if i == 0 {
			out = append(out, pts[k]) // exactly where the nose gear is: a wheelbase ahead of it
			continue
		}
		out = append(out, offsetHeading(q, localBearing(a, b), wb))
	}
	return append(out, pts[n-1])
}

// trail runs the main gear a wheelbase behind the nose gear along g, as
// GroundMover places it, from a wheelbase behind g's start along its first
// metre: where the main gear ends, and in off (unless nil) its heading there.
func trail(g *GroundPath, wb float64, off *float64) airport.LatLon {
	start := g.PointAt(0)
	gear := offsetHeading(start, localBearing(g.PointAt(math.Min(1, g.Length())), start), wb)
	nose := start
	for s := 0.0; ; s = math.Min(s+0.25, g.Length()) {
		nose = g.PointAt(s)
		if localDist(nose, gear) > 1e-6 {
			gear = offsetHeading(nose, localBearing(nose, gear), wb)
		}
		if s >= g.Length() {
			break
		}
	}
	if off != nil {
		*off = localBearing(gear, nose)
	}
	return gear
}

// standTurnPoor reports whether the nose gear on the ground path along pts
// turns tighter than standTurnTighterWheelbases from from metres on, or the
// main gear following it stops more than standTurnOffDeg off heading.
func standTurnPoor(pts []airport.LatLon, from, wb, heading float64, p MotionProfile) bool {
	g, err := newGroundPath(pts, p, firmZone{})
	if err != nil {
		return false
	}
	if pathMinRadius(g, from, g.Length()-2) < standTurnTighterWheelbases*wb {
		return true
	}
	var end float64
	trail(g, wb, &end)
	return math.Abs(headingDiff(end, heading)) > standTurnOffDeg
}

// polyAt is the point s metres along polyline p (cum its running lengths).
func polyAt(p []airport.LatLon, cum []float64, s float64) airport.LatLon {
	for i := 1; i < len(p); i++ {
		if s <= cum[i] || i == len(p)-1 {
			f := 0.0
			if l := cum[i] - cum[i-1]; l > 1e-9 {
				f = math.Max(0, math.Min(1, (s-cum[i-1])/l))
			}
			return airport.LatLon{Lat: p[i-1].Lat + (p[i].Lat-p[i-1].Lat)*f, Lon: p[i-1].Lon + (p[i].Lon-p[i-1].Lon)*f}
		}
	}
	return p[0]
}

// resampleLine is polyline p with a point every step metres, and its ends.
func resampleLine(p []airport.LatLon, step float64) []airport.LatLon {
	if len(p) < 2 {
		return p
	}
	cum := make([]float64, len(p))
	for i := 1; i < len(p); i++ {
		cum[i] = cum[i-1] + localDist(p[i-1], p[i])
	}
	out := []airport.LatLon{}
	for s := 0.0; s < cum[len(p)-1]-step/2; s += step {
		out = append(out, polyAt(p, cum, s))
	}
	return append(out, p[len(p)-1])
}

// pathMinRadius is the tightest radius of g between from and to metres,
// measured over ±TurnWindowMeters as the speed planning does.
func pathMinRadius(g *GroundPath, from, to float64) float64 {
	best := math.Inf(1)
	for s := math.Max(from, TurnWindowMeters); s <= to-TurnWindowMeters; s += 1 {
		a, b, c := g.PointAt(s-TurnWindowMeters), g.PointAt(s), g.PointAt(s+TurnWindowMeters)
		if d := math.Abs(headingDiff(localBearing(a, b), localBearing(b, c))) * math.Pi / 180; d > 1e-4 {
			best = math.Min(best, 2*TurnWindowMeters/d)
		}
	}
	return best
}

// filletAtLeast rounds each interior corner of p on an arc of radius r or
// more: its tangent length at least CornerMeters (as the path's own
// rounding reaches), sharing each leg between the corners at its ends in
// proportion; the first and the last leg are the outer corners' alone.
// Sampled every metre; also the tightest radius it took (+Inf: none).
func filletAtLeast(p []airport.LatLon, r float64) ([]airport.LatLon, float64) {
	n := len(p)
	if n < 3 {
		return p, math.Inf(1)
	}
	o := p[0]
	kx := metersPerDegree * math.Cos(o.Lat*math.Pi/180)
	x, y := make([]float64, n), make([]float64, n)
	for i, q := range p {
		x[i], y[i] = (q.Lon-o.Lon)*kx, (q.Lat-o.Lat)*metersPerDegree
	}
	ll := func(px, py float64) airport.LatLon {
		return airport.LatLon{Lat: o.Lat + py/metersPerDegree, Lon: o.Lon + px/kx}
	}
	leg := func(i int) float64 { return math.Hypot(x[i+1]-x[i], y[i+1]-y[i]) }
	turn := make([]float64, n)
	want := make([]float64, n)
	for i := 1; i < n-1; i++ {
		ux, uy := (x[i]-x[i-1])/leg(i-1), (y[i]-y[i-1])/leg(i-1)
		wx, wy := (x[i+1]-x[i])/leg(i), (y[i+1]-y[i])/leg(i)
		turn[i] = math.Atan2(ux*wy-uy*wx, ux*wx+uy*wy)
		if a := math.Abs(turn[i]); a >= 0.5*math.Pi/180 {
			want[i] = math.Max(r*math.Tan(math.Min(a, 170*math.Pi/180)/2), CornerMeters)
		}
	}
	// share: what corner i may take of leg j (i == j: its outgoing leg).
	share := func(i, j int) float64 {
		l := leg(j)
		a, b := want[j], want[j+1] // the corners at the leg's ends
		if j == 0 {
			a = 0
		}
		if j+1 == n-1 {
			b = 0
		}
		if a+b <= l {
			return l
		}
		return l * want[i] / (a + b)
	}
	out, tightest := []airport.LatLon{p[0]}, math.Inf(1)
	for i := 1; i < n-1; i++ {
		if want[i] == 0 {
			out = append(out, p[i])
			continue
		}
		a := math.Abs(turn[i])
		t := math.Min(want[i], math.Min(share(i, i-1), share(i, i)))
		rr := t / math.Tan(a/2)
		tightest = math.Min(tightest, rr)
		ux, uy := (x[i]-x[i-1])/leg(i-1), (y[i]-y[i-1])/leg(i-1)
		sx, sy := x[i]-ux*t, y[i]-uy*t
		side := math.Copysign(1, turn[i])
		cx, cy := sx-uy*rr*side, sy+ux*rr*side
		a0 := math.Atan2(sy-cy, sx-cx)
		m := max(2, int(rr*a))
		for s := 0; s <= m; s++ {
			ang := a0 + turn[i]*float64(s)/float64(m)
			out = append(out, ll(cx+rr*math.Cos(ang), cy+rr*math.Sin(ang)))
		}
	}
	return append(out, p[n-1]), tightest
}

// moverGear is profile p for the GroundMover of model: pivoting on the
// model's own main gear (gearFor), its point on the path where p has it
// (WheelbaseMeters - RefAheadMeters ahead of the reference point, so stops
// and stand marks are unchanged), and the tail where p has it. Unknown
// models keep p.
func moverGear(p MotionProfile, model string) MotionProfile {
	g, ok := gearFor(model)
	if !ok || g.MainM <= -g.NoseM || p.WheelbaseMeters <= 0 {
		return p
	}
	nose := p.WheelbaseMeters - p.RefAheadMeters // path point ahead of the reference point
	tail := p.TailMeters + p.RefAheadMeters
	if wb := nose + g.MainM; wb > 1 {
		p.WheelbaseMeters, p.RefAheadMeters = wb, g.MainM
		if p.TailMeters > 0 {
			p.TailMeters = math.Max(0, tail-g.MainM)
		}
	}
	return p
}
