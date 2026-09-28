//go:build windows
// +build windows

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

	lights   Lights // phase lights; logo and wing stay as the aircraft had them
	crossing bool   // strobes and landing lights added for a runway crossing
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
	trafficAt     time.Time // last look ahead (every TrafficCheckEvery)

	// Progressive taxi (#322): the clearance limit on the current path.
	limit     float64
	hasLimit  bool
	limitNode airport.NodeID
}

// advance steps the mover to now and places the aircraft.
func (d *groundDrive) advance() (GroundPose, error) {
	now := d.clock()
	dt := math.Max(0, math.Min(now.Sub(d.lastStep).Seconds(), 0.25))
	d.lastStep, d.frameDt = now, dt
	d.followAhead(now)
	pose := d.mover.Step(dt)
	return pose, d.injector.Place(d.object, pose)
}

// setInjectedLights changes the phase lights; logo and wing lights stay as
// the aircraft had them.
func (d *groundDrive) setInjectedLights(l Lights, desc string) {
	l.Logo, l.Wing = d.lights.Logo, d.lights.Wing
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

func pathLen(p []airport.LatLon) float64 {
	d := 0.0
	for i := 1; i < len(p); i++ {
		d += localDist(p[i-1], p[i])
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
		if d.mover != nil {
			d.mover.ClearTrafficStop()
		}
		if d.picture != nil && d.object != 0 {
			d.picture.ReportPath(d.object, nil, 0)
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
	body := d.picture.blocking(d.object, path, s0, TrafficLookMeters, half, now)
	stop := math.Inf(1)
	if !math.IsInf(body, 1) {
		// The nose tip is (pushNoseFactor-1) wheelbases ahead of the nose gear.
		stop = body - (pushNoseFactor-1)*d.prof.WheelbaseMeters - TrafficGapMeters
	}
	// Give way where routes cross or merge: stop short of the conflict
	// (its first point is already a half-span away from the other path).
	if gw := d.picture.giveWay(d.object, path, s0, GiveWayLookMeters, half, now); !math.IsInf(gw, 1) {
		stop = math.Min(stop, gw-(pushNoseFactor-1)*d.prof.WheelbaseMeters-TrafficGapMeters)
	}
	if math.IsInf(stop, 1) {
		d.mover.ClearTrafficStop()
	} else {
		d.mover.SetTrafficStop(stop)
	}
	// Where this aircraft will drive next, for the others to give way.
	to := math.Min(math.Min(d.mover.stop(), path.Length()), s0+GiveWayLookMeters)
	var ahead []airport.LatLon
	for s := s0; s <= to; s += trafficBodyStep {
		ahead = append(ahead, path.PointAt(s))
	}
	d.picture.ReportPath(d.object, ahead, half)
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
