package traffic

import (
	"math"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// GroundPicture is what the injected aircraft on the ground know of each
// other (#334). Each controller reports its aircraft every frame; while
// taxiing, each looks ahead along its own path for another aircraft's body
// in its way — one ahead on the same taxiway, one waiting at the holding
// point, one crossing — and stops behind it at a safe gap, moving on when
// it moves. One picture serves all the controllers at an airport
// (TaxiWithGroundPicture, ArrivalWithGroundPicture).
type GroundPicture struct {
	mu       sync.Mutex
	aircraft map[uint32]groundEntry
	vehicles map[uint32]vehicleEntry // service vehicles (ReportVehicle)
	// follows: by aircraft, the one ground told it to follow (Follow).
	follows map[uint32]uint32
}

type groundEntry struct {
	id         uint32 // the object (set where the picture is read)
	pos        airport.LatLon
	hdg        float64
	nose, tail float64 // meters ahead of and behind the reference point
	at         time.Time
	// Where it will drive next, up to its next stop, every trafficBodyStep
	// meters (ReportPath), and its half-span.
	ahead []airport.LatLon
	half  float64
	// pushing: ahead is a pushback under way, which taxiing traffic gives
	// way to whatever the distances.
	pushing bool
	// waiting: ahead is the way it will taxi once cleared (#452): pushes
	// do not start into it, but it has no priority over moving traffic.
	waiting bool
	// intent: the way it means to taxi from where it is, past any stop
	// (holding short to cross too), OncomingLookMeters of it (#775).
	intent []airport.LatLon
	// behind: the aircraft it stands stopped behind, in its way (not one
	// coming the other way it holds for); 0 none (ReportBehind).
	behind uint32
}

// NewGroundPicture creates an empty picture.
func NewGroundPicture() *GroundPicture {
	return &GroundPicture{aircraft: map[uint32]groundEntry{}}
}

// Report records aircraft id at its reference point pos, heading hdg (true
// degrees), with the airframe of prof.
func (p *GroundPicture) Report(id uint32, pos airport.LatLon, hdg float64, prof MotionProfile, now time.Time) {
	if prof == (MotionProfile{}) {
		prof = DefaultMotionProfile()
	}
	tail := prof.TailMeters
	if tail <= 0 {
		tail = 20.5
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.aircraft[id] = groundEntry{
		pos: pos, hdg: hdg, at: now, ahead: p.aircraft[id].ahead, half: p.aircraft[id].half, pushing: p.aircraft[id].pushing, waiting: p.aircraft[id].waiting,
		intent: p.aircraft[id].intent,
		nose:   prof.WheelbaseMeters*pushNoseFactor - prof.RefAheadMeters,
		tail:   tail + prof.RefAheadMeters,
	}
}

// ReportPath records where aircraft id will drive next: its path ahead up
// to its next stop, sampled every trafficBodyStep meters (nil when it is
// not taxiing), and its half-span, for giving way (#334).
func (p *GroundPicture) ReportPath(id uint32, ahead []airport.LatLon, half float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.aircraft[id]; ok {
		e.ahead, e.half, e.pushing, e.waiting = ahead, half, false, false
		p.aircraft[id] = e
	}
}

// ReportPlanned records the way aircraft id will taxi once cleared, while
// it waits for the clearance after its push (#452; LKPR, live: TVS706 was
// cleared to push onto A1 where TVS795, pushed there a moment before, was
// about to taxi; TVS795 then drove through the push). A push does not
// start across it; taxiing traffic does not give way to it.
func (p *GroundPicture) ReportPlanned(id uint32, ahead []airport.LatLon, half float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.aircraft[id]; ok {
		e.ahead, e.half, e.pushing, e.waiting = ahead, half, false, len(ahead) > 0
		p.aircraft[id] = e
	}
}

// ReportPush records the corridor a pushback under way still sweeps
// (nil once done): taxiing traffic whose path crosses it gives way.
func (p *GroundPicture) ReportPush(id uint32, corridor []airport.LatLon, half float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.aircraft[id]; ok {
		e.ahead, e.half, e.pushing, e.waiting = corridor, half, len(corridor) > 0, false
		p.aircraft[id] = e
	}
}

// giveWay is where along path (after from) this aircraft stops to give
// way: the first point within look where its path comes within both
// half-spans (plus GiveWayMarginMeters) of another aircraft's path ahead,
// when the other is closer to that point (a tie goes to the lower ID).
// The one further away waits, the other goes, so exactly one of them
// stops; an aircraft stopping before the point (holding short, at its
// clearance limit, giving way itself) reports no path there and takes no
// priority. +Inf when there is nobody to give way to.
func (p *GroundPicture) giveWay(id uint32, path *GroundPath, from, look, half float64, now time.Time) float64 {
	at, _ := p.giveWayTo(id, path, from, look, half, now)
	return at
}

// wayFromHereMeters: a way reported for an aircraft that starts further
// from it than this is an old one (wayFromHere).
const wayFromHereMeters = 80.0

// wayFromHere reports that way, reported for an aircraft at pos, is still
// its way: it starts by it. One that took off keeps the way it last
// reported on the ground while its position runs on (live, LKPR: HBGOS
// gave way for good to OKHAQ's line-up path at B after OKHAQ had gone).
func wayFromHere(pos airport.LatLon, way []airport.LatLon) bool {
	return len(way) == 0 || localDist(pos, way[0]) <= wayFromHereMeters
}

// Pushing reports whether aircraft id is pushing back now; Waiting that it
// has pushed and waits for its taxi clearance (its planned way reported).
func (p *GroundPicture) Pushing(id uint32) bool { return p.pushingNow(id) }

// Waiting: see Pushing.
func (p *GroundPicture) Waiting(id uint32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.aircraft[id]
	return ok && e.waiting
}

// pushingNow reports whether aircraft id is pushing back (taxiing traffic
// gives way to it whatever happens).
func (p *GroundPicture) pushingNow(id uint32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.aircraft[id]
	return ok && e.pushing
}

// giveWayTo is giveWay and the aircraft given way to (0 for none).
func (p *GroundPicture) giveWayTo(id uint32, path *GroundPath, from, look, half float64, now time.Time) (float64, uint32) {
	type other struct {
		id uint32
		e  groundEntry
	}
	p.mu.Lock()
	var others []other
	for oid, e := range p.aircraft {
		if oid != id && len(e.ahead) > 0 && !e.waiting && now.Sub(e.at) <= TrafficStaleAfter && wayFromHere(e.pos, e.ahead) {
			others = append(others, other{oid, e})
		}
	}
	me, haveMe := p.aircraft[id]
	lead := p.follows[id]
	followers := map[uint32]bool{}
	for f, l := range p.follows {
		if l == id {
			followers[f] = true
		}
	}
	p.mu.Unlock()
	// bodyInPush: this aircraft's body is where a push under way stops for
	// it (corridorBlocked: the push's half-span and PushClearMarginMeters
	// round any point of the fuselage). It must go on then, or each waits
	// for the other (live, LKPR: CSA383's tail in LOT277's corridor, its nose
	// past it, waiting for the push that waited for it, 19 minutes).
	bodyInPush := func(o groundEntry) bool {
		if !haveMe {
			return false
		}
		oh := o.half
		if oh <= 0 {
			oh = DefaultHalfSpanMeters
		}
		for d := -me.tail; d <= me.nose+0.01; d += trafficBodyStep {
			q := offsetHeading(me.pos, me.hdg, d)
			for _, c := range o.ahead {
				if localDist(q, c) <= oh+PushClearMarginMeters {
					return true
				}
			}
		}
		return false
	}
	best, to := math.Inf(1), uint32(0)
	if len(others) == 0 {
		return best, 0
	}
	end := math.Min(path.Length(), from+look)
	var mine []airport.LatLon // this aircraft's path ahead
	for s := from; s <= end; s += trafficBodyStep {
		mine = append(mine, path.PointAt(s))
	}
	// first is how far along a the first point within reach of b lies
	// (-1 for none): measured the same way for both aircraft, so both
	// come to the same decision.
	first := func(a, b []airport.LatLon, reach float64) float64 {
		for i, q := range a {
			for _, r := range b {
				if localDist(q, r) <= reach {
					return float64(i) * trafficBodyStep
				}
			}
		}
		return -1
	}
	for _, o := range others {
		reach := half + o.e.half + GiveWayMarginMeters
		// In front on this aircraft's path, going the same way: followed at
		// a gap (blocking), not given way to — it was stopped behind it
		// until the other was 250 m on (live, OKOPA behind a B737).
		if !o.e.pushing && sameWayAhead(mine, o.e.pos, o.e.hdg, half+o.e.half) {
			continue
		}
		// Told to follow: behind its leader wherever their ways meet; a
		// follower is never given way to.
		if followers[o.id] {
			continue
		}
		if o.id == lead {
			if at := first(mine, o.e.ahead, reach); at >= 0 {
				if from+at < best {
					best, to = from+at, o.id
				}
				continue
			}
		}
		// Its body still across this path, its way ahead no longer meeting
		// it: the tail of one that has crossed (live, LKPR J/H: CSA194's
		// wing and the tail of QTR1709's B77W, whose path ahead had left the
		// junction). Stop short until it is clear; while their ways still
		// meet, the path rules below decide.
		if first(o.e.ahead, mine, reach) < 0 && !(o.e.pushing && bodyInPush(o.e)) {
			var body []airport.LatLon
			for d := -o.e.tail; d <= o.e.nose+0.01; d += trafficBodyStep {
				body = append(body, offsetHeading(o.e.pos, o.e.hdg, d))
			}
			if bodyTo := first(mine, body, half+GiveWayMarginMeters+TailplaneShare*o.e.half); bodyTo >= 0 {
				if from+bodyTo < best {
					best, to = from+bodyTo, o.id
				}
				continue
			}
		}
		mineTo := first(mine, o.e.ahead, reach)
		// Beside a push under way only an aircraft already close enough for
		// the push to stop for it (corridorBlocked: both half-spans and
		// PushClearMarginMeters) goes on through, or each would wait for the
		// other; one merely within the margin waits where it is (#452:
		// TVS795, waiting at the end of TVS706's corridor, drove through it).
		inIt := mineTo == 0 && (!o.e.pushing || first(mine[:1], o.e.ahead, half+o.e.half+PushClearMarginMeters) == 0)
		if mineTo < 0 || mineTo < half && (!o.e.pushing || inIt) || o.e.pushing && bodyInPush(o.e) {
			continue // no conflict, or already in it: go on through
		}
		// (Beside a push under way it waits where it is unless already in its
		// corridor: at LKPR DLH977, waiting at the edge of TVS1960's push,
		// drove into it; each then stopped for the other for 8 minutes.)
		theirsTo := first(o.e.ahead, mine, reach)
		if !o.e.pushing {
			theirsTo += trafficBodyStep // their path starts a step ahead of them (ReportPath)
		}
		if o.e.pushing || theirsTo < mineTo || (theirsTo == mineTo && o.id < id) {
			if from+mineTo < best {
				best, to = from+mineTo, o.id
			}
		}
	}
	return best, to
}

// corridorBlocked reports another aircraft in the way of a pushback along
// corridor (points the pushing aircraft sweeps, with half its span): its
// fuselage within half+PushClearMarginMeters of the corridor, or its path
// ahead within both half-spans plus GiveWayMarginMeters — traffic taxiing
// behind the stand. Parked neighbours are a stand spacing away and do not
// count. It returns the first such aircraft's ID.
func (p *GroundPicture) corridorBlocked(id uint32, corridor []airport.LatLon, half float64, withPaths bool, now time.Time) (uint32, bool) {
	p.mu.Lock()
	type other struct {
		id uint32
		e  groundEntry
	}
	var others []other
	for oid, e := range p.aircraft {
		if oid != id && now.Sub(e.at) <= TrafficStaleAfter {
			others = append(others, other{oid, e})
		}
	}
	p.mu.Unlock()
	near := func(q airport.LatLon, reach float64) bool {
		for _, c := range corridor {
			if localDist(q, c) <= reach {
				return true
			}
		}
		return false
	}
	for _, o := range others {
		oh := o.e.half
		if oh <= 0 {
			oh = DefaultHalfSpanMeters
		}
		// A moving aircraft (pushing, taxiing) keeps its wings clear too: at
		// LKPR DLH1740 pushed from A3 while TVS1823, pushed up A1, was a
		// fuselage and 3 m from its corridor, a wing inside it (#446). A
		// parked neighbour is a stand spacing away, wing to wing.
		// Only before a push starts (withPaths): under way a push stops for a
		// body in the way, and an aircraft giving way to it, stopped short
		// with a little path left, would hold it for ever while it waits for
		// the push (#466; LKPR, live: AFR657 and AFR1246).
		reach := half + PushClearMarginMeters
		if withPaths && len(o.e.ahead) > 0 {
			reach += oh
		}
		for d := -o.e.tail; d <= o.e.nose+0.01; d += trafficBodyStep {
			if near(offsetHeading(o.e.pos, o.e.hdg, d), reach) {
				return o.id, true
			}
		}
		if !withPaths {
			continue // a push under way stops for bodies only; others give way to it
		}
		// Not started yet: it waits for the way ahead of taxiing traffic and
		// for the rest of a neighbour's push — two pushes into the same
		// corridor would each stop for the other's body and stay there.
		for _, r := range o.e.ahead {
			if near(r, half+oh+GiveWayMarginMeters) {
				return o.id, true
			}
		}
	}
	return 0, false
}

// Forget drops aircraft id (airborne, parked for good, removed).
func (p *GroundPicture) Forget(id uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.aircraft, id)
	delete(p.follows, id)
	for f, l := range p.follows {
		if l == id {
			delete(p.follows, f) // its leader gone
		}
	}
}

// Follow records that aircraft id was told to follow leader on the ground
// ("follow the company Airbus"): it gives way to the leader wherever their
// ways meet, and the leader never to it. A zero leader ends it.
func (p *GroundPicture) Follow(id, leader uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if leader == 0 || leader == id {
		delete(p.follows, id)
		return
	}
	if p.follows == nil {
		p.follows = map[uint32]uint32{}
	}
	p.follows[id] = leader
}

// Following is the aircraft id was told to follow, 0 for none.
func (p *GroundPicture) Following(id uint32) uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.follows[id]
}

// blocking is how far along path (between from and from+look) the nearest
// point of another aircraft's body lies within half meters of it: the
// aircraft to stop behind, returned too. +Inf when the way is free. Reports
// older than TrafficStaleAfter are ignored.
func (p *GroundPicture) blocking(id uint32, path *GroundPath, from, look, half float64, now time.Time) (float64, groundEntry) {
	p.mu.Lock()
	others := make([]groundEntry, 0, len(p.aircraft))
	for oid, e := range p.aircraft {
		if oid != id && now.Sub(e.at) <= TrafficStaleAfter {
			e.id = oid
			others = append(others, e)
		}
	}
	p.mu.Unlock()
	if len(others) == 0 {
		return math.Inf(1), groundEntry{}
	}
	// The path between from and from+look, as sample points with distances.
	pts, cum := path.pts, path.cum
	lo := 0
	for lo < len(cum)-1 && cum[lo+1] < from {
		lo++
	}
	best, who := math.Inf(1), groundEntry{}
	start := path.PointAt(from)
	// In flat metres around start (one cosine per frame, not several per
	// point pair: frames of 130 ms, live at LKPR), the path's segments ahead
	// as start points and unit directions.
	f := newFlat(start)
	hi := lo
	for hi+1 < len(pts) && cum[hi] <= from+look {
		hi++
	}
	sc := groundPool.Get().(*groundScratch)
	defer groundPool.Put(sc)
	segs := grow(sc.segs, hi-lo)
	sc.segs = segs
	for i := lo; i < hi; i++ {
		ax, ay := f.xy(pts[i])
		bx, by := f.xy(pts[i+1])
		l := math.Hypot(bx-ax, by-ay)
		s := groundSeg{ax: ax, ay: ay, l: cum[i+1] - cum[i]}
		if l > 0 {
			s.ux, s.uy = (bx-ax)/l, (by-ay)/l
		}
		segs[i-lo] = s
	}
	half2 := half * half
	near := sc.near[:0]
	defer func() { sc.near = near }()
	for _, o := range others {
		// Out of reach of the path ahead (no point of it is further than
		// look from start along it): not tested point by point, the most
		// of the frame's time with many taxiing (#370).
		ox, oy := f.xy(o.pos)
		if r := look + half + math.Max(o.nose, o.tail) + 1; ox*ox+oy*oy > r*r {
			continue
		}
		hs, hc := math.Sincos(o.hdg * math.Pi / 180)
		// Only the segments that can come within half of its body: their
		// start within body + segment + half of its position.
		body := math.Max(o.nose, o.tail)
		near = near[:0]
		for i := lo; i < hi; i++ {
			sg := segs[i-lo]
			dx, dy := ox-sg.ax, oy-sg.ay
			if r := body + sg.l + half + 1; dx*dx+dy*dy <= r*r {
				near = append(near, i)
			}
		}
		if len(near) == 0 {
			continue
		}
		// Points along the other aircraft's axis, nose to tail.
		for d := -o.tail; d <= o.nose+0.01; d += trafficBodyStep {
			qx, qy := ox+hs*d, oy+hc*d
			for _, i := range near {
				sg := segs[i-lo]
				along := math.Max(0, math.Min(sg.l, (qx-sg.ax)*sg.ux+(qy-sg.ay)*sg.uy))
				s := cum[i] + along
				if s < from || s >= best {
					continue
				}
				dx, dy := qx-(sg.ax+sg.ux*along), qy-(sg.ay+sg.uy*along)
				if dx*dx+dy*dy <= half2 {
					best, who = s, o
				}
			}
		}
	}
	return best, who
}

// sameWayAhead reports whether an aircraft at pos facing hdg is on path
// (the points ahead of this one, nearest first) within reach of it, facing
// within sameWayDeg of the path's direction there: traffic in front going
// the same way.
func sameWayAhead(path []airport.LatLon, pos airport.LatLon, hdg, reach float64) bool {
	for i := 1; i < len(path); i++ {
		if localDist(path[i], pos) <= reach {
			return math.Abs(headingDiff(localBearing(path[i-1], path[i]), hdg)) <= sameWayDeg
		}
	}
	return false
}

// sameWayDeg: facing within this of the path's direction is going the
// same way (sameWayAhead).
const sameWayDeg = 45.0

// ReportIntent records the way aircraft id means to taxi from where it is,
// past any stop (nil none), for oncoming traffic to hold clear of (#775).
func (p *GroundPicture) ReportIntent(id uint32, intent []airport.LatLon) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.aircraft[id]; ok {
		e.intent = intent
		p.aircraft[id] = e
	}
}

// ReportBehind records that aircraft id is stopped behind aircraft by, in
// its way (0: not).
func (p *GroundPicture) ReportBehind(id, by uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.aircraft[id]; ok {
		e.behind = by
		p.aircraft[id] = e
	}
}

// oncoming finds, along the whole of path from from, the first point where
// another aircraft means to taxi the other way along the same pavement
// (its intent within both half-spans and HeadOnMarginMeters, facing at
// least oncomingDeg away) — where, on one taxiway, the two would meet
// nose to nose (#775: LKPR, live, WZZ1023 and TVS1771 on F). hold: this
// aircraft is the one to hold, being further from that shared stretch
// than the other is (a tie: the higher ID holds); one already in it
// (within headOnCommitMeters) never holds. +Inf when none. Both whole
// ways, not the next OncomingLookMeters: a stretch longer than that is
// decided before either is in it.
func (p *GroundPicture) oncoming(id uint32, path *GroundPath, from, half float64, now time.Time) (at float64, other uint32, hold bool) {
	p.mu.Lock()
	type them struct {
		id     uint32
		intent []airport.LatLon
		half   float64
	}
	var others []them
	for oid, e := range p.aircraft {
		if oid == id || now.Sub(e.at) > TrafficStaleAfter {
			continue
		}
		// Stopped behind this one, in its way: not held for, or neither goes
		// (live, LKPR: CSA839 held on J for TVS1120, which stood behind
		// it, 19 minutes until TVS1120 was removed).
		if e.behind == id {
			continue
		}
		way := e.intent
		if !wayFromHere(e.pos, way) {
			way = nil
		}
		if len(way) < 2 && e.waiting {
			// Pushed and waiting for its taxi clearance: its planned way
			// counts too (live, LKPR: TVS770 turned into J nose to nose with
			// TVS1309 waiting there, though told to give way to it).
			way = e.ahead
		}
		// Its whole way: a shared stretch longer than OncomingLookMeters
		// is seen before either is in it (live, LKPR: TVS681 and CSA398
		// met in the middle of F, neither able to give way). Looked at
		// every OncomingCheckEvery, on a grid (the cost of every point of
		// every way each frame once held every aircraft back: frames of
		// 130 ms).
		if len(way) > 1 {
			h := e.half
			if h <= 0 {
				h = DefaultHalfSpanMeters
			}
			others = append(others, them{oid, way, h})
		}
	}
	p.mu.Unlock()
	at = math.Inf(1)
	// This one's whole way ahead, sampled once for all the others in flat
	// metres around where it is, hashed into cells of oncomingCell metres.
	start := path.PointAt(from)
	f := newFlat(start)
	sc := groundPool.Get().(*groundScratch)
	defer groundPool.Put(sc)
	mineAt := sc.mine[:0]
	defer func() { sc.mine = mineAt }()
	end := path.Length()
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for s := from; s <= end; s += oncomingStep {
		a, b := s, math.Min(s+oncomingStep, end) // its heading: on to the next, the last from the one before
		if b-a < 1 {
			a = math.Max(0, s-oncomingStep)
		}
		ax, ay := f.xy(path.PointAt(a))
		bx, by := f.xy(path.PointAt(b))
		sx, sy := f.xy(path.PointAt(s))
		mineAt = append(mineAt, groundSample{s, sx, sy, math.Mod(math.Atan2(bx-ax, by-ay)*180/math.Pi+360, 360)})
		minX, maxX, minY, maxY = math.Min(minX, sx), math.Max(maxX, sx), math.Min(minY, sy), math.Max(maxY, sy)
	}
	if len(mineAt) == 0 {
		return at, other, hold
	}
	// A dense grid over its box: cell c holds mineAt[order[first[c]:first[c+1]]].
	cx0, cy0 := math.Floor(minX/oncomingCell), math.Floor(minY/oncomingCell)
	nx, ny := int(math.Floor(maxX/oncomingCell)-cx0)+1, int(math.Floor(maxY/oncomingCell)-cy0)+1
	cellOf := func(x, y float64) (int, int) {
		return int(math.Floor(x/oncomingCell) - cx0), int(math.Floor(y/oncomingCell) - cy0)
	}
	first := grow(sc.first, nx*ny+1)
	sc.first = first
	clear(first)
	for _, m := range mineAt {
		i, j := cellOf(m.x, m.y)
		first[i*ny+j+1]++
	}
	for c := 1; c < len(first); c++ {
		first[c] += first[c-1]
	}
	order := grow(sc.order, len(mineAt))
	sc.order = order
	fill := grow(sc.fill, nx*ny)
	sc.fill = fill
	copy(fill, first[:nx*ny])
	for k, m := range mineAt {
		i, j := cellOf(m.x, m.y)
		order[fill[i*ny+j]] = int32(k)
		fill[i*ny+j]++
	}
	for _, o := range others {
		reach := half + o.half + HeadOnMarginMeters
		reach2 := reach * reach
		n := int(math.Ceil(reach / oncomingCell))
		// Where each first comes onto the shared stretch, along its own way:
		// the same pair of numbers whichever of the two looks.
		mine, theirs := math.Inf(1), math.Inf(1)
		cum := 0.0
		for i := 0; i+1 < len(o.intent); i++ {
			ax, ay := f.xy(o.intent[i])
			bx, by := f.xy(o.intent[i+1])
			l := math.Hypot(bx-ax, by-ay)
			// A segment off this one's box (grown by reach): not sampled.
			if l < 1e-6 || math.Max(ax, bx) < minX-reach || math.Min(ax, bx) > maxX+reach ||
				math.Max(ay, by) < minY-reach || math.Min(ay, by) > maxY+reach {
				cum += l
				continue
			}
			hdg := math.Mod(math.Atan2(bx-ax, by-ay)*180/math.Pi+360, 360)
			for t := 0.0; t < l; t += oncomingStep {
				x, y := ax+(bx-ax)*t/l, ay+(by-ay)*t/l
				ci, cj := cellOf(x, y)
				for di := max(ci-n, 0); di <= min(ci+n, nx-1); di++ {
					for dj := max(cj-n, 0); dj <= min(cj+n, ny-1); dj++ {
						c := di*ny + dj
						for _, k := range order[first[c]:first[c+1]] {
							m := mineAt[k]
							ex, ey := m.x-x, m.y-y
							if ex*ex+ey*ey <= reach2 && math.Abs(headingDiff(m.hdg, hdg)) >= oncomingDeg {
								if math.IsInf(theirs, 1) {
									theirs = cum + t
								}
								mine = math.Min(mine, m.s-from)
							}
						}
					}
				}
			}
			cum += l
		}
		if math.IsInf(mine, 1) {
			continue
		}
		me := mine > theirs || mine == theirs && id > o.id
		if mine <= headOnCommitMeters && theirs > headOnCommitMeters {
			me = false // in it already, the other not: the other holds
		}
		if from+mine < at {
			at, other, hold = from+mine, o.id, me
		}
	}
	return at, other, hold
}

// Head-on: HeadOnMarginMeters added to both half-spans for two ways to be
// the same pavement; an aircraft within headOnCommitMeters of the shared
// stretch is in it (#775).
const (
	HeadOnMarginMeters = 5.0
	headOnCommitMeters = 15.0
	// OncomingLookMeters: how far ahead a taxiing aircraft reports its
	// intent and looks for one coming the other way.
	OncomingLookMeters = 400.0
)

// ReportUserMotion records where the user aircraft id (flown by the user,
// its way unknown) is going on the ground, for ours and the service
// vehicles to give way to: moving forward, its track ahead along its
// heading for UserLookSeconds at its speed (at least UserLookMeters);
// pushing back (or cleared to push), the corridor behind it,
// UserPushMeters from its tail; stopped, nothing. half is its half-span,
// tail how far its tail is behind pos.
func (p *GroundPicture) ReportUserMotion(id uint32, pos airport.LatLon, hdg, kts, half, tail float64, pushing bool) {
	var pts []airport.LatLon
	switch {
	case pushing:
		from := offsetHeading(pos, hdg+180, tail)
		for d := 0.0; d <= UserPushMeters; d += trafficBodyStep {
			pts = append(pts, offsetHeading(from, hdg+180, d))
		}
		p.ReportPush(id, pts, half)
		return
	case kts > UserMovingKts:
		look := math.Max(UserLookMeters, kts*ktsToMS*UserLookSeconds)
		for d := 0.0; d <= look; d += trafficBodyStep {
			pts = append(pts, offsetHeading(pos, hdg, d))
		}
	}
	p.ReportPath(id, pts, half)
}

// The user aircraft on the ground (ReportUserMotion): moving above
// UserMovingKts; looked ahead UserLookSeconds, at least UserLookMeters; a
// push sweeping UserPushMeters behind its tail.
const (
	UserMovingKts   = 3.0
	UserLookSeconds = 20.0
	UserLookMeters  = 60.0
	UserPushMeters  = 60.0
)

// wayAhead is way up to meters along it from its first point.
func wayAhead(way []airport.LatLon, meters float64) []airport.LatLon {
	d := 0.0
	for i := 1; i < len(way); i++ {
		d += localDist(way[i-1], way[i])
		if d > meters {
			return way[:i+1]
		}
	}
	return way
}

// flat is a local plane around an origin, in metres east (x) and north (y)
// at its latitude's scale: an airport's points without a cosine each
// (localDist, localBearing take one per call).
type flat struct {
	o  airport.LatLon
	kx float64
}

func newFlat(o airport.LatLon) flat {
	return flat{o, metersPerDegree * math.Cos(o.Lat*math.Pi/180)}
}

// at is the point x metres east, y north of the origin (xy's inverse).
func (f flat) at(x, y float64) airport.LatLon {
	return airport.LatLon{Lat: f.o.Lat + y/metersPerDegree, Lon: f.o.Lon + x/f.kx}
}

func (f flat) xy(p airport.LatLon) (x, y float64) {
	return (p.Lon - f.o.Lon) * f.kx, (p.Lat - f.o.Lat) * metersPerDegree
}

// groundScratch is the per-frame working space of the traffic checks
// (blocking, oncoming), pooled: every taxiing aircraft runs them every
// frame, and fresh slices each time fed the garbage collector, whose work
// held every aircraft's frames back.
type groundScratch struct {
	segs             []groundSeg
	mine             []groundSample
	near, cand       []int
	ox, oy, cum, hdg []float64
	// first, order, fill: mine by grid cell (oncoming).
	first, order, fill []int32
}

// The head-on check's grid (oncoming): both ways sampled every
// oncomingStep metres, this one's sorted into cells of oncomingCell metres.
const (
	oncomingStep = 5.0
	oncomingCell = 50.0
)

type groundSeg struct{ ax, ay, ux, uy, l float64 }

type groundSample struct{ s, x, y, hdg float64 }

var groundPool = sync.Pool{New: func() any { return new(groundScratch) }}

// grow is b resized to n, reusing its array when it is large enough.
func grow[T any](b []T, n int) []T {
	if cap(b) < n {
		return make([]T, n)
	}
	return b[:n]
}
