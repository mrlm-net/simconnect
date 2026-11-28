package traffic

import (
	"cmp"
	"errors"
	"math"
	"slices"
	"strconv"
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

// standJoin is where an arrival at nose leaves route for the way onto the stand:
// the last point from from on at least standAxisMeters behind the stop, and the
// axis to it. A stop further back on the stand (a longer nose offset) can leave
// no point that far behind. Then the point furthest behind it with an axis as
// long as that, or, none behind it at all, the route's last point and no axis
// (0): the aircraft turns on the stand. Never nothing of the route, while the
// aircraft is ahead of the axis: that drove straight at the stand from the
// runway, over the grass (live, LKTB 3M: a B738 at 137 kt).
func standJoin(route []airport.LatLon, from int, nose, stopNose airport.LatLon, heading float64) (last int, axisM float64) {
	last = from - 1
	far, behind := -1, -1.0 // a metre or less is not behind it
	for i := from; i < len(route)-1; i++ {
		a := alongHeading(stopNose, heading, route[i])
		if a <= -standAxisMeters {
			last = i
		}
		if a < behind {
			far, behind = i, a
		}
	}
	switch {
	case last >= from || from >= len(route)-1,
		alongHeading(stopNose, heading, nose) <= -standAxisMeters && localDist(nose, stopNose) <= pullInMeters:
		return last, standAxisMeters
	case far >= 0:
		return far, -behind
	}
	return len(route) - 2, 0
}

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
		c.monitorStopped = false
		c.note("monitor period", c.requestMonitor(client, p, 0))
	}
}

// takeover hands the aircraft, still rolling off the runway, from MSFS AI to
// the injector. Nothing visible changes at the switch: the mover starts at
// the aircraft's nose gear with its heading and speed, and the lights are set
// to what the sim shows.
func (c *ArrivalController) takeover(m arrivalMonitor, pos airport.LatLon, onRunway bool) error {
	replan := c.mover != nil // ours already: a new exit on the roll (VacateAt)
	prof := c.profile()
	nose := NoseGear(pos, m.Heading, prof)
	if onRunway && !replan {
		c.exitInReach(m.GroundKts, nose) // too fast for its exit: a later one (#1586)
	}
	var turn []airport.LatLon
	if onRunway && !replan {
		turn = c.endTurn(m.GroundKts, nose) // none in reach: round at the end and back (#1618)
	}
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
	last, axisM := standJoin(route, from, nose, stopNose, c.standHeading)
	axis := offsetHeading(stopNose, c.standHeading, -axisM)
	pts := append([]airport.LatLon{nose}, turn...)
	hold, clear, onRwy := 0.0, 0.0, localDist(nose, route[0])
	turnFrom, turnTo := 0.0, 0.0 // the turn round at the end, path meters
	if len(turn) > 0 {
		onRwy, turnFrom, turnTo = localDist(nose, turn[0]), localDist(nose, turn[0]), pathLen(pts)
	}
	exitAt := 0.0 // the exit's first node off the runway; clear the same, but after a backtrack
	var holds []holdOnPath
	// Keep the route up to its last point before the stand axis start: the
	// final stretch onto the stand is replaced by the axis. (Earlier points
	// may lie in front of the stand; the route can pass it before looping
	// round to its lead-in.)
	faceOut := axisM == 0 || turnsOnStand(c.req.Graph, c.req.Parking, route, from)
	if faceOut {
		last = max(last, len(route)-2) // face-out stands keep the route to the lead-in junction
	}
	apron := apronSpans{g: c.req.Graph}
	for i := from; i <= last; i++ {
		d := pathLen(pts) + localDist(pts[len(pts)-1], route[i])
		apron.add(c.plan.Route.Nodes[i], d)
		if i == c.plan.VacateIndex {
			hold = math.Max(0, d-c.plan.VacateBackMeters)
		}
		if i == len(c.plan.Exit.Path)-1 {
			exitAt = d
		}
		if i == c.plan.ClearAt() {
			clear = d // the first node off the runway surface for good
		}
		if hs := c.req.Graph.Nodes[c.plan.Route.Nodes[i]].HoldShort; hs != nil && hs.Runway != c.plan.Runway.Index {
			holds = append(holds, holdOnPath{runway: hs.Runway, index: i, node: c.plan.Route.Nodes[i], dist: d})
		}
		pts = append(pts, route[i])
	}
	join := pathLen(pts) // where the way onto the stand leaves the taxilane
	if faceOut {
		pts = append(pts, c.turnAround(stopNose)...) // self-manoeuvring stand
	} else {
		pts = append(pts, axis, stopNose)
		wide := widenStandTurn(pts, len(pts)-3, c.moverProfile(), c.standHeading, prof)
		join += pathLen(wide) - pathLen(pts) // the junction as far from the stop as before
		pts = wide
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
	moverProf := c.moverProfile()
	if onRunway {
		// Rollout: one even deceleration from the touchdown to the exit
		// speed at the exit (the user: a smooth slowdown into the first
		// exit), at least rolloutMinDecel, at most braking hard; before, hard
		// braking to SlowKts and a long roll at it to an exit far down.
		exitKts := ro.ExitKts
		if c.plan.Exit.HighSpeed {
			exitKts = ro.HighSpeedExitKts
		}
		if c.rush {
			exitKts += RushExitKts // expedite vacating
		}
		if c.plan.EndTurnoff {
			// A turnoff leading back at the far end: at taxi speed (#1674).
			exitKts = math.Min(exitKts, ExitSpeed(c.plan.Exit))
		}
		v0, ve := m.GroundKts*ktsToMS, exitKts*ktsToMS
		// Slowed to the exit speed by the end of the end's declared landing
		// distance where its exit lies beyond it (#1660), on to it at that.
		slowBy := onRwy
		if c.plan.End.LDAM > 0 {
			slowBy = math.Min(onRwy, math.Max(c.landingLeft(nose), 1))
		}
		even := (v0*v0 - ve*ve) / (2 * math.Max(slowBy-rolloutExitLeadM, 1))
		// The exit speed kept until its tail is clear of the runway as the
		// tower counts it, not only its nose off the surface (offline LKPR:
		// an A320 down to 12 kt at the edge took 61 s from the threshold to
		// clear, the arrival behind cleared to land at 1.0 NM); never past
		// where it stops to wait for its taxi clearance.
		fast := clear + exitClearAheadM
		if hold > clear {
			fast = math.Min(fast, hold)
		}
		if len(turn) > 0 {
			// Rolling to the end (endTurn): one even deceleration to the
			// turn speed at the turn round, then back along the runway and
			// off it at the slower of the backtrack and turn-off speeds.
			vt := TurnSpeedKts * ktsToMS
			evenT := (v0*v0 - vt*vt) / (2 * math.Max(turnFrom, 1))
			path.LimitRange(turnFrom, turnTo, TurnSpeedKts, math.Min(math.Max(evenT, rolloutMinDecel), ro.BrakeDecel))
			path.LimitRange(turnTo, fast, math.Min(BacktrackKts, c.turnOffKts()), ro.BrakeDecel)
		} else if c.plan.Backtrack && clear > exitAt {
			// Turning round off the exit and back along the runway at
			// BacktrackKts (#1563), the exit speed only to the exit.
			path.LimitRange(math.Max(slowBy-rolloutExitLeadM, 0), exitAt, exitKts, math.Min(math.Max(even, rolloutMinDecel), ro.BrakeDecel))
			path.LimitRange(exitAt, fast, BacktrackKts, ro.BrakeDecel)
		} else {
			path.LimitRange(math.Max(slowBy-rolloutExitLeadM, 0), fast, exitKts, math.Min(math.Max(even, rolloutMinDecel), ro.BrakeDecel))
			// Turning a right angle off the runway before the vacate stop:
			// at the exit speed there too, a high-speed exit included.
			path.limitTurnOff(c.plan.End.Heading, 0, math.Max(fast, hold), c.turnOffKts())
		}
		// Then to taxi speed, planned firmly so it does not reach back over
		// the rollout.
		path.LimitRange(fast, path.Length(), prof.CruiseKts, ro.BrakeDecel)
		moverProf.Decel, moverProf.Jerk = ro.BrakeDecel, RolloutJerk
		c.clearDist, c.exitFastTo = clear, fast
		c.backtracking, c.backFrom = false, exitAt
		if len(turn) > 0 {
			c.backtracking, c.backFrom = true, turnTo
		} else if c.plan.Backtrack && clear > exitAt {
			c.backtracking = true
		}
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
	c.standJoin = math.Max(0, pathLen(pts)-join)
	if c.rollThrough {
		// A rolling clearance: slow to RollThroughKts at the vacate point and
		// taxi on without stopping.
		c.mover.SlowAt(hold, RollThroughKts)
	} else {
		c.mover.HoldAt(hold)
	}
	if replan {
		return nil
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

// moverProfile is profile for its GroundMover: turning about the model's
// own main gear (moverGear).
func (c *ArrivalController) moverProfile() MotionProfile { return moverGear(c.profile(), c.req.Model) }

// step advances the mover to now and places the aircraft.
func (c *ArrivalController) step() GroundPose {
	pose, err := c.advance()
	if err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	if hs := c.giveWayHoldShort(c.track); c.givingWay != c.last.GivingWayTo || hs != c.last.GiveWayHoldShort {
		c.last.GivingWayTo, c.last.GiveWayHoldShort = c.givingWay, hs
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
	if n, changed := c.detailS.want(c.detail, c.now(), pose.Position, pose.GroundSpeedKts > 0.5 || c.detailS.moved(pose.Position, pose.Heading), full); changed {
		if client := c.fleet.clientOrNil(); client != nil {
			c.monitorStopped = false
			c.note("monitor detail", c.requestMonitor(client, types.SIMCONNECT_PERIOD_SIM_FRAME, n))
		}
	}
	c.detail.report(c.objectID, c.detailS.interval)
}

// onInjectedFrame runs the ground phase once the injector has the aircraft:
// every sim frame the mover steps and the aircraft is placed.
func (c *ArrivalController) onInjectedFrame() {
	// Off the runway the arrival follows the traffic ahead (#334).
	c.followTraffic = c.state == ArrivalVacating || c.state == ArrivalTaxiing || c.state == ArrivalParking
	c.stopFloor = 0
	// On its landing roll it never rolls through one on the runway ahead
	// (#1680).
	c.runwayAhead, c.runwayTo = c.state == ArrivalRollout, c.clearDist
	// On the rollout it follows none, but does not drive through what
	// stands on its way (#1632).
	c.seeStanding = c.state == ArrivalRollout
	if c.backtracking && c.mover.Pose().Distance >= c.backFrom && (c.state == ArrivalRollout || c.state == ArrivalVacating) {
		// Backtracking at taxi speed: brakes as a taxiing crew does, and
		// sees what is ahead off the runway before it is off it, rather
		// than at that frame (#1640: 10 kt to 0 in one write).
		p := c.moverProfile()
		p.Decel, p.Jerk = backtrackDecel, RolloutJerk
		c.mover.SetProfile(p)
		if c.state == ArrivalRollout {
			c.followTraffic, c.stopFloor = true, c.clearDist+backtrackClearM
		}
	}
	pose := c.step()
	if c.state == ArrivalRollout && c.runwayAhead && c.headOn && c.now().Sub(c.headOnAt) >= headOnRetry {
		if c.vacateFirst(pose) {
			return // planned again from here: the next frame goes on along it
		}
	}
	// Backtracking with one facing it on the runway short of its way off:
	// off at the nearest way it can reach (#1680).
	if c.state == ArrivalRollout && c.backtracking && c.followTraffic && c.bodyFacing && c.bodyAt < c.clearDist && c.now().Sub(c.headOnAt) >= headOnRetry {
		if c.turnOffNearest(pose) {
			return
		}
	}
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
	if at, past := c.holdPoints(pose); setHolds(at, past, &c.last.HoldAt, &c.last.ClearedPast) {
		c.emit(nil, true) // a stop bar to switch
	}
	if !c.flapsUpFrom.IsZero() && c.flapsPct > 0 {
		c.flapsPct = math.Max(0, c.aircraft().Flaps.LandingPct*(1-c.now().Sub(c.flapsUpFrom).Seconds()/(FlapsRetractSeconds*f(c.timing.flaps))))
		c.note("flaps", c.inj.SetFlaps(c.objectID, c.flapsPct))
	}
	c.stepSurfaces(c.frameDt)
	c.reverseIdle(pose.GroundSpeedKts)
	c.taxiLightDue() // TaxiLightDelay after the landing lights went off
	switch c.state {
	case ArrivalRollout:
		// Clear of the runway: taxi behaviour, landing lights and strobes off.
		if pose.Distance >= c.clearDist {
			if c.exitFastTo <= c.clearDist && !c.backtracking {
				c.mover.SetProfile(c.moverProfile())
			}
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
		// Its tail clear too: taxi behaviour.
		if c.exitFastTo > c.clearDist && pose.Distance >= c.exitFastTo {
			c.mover.SetProfile(c.moverProfile())
			c.exitFastTo = 0
		}
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
			c.backtracking = false
			c.mover.SetProfile(c.moverProfile())
			if !c.rollThrough {
				// Waiting, it stands where it stopped: short of the vacate
				// stop for traffic, it crept on to it (#1640).
				c.mover.HoldAt(pose.Distance)
			}
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
			// Down on all its wheels (PlaceRest), not left on extended
			// struts with the main gear in the air.
			c.note("rest on the stand", c.inj.PlaceRest(c.objectID, pose))
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
	c.approach = NewApproachMover(c.plan.End.LandingThreshold(), c.plan.End.Heading, startMeters, c.approachProfile())
	c.approach.SetCrosswind(c.req.CrosswindKts)
	c.approach.SetAimShift(c.aimShift(startMeters, TouchdownSpreadMeters*(2*c.rng.Float64()-1)))
	c.approach.SetSideShift(TouchdownSideMeters * (2*c.rng.Float64() - 1))
	// Its own phases, seeded by the object: no draw from c.rng, whose
	// sequence the other choices keep.
	c.approach.SetBalance(c.req.WindKts, c.req.GustKts, balanceRand(c.objectID))
	at := c.approach.Pose()
	c.seq.add(c.now(), "takeover on final: gear down, approach flaps", at.HeightFt, at.GroundSpeedKts)
	c.monitorEvery(types.SIMCONNECT_PERIOD_SIM_FRAME)
	c.fast = true
	c.lastStep = c.now()
	c.setState(ArrivalApproaching, nil)
	return nil
}

// Short fields (#1618: TFFJ 10, 642 m, its one exit 271 m in): below
// shortFieldMeters of landing distance an injected landing aims to touch
// down shortFieldShare of it past the threshold, at least
// shortFieldMinTouchdownM, with a tenth of the usual spread. At its usual
// point (about 300 m for a DHC6) it touched down past the exit and swung
// round to it at 64 kt. So too on a longer one its usual touchdown and
// roll (landingRollFor) do not fit (#1718), which RequiredLandingRun is.
const (
	shortFieldMeters        = 1200.0
	shortFieldShare         = 0.1
	shortFieldMinTouchdownM = 60.0
)

// shortFieldLanding: on landing meters of landing distance an injected
// landing rolling roll from its touchdown to its exit speed flies the
// short-field landing (aimShift), its usual one not fitting.
func shortFieldLanding(landing, roll float64) bool {
	return landing > 0 && (landing < shortFieldMeters || landing < exitTouchdownMeters+roll)
}

// shortFieldRun is how far past the landing threshold req's injected
// landing is down to its exit speed when it flies the short-field one
// (shortFieldLanding): its touchdown aimed at the latest (aimShift, the
// spread's tenth) and its roll; 0 when it lands as usual, or is not
// injected. Its exit is planned on it (exitReach.short, #1719).
func shortFieldRun(req ArrivalRequest) float64 {
	if !req.InjectApproach || req.Graph == nil {
		return 0
	}
	r, e, ok := req.Graph.Layout.RunwayEnd(req.Runway)
	if !ok || r.Length <= 0 {
		return 0
	}
	landing := e.LandingDistance(r.Length)
	roll := landingRollFor(approachProfileOf(req), withDefaults(req.Rollout, DefaultRolloutProfile()))
	if !shortFieldLanding(landing, roll) {
		return 0
	}
	return math.Max(shortFieldMinTouchdownM, shortFieldShare*landing) + TouchdownSpreadMeters/10 + roll
}

// aimShift is the injected approach's aim shift from startMeters out: the
// touchdown spread drawn, or on a short field the shift to its early
// touchdown.
func (c *ArrivalController) aimShift(startMeters, spread float64) float64 {
	landing := c.plan.End.LandingDistance(c.plan.Runway.Length) // its LDA (#1660)
	roll := landingRollFor(c.approachProfile(), c.rolloutProfile())
	if !shortFieldLanding(landing, roll) {
		return spread
	}
	probe := NewApproachMover(c.plan.End.LandingThreshold(), c.plan.End.Heading, startMeters, c.approachProfile())
	for i := 0; i < 100000 && probe.phase < ApproachDerotate; i++ {
		probe.step(0.05)
	}
	if probe.phase < ApproachDerotate {
		return spread
	}
	if landing >= shortFieldMeters && probe.touchX+spread+roll <= landing {
		return spread // its usual landing fits its landing distance (#1718)
	}
	target := math.Max(shortFieldMinTouchdownM, shortFieldShare*landing)
	return math.Min(0, target-probe.touchX) + spread/10
}

// crewLookFt: from this height above the runway to the touchdown the crew
// of an injected approach looks at its runway (#1680): another aircraft
// on it takes it around, cleared to land or not, with no tower too. A
// controller sends it round earlier, at its decision point.
const crewLookFt = 100.0

// crewGoesAround takes the injected approach around when another
// aircraft is on its runway below crewLookFt, the crew's own go-around as
// for an approach not stable (CrewGoAround, RunwayTaken); whether it did.
// c.mu held.
func (c *ArrivalController) crewGoesAround() bool {
	if c.landsOccupied || c.picture == nil || c.approach == nil || c.plan == nil {
		return false
	}
	if p := c.approach.Pose(); p.OnGround || p.HeightFt > crewLookFt {
		return false
	}
	who := c.picture.OnRunway(c.objectID, c.plan.Runway, c.now())
	if who == 0 {
		return false
	}
	if err := c.goAroundLocked(); err != nil {
		c.note("crew go-around refused", err)
		return false
	}
	c.last.CrewGoAround, c.last.RunwayTaken = true, true
	c.note("crew go-around: runway occupied by object "+strconv.FormatUint(uint64(who), 10), nil)
	c.emit(nil, true)
	return true
}

// onApproachFrame flies the injected approach one frame and hands over to
// the injected rollout once the nose wheel is down.
func (c *ArrivalController) onApproachFrame(m arrivalMonitor) {
	now := c.now()
	if c.crewGoesAround() {
		return
	}
	dt := c.frameStep(now) // no step back with the clock (#105)
	if !c.approachLightsSet {
		c.approachLightsSet = true
		c.lights = m.currentLights()
		// The aircraft spawns with its logo light off; MSFS AI switches it on
		// on approach, so the injected approach does too (consistent look).
		c.lights.Logo = !c.aircraft().Lights.NoLogo
		c.setInjectedLights(lightsRollout, "lights approach (injected)")
	}
	onPath := c.approach.Step(math.Max(dt, 0))
	pose := c.blend.apply(onPath, math.Max(dt, 0))
	if c.blend.liftDue {
		// The reference point's height above the wheels off the join's
		// height offset, once the injector knows it (it writes nothing
		// before), at the blended pitch PlaceAir lifts the main gear by:
		// written where MSFS AI had it (#1682).
		if lift, ok := c.inj.AirLiftFt(c.objectID, ApproachPose{MainGearBehindM: c.profile().RefAheadMeters, PitchDeg: pose.PitchDeg}); ok {
			pose.HeightFt -= lift * c.blend.ftShare()
			c.blend.dFt -= lift
			c.blend.liftDue = false
		}
	}
	if !c.last.Unstable && c.blend.judgeGate(onPath.HeightFt) {
		c.last.Unstable = true
		c.note("not stable at the gate: the hand-over's height still off", nil)
		// More left than can fade before touchdown (#1681): its crew goes
		// round, its own call; one that lands from it has the height faded
		// by unstableTouchdownFt, never snapped onto the runway.
		if t := secondsTo(onPath, unstableTouchdownFt); c.blend.ftLeft > t {
			if c.landsUnstable {
				c.blend.refit(t)
				c.note("landing not stable: the height faded by 50 ft", nil)
			} else if err := c.goAroundLocked(); err == nil {
				c.last.CrewGoAround = true
				c.note("crew go-around: not stable at the gate", nil)
				c.emit(nil, true)
				return
			} else {
				c.note("crew go-around refused", err)
			}
		}
		c.emit(nil, true)
	}
	pose.MainGearBehindM = c.profile().RefAheadMeters // the flare turns on the main gear too
	pose.Landing = true
	pose.RunwayFt = c.plan.Runway.Altitude / 0.3048 // a steady glide path over any terrain
	if err := c.inj.PlaceAir(c.objectID, pose); err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	c.last.Position, c.last.Heading, c.last.GroundSpeed = pose.Position, pose.Heading, pose.GroundSpeedKts
	c.last.AGL, c.last.OnGround = pose.HeightFt, pose.OnGround
	c.reverseIdle(pose.GroundSpeedKts)
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
		if ReverseThrottlePct < 0 {
			c.reversing = true
			c.note("reverse thrust", c.inj.SetThrottle(c.objectID, c.aircraft().EngineCount(), ReverseThrottlePct))
			c.seq.add(now, "reverse thrust", 0, pose.GroundSpeedKts)
		}
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

// pullInMeters and pullInDegrees: how far behind a face-out stand, and how far
// off its heading, a route may come and still pull straight in.
const (
	pullInMeters  = 60.0
	pullInDegrees = 75.0
)

// turnsOnStand says an arrival on route has to turn around to park: the lead-in
// is ahead of the stand, and the route does not already come up from behind it
// (LOWI E1: on along the taxiway to the lead-in, back to the stand and a loop
// behind it, where nothing says there is apron). route[from:] is still ahead.
func turnsOnStand(g *airport.Graph, parking int, route []airport.LatLon, from int) bool {
	if len(route) < 2 || !leadInAhead(g, parking, route[len(route)-2]) {
		return false
	}
	p := g.Layout.Parking[parking]
	for i := len(route) - 2; i >= max(from, 1); i-- {
		along := alongHeading(p.Position, p.Heading, route[i])
		if along > -standAxisMeters {
			continue
		}
		off := math.Abs(headingDiff(p.Heading, localBearing(route[i-1], route[i])))
		return math.Hypot(along, alongHeading(p.Position, p.Heading+90, route[i])) > pullInMeters || off > pullInDegrees
	}
	return true
}

// turnAround is the custom route onto a self-manoeuvring (face-out) stand:
// from the lead-in junction ahead of the stand the aircraft swings out to
// the side with fewer neighbouring stands, loops round behind the stop mark
// and comes back along the stand centreline, facing out, to stop with its
// nose gear at stopNose.
func (c *ArrivalController) turnAround(stopNose airport.LatLon) []airport.LatLon {
	// Scaled for longer aircraft: TurnAroundMeters suits an A320's wheelbase.
	r := TurnAroundMeters * math.Max(1, c.profile().WheelbaseMeters/DefaultMotionProfile().WheelbaseMeters)
	return turnLoop(stopNose, c.standHeading, r, loopSide(c.req.Graph, c.req.Parking, stopNose, c.standHeading, r))
}

// turnLoop is the way round behind the stop mark, on side (+1 right of the
// stand's heading, -1 left), for a turn of radius r.
func turnLoop(stopNose airport.LatLon, h, r, side float64) []airport.LatLon {
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

// loopOffPavementMeters: a loop this much further from mapped pavement than
// the one on the other side gives way to it.
const loopOffPavementMeters = 10.0

// loopSide is the side for the turn: the one with fewer neighbouring stands,
// unless it leaves mapped pavement for ground nothing says is apron and the
// other does not as much (LOWI E1, taxiway L on one side, nothing on the other).
func loopSide(g *airport.Graph, parking int, stopNose airport.LatLon, h, r float64) float64 {
	side := roomySide(g, parking)
	pv := pavementAround(g, stopNose, 4*r)
	furthest := func(pts []airport.LatLon) float64 {
		worst := 0.0
		for _, p := range pts {
			worst = math.Max(worst, pv.off(p))
		}
		return worst
	}
	if furthest(turnLoop(stopNose, h, r, side)) > furthest(turnLoop(stopNose, h, r, -side))+loopOffPavementMeters {
		return -side
	}
	return side
}

// roomySide is +1 or -1: the side of the stand (right or left of its
// heading) with fewer other stands within 60 m, for the turn-around loop.
func roomySide(g *airport.Graph, parking int) float64 {
	stand := g.Layout.Parking[parking]
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
	return RolloutProfile{BrakeDecel: 2.5, SlowKts: 80, HighSpeedExitKts: 32, ExitKts: 12, ExitLateralAccel: 1.1}
}

func (c *ArrivalController) rolloutProfile() RolloutProfile {
	return withDefaults(c.req.Rollout, DefaultRolloutProfile())
}

// turnOffKts is the speed through a right-angle turn off the runway
// (GroundPath.limitTurnOff): the rollout's ExitKts, faster when expediting.
func (c *ArrivalController) turnOffKts() float64 {
	kts := c.rolloutProfile().ExitKts
	if c.rush {
		kts += RushExitKts
	}
	return kts
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

// A backtrack's brakes (m/s²), and how far past the runway's edge traffic
// ahead may stop it at the nearest (#1640).
const (
	backtrackDecel  = 1.0
	backtrackClearM = 1.0
)

// vacateFirst: nose to nose on the runway on its landing roll with one
// coming the other way whose way off is behind it (groundDrive.headOn), it
// vacates first, at the nearest exit ahead short of the other that fits
// and that it can make; the other then goes on to its own way off (#1680:
// each stood for the other, neither way off free). Looked for every
// headOnRetry while they face each other (slower, it makes one it could
// not before); none such: it stands. True when planned again. c.mu held.
func (c *ArrivalController) vacateFirst(pose GroundPose) bool {
	c.headOnAt = c.now()
	if c.plan == nil || c.mover == nil || !c.mover.hasTraffic {
		return false
	}
	exits, err := c.req.Graph.RunwayExits(c.plan.End.Name)
	if err != nil {
		c.note("vacate first", err)
		return false
	}
	prof := c.profile()
	end := c.plan.End
	along := alongHeading(end.Threshold, end.Heading, NoseGear(pose.Position, pose.Heading, prof))
	noseTip := (pushNoseFactor - 1) * prof.WheelbaseMeters
	body := along + c.mover.trafficAt - pose.Distance + noseTip + TrafficGapMeters
	before := body - headOnExitClearM
	slices.SortFunc(exits, func(a, b airport.RunwayExit) int { return cmp.Compare(a.Along, b.Along) })
	for _, x := range exits {
		if x.Along <= along || x.Along > before || x.Node == c.plan.Exit.Node {
			continue
		}
		if err := c.vacateAt(x); err == nil {
			c.note("nose to nose on the runway: vacating first at "+x.Taxiway, nil)
			c.emit(nil, true)
			return true
		}
	}
	c.note("nose to nose on the runway: no way off short of the other", nil)
	return false
}

// turnOffNearest: backtracking, nose to nose on the runway with one whose
// body is short of its way off (groundDrive.bodyFacing; the other one on
// its landing roll that has no way off short of this one either, as
// vacateFirst found), it turns off at the nearest way ahead short of the
// other that it can reach, onto any route to its stand, one turning back
// on itself too: off the runway, neither stands for the other for good
// (#1680). Looked for every headOnRetry while it stands for the other;
// none such: it holds where it stands. True when planned again. c.mu
// held.
func (c *ArrivalController) turnOffNearest(pose GroundPose) bool {
	c.headOnAt = c.now()
	if c.plan == nil || c.mover == nil {
		return false
	}
	g, req := c.req.Graph, c.req
	toward := oppositeEnd(c.plan.Runway, c.plan.End)
	exits, err := g.RunwayExits(toward.Name)
	if err != nil {
		c.note("turn off sooner", err)
		return false
	}
	prof := c.profile()
	along := alongHeading(toward.Threshold, toward.Heading, NoseGear(pose.Position, pose.Heading, prof))
	before := along + c.bodyAt - pose.Distance - headOnExitClearM
	v, decel := pose.GroundSpeedKts*ktsToMS, c.rolloutProfile().BrakeDecel
	slices.SortFunc(exits, func(a, b airport.RunwayExit) int { return cmp.Compare(a.Along, b.Along) })
	for _, x := range exits {
		vx := ExitSpeed(x) * ktsToMS
		need := math.Max(0, (v*v-vx*vx)/(2*decel)) + prof.WheelbaseMeters
		if x.Along < along+need || x.Along > before || x.Node == c.plan.Exit.Node {
			continue
		}
		plan, err := PlanArrival(g, toward.Name, req.Parking, ArrivalOptions{
			SpawnNm: c.plan.SpawnNm, Exit: &x, Route: req.Options, GroundAGL: req.GroundAGL, NoseOffset: req.NoseOffset, StandSide: req.StandSide,
			TouchdownKts: approachProfileOf(req).TouchdownKts, BrakeDecel: req.Rollout.BrakeDecel,
		})
		if err != nil {
			continue
		}
		plan.Spawn = c.plan.Spawn
		oldPlan, oldTrack, oldExit, oldVacate := c.plan, c.track, c.exitAlong, c.vacateAlong
		c.plan, c.track = plan, newRouteTracker(plan.Route)
		c.exitAlong = c.track.cum[len(plan.Exit.Path)-1]
		c.vacateAlong = c.track.cum[plan.VacateIndex] - plan.VacateBackMeters
		m := arrivalMonitor{Latitude: pose.Position.Lat, Longitude: pose.Position.Lon, Heading: pose.Heading, GroundKts: pose.GroundSpeedKts}
		if err := c.takeover(m, pose.Position, true); err != nil {
			c.plan, c.track, c.exitAlong, c.vacateAlong = oldPlan, oldTrack, oldExit, oldVacate
			continue
		}
		c.note("nose to nose on the runway: turning off sooner at "+x.Taxiway, nil)
		c.emit(nil, true)
		return true
	}
	c.note("nose to nose on the runway: no way off short of the other, holding", nil)
	return false
}

// headOnRetry: nose to nose on the runway, how often a way off is looked
// for again (vacateFirst, turnOffNearest; the last look at headOnAt).
const headOnRetry = 2 * time.Second

// headOnExitClearM: an exit taken to vacate first is at least this far
// short of the body of the one it stood for (vacateFirst): the turn off,
// its wings, clear of it.
const headOnExitClearM = 40.0

// reverseIdle takes the reverse thrust back to idle below ReverseIdleKts.
func (c *ArrivalController) reverseIdle(kts float64) {
	if !c.reversing || kts >= ReverseIdleKts {
		return
	}
	c.reversing = false
	c.note("reverse idle", c.inj.SetThrottle(c.objectID, c.aircraft().EngineCount(), 0))
	c.seq.add(c.now(), "reverse idle", 0, kts)
}

// landingLeft is how far ahead of nose, along the landing end, its declared
// landing distance ends (RunwayEnd.LandingEnd); negative past it. c.mu
// held.
func (c *ArrivalController) landingLeft(nose airport.LatLon) float64 {
	e := c.plan.End.LandingEnd(c.plan.Runway.Length)
	off := (localBearing(nose, e) - c.plan.End.Heading) * math.Pi / 180
	return localDist(nose, e) * math.Cos(off)
}

// The rollout's even deceleration (injected landing): to the exit speed
// rolloutExitLeadM before the exit's turn, at least rolloutMinDecel (m/s²)
// so a far exit is not crawled to.
const (
	rolloutExitLeadM = 60.0
	rolloutMinDecel  = 1.0
	// exitClearAheadM: past the exit's first node off the runway, the way
	// on until the tail is clear as the tower counts it (40 m beyond the
	// edge for the reference point, and the tail behind it).
	exitClearAheadM = 60.0
)
