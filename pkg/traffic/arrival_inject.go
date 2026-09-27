//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Hybrid arrival (#309): MSFS AI flies the approach, landing and rollout;
// once the aircraft is clear of the runway an Injector takes it over and a
// GroundMover drives it to the vacate stop, holds it there and taxis it to
// the stand. Only the injector keeps the lights as set.

// ArrivalWithInjector drives the ground phase after the runway by position
// injection with inj (shared by all controllers; feed it every message as
// well). Without it the whole arrival is flown and taxied by MSFS AI.
func ArrivalWithInjector(inj *Injector) ArrivalOption {
	return func(c *ArrivalController) { c.inj = inj }
}

// Stand axis: the injected path ends with standAxisMeters straight along
// the stand heading, so the aircraft stops aligned with the stand.
const standAxisMeters = 25.0

// currentLights is the light state the sim reports.
func (m arrivalMonitor) currentLights() Lights {
	return Lights{
		Landing: m.Lights[0] != 0, Taxi: m.Lights[1] != 0, Strobe: m.Lights[2] != 0,
		Beacon: m.Lights[3] != 0, Nav: m.Lights[4] != 0, Logo: m.Logo != 0, Wing: m.Wing != 0,
	}
}

// watchGround starts the injector's ground height requests at touchdown and
// reads the aircraft every sim frame, so the takeover starts from a fresh
// position with the ground height known.
func (c *ArrivalController) watchGround() {
	if c.inj == nil {
		return
	}
	c.note("injector watch", c.inj.Watch(c.objectID))
	c.monitorEvery(types.SIMCONNECT_PERIOD_SIM_FRAME)
	c.fast = true
}

func (c *ArrivalController) monitorEvery(p types.SIMCONNECT_PERIOD) {
	if client := c.fleet.clientOrNil(); client != nil {
		c.note("monitor period", client.RequestDataOnSimObject(c.reqBase+arrReqMonitor, c.defBase+arrDefMonitor, c.objectID,
			p, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0))
	}
}

// takeover hands the aircraft, still rolling off the runway, from MSFS AI to
// the injector. Nothing visible changes at the switch: the mover starts at
// the aircraft's nose gear with its heading and speed, and the lights are set
// to what the sim shows.
func (c *ArrivalController) takeover(m arrivalMonitor, pos airport.LatLon, onRunway bool) error {
	prof := c.profile()
	nose := NoseGear(pos, m.Heading, prof)
	route := c.plan.Route.Points

	// The path: from the nose gear on along the route, ending straight along
	// the stand axis with the reference point on the stop mark. On the runway
	// the route starts at the exit's runway node ahead of the nose; clear of
	// it, at the first route point ahead of the nose.
	from := 0
	if !onRunway {
		from = c.track.segmentAt(c.track.pos+prof.WheelbaseMeters-prof.RefAheadMeters) + 1
	}
	stopNose := NoseGear(c.plan.Stop, c.standHeading, prof)
	axis := offsetHeading(stopNose, c.standHeading, -standAxisMeters)
	pts := []airport.LatLon{nose}
	hold, clear, onRwy := 0.0, 0.0, localDist(nose, route[0])
	var holds []holdOnPath
	for i := from; i < len(route)-1; i++ {
		// Drop route points on or past the stand axis start.
		if alongHeading(stopNose, c.standHeading, route[i]) > -standAxisMeters {
			break
		}
		d := pathLen(pts) + localDist(pts[len(pts)-1], route[i])
		if i == c.plan.VacateIndex {
			hold = d
		}
		if i == len(c.plan.Exit.Path)-1 {
			clear = d // the exit's first node off the runway surface
		}
		if hs := c.req.Graph.Nodes[c.plan.Route.Nodes[i]].HoldShort; hs != nil && hs.Runway != c.plan.Runway.Index {
			holds = append(holds, holdOnPath{runway: hs.Runway, index: i, dist: d})
		}
		pts = append(pts, route[i])
	}
	pts = append(pts, axis, stopNose)
	c.crossZones = c.crossingZones(holds)
	fast := prof
	fast.CruiseKts = math.Max(prof.CruiseKts, m.GroundKts+1) // no braking before the planned points
	path, err := NewGroundPath(pts, fast)
	if err != nil {
		return err
	}
	moverProf := prof
	if onRunway {
		// Taxi speed from clear of the runway, exit speed from the exit's
		// runway node, braked to at rollout deceleration.
		exitKts := InjectExitKts
		if c.plan.Exit.HighSpeed {
			exitKts = InjectExitHighSpeedKts
		}
		// Both at rollout deceleration: a gentle taxi braking curve would reach
		// back over the whole runway (live: 1 kt/s from 70 kt, over a km).
		path.LimitRange(clear, path.Length(), prof.CruiseKts, RolloutDecel)
		path.LimitRange(onRwy, clear, exitKts, RolloutDecel)
		moverProf.Decel, moverProf.Jerk = RolloutDecel, RolloutJerk
		c.clearDist = clear
	} else {
		path.LimitRange(0, path.Length(), prof.CruiseKts, prof.Decel)
	}
	// Enter the stand slowly: StandTaxiSpeedKts over the last StandSlowMeters.
	path.LimitEnd(StandSlowMeters, StandTaxiSpeedKts)
	c.mover = NewGroundMoverFrom(path, moverProf, m.Heading, m.GroundKts)
	// Hold at the vacate stop, or as soon as comfortably possible when the
	// aircraft is already past it.
	v := m.GroundKts * ktsToMS
	c.mover.HoldAt(math.Max(hold, v*v/(2*moverProf.Decel)+2))

	if err := c.inj.Takeover(c.objectID); err != nil {
		c.mover = nil
		return err
	}
	c.note("injector takeover", nil)
	c.lights = m.currentLights()
	if onRunway {
		// MSFS AI switches the lights off during its rollout: landing
		// lights and strobes on while on the runway.
		c.setInjectedLights(lightsRollout, "lights rollout (injected)")
	} else {
		// Already clear of the runway: landing lights and strobes off.
		c.note("injector lights at takeover", c.inj.SetLights(c.objectID, c.lights))
		c.setInjectedLights(lightsVacated, "lights vacated")
	}
	c.lastStep = c.now()
	c.step()
	return nil
}

func (c *ArrivalController) profile() MotionProfile {
	if c.req.Profile != (MotionProfile{}) {
		return c.req.Profile
	}
	return DefaultMotionProfile()
}

// setInjectedLights changes the lights of the injected aircraft; logo and
// wing lights stay as the aircraft had them.
func (c *ArrivalController) setInjectedLights(l Lights, desc string) {
	l.Logo, l.Wing = c.lights.Logo, c.lights.Wing
	c.lights = l
	c.applyLights(desc)
}

// applyLights sends the phase lights, with strobes and landing lights on
// while crossing a runway: the aircraft must be conspicuous on it.
func (c *ArrivalController) applyLights(desc string) {
	l := c.lights
	if c.crossing {
		l.Strobe, l.Landing = true, true
	}
	c.note(desc, c.inj.SetLights(c.objectID, l))
}

// checkCrossing switches the crossing lights when the nose gear or the
// reference point comes within RunwayClearMeters of a runway (other than
// the one just vacated, while vacating) and back once both are clear.
func (c *ArrivalController) checkCrossing(pose GroundPose) {
	g := c.req.Graph
	nose := NoseGear(pose.Position, pose.Heading, c.profile())
	// Between the hold-short lines of a crossing: from the nose reaching the
	// first until the tail is past the opposite one.
	on := false
	prof := c.profile()
	for _, z := range c.crossZones {
		if pose.Distance >= z.from && pose.Distance-prof.WheelbaseMeters-CrossingTailMeters <= z.to {
			on = true
		}
	}
	for _, p := range []airport.LatLon{nose, pose.Position} {
		if r := g.RunwayAt(p, RunwayClearMeters); r >= 0 && !((c.state == ArrivalRollout || c.state == ArrivalVacating) && r == c.plan.Runway.Index) {
			on = true
		}
	}
	if on != c.crossing {
		c.crossing = on
		desc := "lights runway crossing off"
		if on {
			desc = "lights runway crossing"
		}
		c.applyLights(desc)
	}
}

// step advances the mover to now and places the aircraft.
func (c *ArrivalController) step() GroundPose {
	now := c.now()
	dt := math.Min(now.Sub(c.lastStep).Seconds(), 0.25)
	c.lastStep = now
	pose := c.mover.Step(math.Max(dt, 0))
	if err := c.inj.Place(c.objectID, pose); err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	return pose
}

// onInjectedFrame runs the ground phase once the injector has the aircraft:
// every sim frame the mover steps and the aircraft is placed.
func (c *ArrivalController) onInjectedFrame() {
	pose := c.step()
	path := c.mover.Path()
	c.last.Position, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pose.Position, pose.Heading, pose.GroundSpeedKts, true
	c.last.Remaining = math.Max(0, path.Length()-pose.Distance)
	if seg, _ := c.track.advance(pose.Position); seg >= 0 {
		c.last.Taxiway = c.track.taxiwayAt(seg)
	}
	c.checkCrossing(pose)
	switch c.state {
	case ArrivalRollout:
		// Clear of the runway: taxi behaviour, landing lights and strobes off.
		if pose.Distance >= c.clearDist {
			c.mover.SetProfile(c.profile())
			c.setInjectedLights(lightsVacated, "lights vacated")
			c.setState(ArrivalVacating, nil)
			return
		}
	case ArrivalVacating:
		// Stopped clear of the runway: landing lights off, and a moment
		// later the taxi light on, then wait for the taxi clearance.
		if pose.Stopped {
			c.setInjectedLights(lightsStopped, "lights landing off")
			c.taxiLightAt = c.now().Add(TaxiLightDelay)
			dwell := c.req.AfterLandingDwell
			if dwell <= 0 {
				dwell = DefaultAfterLandingDwell
			}
			c.clearAt = c.now().Add(dwell)
			c.setState(ArrivalAwaitingTaxi, nil)
			return
		}
	case ArrivalAwaitingTaxi:
		if !c.lights.Taxi && !c.now().Before(c.taxiLightAt) {
			c.setInjectedLights(LightsTaxi, "lights taxi")
		}
		if c.cleared || (!c.req.HoldForClearance && !c.now().Before(c.clearAt)) {
			c.startTaxi()
			return
		}
	case ArrivalTaxiing:
		if c.last.Remaining <= standAxisMeters+StandSlowMeters {
			c.setState(ArrivalParking, nil)
			return
		}
	case ArrivalParking:
		if pose.Arrived {
			// The aircraft stays frozen on the stand under the injector;
			// Release it to hand it back to MSFS AI.
			c.setInjectedLights(LightsParked, "lights parked")
			c.stopMonitor()
			c.setState(ArrivalParked, nil)
			return
		}
	}
	c.emit(nil, false)
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

// Lights after landing: landing lights and strobes while on the runway;
// strobes off once clear of it, landing lights off at the vacate stop and
// the taxi light on TaxiLightDelay later (LightsTaxi).
var (
	lightsRollout = Lights{Nav: true, Beacon: true, Strobe: true, Landing: true}
	lightsVacated = Lights{Nav: true, Beacon: true, Landing: true}
	lightsStopped = Lights{Nav: true, Beacon: true}
)

// holdOnPath is a hold-short node on the injected path.
type holdOnPath struct {
	runway, index int
	dist          float64
}

// crossZone is a runway crossing between two hold-short lines on the path,
// in path distance.
type crossZone struct{ from, to float64 }

// crossingZones pairs consecutive hold-shorts of the same runway with the
// route crossing that runway between them: the crossing lights come on at
// the first and go off once the aircraft is past the second (#309).
func (c *ArrivalController) crossingZones(holds []holdOnPath) []crossZone {
	g, route := c.req.Graph, c.plan.Route.Points
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
			zones = append(zones, crossZone{a.dist, b.dist})
		}
	}
	return zones
}
