package traffic

import (
	"errors"
	"math"
	"slices"
	"time"

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

// ArrivalWithClock runs the arrival on clock (e.g. SimClock.Now: the
// simulation rate, stopped while paused) instead of the wall clock (#413).
func ArrivalWithClock(clock func() time.Time) ArrivalOption {
	return func(c *ArrivalController) { c.now = clock }
}

// ArrivalWithDetail drives the injected taxi-in on fewer sim frames when it
// is far from the viewer or standing still (#370); on the runway always on
// every frame.
func ArrivalWithDetail(d *Detail) ArrivalOption {
	return func(c *ArrivalController) { c.detail = d }
}

// ArrivalWithGroundPicture shares the ground picture with the other aircraft
// at the airport: the injected arrival reports itself on the ground and,
// off the runway, stops behind the traffic ahead (#334).
func ArrivalWithGroundPicture(p *GroundPicture) ArrivalOption {
	return func(c *ArrivalController) { c.picture = p }
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
	// Keep the route up to its last point before the stand axis start: the
	// final stretch onto the stand is replaced by the axis. (Earlier points
	// may lie in front of the stand; the route can pass it before looping
	// round to its lead-in.)
	faceOut := len(route) > 1 && leadInAhead(c.req.Graph, c.req.Parking, route[len(route)-2])
	last := from - 1
	for i := from; i < len(route)-1; i++ {
		if faceOut || alongHeading(stopNose, c.standHeading, route[i]) <= -standAxisMeters {
			last = i // face-out stands keep the route to the lead-in junction
		}
	}
	apron := apronSpans{g: c.req.Graph}
	for i := from; i <= last; i++ {
		d := pathLen(pts) + localDist(pts[len(pts)-1], route[i])
		apron.add(c.plan.Route.Nodes[i], d)
		if i == c.plan.VacateIndex {
			hold = math.Max(0, d-c.plan.VacateBackMeters)
		}
		if i == len(c.plan.Exit.Path)-1 {
			clear = d // the exit's first node off the runway surface
		}
		if hs := c.req.Graph.Nodes[c.plan.Route.Nodes[i]].HoldShort; hs != nil && hs.Runway != c.plan.Runway.Index {
			holds = append(holds, holdOnPath{runway: hs.Runway, index: i, dist: d})
		}
		pts = append(pts, route[i])
	}
	if faceOut {
		pts = append(pts, c.turnAround(stopNose)...) // self-manoeuvring stand
	} else {
		pts = append(pts, axis, stopNose)
	}
	c.crossZones = crossingZones(c.req.Graph, route, holds)
	fast := prof
	fast.CruiseKts = math.Max(prof.CruiseKts, m.GroundKts+1) // no braking before the planned points
	firm := 0.0
	if onRunway {
		firm = clear // on the runway, plan braking firmly (the turns after the exit must not reach back over the rollout)
	}
	ro := c.rolloutProfile()
	path, err := newGroundPath(pts, fast, firmZone{meters: firm, decel: ro.BrakeDecel, lateral: ro.ExitLateralAccel})
	if err != nil {
		return err
	}
	moverProf := prof
	if onRunway {
		// Rollout (live feedback): hard braking to SlowKts, then slowing
		// gently and evenly all the way to the exit speed at the exit.
		exitKts := ro.ExitKts
		if c.plan.Exit.HighSpeed {
			exitKts = ro.HighSpeedExitKts
		}
		if c.rush {
			exitKts += RushExitKts // expedite vacating
		}
		v0, vs, ve := m.GroundKts*ktsToMS, ro.SlowKts*ktsToMS, exitKts*ktsToMS
		slowAt := math.Max(0, (v0*v0-vs*vs)/(2*ro.BrakeDecel))
		gentle := math.Max(0.2, (vs*vs-ve*ve)/(2*math.Max(onRwy-slowAt, 1)))
		path.LimitRange(onRwy, clear, exitKts, gentle)
		path.LimitRange(slowAt, onRwy, ro.SlowKts, ro.BrakeDecel)
		// Off the exit to taxi speed, planned firmly so it does not reach
		// back over the rollout.
		path.LimitRange(clear, path.Length(), prof.CruiseKts, ro.BrakeDecel)
		moverProf.Decel, moverProf.Jerk = ro.BrakeDecel, RolloutJerk
		c.clearDist = clear
	} else {
		path.LimitRange(0, path.Length(), prof.CruiseKts, prof.Decel)
	}
	apron.limit(path, c.req.Airport, prof.Decel)
	// Enter the stand slowly: StandTaxiSpeedKts over the last StandSlowMeters.
	path.LimitEnd(StandSlowMeters, StandTaxiSpeedKts)
	c.mover = NewGroundMoverFrom(path, moverProf, m.Heading, m.GroundKts)
	// Hold at the vacate stop, or as soon as comfortably possible when the
	// aircraft is already past it.
	v := m.GroundKts * ktsToMS
	hold = math.Max(hold, v*v/(2*moverProf.Decel)+2)
	c.vacateDist = hold
	if c.rollThrough {
		// A rolling clearance: slow to RollThroughKts at the vacate point and
		// taxi on without stopping.
		c.mover.SlowAt(hold, RollThroughKts)
	} else {
		c.mover.HoldAt(hold)
	}

	if err := c.inj.Takeover(c.objectID); err != nil {
		c.mover = nil
		return err
	}
	c.inj.SetModel(c.objectID, c.req.Model)
	c.note("injector takeover", nil)
	c.initDrive()
	c.ignoreRunway = c.plan.Runway.Index // runway lights come from the phase until vacated
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
	p := withDefaults(c.req.Profile, DefaultMotionProfile())
	limit := 0.0
	if c.req.Airport != nil {
		limit = c.req.Airport.TaxiMaxKts
	}
	return taxiSpeed(p, c.timing.taxiSpeed, limit)
}

// step advances the mover to now and places the aircraft.
func (c *ArrivalController) step() GroundPose {
	pose, err := c.advance()
	if err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	if c.givingWay != c.last.GivingWayTo {
		c.last.GivingWayTo = c.givingWay
		c.emit(nil, true)
	}
	if by := map[bool]string{true: c.stoppedBy(pose)}[c.state == ArrivalTaxiing]; by != c.last.StoppedBy {
		c.last.StoppedBy = by
		c.emit(nil, true)
	}
	return pose
}

// initDrive points the shared ground driving at this arrival's aircraft.
func (c *ArrivalController) initDrive() {
	c.injector, c.object, c.graph, c.prof = c.inj, c.objectID, c.req.Graph, c.profile()
	c.noLogo = WakeFor(c.req.Model).ICAO == WakeLight
	c.holdAtCrossings = c.req.HoldAtCrossings
}

// frameDetail sets how often the taxi-in is driven (#370): on the runway
// every frame, moving by its distance from the viewer, otherwise standing
// still.
func (c *ArrivalController) frameDetail(pose GroundPose) {
	if c.detail == nil {
		return
	}
	full := c.state <= ArrivalVacating || c.followMeDriving()
	if n, changed := c.detailS.want(c.detail, c.now(), pose.Position, pose.GroundSpeedKts > 0.5, full); changed {
		if client := c.fleet.clientOrNil(); client != nil {
			c.note("monitor detail", requestFrames(client, c.reqBase+arrReqMonitor, c.defBase+arrDefMonitor, c.objectID, n))
		}
	}
	c.detail.report(c.objectID, c.detailS.interval)
}

// onInjectedFrame runs the ground phase once the injector has the aircraft:
// every sim frame the mover steps and the aircraft is placed.
func (c *ArrivalController) onInjectedFrame() {
	// Off the runway the arrival follows the traffic ahead (#334).
	c.followTraffic = c.state == ArrivalVacating || c.state == ArrivalTaxiing || c.state == ArrivalParking
	pose := c.step()
	c.updateFollowMe(pose)
	c.frameDetail(pose)
	path := c.mover.Path()
	c.last.Position, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pose.Position, pose.Heading, pose.GroundSpeedKts, true
	c.last.Remaining = math.Max(0, path.Length()-pose.Distance)
	if seg, _ := c.track.advance(pose.Position); seg >= 0 {
		c.last.Taxiway = c.track.taxiwayAt(seg)
	}
	c.checkCrossing(pose)
	c.last.LimitNode = -1
	if c.hasLimit {
		c.last.LimitNode = c.limitNode
	}
	if at := c.atLimit(pose); at != c.last.AtLimit {
		c.last.AtLimit = at
		c.emit(nil, true) // holding at the clearance limit, or moving on
	}
	if !c.flapsUpFrom.IsZero() && c.flapsPct > 0 {
		c.flapsPct = math.Max(0, c.aircraft().Flaps.LandingPct*(1-c.now().Sub(c.flapsUpFrom).Seconds()/(FlapsRetractSeconds*f(c.timing.flaps))))
		c.note("flaps", c.inj.SetFlaps(c.objectID, c.flapsPct))
	}
	c.stepSurfaces(c.frameDt)
	c.taxiLightDue() // TaxiLightDelay after the landing lights went off
	switch c.state {
	case ArrivalRollout:
		// Clear of the runway: taxi behaviour, landing lights and strobes off.
		if pose.Distance >= c.clearDist {
			c.mover.SetProfile(c.profile())
			// Clear of the runway: strobes and landing lights off, flaps and
			// spoilers up, as the after-landing flow does.
			c.setInjectedLights(lightsStopped, "lights vacated (strobes and landing lights off)")
			c.taxiLightAt = c.now().Add(time.Duration(float64(TaxiLightDelay) * f(c.timing.taxiLight)))
			c.seq.add(c.now(), "vacated: strobes and landing lights off, flaps and spoilers retracting", 0, pose.GroundSpeedKts)
			if c.req.InjectApproach {
				c.flapsUpFrom = c.now() // after-landing flaps up once clear
				c.spoilers.target = 0
			}
			c.setState(ArrivalVacating, nil)
			return
		}
	case ArrivalVacating:
		// Stopped clear of the runway (or, rolling through, at the slowest
		// point): wait for the taxi clearance, or roll on.
		if pose.Stopped || (c.rollThrough && pose.Distance >= c.vacateDist-0.5) {
			// Taken over already clear of the runway (hybrid): the landing
			// lights go off here, and a moment later the taxi light comes on.
			if c.lights.Landing {
				c.setInjectedLights(lightsStopped, "lights landing off")
				c.taxiLightAt = c.now().Add(time.Duration(float64(TaxiLightDelay) * f(c.timing.taxiLight)))
			}
			c.clearAt = c.now().Add(c.dwell())
			c.ignoreRunway = -1 // clear of the landing runway now
			c.setState(ArrivalAwaitingTaxi, nil)
			if c.rollThrough {
				c.startTaxi()
			}
			return
		}
	case ArrivalAwaitingTaxi:
		if c.cleared || (!c.req.HoldForClearance && !c.now().Before(c.clearAt)) {
			c.startTaxi()
			return
		}
	case ArrivalTaxiing:
		// Stopped at the hold-short line of a crossing not cleared yet.
		if rwy, ok := c.atCrossingHold(pose); ok {
			c.last.HoldingShortOf = rwy
			c.setState(ArrivalHoldingShort, nil)
			return
		}
		if c.last.Remaining <= standAxisMeters+StandSlowMeters {
			c.setState(ArrivalParking, nil)
			return
		}
	case ArrivalParking:
		if pose.Arrived {
			// The aircraft stays frozen on the stand under the injector;
			// Release it to hand it back to MSFS AI.
			c.setInjectedLights(LightsParked, "lights parked")
			// Engines off on the stand: they ran on at idle (live, QTR1709's
			// B77W at B14), and no jetway comes to a running aircraft.
			c.note("engines off", c.inj.SetEngines(c.objectID, c.aircraft().EngineCount(), false))
			if c.fm == nil {
				c.stopMonitor() // else its frames drive the follow-me car home (#890)
			}
			c.setState(ArrivalParked, nil)
			return
		}
	}
	c.emit(nil, false)
}

// Lights after landing: landing lights and strobes while on the runway;
// strobes off once clear of it, landing lights off at the vacate stop and
// the taxi light on TaxiLightDelay later (LightsTaxi).
var (
	lightsRollout = Lights{Nav: true, Beacon: true, Strobe: true, Landing: true}
	lightsVacated = Lights{Nav: true, Beacon: true, Landing: true}
	lightsStopped = Lights{Nav: true, Beacon: true}
)

// dwell is how long the aircraft waits clear of the runway: the requested
// or default after-landing dwell, varied by ±DwellJitter.
func (c *ArrivalController) dwell() time.Duration {
	d := c.req.AfterLandingDwell
	if d <= 0 {
		d = DefaultAfterLandingDwell
	}
	return time.Duration(float64(d) * (1 + DwellJitter*(2*c.rng.Float64()-1)))
}

// ClearToCross clears an injected arrival holding short of a runway
// crossing (HoldAtCrossings) to cross it. Given earlier, it clears the next
// crossing ahead, so the aircraft does not stop there.
func (c *ArrivalController) ClearToCross() {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case c.state == ArrivalHoldingShort:
		c.crossingCleared()
		c.last.HoldingShortOf = ""
		c.setState(ArrivalTaxiing, nil)
	case c.mover != nil && c.state == ArrivalTaxiing && c.nextCross < len(c.crossZones):
		c.crossingCleared() // the hold ahead is already set
	default:
		c.crossClears++
	}
}

// spawnCGFt is the A320's reference point height above the wheels, used
// (as AircraftProfile.CGHeightM) for the spawn altitude before the sim
// reports the aircraft's own.
const spawnCGFt = 12.0

func approachProfileOf(req ArrivalRequest) ApproachProfile {
	return withDefaults(req.Approach, DefaultApproachProfile())
}

func (c *ArrivalController) approachProfile() ApproachProfile { return approachProfileOf(c.req) }

// startInjectedApproach takes the aircraft over as soon as it exists
// (InjectApproach): frozen, gear down, flaps full, approach lights, read
// every frame; the ApproachMover flies it from here.
func (c *ArrivalController) startInjectedApproach(startMeters float64) error {
	if err := c.inj.Takeover(c.objectID); err != nil {
		return err
	}
	c.inj.SetModel(c.objectID, c.req.Model)
	c.note("injector takeover on final", nil)
	c.initDrive()
	c.seq.begin(c.now())
	c.approachPhase, c.landingFlaps, c.landingFlapsSet = ApproachFinal, false, false
	c.note("gear down", c.inj.SetGear(c.objectID, true))
	c.flapsPct = c.aircraft().Flaps.ApproachPct
	c.note("approach flaps", c.inj.SetFlaps(c.objectID, c.flapsPct))
	c.note("approach thrust", c.inj.SetThrottle(c.objectID, c.aircraft().EngineCount(), ApproachThrottlePct))

	// Approach lights on the first frame, once the sim has reported the
	// aircraft's own logo and wing lights (see onApproachFrame).
	c.approachLightsSet = false
	c.approach = NewApproachMover(c.plan.End.Threshold, c.plan.End.Heading, startMeters, c.approachProfile())
	c.approach.SetCrosswind(c.req.CrosswindKts)
	c.approach.SetAimShift(TouchdownSpreadMeters * (2*c.rng.Float64() - 1))
	c.approach.SetSideShift(TouchdownSideMeters * (2*c.rng.Float64() - 1))
	at := c.approach.Pose()
	c.seq.add(c.now(), "takeover on final: gear down, approach flaps", at.HeightFt, at.GroundSpeedKts)
	c.monitorEvery(types.SIMCONNECT_PERIOD_SIM_FRAME)
	c.fast = true
	c.lastStep = c.now()
	c.setState(ArrivalApproaching, nil)
	return nil
}

// onApproachFrame flies the injected approach one frame and hands over to
// the injected rollout once the nose wheel is down.
func (c *ArrivalController) onApproachFrame(m arrivalMonitor) {
	now := c.now()
	dt := math.Max(0, math.Min(now.Sub(c.lastStep).Seconds(), MaxFrameStepSeconds)) // no step back with the clock (#105)
	c.lastStep = now
	if !c.approachLightsSet {
		c.approachLightsSet = true
		c.lights = m.currentLights()
		// The aircraft spawns with its logo light off; MSFS AI switches it on
		// on approach, so the injected approach does too (consistent look).
		c.lights.Logo = !c.aircraft().Lights.NoLogo
		c.setInjectedLights(lightsRollout, "lights approach (injected)")
	}
	pose := c.blend.apply(c.approach.Step(math.Max(dt, 0)), math.Max(dt, 0))
	pose.RunwayFt = c.plan.Runway.Altitude / 0.3048 // a steady glide path over any terrain
	if err := c.inj.PlaceAir(c.objectID, pose); err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	c.last.Position, c.last.Heading, c.last.GroundSpeed = pose.Position, pose.Heading, pose.GroundSpeedKts
	c.last.AGL, c.last.OnGround = pose.HeightFt, pose.OnGround
	c.stepSurfaces(math.Max(dt, 0))
	// Landing flaps: from the approach setting to full over
	// FlapsFullSeconds when passing FlapsFullFt, the stabilised gate.
	if fl := c.aircraft().Flaps; pose.HeightFt < fl.FullFt && c.flapsPct < fl.LandingPct && !pose.OnGround {
		if !c.landingFlaps {
			c.landingFlaps = true
			c.seq.add(now, "landing flaps extending", pose.HeightFt, pose.GroundSpeedKts)
		}
		c.flapsPct = math.Min(fl.LandingPct, c.flapsPct+(fl.LandingPct-fl.ApproachPct)/(FlapsFullSeconds*f(c.timing.flaps))*math.Max(dt, 0))
		c.note("flaps", c.inj.SetFlaps(c.objectID, c.flapsPct))
		if c.flapsPct >= fl.LandingPct && !c.landingFlapsSet {
			c.landingFlapsSet = true
			c.seq.add(now, "landing flaps set", pose.HeightFt, pose.GroundSpeedKts)
		}
	}
	if pose.Phase != c.approachPhase {
		c.approachPhase = pose.Phase
		switch pose.Phase {
		case ApproachFlare:
			c.seq.add(now, "flare", pose.HeightFt, pose.GroundSpeedKts)
		case ApproachDerotate:
			c.seq.add(now, "touchdown, spoilers", pose.HeightFt, pose.GroundSpeedKts)
		case ApproachDone:
			c.seq.add(now, "nose wheel down", pose.HeightFt, pose.GroundSpeedKts)
		}
	}
	switch {
	case c.state == ArrivalApproaching && pose.HeightFt < LandingAGLFt:
		c.note("thrust idle", c.inj.SetThrottle(c.objectID, c.aircraft().EngineCount(), 0)) // the flare: idle
		c.setState(ArrivalLanding, nil)
		return
	case c.state == ArrivalLanding && pose.OnGround:
		c.last.Touchdown, c.last.TouchdownFpm = pose.Touchdown, pose.TouchdownFpm
		c.touchdownAt = now
		// Main wheels down: ground spoilers out. (Thrust reversers cannot be
		// animated on an AI aircraft: the nozzle SimVar is not settable and
		// the reverse thrust events are ignored, #318.)
		c.spoilers = surfaceRamp{target: 100, rate: 100 / SpoilerDeploySeconds}
		c.setState(ArrivalRollout, nil)
		return
	case pose.Phase == ApproachDone && c.tngLeft > 0 && c.req.Circuit != nil:
		c.startTouchAndGo(pose) // off again: the circuit once more (#569)
		return
	case pose.Phase == ApproachDone:
		// Nose wheel down: the injected rollout, exit and taxi-in take over,
		// continuing from exactly this pose.
		at := m
		at.Latitude, at.Longitude, at.Heading, at.GroundKts = pose.Position.Lat, pose.Position.Lon, pose.Heading, pose.GroundSpeedKts
		c.approach = nil
		c.takeoverTried = true
		if err := c.takeover(at, pose.Position, true); err != nil {
			c.fail(err)
		}
		return
	}
	c.emit(nil, false)
}

// stepSurfaces moves the ground spoilers of an injected arrival.
func (c *ArrivalController) stepSurfaces(dt float64) {
	if c.spoilers.step(dt) {
		c.note("spoilers", c.inj.SetSpoilers(c.objectID, c.spoilers.pct))
	}
}

// turnAround is the custom route onto a self-manoeuvring (face-out) stand:
// from the lead-in junction ahead of the stand the aircraft swings out to
// the side with fewer neighbouring stands, loops round behind the stop mark
// and comes back along the stand centreline, facing out, to stop with its
// nose gear at stopNose.
func (c *ArrivalController) turnAround(stopNose airport.LatLon) []airport.LatLon {
	h := c.standHeading
	side := c.roomySide()
	// Scaled for longer aircraft: TurnAroundMeters suits an A320's wheelbase.
	r := TurnAroundMeters * math.Max(1, c.profile().WheelbaseMeters/DefaultMotionProfile().WheelbaseMeters)
	at := func(u, v float64) airport.LatLon {
		return offsetHeading(offsetHeading(stopNose, h, u*r), h+90, v*r*side)
	}
	return []airport.LatLon{
		at(-0.3, 0.6), // swing out to the side
		at(-1.6, 1.0), // round behind the stop mark
		at(-3.0, 0.6),
		at(-3.3, -0.05), // joining the centreline almost parallel
		at(-2.8, 0),     // on the centreline, facing out: about three
		stopNose,        // wheelbases of straight for the main gear to line up
	}
}

// roomySide is +1 or -1: the side of the stand (right or left of its
// heading) with fewer other stands within 60 m, for the turn-around loop.
func (c *ArrivalController) roomySide() float64 {
	g := c.req.Graph
	stand := g.Layout.Parking[c.req.Parking]
	sum := 0.0
	for _, p := range g.Layout.Parking {
		if p.Index == stand.Index || localDist(p.Position, stand.Position) > 60 {
			continue
		}
		sum += math.Copysign(1, alongHeading(stand.Position, stand.Heading+90, p.Position))
	}
	if sum > 0 {
		return -1
	}
	return 1
}

// RolloutProfile is how an injected landing rolls out and leaves the runway
// (per aircraft type; A320 defaults): hard braking to SlowKts, then slowing
// gently and evenly to the exit speed at the exit.
type RolloutProfile struct {
	// BrakeDecel (m/s²) is the braking after touchdown down to SlowKts.
	BrakeDecel float64
	SlowKts    float64
	// HighSpeedExitKts and ExitKts are the speeds at a high-speed exit and
	// at any other exit.
	HighSpeedExitKts, ExitKts float64
	// ExitLateralAccel (m/s²) is the cornering allowed through the exit.
	ExitLateralAccel float64
}

// DefaultRolloutProfile is an A320 family rollout.
func DefaultRolloutProfile() RolloutProfile {
	return RolloutProfile{BrakeDecel: 2.5, SlowKts: 80, HighSpeedExitKts: 32, ExitKts: 12, ExitLateralAccel: 1.5}
}

func (c *ArrivalController) rolloutProfile() RolloutProfile {
	return withDefaults(c.req.Rollout, DefaultRolloutProfile())
}

// ClearUpTo clears an injected arrival to taxi up to a node of its route and
// hold there (progressive taxi, #322). Given before the taxi-in starts it is
// the taxi clearance with a limit; while taxiing it moves the limit.
// ClearToTaxi removes the limit.
func (c *ArrivalController) ClearUpTo(node airport.NodeID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil || c.plan == nil {
		return ErrNotInjected
	}
	if !slices.Contains(c.plan.Route.Nodes, node) {
		return ErrNotOnRoute
	}
	switch c.state {
	case ArrivalTaxiing, ArrivalHoldingShort:
		if c.mover == nil {
			return ErrNotOnRoute
		}
		return c.setLimit(node)
	case ArrivalParking, ArrivalParked:
		return ErrNotOnRoute
	}
	// Before the taxi-in: the vacate stop comes first.
	c.pendingLimit, c.hasPendingLimit, c.cleared = node, true, true
	if c.state == ArrivalAwaitingTaxi && c.mover != nil {
		c.startTaxi()
	}
	return nil
}
