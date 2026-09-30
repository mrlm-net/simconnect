//go:build windows
// +build windows

package traffic

import (
	"cmp"
	"math"
	"slices"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Pushback to a pose: a push ends with the aircraft on a taxiway, aligned
// with it, facing the way it taxis out. The lead-in line of the stand is
// only the way in (EDDF: every lead-in is one way onto the stand), so the
// push does not follow it back: the planner chooses the end pose first —
// a point on a taxiway near the stand and a facing along it — then the
// path the tug can push to it (a Dubins path of bounded curvature) clear
// of the neighbouring stands and on the pavement. A manual pushback
// ("push onto L facing west") is the same path to a pose given by ATC.

// pushPose is where a push ends: the nose on the taxiway edge from→to (or
// short of from on its line), facing to (heading, true degrees).
type pushPose struct {
	nose     airport.LatLon
	heading  float64
	from, to airport.NodeID
	lane     string
	tight    bool           // on a lane without wingtip clearance from the stands beside it
	own      airport.NodeID // the junction of the stand's lead-in (poseBlocks)
	taxi     float64        // meters on from the nose to the far node
	lb       float64        // lower bound of the cost of a push to it
	lbTaxi   float64        // lower bound of the taxi-out from it
	fixed    float64        // its cost but the push itself, once out is planned
	out      *airport.Route // the taxi-out from to (poseRoute)
}

const (
	// pushPoseReachMeters: a push ends on a taxiway within this of the stand.
	pushPoseReachMeters = 120.0
	// pushPoseStepMeters: poses along a taxiway edge, this apart.
	pushPoseStepMeters = 5.0
	// laneEndPoseMeters: a pose's nose up to this far short of its edge.
	laneEndPoseMeters = 20.0
	// pushMaxTurnDeg: a tug turns the aircraft at most this in all, and at
	// most pushMaxSwerveDeg more than from the stand's heading to the pose's.
	pushMaxTurnDeg   = 200.0
	pushMaxSwerveDeg = 60.0
	// pushTaxiAlignMeters: the taxi-out from a pose runs within
	// pushTaxiAlignDeg of its heading this far ahead, or the pose costs
	// pushMisalignPenalty (a turn from a standstill on a junction).
	pushTaxiAlignMeters = 20.0
	pushTaxiAlignDeg    = 45.0
	pushTaxiStartMeters = 10.0
	pushTaxiStartDeg    = 20.0
	// pushTightPenalty: a pose on a lane whose wingtip clearance from the
	// stands beside it is short (Graph.Fits) holds those stands while the
	// engines start; a lane with clearance wins (LKPR A3: onto A1, not the
	// AA stub beside A2), one without is still taken where nothing else is
	// (LKPR C31: H1 beside C30).
	pushTightPenalty = 500.0
	// pushPoseMaxMeters: the longest push.
	pushPoseMaxMeters = 150.0
	// pushPoseRoutes: taxi-outs planned at most, for the cheapest pushes,
	// in pushPoseSearches route searches at most (a pose facing a dead end
	// has none).
	pushPoseRoutes   = 30
	pushPoseSearches = 60
)

// pushTo is the main gear path of a push from gear (the aircraft facing
// standHdg) to the pose: straight back PushStraightMeters clear of the
// stand, then the shortest path of turn radius r; nil if none.
func pushTo(gear airport.LatLon, standHdg float64, pose pushPose, r float64, prof MotionProfile) []airport.LatLon {
	pushDir := standHdg + 180
	end := offsetHeading(pose.nose, pose.heading+180, prof.WheelbaseMeters)
	turn := dubins(offsetHeading(gear, pushDir, PushStraightMeters), pushDir, end, pose.heading+180, r, 0.5)
	if turn == nil {
		return nil
	}
	// The last sample can fall a few centimeters short of the end: a sliver
	// that reads as a kink.
	if n := len(turn); n > 2 && localDist(turn[n-2], turn[n-1]) < 0.25 {
		turn = append(turn[:n-2], turn[n-1])
	}
	return append([]airport.LatLon{gear}, turn...)
}

// totalTurn is the heading change along pts, summed, in degrees.
func totalTurn(pts []airport.LatLon) float64 {
	sum, prev, have := 0.0, 0.0, false
	for i := 2; i < len(pts); i += 2 {
		if localDist(pts[i-2], pts[i]) < 0.2 {
			continue
		}
		h := localBearing(pts[i-2], pts[i])
		if have {
			sum += math.Abs(headingDiff(prev, h))
		}
		prev, have = h, true
	}
	return sum
}

// pushFits reports how far off the pavement pv a tug pushes the main gear
// along pts, if it can: not longer than pushPoseMaxMeters, turning
// pushMaxTurnDeg at most and no more than pushMaxSwerveDeg beyond the
// turn from the stand to the pose (no loops), the main gear within tol of the pavement, the
// tail and wings no deeper into a neighbouring stand (or the terminal)
// than base+pushClearanceSlackMeters.
func pushFits(pts []airport.LatLon, prof MotionProfile, pv *flatPave, tol, base float64) (float64, bool) {
	if pathLen(pts) > pushPoseMaxMeters {
		return 0, false
	}
	n := len(pts)
	net := math.Abs(headingDiff(localBearing(pts[0], pts[min(2, n-1)]), localBearing(pts[max(0, n-3)], pts[n-1])))
	if turn := totalTurn(pts); turn > pushMaxTurnDeg || turn > net+pushMaxSwerveDeg {
		return 0, false
	}
	off := 0.0
	for i := 0; i < len(pts); i += 2 {
		if off = math.Max(off, pv.off(pts[i])); off > tol {
			return off, false
		}
	}
	return off, pv.intrusion(pts, prof) <= base+pushClearanceSlackMeters
}

// pushPoses are the poses a push from gear may end in: along every
// taxiway edge within pushPoseReachMeters, both ways, and up to
// laneEndPoseMeters before its start on its line: the nose short of where
// the lane begins, the tail on the apron behind (LKPR C31, where the lane
// ends at the stand's junction; C17, where JB turns into a connector
// there: with the nose on the node the tail has no room to swing). Each
// has a lower
// bound of its cost (the push and the taxi straight to the nearest hold of
// the runway), cheapest first. Their taxi-outs are planned only when
// needed (poseRoute).
func (c *TaxiController) pushPoses(gear airport.LatLon) []pushPose {
	g := c.req.Graph
	toRunway := c.metersToRunway()
	limits := c.req.Options.TaxiwayMaxSpan
	if limits == nil {
		limits = airport.KnownTaxiwayMaxSpan[g.Layout.ICAO]
	}
	span := 2 * c.halfSpan()
	var poses []pushPose
	for a := range g.Adj {
		from := airport.NodeID(a)
		pa := g.Nodes[from].Position
		if g.Nodes[from].Kind == airport.NodeParking || localDist(pa, gear) > pushPoseReachMeters+300 {
			continue
		}
		for _, e := range g.Adj[a] {
			// The taxiway's span limit (a 777 not on LKPR JO); the wingtips'
			// clearance from the stands beside it is the push's own check
			// (pushFits), as the lane beside the stand's neighbours is where
			// the push ends (LKPR H1 at C31).
			if max, ok := limits[e.Name]; !pushEdge(g, e) || ok && e.Name != "" && span > max {
				continue
			}
			pb := g.Nodes[e.To].Position
			l, h := localDist(pa, pb), localBearing(pa, pb)
			lane, tight := "", !g.Fits(e, c.req.Options)
			for x := -laneEndPoseMeters; x <= l; x += pushPoseStepMeters {
				nose := offsetHeading(pa, h, x)
				d := localDist(nose, gear)
				if d > pushPoseReachMeters {
					continue
				}
				if lane == "" {
					lane = laneName(g, from, e)
				}
				taxi, ok := toRunway[e.To]
				if !ok {
					continue // no way to the runway from there
				}
				taxi += l - x
				poses = append(poses, pushPose{nose: nose, heading: h, from: from, to: e.To, lane: lane, tight: tight, taxi: l - x, lbTaxi: taxi, lb: math.Max(0, d-c.profile().WheelbaseMeters)*pushCostFactor + taxi})
			}
		}
	}
	slices.SortFunc(poses, func(p, q pushPose) int { return cmp.Compare(p.lb, q.lb) })
	return poses
}

// metersToRunway is, for every node, the shortest way along the taxi
// graph to the hold-short the route from the stand ends at (the one the
// runway's routes choose: nearest the threshold): a lower bound of the
// cost of a route from there (turns and crossings only add to it).
func (c *TaxiController) metersToRunway() map[airport.NodeID]float64 {
	g := c.req.Graph
	back := map[airport.NodeID][]airport.Edge{}
	for a, es := range g.Adj {
		for _, e := range es {
			back[e.To] = append(back[e.To], airport.Edge{To: airport.NodeID(a), Length: e.Length})
		}
	}
	dist := map[airport.NodeID]float64{}
	var q []airport.NodeID
	if n := len(c.route.Nodes); n > 0 {
		dist[c.route.Nodes[n-1]] = 0
		q = append(q, c.route.Nodes[n-1])
	}
	done := map[airport.NodeID]bool{}
	for len(q) > 0 {
		bi := 0
		for i := range q {
			if dist[q[i]] < dist[q[bi]] {
				bi = i
			}
		}
		u := q[bi]
		q[bi] = q[len(q)-1]
		q = q[:len(q)-1]
		if done[u] {
			continue
		}
		done[u] = true
		for _, e := range back[u] {
			if d, ok := dist[e.To]; !ok || dist[u]+e.Length < d {
				dist[e.To] = dist[u] + e.Length
				q = append(q, e.To)
			}
		}
	}
	return dist
}

// aligned reports whether the taxi-out from the pose starts along its
// heading — within pushTaxiStartDeg pushTaxiStartMeters ahead of the nose
// (a pose on a bend of the lane turns from a standstill: EDDF B25, LFPG
// E42) — and runs within pushTaxiAlignDeg of it pushTaxiAlignMeters ahead.
func (p pushPose) aligned() bool {
	return p.within(pushTaxiStartMeters, pushTaxiStartDeg) && p.within(pushTaxiAlignMeters, pushTaxiAlignDeg)
}

// within reports whether the taxi-out's point meters ahead of the nose
// lies within deg of its heading.
func (p pushPose) within(meters, deg float64) bool {
	pts := append([]airport.LatLon{p.nose}, p.out.Points...)
	walked := 0.0
	for i := 1; i < len(pts); i++ {
		walked += localDist(pts[i-1], pts[i])
		if walked >= meters {
			ahead := offsetHeading(pts[i], localBearing(pts[i], pts[i-1]), walked-meters)
			return math.Abs(headingDiff(p.heading, localBearing(p.nose, ahead))) <= deg
		}
	}
	return true
}

// poseRoute plans the taxi-out of pose p from the far node of its edge,
// not turning back (cached per edge in routes); nil if there is none.
func (c *TaxiController) poseRoute(p pushPose, routes map[[2]airport.NodeID]*airport.Route) *airport.Route {
	k := [2]airport.NodeID{p.from, p.to}
	if r, ok := routes[k]; ok {
		return r
	}
	r, err := c.req.Graph.RouteToRunwayFrom(p.to, p.from, c.req.Runway, c.req.Entry, c.req.Options)
	if err != nil || len(r.Nodes) > 1 && r.Nodes[1] == p.from {
		r = nil
	}
	routes[k] = r
	return r
}

// laneEndMeters: the apron goes on at least this far past the end of a
// taxilane (LKPR A1: the lane stops at the last stand's lead-in, the apron
// beyond it takes the tail of an aircraft pushed to face up the lane).
const laneEndMeters = 45.0

// laneEnds is the pavement past the ends of the taxilanes within radius of
// center: each lane's line on for laneEndMeters, as wide, where it does
// not go on straight (within laneEndDeg) and no other named taxiway meets
// it — a dead end (LKPR A1, C31), or a lane turning into an unnamed
// connector (LKPR JB at C17). A lane ending at another taxiway may have
// grass beyond it.
func laneEnds(g *airport.Graph, center airport.LatLon, radius float64) []paveSeg {
	var segs []paveSeg
	for a := range g.Adj {
		end := g.Nodes[a]
		if end.Kind == airport.NodeParking || end.HoldShort != nil || localDist(end.Position, center) > radius {
			continue
		}
		for _, in := range g.Adj[a] {
			if !pushEdge(g, in) {
				continue
			}
			out := localBearing(g.Nodes[in.To].Position, end.Position)
			open := true
			for _, e := range g.Adj[a] {
				if e.To == in.To || !pushEdge(g, e) {
					continue
				}
				if e.Name != "" || math.Abs(headingDiff(out, localBearing(end.Position, g.Nodes[e.To].Position))) <= laneEndDeg {
					open = false
				}
			}
			if !open {
				continue
			}
			half := 12.5
			if in.Path >= 0 && in.Path < len(g.Layout.TaxiPaths) && g.Layout.TaxiPaths[in.Path].Width > 0 {
				half = g.Layout.TaxiPaths[in.Path].Width / 2
			}
			segs = append(segs, paveSeg{end.Position, offsetHeading(end.Position, out, laneEndMeters), half})
		}
	}
	return segs
}

// laneEndDeg: a lane going on within this of straight does not end.
const laneEndDeg = 30.0

// flatPave is a pavement in flat meters east and north of a centre, for
// the many checks of a push plan, with what it found per square meter
// kept: the pushes planned from a stand cross the same ground.
type flatPave struct {
	c      airport.LatLon
	kx     float64
	stands [][3]float64 // x, y, radius
	segs   [][5]float64 // a (x, y), b-a (x, y), half width
	offAt  map[[2]int32]float64
	// The neighbouring stands (x, y, radius) and the gates' parked noses
	// (x, y, heading, radius) the push keeps clear of (withStands).
	near, gates [][4]float64
	depthAt     map[[2]int32]float64
}

func newFlatPave(pv pavement, c airport.LatLon) *flatPave {
	f := &flatPave{c: c, kx: metersPerDegree * math.Cos(c.Lat*math.Pi/180), offAt: map[[2]int32]float64{}, depthAt: map[[2]int32]float64{}}
	for _, p := range pv.stands {
		x, y := f.xy(p.Position)
		f.stands = append(f.stands, [3]float64{x, y, p.Radius})
	}
	for _, s := range pv.segs {
		ax, ay := f.xy(s.a)
		bx, by := f.xy(s.b)
		f.segs = append(f.segs, [5]float64{ax, ay, bx - ax, by - ay, s.half})
	}
	return f
}

func (f *flatPave) xy(q airport.LatLon) (float64, float64) {
	return (q.Lon - f.c.Lon) * f.kx, (q.Lat - f.c.Lat) * metersPerDegree
}

func cell(x, y float64) [2]int32 {
	return [2]int32{int32(math.Floor(x)), int32(math.Floor(y))}
}

// off is how far (meters) q lies off the pavement; 0 on it (to a meter).
func (f *flatPave) off(q airport.LatLon) float64 {
	x, y := f.xy(q)
	k := cell(x, y)
	if d, ok := f.offAt[k]; ok {
		return d
	}
	d := f.offXY(float64(k[0])+0.5, float64(k[1])+0.5)
	f.offAt[k] = d
	return d
}

func (f *flatPave) offXY(x, y float64) float64 {
	best := math.Inf(1)
	for _, s := range f.stands {
		d := math.Hypot(x-s[0], y-s[1]) - s[2]
		if d <= 0 {
			return 0
		}
		best = math.Min(best, d)
	}
	for _, s := range f.segs {
		px, py := x-s[0], y-s[1]
		t := 0.0
		if l2 := s[2]*s[2] + s[3]*s[3]; l2 > 0 {
			t = math.Max(0, math.Min(1, (px*s[2]+py*s[3])/l2))
		}
		d := math.Hypot(px-t*s[2], py-t*s[3]) - s[4]
		if d <= 0 {
			return 0
		}
		best = math.Min(best, d)
	}
	return best
}

// withStands adds the stands within 200 m of the centre, and the gates,
// as standIntrusion sees them; not own, nor the stands overlapping it
// (KJFK C1002 on C10): nobody parks there while own is taken.
func (f *flatPave) withStands(g *airport.Graph, own int) {
	skip := g.Layout.ParkingConflicts(own)
	for _, p := range g.Layout.Parking {
		if localDist(p.Position, f.c) >= 200 {
			continue
		}
		if p.Index != own && !slices.Contains(skip, p.Index) {
			x, y := f.xy(p.Position)
			f.near = append(f.near, [4]float64{x, y, p.Radius})
		}
		if p.IsGate() {
			x, y := f.xy(offsetHeading(StandPoint(p, 0), p.Heading, DefaultNoseOffsetMeters))
			f.gates = append(f.gates, [4]float64{x, y, p.Heading * math.Pi / 180, p.Radius})
		}
	}
}

// depth is how deep (meters) q lies in a neighbouring stand or the
// terminal ahead of a gate (standIntrusion), to a meter.
func (f *flatPave) depth(q airport.LatLon) float64 {
	x, y := f.xy(q)
	k := cell(x, y)
	if d, ok := f.depthAt[k]; ok {
		return d
	}
	x, y = float64(k[0])+0.5, float64(k[1])+0.5
	worst := math.Inf(-1)
	for _, p := range f.near {
		worst = math.Max(worst, p[2]-math.Hypot(x-p[0], y-p[1]))
	}
	for _, p := range f.gates {
		sin, cos := math.Sincos(p[2])
		dx, dy := x-p[0], y-p[1]
		ahead, side := dx*sin+dy*cos, dx*cos-dy*sin
		if ahead > -pushTerminalMarginMeters && ahead < pushTerminalDepthMeters && math.Abs(side) <= p[3] {
			worst = math.Max(worst, ahead+pushTerminalMarginMeters)
		}
	}
	f.depthAt[k] = worst
	return worst
}

// intrusion is standIntrusion of the push along pts, from the kept depths.
func (f *flatPave) intrusion(pts []airport.LatLon, prof MotionProfile) float64 {
	span, tailLen := prof.SpanMeters, prof.TailMeters
	if span <= 0 {
		span = 35.8
	}
	if tailLen <= 0 {
		tailLen = 20.5
	}
	noseLen := prof.WheelbaseMeters * pushNoseFactor
	worst := math.Inf(-1)
	for i := 2; i < len(pts); i += 2 {
		gear, hdg := pts[i], localBearing(pts[i], pts[i-2])
		wing := offsetHeading(gear, hdg, 2)
		for _, q := range []airport.LatLon{offsetHeading(gear, hdg+180, tailLen), offsetHeading(wing, hdg-90, span/2), offsetHeading(wing, hdg+90, span/2), offsetHeading(gear, hdg, noseLen)} {
			worst = math.Max(worst, f.depth(q))
		}
	}
	return worst
}

// pushCand is a way to a pose: the push (main gear points), then the
// nose gear towed forward (tow, nil for none); push is its cost in taxi
// meters, cost a lower bound of the whole until the taxi-out is planned.
type pushCand struct {
	pts        []airport.LatLon
	at         int
	push, cost float64
	near       bool // within pushOffPavementMeters of the pavement
	tow        []airport.LatLon
}

// Push and pull: where no push alone leaves the aircraft facing the way
// out, the tug pushes it back to a pose (out of an alley, onto the lane)
// and tows it forward to another, turning it there — as tugs do, and as
// an alley push that starts the engines on the lane does.
const (
	// towMaxMeters: the longest tow after a push, the straight onto the pose
	// (towAlignMeters) included.
	towMaxMeters = 80.0
	// towPenalty: taxi meters a tow costs besides its length (the stop and
	// the tug's change of direction), so a push alone wins where it works.
	towPenalty = 150.0
	// towPushes: the cheapest pushes a tow may start from.
	towPushes = 12
	// towAlignMeters: a tow ends this far straight along the pose.
	towAlignMeters = 20.0
)

// pushAndTow are the pushes of pushes (cheapest first, towPushes of them)
// followed by a tow forward to another pose within towMaxMeters: the
// shortest path of turn radius PushbackMinArcMeters to PushbackArcMeters
// for the nose gear, the same clearances as a push. One per pose, the
// cheapest.
func (c *TaxiController) pushAndTow(poses []pushPose, pushes []pushCand, tailOffs []float64, prof MotionProfile, pv *flatPave, base float64) []pushCand {
	from := slices.Clone(pushes)
	slices.SortFunc(from, func(a, b pushCand) int { return cmp.Compare(a.push, b.push) })
	from = from[:min(len(from), towPushes)]
	best := map[int]pushCand{}
	for _, f := range from {
		start := poses[f.at]
		for j, p := range poses {
			if j == f.at || tailOffs[j] > pushOffPavementWideMeters || localDist(start.nose, p.nose) > towMaxMeters {
				continue
			}
			for r := PushbackArcMeters; r >= PushbackMinArcMeters-0.01; r -= 4 {
				tow := towTo(start, p, r)
				if tow == nil {
					continue
				}
				cost := f.push + pathLen(tow)*pushCostFactor + towPenalty + pushTurnRadiusCost*(PushbackArcMeters-r)
				if b, ok := best[j]; ok && cost >= b.push {
					continue
				}
				off, ok := towFits(tow, prof, pv, pushOffPavementWideMeters, base)
				if !ok {
					continue
				}
				near := f.near && math.Max(off, tailOffs[j]) <= pushOffPavementMeters
				best[j] = pushCand{pts: f.pts, at: j, push: cost, cost: p.lbTaxi + cost, near: near, tow: tow}
			}
		}
	}
	var out []pushCand
	for _, b := range best {
		out = append(out, b)
	}
	return out
}

// towTo is the nose gear path of a tow forward from pose a to pose b on
// turns of radius r, ending towAlignMeters straight onto b: towed, the main
// gear trails the nose gear and lines up only along a straight; nil if none.
func towTo(a, b pushPose, r float64) []airport.LatLon {
	pts := dubins(a.nose, a.heading, offsetHeading(b.nose, b.heading+180, towAlignMeters), b.heading, r, 0.5)
	if pts == nil {
		return nil
	}
	if n := len(pts); n > 2 && localDist(pts[n-2], pts[n-1]) < 0.25 {
		pts = append(pts[:n-2], pts[n-1])
	}
	return append(pts, b.nose)
}

// towFits is pushFits for a tow: not longer than towMaxMeters, no loop,
// the nose gear within tol of the pavement, the tail and wings clear of the
// neighbouring stands as for a push.
func towFits(pts []airport.LatLon, prof MotionProfile, pv *flatPave, tol, base float64) (float64, bool) {
	n := len(pts)
	if n < 3 || pathLen(pts) > towMaxMeters {
		return 0, false
	}
	net := math.Abs(headingDiff(localBearing(pts[0], pts[2]), localBearing(pts[n-3], pts[n-1])))
	if turn := totalTurn(pts); turn > pushMaxTurnDeg || turn > net+pushMaxSwerveDeg {
		return 0, false
	}
	off := 0.0
	for i := 0; i < n; i += 2 {
		if off = math.Max(off, pv.off(pts[i])); off > tol {
			return off, false
		}
	}
	// The main gear a wheelbase behind the nose gear, last first: the
	// intrusion of a push runs the other way.
	gear := make([]airport.LatLon, 0, n)
	for i := n - 1; i >= 1; i-- {
		gear = append(gear, offsetHeading(pts[i], localBearing(pts[i], pts[i-1]), prof.WheelbaseMeters))
	}
	return off, pv.intrusion(gear, prof) <= base+pushClearanceSlackMeters
}

// poseBlocks counts the junctions of other taxiways under the aircraft
// standing at the pose: nose to tail, half its span either side; not own,
// the junction of the stand's lead-in, which every push from it passes.
func (c *TaxiController) poseBlocks(p pushPose, own airport.NodeID) int {
	g, prof := c.req.Graph, c.profile()
	tail := prof.TailMeters
	if tail <= 0 {
		tail = 20.5
	}
	// From the nose, a wheelbase and a third ahead of the nose gear, to the
	// tail; the wings over the other taxiway block it too.
	ahead := prof.WheelbaseMeters * (pushNoseFactor - 1)
	length := prof.WheelbaseMeters + tail
	reach := c.halfSpan()
	n := 0
	for id, nd := range g.Nodes {
		if airport.NodeID(id) == own || nd.Kind == airport.NodeParking || len(g.Adj[id]) < 3 || localDist(nd.Position, p.nose) > length+reach {
			continue
		}
		other := false
		for _, a := range g.Adj[id] {
			if a.Name != "" && a.Name != p.lane && g.Nodes[a.To].Kind != airport.NodeParking {
				other = true
			}
		}
		if !other {
			continue
		}
		back := alongHeading(p.nose, p.heading+180, nd.Position)
		if back >= -ahead && back <= length && math.Abs(alongHeading(p.nose, p.heading+90, nd.Position)) <= reach {
			n++
		}
	}
	return n
}

// planPushPose chooses the pose the push ends in and the push to it: of
// the poses a tug can reach, the cheapest by the push (pushCostFactor per
// meter), the taxi-out from there, the taxiways left blocked and the turn
// radius given up. It sets the push and the route (from the pose's edge
// to the runway); false if no pose is reachable.
func (c *TaxiController) planPushPose() bool {
	g, prof := c.req.Graph, c.profile()
	stand := g.Layout.Parking[c.req.Parking]
	gear := offsetHeading(StandPoint(stand, c.req.NoseOffset), stand.Heading, -prof.RefAheadMeters)
	poses := c.pushPoses(gear)
	if len(poses) == 0 {
		return false
	}
	own := airport.NodeID(-1)
	if len(c.route.Nodes) > 1 {
		own = c.route.Nodes[1]
	}
	around := pavementAround(g, gear, pushPoseReachMeters+50)
	around.segs = append(around.segs, laneEnds(g, gear, pushPoseReachMeters+50)...)
	pv := newFlatPave(around, gear)
	pv.withStands(g, c.req.Parking)
	// As deep into a neighbouring stand as the parked aircraft already
	// reaches (sampled as a push is), or to its edge: a parked aircraft well
	// clear of its neighbours does not keep its push as far (LFPG M6).
	base := math.Max(0, pv.intrusion([]airport.LatLon{offsetHeading(gear, stand.Heading, 2), offsetHeading(gear, stand.Heading, 1), gear}, prof))
	tail := prof.TailMeters
	if tail <= 0 {
		tail = 20.5
	}
	// The pushes a tug can make, one per pose: the widest radius that fits.
	var cands []pushCand
	tailOffs := make([]float64, len(poses))
	for i := range poses {
		p := &poses[i]
		// The nose and the tail where the push ends, on the pavement too.
		tailOff := math.Max(pv.off(p.nose), pv.off(offsetHeading(p.nose, p.heading+180, prof.WheelbaseMeters*pushNoseFactor+tail)))
		tailOffs[i] = tailOff
		if tailOff > pushOffPavementWideMeters {
			continue
		}
		// The cheapest radius: a wide one can loop round where a tighter one
		// turns straight onto the taxiway (LKPR B14).
		var best *pushCand
		for r := PushbackArcMeters; r >= PushbackMinArcMeters-0.01; r -= 4 {
			pts := pushTo(gear, stand.Heading, *p, r, prof)
			if pts == nil {
				continue
			}
			push := pathLen(pts)*pushCostFactor + pushTurnRadiusCost*(PushbackArcMeters-r)
			if best != nil && push >= best.push {
				continue
			}
			if off, ok := pushFits(pts, prof, pv, pushOffPavementWideMeters, base); ok {
				best = &pushCand{pts: pts, at: i, push: push, cost: p.lbTaxi + push, near: math.Max(off, tailOff) <= pushOffPavementMeters}
			}
		}
		if best != nil {
			cands = append(cands, *best)
		}
	}
	// Then the taxi-outs (a route search each), cheapest bound first: within
	// pushOffPavementMeters of the pavement as modelled (stand circles,
	// taxiway strips) if any push is, else within pushOffPavementWideMeters
	// (the apron is wider than the model).
	routes := map[[2]airport.NodeID]*airport.Route{}
	found := 0 // taxi-outs planned (the searches that found none do not count)
	choose := func(cands []pushCand) *pushCand {
		slices.SortFunc(cands, func(a, b pushCand) int { return cmp.Compare(a.cost, b.cost) })
		var best *pushCand
		for _, near := range []bool{true, false} {
			for i := range cands {
				cd := &cands[i]
				if cd.near != near {
					continue
				}
				if best != nil && cd.cost >= best.cost {
					break // the rest cost more
				}
				p := &poses[cd.at]
				if p.out == nil {
					if _, planned := routes[[2]airport.NodeID{p.from, p.to}]; !planned && (found >= pushPoseRoutes || len(routes) >= pushPoseSearches) {
						continue
					}
					if p.out = c.poseRoute(*p, routes); p.out == nil {
						continue
					}
					found++
					p.fixed = p.taxi + p.out.Cost + float64(c.poseBlocks(*p, own))*pushBlockPenalty
					if hairpinAfterPush(p.out) {
						p.fixed += pushHairpinPenalty
					}
					if !p.aligned() {
						p.fixed += pushMisalignPenalty
					}
					if p.tight {
						p.fixed += pushTightPenalty
					}
				}
				if cd.cost = p.fixed + cd.push; best == nil || cd.cost < best.cost {
					best = cd
				}
			}
			if best != nil {
				break
			}
		}
		if best == nil {
			return nil
		}
		b := *best
		return &b
	}
	best := choose(cands)
	// A tow after the push only where no push alone ends cleanly: the pose
	// costs more than its taxi-out (a turn off the nose, a hairpin, a lane
	// held, a junction blocked), or there is none.
	if best == nil || poses[best.at].fixed > poses[best.at].taxi+poses[best.at].out.Cost {
		if tows := c.pushAndTow(poses, cands, tailOffs, prof, pv, base); len(tows) > 0 {
			if b := choose(append(cands, tows...)); b != nil {
				best = b
			}
		}
	}
	if best == nil {
		return false
	}
	p := poses[best.at]
	p.own = own
	full, err := g.RouteFromNodes(append([]airport.NodeID{p.from}, p.out.Nodes...))
	if err != nil {
		return false
	}
	full.Runway, full.RunwayEnd, full.Entry, full.HoldShort, full.Tight = p.out.Runway, p.out.RunwayEnd, p.out.Entry, p.out.HoldShort, p.out.Tight
	c.route, c.pushJunction, c.pushPts, c.towPts, c.pushPose = full, 0, best.pts, best.tow, &p
	return true
}
