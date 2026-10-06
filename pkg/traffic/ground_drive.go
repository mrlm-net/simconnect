package traffic

import (
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// groundDrive is the injected ground driving shared by arrivals and
// departures (#309, #320), so both follow the same rules: the mover, the
// phase lights with strobes and landing lights added while crossing a
// runway, the runway-crossing zones between hold-short lines and their
// clearance holds, and the taxi light a moment after the landing lights.
type groundDrive struct {
	injector *Injector
	object   uint32
	graph    *airport.Graph
	prof     MotionProfile
	clock    func() time.Time
	record   func(desc string, err error)

	mover    *GroundMover
	lastStep time.Time
	frameDt  float64 // seconds since the previous frame

	lights Lights // phase lights; logo and wing stay as the aircraft had them
	// noLogo: a light aircraft, with neither logo nor wing light (live,
	// OKFHP: an FSLTL King Air parked with its tail light flashing).
	noLogo   bool
	crossing bool // strobes and landing lights added for a runway crossing
	// ignoreRunway is excluded from the geometric crossing check (the runway
	// being vacated or lined up on); -1 for none.
	ignoreRunway int
	taxiLightAt  time.Time // the taxi light comes on then, once

	crossZones      []crossZone
	nextCross       int  // next crossing zone ahead
	crossClears     int  // crossing clearances not used yet
	holdAtCrossings bool // stop short of crossings until cleared

	// Ground traffic (#334): the picture shared with the other aircraft,
	// and whether this one follows the traffic ahead (while taxiing).
	picture       *GroundPicture
	followTraffic bool
	// planned is the way it will taxi, reported while it waits for its taxi
	// clearance (#452).
	planned []airport.LatLon
	// givingWay is the aircraft it gives way to now (followAhead), 0 none.
	givingWay uint32
	// blockedBy is the aircraft ahead it stops behind (followAhead), 0 none.
	blockedBy uint32
	// facing: blockedBy is oncoming, held clear of at a junction (#775).
	facing bool
	trafficAt time.Time // last look ahead (every TrafficCheckEvery)

	// A stop of its own on the path (a de-icing pad, #323), apart from the
	// clearance limit: no taxi clearance takes it away.
	padStop float64
	hasPad  bool

	// Progressive taxi (#322): the clearance limit on the current path.
	limit     float64
	hasLimit  bool
	limitNode airport.NodeID
}

// advance steps the mover to now and places the aircraft.
func (d *groundDrive) advance() (GroundPose, error) {
	now := d.clock()
	dt := math.Max(0, math.Min(now.Sub(d.lastStep).Seconds(), MaxFrameStepSeconds))
	d.lastStep, d.frameDt = now, dt
	d.followAhead(now)
	pose := d.mover.Step(dt)
	return pose, d.injector.Place(d.object, pose)
}

// setInjectedLights changes the phase lights; logo and wing lights stay as
// the aircraft had them.
func (d *groundDrive) setInjectedLights(l Lights, desc string) {
	l.Logo, l.Wing = d.lights.Logo, d.lights.Wing
	if d.noLogo {
		l.Logo, l.Wing = false, false
	}
	d.lights = l
	d.applyLights(desc)
}

// applyLights sends the phase lights, with strobes and landing lights on
// while crossing a runway: the aircraft must be conspicuous on it.
func (d *groundDrive) applyLights(desc string) {
	l := d.lights
	if d.crossing {
		l.Strobe, l.Landing = true, true
	}
	d.record(desc, d.injector.SetLights(d.object, l))
}

// taxiLightDue switches the taxi light on once taxiLightAt has passed.
func (d *groundDrive) taxiLightDue() {
	if !d.taxiLightAt.IsZero() && !d.lights.Taxi && !d.clock().Before(d.taxiLightAt) {
		d.setInjectedLights(LightsTaxi, "lights taxi")
	}
}

// checkCrossing switches the crossing lights: between the hold-short lines
// of a crossing, from just after the nose gear passes the first until a
// moment after the tail has passed the opposite one, and whenever the nose
// gear or the reference point is within RunwayClearMeters of a runway other
// than ignoreRunway.
func (d *groundDrive) checkCrossing(pose GroundPose) {
	on := false
	for _, z := range d.crossZones {
		if pose.Distance >= z.from+CrossingOnMeters && pose.Distance-d.prof.WheelbaseMeters-CrossingTailMeters <= z.to {
			on = true
		}
	}
	nose := NoseGear(pose.Position, pose.Heading, d.prof)
	for _, p := range []airport.LatLon{nose, pose.Position} {
		if r := d.graph.RunwayAt(p, RunwayClearMeters); r >= 0 && r != d.ignoreRunway {
			on = true
		}
	}
	if on != d.crossing {
		d.crossing = on
		desc := "lights runway crossing off"
		if on {
			desc = "lights runway crossing"
		}
		d.applyLights(desc)
	}
}

// stoppedBy is why the aircraft stands still short of the end of its path,
// "" when it moves or has arrived: traffic ahead or given way to (with the
// object, "traffic ahead #123"), a crossing hold, its clearance limit, a
// de-icing pad, else where it stands against its stops — for the log, so a
// stuck aircraft says why (live, OKSTM stopped 39 m short of the holding
// point with nothing near).
func (d *groundDrive) stoppedBy(pose GroundPose) string {
	m := d.mover
	if m == nil || !pose.Stopped {
		return ""
	}
	s, end := pose.Distance, m.path.Length()
	if s >= end-1 {
		return ""
	}
	if m.hasTraffic && m.trafficAt <= m.hold && s >= m.trafficAt-2 {
		if m.giveWay {
			return fmt.Sprintf("giving way to #%d", d.givingWay)
		}
		if d.facing {
			return fmt.Sprintf("holding for oncoming #%d", d.blockedBy)
		}
		return fmt.Sprintf("traffic ahead #%d", d.blockedBy)
	}
	if s >= m.hold-2 {
		switch {
		case d.hasPad && math.Abs(m.hold-d.padStop) < 0.5:
			return "de-icing pad"
		case d.hasLimit && math.Abs(m.hold-d.limit) < 0.5:
			return "clearance limit"
		case d.holdAtCrossings && d.nextCross < len(d.crossZones):
			return "hold short of runway " + d.crossZones[d.nextCross].runway + " (crossing)"
		}
		return fmt.Sprintf("hold at %.0f m of %.0f m", m.hold, end)
	}
	return fmt.Sprintf("stopped at %.0f m of %.0f m (hold %.0f m, traffic stop %v at %.0f m)", s, end, m.hold, m.hasTraffic, m.trafficAt)
}

// holdNextCrossing sets the hold short of the next runway crossing ahead,
// skipping crossings already cleared (holdAtCrossings).
func (d *groundDrive) holdNextCrossing() {
	for d.holdAtCrossings && d.nextCross < len(d.crossZones) && d.crossClears > 0 {
		d.crossClears--
		d.nextCross++
	}
	d.updateHold()
}

// updateHold holds the mover at the nearer of the next uncleared crossing
// (holdAtCrossings) and the clearance limit, or nowhere.
func (d *groundDrive) updateHold() {
	h := math.Inf(1)
	if d.holdAtCrossings && d.nextCross < len(d.crossZones) {
		h = d.crossZones[d.nextCross].from - HoldShortStopMeters
	}
	if d.hasLimit {
		h = math.Min(h, d.limit)
	}
	if d.hasPad {
		h = math.Min(h, d.padStop)
	}
	if math.IsInf(h, 1) {
		d.mover.ClearHold()
	} else {
		d.mover.HoldAt(h)
	}
}

// setLimit clears the aircraft up to a route node on the current path (the
// nose gear stops on it, or HoldShortStopMeters short of a hold-short
// line). It fails for a node not ahead on the path.
func (d *groundDrive) setLimit(node airport.NodeID) error {
	if d.mover == nil || int(node) < 0 || int(node) >= len(d.graph.Nodes) {
		return ErrNotOnRoute
	}
	n := d.graph.Nodes[node]
	dist, off := d.mover.Path().DistanceTo(n.Position)
	if off > 15 || dist <= d.mover.Pose().Distance+1 {
		return ErrNotOnRoute
	}
	if n.HoldShort != nil {
		dist -= HoldShortStopMeters
	}
	d.limit, d.hasLimit, d.limitNode = dist, true, node
	d.updateHold()
	return nil
}

// clearLimit removes the clearance limit.
func (d *groundDrive) clearLimit() {
	d.hasLimit, d.limitNode = false, -1
	if d.mover != nil {
		d.updateHold()
	}
}

// atLimit reports whether the aircraft has stopped at its clearance limit.
func (d *groundDrive) atLimit(pose GroundPose) bool {
	return d.hasLimit && pose.Stopped && pose.Distance >= d.limit-0.5
}

// crossingCleared releases the hold short of the next crossing and sets the
// one after it.
func (d *groundDrive) crossingCleared() {
	d.nextCross++
	d.holdNextCrossing()
}

// atCrossingHold reports whether the aircraft has stopped at the hold-short
// line of the next crossing, and names its runway.
func (d *groundDrive) atCrossingHold(pose GroundPose) (string, bool) {
	if !d.holdAtCrossings || d.nextCross >= len(d.crossZones) || !pose.Stopped {
		return "", false
	}
	z := d.crossZones[d.nextCross]
	return z.runway, pose.Distance >= z.from-HoldShortStopMeters-1
}

// holdOnPath is a hold-short node on an injected path.
type holdOnPath struct {
	runway, index int
	dist          float64
}

// crossZone is a runway crossing between two hold-short lines on the path,
// in path distance.
type crossZone struct {
	from, to float64
	runway   string
}

// crossingZones pairs consecutive hold-shorts of the same runway with the
// route crossing that runway between them (route holds the route points the
// hold indexes refer to).
func crossingZones(g *airport.Graph, route []airport.LatLon, holds []holdOnPath) []crossZone {
	var zones []crossZone
	for k := 1; k < len(holds); k++ {
		a, b := holds[k-1], holds[k]
		if a.runway != b.runway {
			continue
		}
		mid := airport.LatLon{Lat: (route[a.index].Lat + route[b.index].Lat) / 2, Lon: (route[a.index].Lon + route[b.index].Lon) / 2}
		crosses := g.RunwayAt(mid, 0) == a.runway
		for j := a.index + 1; j < b.index && !crosses; j++ {
			crosses = g.RunwayAt(route[j], 0) == a.runway
		}
		if crosses {
			zones = append(zones, crossZone{a.dist, b.dist, g.Layout.Runways[a.runway].Name()})
		}
	}
	return zones
}

// surfaceRamp moves a control surface (percent) towards target at rate
// percent per second.
type surfaceRamp struct{ pct, target, rate float64 }

// step moves the surface and reports whether it moved.
func (r *surfaceRamp) step(dt float64) bool {
	if r.pct == r.target || r.rate <= 0 {
		return false
	}
	if r.pct < r.target {
		r.pct = math.Min(r.target, r.pct+r.rate*dt)
	} else {
		r.pct = math.Max(r.target, r.pct-r.rate*dt)
	}
	return true
}

// offsetHeading returns the point d meters from p along heading (true
// degrees), in local meters.
func offsetHeading(p airport.LatLon, heading, d float64) airport.LatLon {
	h := heading * math.Pi / 180
	kx := metersPerDegree * math.Cos(p.Lat*math.Pi/180)
	return airport.LatLon{Lat: p.Lat + math.Cos(h)*d/metersPerDegree, Lon: p.Lon + math.Sin(h)*d/kx}
}

// alongHeading is how far q lies from p along heading (negative behind).
func alongHeading(p airport.LatLon, heading float64, q airport.LatLon) float64 {
	h := heading * math.Pi / 180
	kx := metersPerDegree * math.Cos(p.Lat*math.Pi/180)
	return (q.Lon-p.Lon)*kx*math.Sin(h) + (q.Lat-p.Lat)*metersPerDegree*math.Cos(h)
}

// pathLen is p's length in meters, at its first point's scale (an
// airport's paths: the scale changes by less than a part in ten thousand).
func pathLen(p []airport.LatLon) float64 {
	if len(p) < 2 {
		return 0
	}
	kx := metersPerDegree * math.Cos(p[0].Lat*math.Pi/180)
	d := 0.0
	for i := 1; i < len(p); i++ {
		dx, dy := (p[i].Lon-p[i-1].Lon)*kx, (p[i].Lat-p[i-1].Lat)*metersPerDegree
		d += math.Sqrt(dx*dx + dy*dy)
	}
	return d
}

// leadInAhead reports whether a route reaches (or leaves) the stand through
// a lead-in junction ahead of the parked aircraft (it faces the taxilane
// there): a departure then taxis straight out without a pushback and an
// arrival has to turn around on the apron to park. Stands can have lead-ins
// on both sides, so it depends on the route, not the stand alone.
func leadInAhead(g *airport.Graph, parking int, junction airport.LatLon) bool {
	p := g.Layout.Parking[parking]
	return alongHeading(p.Position, p.Heading, junction) > 0
}

// followAhead stops the mover behind the traffic ahead on its path, at a
// safe gap from its body, while taxiing (#334).
func (d *groundDrive) followAhead(now time.Time) {
	if d.mover != nil && d.mover.reverse {
		return // a pushback keeps its own traffic stop (holdPushForTraffic)
	}
	if d.picture == nil || !d.followTraffic || d.mover == nil {
		d.givingWay = 0
		if d.mover != nil {
			d.mover.ClearTrafficStop()
		}
		if d.picture != nil && d.object != 0 {
			d.picture.ReportIntent(d.object, nil) // not taxiing: no intent (#775)
			if len(d.planned) > 0 {
				d.picture.ReportPlanned(d.object, d.planned, d.prof.SpanMeters/2)
			} else {
				d.picture.ReportPath(d.object, nil, 0)
			}
		}
		return
	}
	if now.Sub(d.trafficAt) < TrafficCheckEvery {
		return // the stop is a place on the path: it holds until the next look
	}
	d.trafficAt = now
	half := d.prof.SpanMeters / 2
	if half <= 0 {
		half = DefaultHalfSpanMeters
	}
	s0 := d.mover.Pose().Distance
	path := d.mover.Path()
	body, who := d.picture.blocking(d.object, path, s0, TrafficLookMeters, half, now)
	d.blockedBy, d.facing = 0, false
	if !math.IsInf(body, 1) {
		d.blockedBy = who.id
	}
	stop := math.Inf(1)
	noseTip := (pushNoseFactor - 1) * d.prof.WheelbaseMeters // ahead of the nose gear
	if !math.IsInf(body, 1) {
		stop = body - noseTip - TrafficGapMeters
		// Facing an aircraft coming the other way, keep the last junction
		// before it clear: it turns off there (#444).
		if math.Abs(headingDiff(who.hdg, localBearing(path.PointAt(math.Max(s0, body-5)), path.PointAt(body)))) >= oncomingDeg {
			oh := who.half
			if oh <= 0 {
				oh = DefaultHalfSpanMeters
			}
			stop = math.Min(stop, d.junctionStop(path, s0, math.Min(stop, body), noseTip, math.Max(half, oh)+GiveWayMarginMeters))
		}
	}
	// One coming the other way along the same taxiway further on (#775):
	// the one further from the shared stretch holds at its last junction
	// before it, clear for the other to pass; the nearer goes on.
	if at, o, hold := d.picture.oncoming(d.object, path, s0, OncomingLookMeters, half, now); hold && !math.IsInf(at, 1) {
		hs := d.junctionStop(path, s0, at-noseTip-TrafficGapMeters, noseTip, half+DefaultHalfSpanMeters+GiveWayMarginMeters)
		if hs < stop {
			stop = hs
			d.blockedBy, d.facing = o, true
		}
	}
	// Give way where routes cross or merge: stop short of the conflict
	// (its first point is already a half-span away from the other path).
	gw, whom := d.picture.giveWayTo(d.object, path, s0, GiveWayLookMeters, half, now)
	giving := false // the give-way point is the stop, not traffic ahead
	if !math.IsInf(gw, 1) {
		at := gw - (pushNoseFactor-1)*d.prof.WheelbaseMeters - TrafficGapMeters
		// In the junction already (past where it would have stopped): it
		// clears the junction rather than stopping in it — live, QTR1788 was
		// pulled up short inside a crossing. Short of the stop it still gives
		// way, and the traffic actually ahead on its path (body) still stops it.
		if at < s0-1 && !d.picture.pushingNow(whom) { // a pushback is given way to always
			whom, gw = 0, math.Inf(1)
		} else if at < stop {
			stop, giving = at, true
		}
	}
	d.givingWay = whom
	switch {
	case math.IsInf(stop, 1):
		d.mover.ClearTrafficStop()
	case giving:
		d.mover.SetGiveWayStop(stop)
	default:
		d.mover.SetTrafficStop(stop)
	}
	// Where this aircraft will drive next, for the others to give way: the
	// points ahead of it, none when it is not going anywhere (holding at a
	// limit or a hold-short takes no priority over moving traffic).
	to := math.Min(math.Min(d.mover.stop(), path.Length()), s0+GiveWayLookMeters)
	var ahead []airport.LatLon
	for s := s0 + trafficBodyStep; s <= to; s += trafficBodyStep {
		ahead = append(ahead, path.PointAt(s))
	}
	d.picture.ReportPath(d.object, ahead, half)
	// Where it means to go, past any stop: oncoming traffic holds clear.
	var intent []airport.LatLon
	for s := s0; s <= math.Min(path.Length(), s0+OncomingLookMeters); s += trafficBodyStep {
		intent = append(intent, path.PointAt(s))
	}
	d.picture.ReportIntent(d.object, intent)
}

// oncomingDeg is how far from the path's heading an aircraft ahead faces to
// count as coming the other way.
const oncomingDeg = 120.0

// junctionStop is where to stop, at most at stop, facing an oncoming
// aircraft further along path (#444): with the body, nose tip noseTip
// ahead of the reference, at least clear from every other branch of the
// last junction on the path before stop, so the oncoming aircraft can turn
// off there (LKPR, live: CSA273 stopped at the gap behind WZZ1529 with its
// nose over the Z junction WZZ1529 was to turn through; neither moved
// again). stop when there is no junction; s0, where it is, when no place
// behind is clear.
func (d *groundDrive) junctionStop(path *GroundPath, s0, stop, noseTip, clear float64) float64 {
	g := d.graph
	if g == nil || stop <= s0 {
		return stop
	}
	// The last junction on the path between here and the stop (well, within
	// the nose ahead of it: the stop is where the reference halts).
	const step = 2.0
	junction, sJ := airport.NodeID(-1), 0.0
	from, to := path.PointAt(s0), path.PointAt(stop+noseTip)
	reach := localDist(from, to) + stop + noseTip - s0 // a bound on how far the path strays
	for id, n := range g.Nodes {
		if n.Kind == airport.NodeParking || len(g.Adj[id]) < 3 || localDist(from, n.Position) > reach {
			continue
		}
		for s := s0; s <= stop+noseTip; s += step {
			if localDist(path.PointAt(s), n.Position) <= 3 {
				if s > sJ || junction < 0 {
					junction, sJ = airport.NodeID(id), s
				}
				break
			}
		}
	}
	if junction < 0 {
		return stop
	}
	// Its other branches: those leaving it off the path (not back along it,
	// not on along it), walked a stretch as an aircraft turning there would.
	jp := g.Nodes[junction].Position
	back := localBearing(jp, path.PointAt(math.Max(0, sJ-10)))
	on := localBearing(jp, path.PointAt(math.Min(path.Length(), sJ+10)))
	var branch []airport.LatLon
	for _, e := range g.Adj[junction] {
		b := localBearing(jp, g.Nodes[e.To].Position)
		if !pushEdge(g, e) || math.Abs(headingDiff(b, back)) < 20 || math.Abs(headingDiff(b, on)) < 20 {
			continue
		}
		prev := jp
		for _, q := range walkTaxiway(g, junction, e.To, junctionBranchMeters) {
			for f := step; f < localDist(prev, q); f += step {
				branch = append(branch, offsetHeading(prev, localBearing(prev, q), f))
			}
			branch = append(branch, q)
			prev = q
		}
	}
	if len(branch) == 0 {
		return stop
	}
	for s := math.Min(stop, sJ); s > s0; s -= step {
		free := true
		for x := s; free; x += step {
			x = math.Min(x, s+noseTip) // up to the nose tip itself
			p := path.PointAt(x)
			for _, q := range branch {
				if localDist(p, q) < clear {
					free = false
					break
				}
			}
			if x >= s+noseTip {
				break
			}
		}
		if free {
			return s
		}
	}
	return s0
}

// reportGround puts this aircraft in the ground picture.
func (d *groundDrive) reportGround(id uint32, pos airport.LatLon, hdg float64, now time.Time) {
	if d.picture != nil && id != 0 {
		d.picture.Report(id, pos, hdg, d.prof, now)
	}
}

// applyPendingLimit sets a clearance limit given before the taxi path
// existed (during the pushback, the approach or the rollout). A limit no
// longer ahead (passed during the push or on the runway exit) holds the
// aircraft where it is, never beyond its clearance, and returns
// ErrNotOnRoute for the caller to report: it waits for a new clearance.
func (d *groundDrive) applyPendingLimit(node airport.NodeID) error {
	err := d.setLimit(node)
	if err == nil || d.mover == nil {
		return err
	}
	d.limit, d.hasLimit, d.limitNode = d.mover.Pose().Distance, true, node
	d.updateHold()
	return fmt.Errorf("%w: node %d was passed before the taxi started; holding for a new clearance", ErrNotOnRoute, node)
}
