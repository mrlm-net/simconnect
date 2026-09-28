//go:build windows
// +build windows

package traffic

import (
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
}

// advance steps the mover to now and places the aircraft.
func (d *groundDrive) advance() (GroundPose, error) {
	now := d.clock()
	dt := math.Max(0, math.Min(now.Sub(d.lastStep).Seconds(), 0.25))
	d.lastStep, d.frameDt = now, dt
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
	if !d.holdAtCrossings {
		return
	}
	for ; d.nextCross < len(d.crossZones); d.nextCross++ {
		if d.crossClears > 0 {
			d.crossClears--
			continue
		}
		d.mover.HoldAt(d.crossZones[d.nextCross].from - HoldShortStopMeters)
		return
	}
}

// crossingCleared releases the hold short of the next crossing and sets the
// one after it.
func (d *groundDrive) crossingCleared() {
	d.nextCross++
	d.mover.ClearHold()
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
