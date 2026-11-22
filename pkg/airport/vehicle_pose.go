package airport

import (
	"container/heap"
	"math"
)

// Vehicle ways from a pose (#1630). A vehicle leaving where it stands
// faces a way; one arriving at a spot must face a way there. Joining the
// graph at the nearest road point or node whatever its heading, it turned
// round on the spot (a hairpin) or drove back under the aircraft it had
// served. VehicleRoutePose weighs every road within VehicleRoadReachM and
// the nodes near the nearest by the length of the way, turns included, and
// cuts what doubles back (despike).

// vehicleTurnRadiusM prices a turn at a join: its length on this radius,
// and vehicleTurnBackM more for one beyond vehicleTurnBackDeg (turning
// round needs room the way does not have). vehicleJoinWeight prices the
// straight legs off the graph to and from a join, across the apron.
const (
	vehicleTurnRadiusM  = 10.0
	vehicleTurnBackDeg  = 120.0
	vehicleTurnBackM    = 60.0
	vehicleJoinWeight   = 2.0
	vehicleJoinNodeM    = 30.0
	vehicleDespikeDeg   = 150.0
	vehicleDespikeLegsM = 12.0
)

// VehicleRoutePose is VehicleRoute for a vehicle at a facing aHdg (true
// degrees; NaN: any way) to b, arriving there facing bHdg (NaN: any way).
// It joins the graph where the way, turns included, is shortest: never
// turning round on the spot where a road ahead will do.
func (l *Layout) VehicleRoutePose(a LatLon, aHdg float64, b LatLon, bHdg float64) ([]LatLon, error) {
	g := l.vehicleGraph()
	from, to := g.joins(a), g.joins(b)
	if len(from) == 0 || len(to) == 0 {
		return nil, ErrNoVehicleRoute
	}
	inf := math.Inf(1)
	// Each join's cost from a: the leg to it, the turns onto it and onto
	// the segment towards each of its nodes.
	dist := make([]float64, len(g.pos))
	prev := make([]int, len(g.pos))
	seed := make([]int, len(g.pos))
	for i := range dist {
		dist[i], prev[i], seed[i] = inf, -1, -1
	}
	startCost := func(j vehicleEnd) (float64, float64) {
		in := vehBearing(a, j.at)
		c := vehicleJoinWeight * distM(a, j.at)
		if distM(a, j.at) < 0.5 {
			in = aHdg
		} else {
			c += vehicleTurnCost(aHdg, in)
		}
		return c, in
	}
	endCost := func(j vehicleEnd, arrive float64) float64 {
		c := vehicleJoinWeight * distM(j.at, b)
		out := arrive
		if distM(j.at, b) >= 0.5 {
			out = vehBearing(j.at, b)
			c += vehicleTurnCost(arrive, out)
		}
		return c + vehicleTurnCost(out, bHdg)
	}
	q := &vehicleQueue{}
	for k, j := range from {
		c, in := startCost(j)
		for _, n := range j.seg {
			d := c + distM(j.at, g.pos[n])
			if distM(j.at, g.pos[n]) >= 0.5 {
				d += vehicleTurnCost(in, vehBearing(j.at, g.pos[n]))
			}
			if d < dist[n] {
				dist[n], seed[n] = d, k
				heap.Push(q, vehicleItem{n, d})
			}
		}
	}
	for q.Len() > 0 {
		it := heap.Pop(q).(vehicleItem)
		if it.d > dist[it.n] {
			continue
		}
		for _, e := range g.adj[it.n] {
			if d := it.d + e.cost; d < dist[e.to] {
				dist[e.to], prev[e.to], seed[e.to] = d, it.n, seed[it.n]
				heap.Push(q, vehicleItem{e.to, d})
			}
		}
	}
	best, bestEnd, bestLast, bestFrom := inf, -1, -1, -1
	for k, j := range to {
		for _, n := range j.seg {
			if math.IsInf(dist[n], 1) {
				continue
			}
			arrive := vehBearing(g.pos[n], j.at)
			if distM(g.pos[n], j.at) < 0.5 && prev[n] >= 0 {
				arrive = vehBearing(g.pos[prev[n]], g.pos[n])
			}
			if d := dist[n] + distM(g.pos[n], j.at) + endCost(j, arrive); d < best {
				best, bestEnd, bestLast, bestFrom = d, k, n, -1
			}
		}
	}
	// Both on one segment: along it, not round by a node.
	for k, j := range from {
		for m, e := range to {
			if j.seg != e.seg && (j.seg[0] != e.seg[1] || j.seg[1] != e.seg[0]) {
				continue
			}
			c, in := startCost(j)
			along := in
			if distM(j.at, e.at) >= 0.5 {
				along = vehBearing(j.at, e.at)
				c += vehicleTurnCost(in, along)
			}
			if d := c + distM(j.at, e.at) + endCost(e, along); d < best {
				best, bestEnd, bestLast, bestFrom = d, m, -1, k
			}
		}
	}
	if bestEnd < 0 {
		return nil, ErrNoVehicleRoute
	}
	if bestLast < 0 {
		return despike(dedupeLatLon([]LatLon{a, from[bestFrom].at, to[bestEnd].at, b})), nil
	}
	var rev []LatLon
	for n := bestLast; n >= 0; n = prev[n] {
		rev = append(rev, g.pos[n])
	}
	out := []LatLon{a, from[seed[bestLast]].at}
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return despike(dedupeLatLon(append(out, to[bestEnd].at, b))), nil
}

// joins are where a way from or to p may join the graph: the nearest
// point of every vehicle road within VehicleRoadReachM; without one, the
// nodes within vehicleJoinNodeM of the nearest (never a parking spot:
// under a parked aircraft, #1630).
func (g *vehicleGraph) joins(p LatLon) []vehicleEnd {
	var out []vehicleEnd
	for _, r := range g.roads {
		q := closestOnSegment(p, g.pos[r[0]], g.pos[r[1]])
		if distM(p, q) < VehicleRoadReachM && !g.acrossStand(p, q) {
			out = append(out, vehicleEnd{at: q, seg: r})
		}
	}
	if len(out) > 0 {
		return out
	}
	if e, ok := g.end(p); ok && e.seg[0] != e.seg[1] {
		// Only across a stand: the nearest road, as before.
		return []vehicleEnd{e}
	}
	n, ok := g.nearest(p)
	if !ok {
		return nil
	}
	near := distM(p, g.pos[n])
	for i, q := range g.pos {
		if len(g.adj[i]) > 0 && g.joinable(i) && distM(p, q) <= near+vehicleJoinNodeM {
			out = append(out, vehicleEnd{at: q, seg: [2]int{i, i}})
		}
	}
	return out
}

// acrossStand reports whether the straight leg from p to q passes within
// vehicleStandClearM of another stand's parking point (where an aircraft
// may stand, #1630): one whose circle p is not in.
func (g *vehicleGraph) acrossStand(p, q LatLon) bool {
	for k, r := range g.radius {
		s := g.pos[g.np+k]
		if g.depot[k] || distM(p, s) <= r || distM(q, s) <= vehicleStandClearM {
			continue
		}
		if distM(s, closestOnSegment(s, p, q)) < math.Max(vehicleStandClearM, r*vehicleStandShare) {
			return true
		}
	}
	return false
}

// VehicleAcrossStand reports whether a vehicle's leg p to q, serving the
// aircraft at own, crosses another stand (#1630): it passes near the
// parking point of another stand than own's, where an aircraft may park.
// Own's are the nearest stand whose circle own lies in and the ones
// overlapping it (ParkingConflicts); none for an aircraft pushed off its
// stand. Vehicle depots are no stands. clear is how near counts (0:
// vehicleStandClearM, or half the stand's radius where more).
func (l *Layout) VehicleAcrossStand(own, p, q LatLon, clear float64) bool {
	g := l.vehicleGraph()
	mine, near := -1, math.Inf(1)
	for k, r := range g.radius {
		if d := distM(own, g.pos[g.np+k]); !g.depot[k] && d <= r && d < near {
			mine, near = k, d
		}
	}
	for k, r := range g.radius {
		s := g.pos[g.np+k]
		if g.depot[k] || k == mine {
			continue
		}
		if mine >= 0 && distM(g.pos[g.np+mine], s) < g.radius[mine]+r-standOverlapToleranceMeters {
			continue
		}
		c := clear
		if c <= 0 {
			c = math.Max(vehicleStandClearM, r*vehicleStandShare)
		}
		if distM(s, closestOnSegment(s, p, q)) < c {
			return true
		}
	}
	return false
}

// vehicleStandClearM: a leg across the apron this near another stand's
// parking point passes through it.
const vehicleStandClearM = 8.0

// vehicleStandShare: nor within this share of that stand's radius (an
// aircraft on it fills its circle).
const vehicleStandShare = 0.5

// vehicleTurnCost prices turning from heading a to b (NaN either: none).
func vehicleTurnCost(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return 0
	}
	t := math.Abs(math.Remainder(b-a, 360))
	c := vehicleTurnRadiusM * t * math.Pi / 180
	if t > vehicleTurnBackDeg {
		c += vehicleTurnBackM
	}
	return c
}

// vehBearing is the bearing from a to b (true degrees, local, flat).
func vehBearing(a, b LatLon) float64 {
	kx := 111320 * math.Cos((a.Lat+b.Lat)/2*math.Pi/180)
	return math.Mod(math.Atan2((b.Lon-a.Lon)*kx, (b.Lat-a.Lat)*111320)*180/math.Pi+360, 360)
}

// despike drops the points where a way comes straight back over its last
// leg (#1630): turning more than vehicleDespikeDeg at a point, or within
// vehicleDespikeLegsM over two short legs. The ends stay.
func despike(pts []LatLon) []LatLon {
	for changed := true; changed; {
		changed = false
		for i := 1; i+1 < len(pts); i++ {
			in, out := vehBearing(pts[i-1], pts[i]), vehBearing(pts[i], pts[i+1])
			turn := math.Abs(math.Remainder(out-in, 360))
			if turn <= vehicleDespikeDeg && i+2 < len(pts) && distM(pts[i], pts[i+1]) < vehicleDespikeLegsM {
				// A U over two short legs: in, across, back.
				back := vehBearing(pts[i+1], pts[i+2])
				if math.Abs(math.Remainder(back-in, 360)) > vehicleDespikeDeg {
					pts = append(pts[:i:i], pts[i+2:]...)
					changed = true
					break
				}
			}
			if turn > vehicleDespikeDeg {
				pts = append(pts[:i:i], pts[i+1:]...)
				changed = true
				break
			}
		}
	}
	return pts
}
