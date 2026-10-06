package traffic

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Injected departure (#320): with TaxiWithInjector the whole departure is
// driven by position injection — pushback, taxi, line-up, take-off and the
// initial climb — through clearance gates, with the same light rules as
// injected arrivals (groundDrive).

// TaxiWithInjector drives the departure by position injection with inj
// (shared by all controllers; feed it every message as well).
func TaxiWithInjector(inj *Injector) TaxiOption {
	return func(c *TaxiController) { c.inj = inj }
}

// TaxiWithClock runs the departure on clock (e.g. SimClock.Now: the
// simulation rate, stopped while paused) instead of the wall clock (#413).
func TaxiWithClock(clock func() time.Time) TaxiOption {
	return func(c *TaxiController) { c.now = clock }
}

// TaxiWithDetail drives the injected departure on fewer sim frames when it
// is far from the viewer or standing still (#370); on the runway always on
// every frame.
func TaxiWithDetail(d *Detail) TaxiOption {
	return func(c *TaxiController) { c.detail = d }
}

// TaxiWithGroundPicture shares the ground picture with the other aircraft at
// the airport: the injected departure reports itself and, while taxiing,
// stops behind the traffic ahead (#334).
func TaxiWithGroundPicture(p *GroundPicture) TaxiOption {
	return func(c *TaxiController) { c.picture = p }
}

// TaxiWithServices takes the departure's tug and fuel truck from the
// airport's fleet (#830): sent only when one is free.
func TaxiWithServices(f ServiceFleet) TaxiOption {
	return func(c *TaxiController) { c.services = f }
}

// take reserves a vehicle of kind from the fleet (none: always).
func (c *TaxiController) take(kind VehicleKind) bool {
	return c.services == nil || c.services.Take(kind, c.req.Tail)
}

// give returns the departure's vehicle of kind to the fleet.
func (c *TaxiController) give(kind VehicleKind) {
	if c.services != nil {
		c.services.Give(kind, c.req.Tail)
	}
}

// Departure lights by phase: parked with nav lights, beacon on from the
// pushback clearance, taxi light for taxiing, strobes when entering the
// runway, landing lights with the take-off clearance, taxi light off once
// airborne. Runway crossings add strobes and landing lights (groundDrive).
var (
	lightsLineUp  = Lights{Nav: true, Beacon: true, Strobe: true, Taxi: true}
	lightsTakeoff = Lights{Nav: true, Beacon: true, Strobe: true, Taxi: true, Landing: true}
	lightsClimb   = Lights{Nav: true, Beacon: true, Strobe: true, Landing: true}
)

func (c *TaxiController) profile() MotionProfile {
	p := DefaultMotionProfile()
	if c.req.Profile != (MotionProfile{}) {
		p = c.req.Profile
	}
	limit := 0.0
	if c.req.Airport != nil {
		limit = c.req.Airport.TaxiMaxKts
	}
	return taxiSpeed(p, c.timing.taxiSpeed, limit)
}

func (c *TaxiController) takeoffProfile() TakeoffProfile {
	if c.req.Takeoff != (TakeoffProfile{}) {
		return c.req.Takeoff
	}
	return DefaultTakeoffProfile()
}

// note records the send ID of the request just made for exception reports.
func (c *TaxiController) note(desc string, err error) {
	if client := c.fleet.clientOrNil(); client != nil && c.sent != nil {
		if id, idErr := client.GetLastSentPacketID(); idErr == nil {
			c.sent[id] = desc
		}
	}
	_ = err
}

// gate reports whether a gate is passed: cleared, or — without
// HoldForClearances — its automatic wait is over.
func (c *TaxiController) gate(cleared bool) bool {
	return cleared || (!c.req.HoldForClearances && !c.now().Before(c.gateAt))
}

// runwayGate is a gate onto the runway (line-up, take-off): with
// HoldForRunway only its clearance passes it (#393).
func (c *TaxiController) runwayGate(cleared bool) bool {
	return cleared || !c.req.HoldForRunway && c.gate(false)
}

// setRequest updates what the crew asks for (TaxiEvent.Request): held for
// a clearance and ready — its automatic wait over, the tug clear for taxi.
func (c *TaxiController) setRequest(now time.Time) {
	req := ""
	if c.req.HoldForClearances && !now.Before(c.gateAt) {
		switch {
		case c.state == TaxiAwaitingPushback && !c.pushCleared && c.facesOut():
			req = "start_up" // taxis straight out: no pushback to ask for
		case c.state == TaxiAwaitingPushback && !c.pushCleared:
			req = "pushback"
		case c.state == TaxiAwaitingTaxi && !c.startUpCleared && !c.taxiCleared && c.tugClear():
			req = "start_up" // the tug gone: the crew asks to start
		case c.state == TaxiAwaitingTaxi && !c.taxiCleared && c.tugClear() && c.enginesReady(now):
			req = "taxi"
		}
	}
	if req != c.last.Request {
		c.last.Request = req
		c.emit(nil, true)
	}
}

// openGate starts a gate's automatic wait of about d.
func (c *TaxiController) openGate(d time.Duration) {
	if c.rush {
		d = time.Duration(float64(d) * RushDelayFactor) // expedited: no lingering
	}
	c.gateAt = c.now().Add(time.Duration(float64(d) * (1 + DwellJitter*(2*c.rng.Float64()-1))))
}

// Expedite has the crew hurry (#510): the waits before taxi, line-up and
// take-off shrink to RushDelayFactor; a gate already open closes sooner.
// The clearance says it (Rushed).
func (c *TaxiController) Expedite(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if on && !c.rush && !c.gateAt.IsZero() {
		if left := c.gateAt.Sub(c.now()); left > 0 {
			c.gateAt = c.now().Add(time.Duration(float64(left) * RushDelayFactor))
		}
	}
	c.rush = on
}

// startInjectedDeparture takes the aircraft over on the stand.
func (c *TaxiController) startInjectedDeparture() error {
	if err := c.inj.Takeover(c.objectID); err != nil {
		return err
	}
	c.injector, c.object, c.graph, c.prof = c.inj, c.objectID, c.req.Graph, c.profile()
	c.noLogo = WakeFor(c.req.Model).ICAO == WakeLight
	// Cold on the stand: the engines start once the tug has gone.
	c.note("engines off", c.inj.SetEngines(c.objectID, c.aircraft().EngineCount(), false))
	c.enginesOn = false
	c.holdAtCrossings = c.req.HoldForClearances || c.req.HoldForRunway
	client := c.fleet.clientOrNil()
	if client == nil {
		return ErrNotConnected
	}
	if err := client.RequestDataOnSimObject(c.reqBase+reqOffMonitor, c.defBase+defOffMonitor, c.objectID,
		types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
		return err
	}
	c.last.LimitNode = -1
	c.fast = true
	c.openGate(PushbackDelay)
	if at := c.req.PushbackAt; at.After(c.gateAt) {
		c.gateAt = at // boarding until the STD
	}
	c.setState(TaxiAwaitingPushback, nil)
	return nil
}

// frameDetail sets how often the departure is driven (#370): moving (the
// push, taxiing) by its distance from the viewer, on the runway every
// frame, otherwise standing still. A tug driving (in, pushing, backing off
// and away) is moved on these frames too: every frame, wherever it is — at
// fewer its drive-off after the push stuttered at a still aircraft's rate.
func (c *TaxiController) frameDetail(now time.Time, pos airport.LatLon) {
	if c.detail == nil {
		return
	}
	speed := 0.0
	if c.mover != nil {
		speed = c.mover.Pose().GroundSpeedKts
	}
	// Driving in from its depot counts too: the aircraft waits still on the
	// stand meanwhile, and at a still aircraft's rate the tug jumped along
	// the road (live, 2026-10-02).
	arriving := false
	if t, ok := c.req.Tug.(interface{ Connected() bool }); ok && c.tugAttached {
		arriving = !t.Connected()
	}
	tugDriving := c.req.Tug != nil && !c.req.Tug.Done() &&
		(c.state == TaxiAwaitingPushback && (!c.tugAttached || !c.pushAt.IsZero() || arriving) || c.state >= TaxiPushback && c.tugAttached)
	tugDriving = tugDriving || c.fuelDriving()
	moving := c.state == TaxiPushback || speed > 0.5 || tugDriving
	full := c.state >= TaxiLiningUp || tugDriving
	if n, changed := c.detailS.want(c.detail, now, pos, moving, full); changed {
		if client := c.fleet.clientOrNil(); client != nil {
			c.note("monitor detail", requestFrames(client, c.reqBase+reqOffMonitor, c.defBase+defOffMonitor, c.objectID, n))
		}
	}
	c.detail.report(c.objectID, c.detailS.interval)
}

// onDepartureFrame runs the injected departure one sim frame.
func (c *TaxiController) onDepartureFrame(m taxiMonitor) {
	now := c.now()
	// While taxiing the departure follows the traffic ahead (#334); pushed
	// and waiting for the taxi clearance, it shows where it will go (#452).
	c.followTraffic = c.state == TaxiTaxiing
	c.planned = nil
	if c.state == TaxiAwaitingTaxi && c.picture != nil && c.mover != nil {
		c.planned = c.plannedTaxi(c.mover.Pose().Position)
	}
	if c.state < TaxiDeparting {
		pos, hdg := airport.LatLon{Lat: m.Latitude, Lon: m.Longitude}, m.Heading
		if c.mover != nil { // injected: where it is placed
			p := c.mover.Pose()
			pos, hdg = p.Position, p.Heading
		}
		c.reportGround(c.objectID, pos, hdg, now)
		if c.last.Position == (airport.LatLon{}) {
			c.last.Position, c.last.Heading = pos, hdg // known from the first frame, before it moves
		}
		c.frameDetail(now, pos)
	} else if c.picture != nil {
		c.picture.Forget(c.objectID) // on the take-off roll or airborne
	}
	if !c.lightsSet {
		// Parked: nav lights, logo and wing as the aircraft has them.
		c.lightsSet = true
		c.lights = m.currentLights()
		c.setInjectedLights(LightsParked, "lights parked")
	}
	c.last.Lights = m.currentLights()
	// Flaps move on every frame, whatever the phase.
	if !c.frameAt.IsZero() && c.flaps.step(math.Min(now.Sub(c.frameAt).Seconds(), MaxFrameStepSeconds)) {
		c.note("flaps", c.inj.SetFlaps(c.objectID, c.flaps.pct))
	}
	if !c.frameAt.IsZero() {
		c.updateTug(math.Min(now.Sub(c.frameAt).Seconds(), MaxFrameStepSeconds))
		c.updateFuel(math.Min(now.Sub(c.frameAt).Seconds(), MaxFrameStepSeconds))
	}
	c.frameAt = now
	c.setRequest(now)
	switch c.state {
	case TaxiAwaitingPushback:
		// De-icing on the stand: once cleared to push, the treatment first.
		if d := c.req.Deice; d != nil && d.Pad == nil && !c.deiced {
			if c.deiceUntil.IsZero() {
				if !c.gate(c.pushCleared) {
					c.emit(nil, false)
					return
				}
				c.startDeicing(now)
			}
			if now.Before(c.deiceUntil) {
				c.emit(nil, false)
				return
			}
			c.finishDeicing(LightsParked)
		}
		if c.pushAt.IsZero() && c.gate(c.pushCleared) && !c.pushStopped {
			// Beacon on, and the push starts BeaconLeadTime later.
			c.lights.Logo = !c.aircraft().Lights.NoLogo // as MSFS AI shows it; aircraft spawn with it off
			c.setInjectedLights(LightsPushback, "lights beacon (pushback)")
			c.pushAt = now.Add(time.Duration(float64(BeaconLeadTime) * f(c.timing.beacon)))
		}
		if !c.pushAt.IsZero() && !now.Before(c.pushAt) {
			// Nobody pushes into traffic: wait while the corridor behind
			// the stand is not clear.
			if !c.facesOut() && c.pushBlocked(now) || !c.fuelClear() {
				c.emit(nil, false)
				return
			}
			if c.facesOut() {
				// Self-manoeuvring stand: no pushback — engines start on the
				// stand and the aircraft taxis straight out.
				if err := c.standInPlace(); err != nil {
					c.fail(err)
					return
				}
				c.openGate(TaxiAfterPushDelay)
				c.setState(TaxiAwaitingTaxi, nil)
				return
			}
			// The tug drives in from its depot first.
			if !c.tugConnected() {
				c.emit(nil, false)
				return
			}
			if err := c.startPushback(); err != nil {
				c.fail(err)
				return
			}
			c.setState(TaxiPushback, nil)
		}
		c.emit(nil, false)
		return
	case TaxiAwaitingTaxi:
		// The tug gone (or none, on a nose-out stand) and the start-up
		// approved: the engines start, one after the other.
		// A taxi clearance given without one covers the start-up too.
		if !c.enginesOn && c.tugClear() && c.gate(c.startUpCleared || c.taxiCleared) {
			n := c.aircraft().EngineCount()
			c.enginesOn = true
			c.enginesReadyAt = now.Add(time.Duration(float64(n) * float64(EngineStartTime) * f(c.timing.tug)))
			c.note("engines start", c.inj.SetEngines(c.objectID, n, true))
		}
		// Never taxi into the tug: it disconnects, backs off and drives
		// clear first, whatever the clearance says; nor before the engines
		// run.
		if c.moveAt.IsZero() && c.tugClear() && c.enginesReady(now) && c.gate(c.taxiCleared) {
			// Taxi light on, then release the brakes TaxiLightDelay later.
			c.setInjectedLights(LightsTaxi, "lights taxi")
			// Take-off flaps set after engine start, while taxiing out.
			to := c.aircraft().Flaps.TakeoffPct
			c.flaps = surfaceRamp{target: to, rate: to / (FlapsSetSeconds * f(c.timing.flaps))}
			c.moveAt = now.Add(time.Duration(float64(TaxiLightDelay) * f(c.timing.taxiLight)))
		}
		if !c.moveAt.IsZero() && !now.Before(c.moveAt) {
			if err := c.startTaxiOut(); err != nil {
				c.fail(err)
				return
			}
			c.setState(TaxiTaxiing, nil)
		}
		c.emit(nil, false)
		return
	case TaxiLinedUp:
		// A cancelled take-off clearance holds it lined up until the next
		// ClearForTakeoff, even without held gates.
		if c.runwayGate(c.takeoffCleared) && (c.takeoffCleared || !c.takeoffHeld) {
			c.startTakeoff()
			return
		}
		c.emit(nil, false)
		return
	case TaxiDeparting:
		c.onTakeoffFrame()
		return
	}
	if c.mover == nil {
		return
	}
	if c.state == TaxiPushback {
		c.holdPushForTraffic(now)
	}
	pose, err := c.advance()
	if err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	if c.givingWay != c.last.GivingWayTo {
		c.last.GivingWayTo = c.givingWay
		c.emit(nil, true)
	}
	if by := map[bool]string{true: c.stoppedBy(pose)}[c.state == TaxiTaxiing]; by != c.last.StoppedBy {
		c.last.StoppedBy = by
		c.emit(nil, true)
	}
	c.last.Position, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pose.Position, pose.Heading, pose.GroundSpeedKts, true
	c.last.Remaining = math.Max(0, c.mover.Path().Length()-pose.Distance)
	if c.state != TaxiPushback {
		// The taxiway it is on: said in its reports ("holding short of
		// runway 12 at F").
		if c.track != nil {
			if seg, _ := c.track.advance(pose.Position); seg >= 0 {
				c.last.Taxiway = c.track.taxiwayAt(seg)
			}
		}
		c.checkCrossing(pose)
	}
	c.last.LimitNode = -1
	if c.hasLimit {
		c.last.LimitNode = c.limitNode
	}
	if at := c.atLimit(pose); at != c.last.AtLimit {
		c.last.AtLimit = at
		c.emit(nil, true) // holding at the clearance limit, or moving on
	}
	switch c.state {
	case TaxiPushback:
		if pose.Arrived && c.towPts != nil && !c.towing {
			// Pushed back: the tug tows the aircraft on forward to where the
			// push ends (planPushPose).
			if err := c.startTow(pose); err != nil {
				c.note("tow after the push", err)
			} else {
				return
			}
		}
		if pose.Arrived {
			c.setPushHeld(false) // the push is done: nothing to hold for any more
			if c.picture != nil {
				// Its way on at once, not a frame later: a neighbour checking
				// now must not see an empty picture and push into it (#466).
				c.picture.ReportPush(c.objectID, nil, 0)
				if planned := c.plannedTaxi(pose.Position); len(planned) > 0 {
					c.picture.ReportPlanned(c.objectID, planned, c.halfSpan())
				}
			}
			c.openGate(TaxiAfterPushDelay)
			c.setState(TaxiAwaitingTaxi, nil)
			return
		}
	case TaxiTaxiing:
		// At the de-icing pad: engines running, taxi light off, treated.
		if c.hasPad && pose.Stopped && pose.Distance >= c.padStop-0.5 {
			if c.deiceUntil.IsZero() {
				c.startDeicing(now)
				c.setInjectedLights(LightsPushback, "lights de-icing (taxi light off)")
			}
			if !now.Before(c.deiceUntil) {
				c.hasPad = false
				c.updateHold()
				c.finishDeicing(LightsTaxi)
			}
			c.emit(nil, false)
			return
		}
		if rwy, ok := c.atCrossingHold(pose); ok {
			c.holdingCrossing = true
			c.last.HoldingShortOf = rwy
			c.setState(TaxiHoldingShort, nil)
			return
		}
		// Cleared for take-off on the way: no stop at the holding point, on
		// into the line-up at its speed (traffic ahead still stops it: the
		// mover keeps its gap).
		if c.takeoffCleared && !c.hasPad && c.runwayGate(true) && c.mover.Path().Length()-pose.Distance <= rollOnMeters {
			c.holdingCrossing = false
			c.startLineUp()
			return
		}
		if pose.Arrived {
			c.holdingCrossing = false
			c.last.HoldingShortOf = c.runway.Name()
			c.openGate(LineUpDelay)
			// Without held gates some departures get line-up and take-off in one
			// clearance and roll straight into the take-off.
			if chance := c.req.RollingTakeoffChance; !c.req.HoldForClearances && !c.req.HoldForRunway && chance >= 0 {
				if chance == 0 {
					chance = DefaultRollingTakeoffChance
				}
				c.takeoffCleared = c.takeoffCleared || c.rng.Float64() < chance
			}
			c.setState(TaxiHoldingShort, nil)
			return
		}
	case TaxiHoldingShort:
		if !c.holdingCrossing && c.runwayGate(c.lineUpCleared || c.takeoffCleared) {
			c.startLineUp()
			return
		}
	case TaxiLiningUp:
		if c.takeoffCleared && pose.Distance >= c.alignDist-0.5 {
			c.startTakeoff() // rolling take-off
			return
		}
		if pose.Arrived || (pose.Stopped && pose.Distance >= c.alignDist-0.5) {
			c.openGate(TakeoffDelay)
			c.setState(TaxiLinedUp, nil)
			return
		}
	}
	c.emit(nil, false)
}

// facesOut reports a self-manoeuvring stand (standFacesOut): it taxis out
// without a pushback.
func (c *TaxiController) facesOut() bool {
	return c.faceOut
}

// standFacesOut reports whether the lead-in junction of the route planned
// from the stand lies ahead of the parked aircraft's nose gear, within
// faceOutMaxDeg of its heading. Ahead of the stand's reference point is not
// enough: a junction under the aircraft, behind its nose, is taxied to with
// a turn from a standstill (EDDF B10, KJFK A15: 115°–163°). Decided once,
// from the route to the stand's junction: a push to a pose re-plans the
// route from a taxiway, whose nodes say nothing of the stand.
func (c *TaxiController) standFacesOut() bool {
	if len(c.route.Points) < 2 || !leadInAhead(c.req.Graph, c.req.Parking, c.route.Points[1]) {
		return false
	}
	stand := c.req.Graph.Layout.Parking[c.req.Parking]
	nose := NoseGear(StandPoint(stand, c.req.NoseOffset), stand.Heading, c.profile())
	j := c.route.Points[1]
	return alongHeading(nose, stand.Heading, j) > 0 && math.Abs(headingDiff(stand.Heading, localBearing(nose, j))) <= faceOutMaxDeg
}

// TugLeadTime is how long before the pushback request the tug is sent.
var TugLeadTime = 3 * time.Minute

// faceOutMaxDeg: a self-manoeuvring stand's junction lies at most this
// off the nose.
const faceOutMaxDeg = 60.0

// updateTug connects the tug while the aircraft waits for its pushback
// and moves it with the aircraft until it has driven off.
func (c *TaxiController) updateTug(dt float64) {
	t := c.req.Tug
	if t == nil {
		return
	}
	if t.Done() {
		c.give(VehicleTug) // driven off (or given up): free for the next
		return
	}
	stand := c.req.Graph.Layout.Parking[c.req.Parking]
	pose := GroundPose{Position: StandPoint(stand, c.req.NoseOffset), Heading: stand.Heading}
	if c.mover != nil {
		pose = c.mover.Pose()
	}
	if !c.tugAttached {
		if c.state != TaxiAwaitingPushback || c.facesOut() {
			return
		}
		// Sent TugLeadTime before the crew is due to ask for the push (its
		// departure time), or at once when the push is cleared: not ten
		// minutes early at the nose (live, EZY775 at C29).
		if !c.pushCleared && c.now().Before(c.gateAt.Add(-TugLeadTime)) {
			return
		}
		if !c.take(VehicleTug) {
			return // all the airport's tugs out: the push waits for one
		}
		c.tugAttached, c.tugAttachedAt = true, c.now()
		c.giveTraffic(t)
		if d, ok := t.(disconnectDelayer); ok {
			d.SetDisconnectDelay(TugDisconnectSeconds * f(c.timing.tug))
		}
		if err := t.Attach(pose); err != nil {
			c.tugErr(err)
			c.tugErr(t.Remove()) // none coming: the push goes on without it
		}
		return
	}
	c.tugErr(t.Update(pose, c.state == TaxiAwaitingPushback || c.state == TaxiPushback, dt))
}

// startDeicing starts the treatment: DefaultDeicingDwell (or the
// request's), varied by DwellJitter.
func (c *TaxiController) startDeicing(now time.Time) {
	d := c.req.Deice.Dwell
	if d <= 0 {
		d = DefaultDeicingDwell
	}
	c.deiceUntil = now.Add(time.Duration(float64(d) * (1 + DwellJitter*(2*c.rng.Float64()-1))))
	c.last.Deicing = true
	c.note("de-icing", nil)
	c.emit(nil, true)
}

// finishDeicing ends the treatment and sets the lights to go on with.
func (c *TaxiController) finishDeicing(l Lights) {
	c.deiced, c.last.Deicing = true, false
	c.setInjectedLights(l, "lights after de-icing")
	c.note("de-icing done", nil)
	c.emit(nil, true)
}

// tugConnected reports that the pushback tug is at the nose (or there is
// none): one driving in from its depot (SimObjectTug.Layout) is not yet.
func (c *TaxiController) tugConnected() bool {
	t := c.req.Tug
	if t == nil {
		return true
	}
	if !c.tugAttached {
		return false
	}
	if t.Done() {
		return true // given up, or gone
	}
	if a, ok := t.(interface{ Connected() bool }); ok && !a.Connected() {
		// Not there after its way in and a margin (ArriveWithin, at least
		// TugArriveTimeout: stuck on its way), or never
		// created after TugCreateTimeout: the push goes on without it.
		wait := TugArriveTimeout
		if w, ok := t.(interface{ ArriveWithin() time.Duration }); ok {
			wait = w.ArriveWithin()
		}
		if o, ok := t.(interface{ ObjectID() uint32 }); ok && o.ObjectID() == 0 {
			wait = TugCreateTimeout
		}
		if c.now().Sub(c.tugAttachedAt) < wait {
			return false
		}
		// Never created: once more a little further along its way in.
		if r, ok := t.(interface{ RetryCreate() bool }); ok && wait == TugCreateTimeout && r.RetryCreate() {
			c.tugAttachedAt = c.now()
			c.note("tug created again further along its way", nil)
			return false
		}
		c.tugErr(errors.New("the tug did not arrive: pushing without it"))
		c.tugErr(t.Remove())
	}
	return true
}

// tugClear reports that no pushback tug is at the aircraft any more: none
// was used, or it has driven off.
func (c *TaxiController) tugClear() bool {
	t := c.req.Tug
	if t == nil || !c.tugAttached || t.Done() {
		return true
	}
	// Off the aircraft, on its way home: no need to wait for the depot.
	if a, ok := t.(interface{ Clear() bool }); ok {
		return a.Clear()
	}
	return false
}

// tugErr reports a tug error as an event; the departure goes on without it.
func (c *TaxiController) tugErr(err error) {
	if err != nil {
		c.emit(fmt.Errorf("traffic: pushback tug: %w", err), true)
	}
}

// standInPlace gives an aircraft on a self-manoeuvring stand a stationary
// mover at its parked pose, which the taxi-out starts from.
func (c *TaxiController) standInPlace() error {
	g, prof, route := c.req.Graph, c.profile(), c.route
	stand := g.Layout.Parking[c.req.Parking]
	nose := NoseGear(StandPoint(stand, c.req.NoseOffset), stand.Heading, prof)
	path, err := NewGroundPath([]airport.LatLon{nose, offsetHeading(nose, stand.Heading, 10), route.Points[len(route.Points)-1]}, prof)
	if err != nil {
		return err
	}
	c.mover = NewGroundMoverFrom(path, prof, stand.Heading, 0)
	c.lastStep = c.now()
	return nil
}

// startPushback builds the push path: the main gear from the stand back to
// the taxiway junction and on along the taxiway, away from the taxi
// direction, so the aircraft ends up facing the way it will taxi.
func (c *TaxiController) startPushback() error {
	// The stands around as they are now: a neighbour taken or freed since
	// the push was planned plans it again.
	if c.pushPose != nil && c.req.StandOccupied != nil && !slices.Equal(c.emptyStands(), c.emptyNear) {
		c.note("stands around changed: pushback planned again", nil)
		c.route = c.origRoute
		c.planPushback()
		c.track = newRouteTracker(c.route)
	}
	path, err := c.pushPath()
	if err != nil {
		return err
	}
	c.pushPlanned = nil
	c.mover = NewPushbackMover(path, c.pushProfile(), c.req.Graph.Layout.Parking[c.req.Parking].Heading)
	c.lastStep = c.now()
	return nil
}

// startTow starts the tow forward after the push: the nose gear along
// towPts at the tug's pace, from where the push stopped (pose).
func (c *TaxiController) startTow(pose GroundPose) error {
	path, err := NewSmoothPath(c.towPts, c.pushProfile())
	if err != nil {
		return err
	}
	c.mover = NewGroundMoverFrom(path, c.pushProfile(), pose.Heading, 0)
	c.towing = true
	c.lastStep = c.now()
	c.note("pushed back: towing forward onto the taxiway", nil)
	return nil
}

// pushProfile is the aircraft's motion at pushback speed.
func (c *TaxiController) pushProfile() MotionProfile {
	push := c.profile()
	push.CruiseKts, push.MinTurnKts, push.Accel, push.Decel = c.aircraft().PushbackKts*f(c.timing.pushSpeed), 1, 0.15, 0.25
	return push
}

// pushPath is the path the main gear follows during the pushback (planned
// once, the first time it is needed).
func (c *TaxiController) pushPath() (*GroundPath, error) {
	if c.pushPlanned != nil {
		return c.pushPlanned, nil
	}
	g, prof, route := c.req.Graph, c.profile(), c.route
	stand := g.Layout.Parking[c.req.Parking]
	gear := offsetHeading(StandPoint(stand, c.req.NoseOffset), stand.Heading, -prof.RefAheadMeters)
	pts := []airport.LatLon{gear}
	if c.pushPose != nil {
		path, err := NewSmoothPath(c.pushPts, c.pushProfile())
		if err != nil {
			return nil, err
		}
		c.pushPlanned = path
		return path, nil
	}
	if len(route.Points) > 1 {
		var tail []airport.LatLon
		if len(route.Nodes) > c.pushJunction+1 {
			tail = c.behindJunction(localBearing(gear, route.Points[c.pushJunction]))
		}
		if c.pushPts != nil {
			pts = c.pushPts // up the alley to a later junction
		} else if turn := c.pushTurnPoints(gear); turn != nil {
			pts = turn // push and turn on the apron (#341)
		} else {
			pts = append(pts, pushPlan(g, c.req.Parking, gear, stand.Heading, route.Points[c.pushJunction], tail, prof)...)
		}
	}
	push := c.pushProfile()
	// pushPlan already shaped the arc; the fillet only rounds what is left.
	// Alley pushes and push-and-turns come smooth already.
	var path *GroundPath
	var err error
	if c.pushPts != nil || c.pushTurn {
		path, err = NewSmoothPath(pts, push)
	} else {
		path, err = NewArcPath(pts, push, PushbackMinArcMeters)
	}
	if err != nil {
		return nil, err
	}
	c.pushPlanned = path
	return path, nil
}

// pushCorridor is what the pushback still sweeps from from meters along
// the path: the main gear's path, and the tail beyond its end.
func pushCorridor(path *GroundPath, from float64, prof MotionProfile) []airport.LatLon {
	var pts []airport.LatLon
	end := path.Length()
	for s := math.Max(0, from); s <= end; s += trafficBodyStep {
		pts = append(pts, path.PointAt(s))
	}
	if end > 1 {
		a, b := path.PointAt(end-1), path.PointAt(end)
		h := localBearing(a, b)
		tail := prof.TailMeters
		if tail <= 0 {
			tail = 20.5
		}
		for d := trafficBodyStep; d <= tail; d += trafficBodyStep {
			pts = append(pts, offsetHeading(b, h, d))
		}
	}
	return pts
}

// pushBlocked reports traffic in the corridor of the pushback not started
// yet, and notes when the hold begins and ends.
func (c *TaxiController) pushBlocked(now time.Time) bool {
	if c.picture == nil || now.Sub(c.trafficAt) < TrafficCheckEvery {
		return c.last.PushbackHeld
	}
	c.trafficAt = now
	path, err := c.pushPath()
	if err != nil {
		return false
	}
	_, blocked := c.picture.corridorBlocked(c.objectID, append(pushCorridor(path, 0, c.profile()), c.towPts...), c.halfSpan(), true, now)
	c.setPushHeld(blocked)
	return blocked
}

// holdPushForTraffic stops a pushback under way while traffic is in what
// it still has to sweep, and lets it go on once clear.
func (c *TaxiController) holdPushForTraffic(now time.Time) {
	if c.picture == nil || c.mover == nil || now.Sub(c.trafficAt) < TrafficCheckEvery {
		return
	}
	c.trafficAt = now
	pose := c.mover.Pose()
	var rest []airport.LatLon
	if c.towing {
		for s := pose.Distance + 1; s <= c.mover.Path().Length(); s += trafficBodyStep {
			rest = append(rest, c.mover.Path().PointAt(s))
		}
	} else {
		rest = append(pushCorridor(c.mover.Path(), pose.Distance+1, c.profile()), c.towPts...)
	}
	// Under way the push has priority: taxiing traffic sees where it goes
	// and gives way; it stops only for an aircraft actually in the way.
	c.picture.ReportPush(c.objectID, rest, c.halfSpan())
	_, blocked := c.picture.corridorBlocked(c.objectID, rest, c.halfSpan(), false, now)
	if blocked {
		v := pose.GroundSpeedKts * ktsToMS
		c.mover.SetTrafficStop(pose.Distance + v*v/(2*0.25) + 0.2)
	} else {
		c.mover.ClearTrafficStop()
	}
	c.setPushHeld(blocked)
}

func (c *TaxiController) setPushHeld(held bool) {
	if held == c.last.PushbackHeld {
		return
	}
	c.last.PushbackHeld = held
	if held {
		c.note("pushback holding for traffic behind", nil)
	} else {
		c.note("pushback clear of traffic", nil)
	}
	c.emit(nil, true)
}

func (c *TaxiController) halfSpan() float64 {
	if h := c.profile().SpanMeters / 2; h > 0 {
		return h
	}
	return DefaultHalfSpanMeters
}

// behindJunction chooses where the tail goes after the stand: of the
// taxiway branches at the junction the push can swing onto (at most
// maxPushSwingDeg from the push direction), the one that leaves the nose
// pointing most nearly along the taxi route. It walks that taxiway for a
// pushWalkMeters, taking the straightest continuation at each node
// (segments can be a few meters long), and returns its centreline, which
// pushPlan fits the push to; none keeps the push straight (a dead-end stand).
// pushEdge reports whether a pushback may put the tail onto e: a taxiway,
// not a stand, runway or a path along a runway.
func pushEdge(g *airport.Graph, e airport.Edge) bool {
	return g.Nodes[e.To].Kind != airport.NodeParking && e.Type != types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING &&
		e.Type != types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY && !e.AlongRunway
}

// planPushback plans the push to a pose on a taxiway (planPushPose); where
// no pose is reachable, it chooses the taxiway branch the tail is pushed
// onto by where the aircraft can go from there: for every branch at the
// stand's junction
// the push can swing onto, the taxi-out is planned from the junction facing
// away from it (RouteToRunwayFrom); the cheapest wins and the route becomes
// stand → junction → that taxi-out. Without it the pushback guessed from
// the route planned from the stand, which at LKPR C17 went on straight
// ahead of the push and left the aircraft facing away from its route.
func (c *TaxiController) planPushback() {
	if c.origRoute == nil {
		c.origRoute = c.route
	}
	orig := c.route
	excl := map[pushChoice]bool{}
	for try := 0; try < pushPlanTries; try++ {
		c.route, c.pushJunction, c.pushPlanned = orig, 1, nil
		c.pushTurn, c.pushTurnDir, c.havePushBranch, c.pushBranch, c.pushPts, c.pushPose, c.towPts = false, 0, false, 0, nil, nil, nil
		if try == 0 && len(orig.Nodes) >= 3 && !c.facesOut() && c.planPushPose() {
			return
		}
		c.choosePushback(excl)
		if !c.havePushBranch {
			return // a push-and-turn, or straight back: nothing else to choose
		}
		if p, err := c.pushPath(); err == nil && pushPathFits(p) {
			return
		}
		// Tighter than a tug turns the aircraft: another push (EDDF B42: an
		// alley push ending in a 121° swing, a 2 m kink).
		excl[pushChoice{c.pushJunction, c.pushBranch}] = true
	}
	c.pushPlanned = nil
}

// pushChoice is a candidate push: onto branch at the route's junction at.
type pushChoice struct {
	at     int
	branch airport.NodeID
}

// pushPlanTries bounds how many pushes planPushback builds before it keeps
// the last.
const pushPlanTries = 6

// pushPathFits reports whether a tug can push along p: no turn tighter
// than PushbackMinArcMeters (with the 3 m of slack its test allows).
func pushPathFits(p *GroundPath) bool {
	if p.Length() <= 20 {
		return true
	}
	var pts []airport.LatLon
	for s := 0.0; s <= p.Length(); s += 2 {
		pts = append(pts, p.PointAt(s))
	}
	return tightestTurn(pts) >= PushbackMinArcMeters-3
}

// choosePushback is planPushback's choice, with the pushes in excl left out.
func (c *TaxiController) choosePushback(excl map[pushChoice]bool) {
	g, r := c.req.Graph, c.route
	if len(r.Nodes) < 3 || c.facesOut() {
		return
	}
	j := r.Nodes[1]
	jp := g.Nodes[j].Position
	stand := g.Layout.Parking[c.req.Parking]
	gear := offsetHeading(StandPoint(stand, c.req.NoseOffset), stand.Heading, -c.profile().RefAheadMeters)
	pushDir := localBearing(gear, jp)
	// Junctions the tail may swing at: the first, and on along the route
	// while it runs straight back behind the stand (LKPR B14: a connector
	// from the junction on JO, which a 777 does not fit, straight back to J).
	cands := []int{1}
	alley := map[int]bool{} // pushed along the route, not straight back
	for i, along := 2, localDist(gear, jp); i < len(r.Points)-1; i++ {
		along += localDist(r.Points[i-1], r.Points[i])
		seg := localBearing(r.Points[i-1], r.Points[i])
		// Up an alley: out of a dead end the tug pushes the aircraft along
		// the taxilane to the next taxiway (LKPR A7, B9), however the lane
		// bends, as long as each bend is gentle.
		if along > pushAlleyMeters || (i > 2 && math.Abs(headingDiff(localBearing(r.Points[i-2], r.Points[i-1]), seg)) > pushAlleyTurnDeg) {
			break
		}
		straight := along <= pushCorridorMeters && math.Abs(headingDiff(pushDir, seg)) <= pushCorridorDeg &&
			math.Abs(alongHeading(gear, pushDir+90, r.Points[i])) <= pushOffAxisMeters
		alley[i] = !straight
		cands = append(cands, i)
	}
	var best *airport.Route
	var bestPts []airport.LatLon
	branch, at, bestCost, bestBlocks, bestBad := airport.NodeID(-1), 1, math.Inf(1), 0, false
	// A wider swing only where no ordinary push leaves the taxiways clear.
	for pass, maxSwing := range []float64{maxPushSwingDeg, maxPushSwingWideDeg} {
		if pass == 1 && best != nil && bestBlocks == 0 {
			break
		}
		for _, i := range cands {
			k, kp := r.Nodes[i], r.Points[i]
			in := localBearing(r.Points[i-1], kp) // the push arriving at k
			pushed := localDist(gear, jp)
			for n := 2; n <= i; n++ {
				pushed += localDist(r.Points[n-1], r.Points[n])
			}
			for _, e := range g.Adj[k] {
				swing := math.Abs(headingDiff(in, localBearing(kp, g.Nodes[e.To].Position)))
				if excl[pushChoice{i, e.To}] || e.To == r.Nodes[i-1] || !pushEdge(g, e) || !g.Fits(e, c.req.Options) || swing > maxSwing || pass == 1 && swing <= maxPushSwingDeg {
					continue
				}
				// Straight on across a taxiway behind the stand leaves the nose
				// facing the stand it came from (LKPR A4: E190s pushed across B1
				// and faced back at the lead-in, a dead end): a tug swings the
				// tail onto the taxiway. Straight on along the lead-in (unnamed:
				// LKPR C17's goes on to J) and up an alley are pushes along it.
				if !alley[i] && swing < minPushSwingDeg && e.Name != "" {
					continue
				}
				// A custom route (Via, Taxiways) goes on from what the push passed.
				opts := g.RemainingOptions(c.req.Options, r.Nodes[:i+1])
				out, err := g.RouteToRunwayFrom(k, e.To, c.req.Runway, c.req.Entry, opts)
				if err != nil || len(out.Nodes) < 2 || out.Nodes[1] == e.To {
					continue // no way on, or only back over the branch it was pushed onto
				}
				// Pushing is slow: each meter costs pushCostFactor taxi meters; a
				// wide swing and every other taxiway left blocked cost more.
				blocks := c.pushBlocks(k, e)
				if alley[i] {
					blocks += c.alleyBlocks(i)
				}
				cost := out.Cost + pushed*pushCostFactor + float64(blocks)*pushBlockPenalty
				bad := false
				if hairpinAfterPush(out) {
					cost += pushHairpinPenalty
					bad = true
				}
				// Facing the way out: a push that leaves the nose off the first leg
				// of the taxi-out (a turn from a standstill) costs more than the
				// lanes it holds for a minute.
				if misalignedAfterPush(out, g.Nodes[e.To].Position) {
					cost += pushMisalignPenalty
					bad = true
				}
				if swing > maxPushSwingDeg {
					cost += pushWideSwingPenalty
				}
				if cost >= bestCost {
					continue
				}
				var pts []airport.LatLon
				if alley[i] {
					if pts = c.alleyPush(gear, i, e.To); pts == nil {
						continue // too tight or into the neighbours
					}
				}
				best, bestPts, branch, at, bestCost, bestBlocks, bestBad = out, pts, e.To, i, cost, blocks, bad
			}
		}
	}
	// No branch to push the tail onto (the only taxiway at the junction is
	// the way out: LKPR B9), or none that leaves the nose facing the way out
	// (LFPG M6-M14: the lane's alley too tight): push and turn on the apron
	// to face it, where one fits cleanly.
	if best == nil || bestBad && c.cleanPushTurn(gear, stand.Heading+180) {
		far, walked := r.Points[2], 0.0
		for i := 2; i < len(r.Points) && walked < pushRouteLookMeters; i++ {
			walked += localDist(r.Points[i-1], r.Points[i])
			far = r.Points[i]
		}
		c.pushTurn, c.pushTurnDir = true, localBearing(jp, far)
		return
	}
	full, err := g.RouteFromNodes(append(slices.Clone(r.Nodes[:at]), best.Nodes...))
	if err != nil {
		return
	}
	full.Runway, full.RunwayEnd, full.Entry, full.HoldShort = best.Runway, best.RunwayEnd, best.Entry, best.HoldShort
	c.route, c.pushBranch, c.havePushBranch, c.pushJunction, c.pushPts = full, branch, true, at, bestPts
}

// pushBlocks counts the junctions of other taxiways a push onto branch e
// at junction k would leave the aircraft on: its body where the push ends
// — along the lane from k the wheelbase and push tail, then its tail —
// and half its span either side. A junction of the lane itself (named by
// the first taxiway along it: an unnamed stub off the stand that becomes J
// is J), or of the stand's lead-in, is not another taxiway.
func (c *TaxiController) pushBlocks(k airport.NodeID, e airport.Edge) int {
	g, prof := c.req.Graph, c.profile()
	tail := prof.TailMeters
	if tail <= 0 {
		tail = 20.5
	}
	body := walkTaxiway(g, k, e.To, prof.WheelbaseMeters+PushTailMeters+tail)
	reach := c.halfSpan() + 5
	lane := laneName(g, k, e)
	n := 0
	for id, nd := range g.Nodes {
		if nd.Kind == airport.NodeParking || airport.NodeID(id) == k || len(g.Adj[id]) < 3 {
			continue
		}
		other := false
		for _, a := range g.Adj[id] {
			if a.Name != "" && a.Name != lane && g.Nodes[a.To].Kind != airport.NodeParking {
				other = true
			}
		}
		if !other {
			continue
		}
		for _, p := range body {
			if localDist(p, nd.Position) <= reach {
				n++
				break
			}
		}
	}
	return n
}

// plannedTaxi is the way the departure will taxi from pos: along its route
// from the route point nearest to it, sampled every trafficBodyStep up to
// GiveWayLookMeters.
func (c *TaxiController) plannedTaxi(pos airport.LatLon) []airport.LatLon {
	if c.route == nil || len(c.route.Points) < 2 {
		return nil
	}
	pts := c.route.Points
	near := 1
	for i := 1; i < len(pts); i++ {
		if localDist(pts[i], pos) < localDist(pts[near], pos) {
			near = i
		}
	}
	line := append([]airport.LatLon{pos}, pts[near:]...)
	var out []airport.LatLon
	walked := 0.0
	for i := 1; i < len(line) && walked < GiveWayLookMeters; i++ {
		seg := localDist(line[i-1], line[i])
		h := localBearing(line[i-1], line[i])
		for f := trafficBodyStep; f <= seg && walked+f <= GiveWayLookMeters; f += trafficBodyStep {
			out = append(out, offsetHeading(line[i-1], h, f))
		}
		walked += seg
	}
	return out
}

// alleyBlocks counts the junctions of other taxiways a push up the route to
// its point i passes (LKPR A3, live: an A321 pushed 190 m up A1 and along
// Z, across the taxi-out of the aircraft pushed before it, and the two met
// head on): the push holds each of them while it passes, far longer than
// a push ending on one. A dead-end alley passes none.
func (c *TaxiController) alleyBlocks(i int) int {
	g, r := c.req.Graph, c.route
	n := 0
	for j := 1; j <= i && j+1 < len(r.Nodes); j++ {
		id := r.Nodes[j]
		if len(g.Adj[id]) < 3 {
			continue
		}
		// A crossroads of lanes passed or ended on, whatever the branches
		// are called (#489: LKPR A5 for 24 was pushed 134 m west along the B1
		// lanes into their crossroads, and waited there for its taxi).
		if crossroads(g, id) {
			n++
			continue
		}
		if j == i {
			break // the end: pushBlocks counts the taxiways there
		}
		along := map[string]bool{}
		for _, e := range g.Adj[id] {
			if e.To == r.Nodes[j-1] || e.To == r.Nodes[j+1] {
				along[e.Name] = true
			}
		}
		for _, e := range g.Adj[id] {
			if e.Name != "" && !along[e.Name] && g.Nodes[e.To].Kind != airport.NodeParking {
				n++
				break
			}
		}
	}
	return n
}

// crossroads reports a node where three or more taxiway branches meet (the
// ways to stands and runways aside): traffic crosses there whatever the
// branches are called.
func crossroads(g *airport.Graph, id airport.NodeID) bool {
	n := 0
	for _, e := range g.Adj[id] {
		if pushEdge(g, e) && leadsOn(g, id, e.To) {
			n++
		}
	}
	return n >= 3
}

// leadsOn reports whether the branch from over to leads somewhere: to
// another junction or on for crossroadsBranchMeters; a lead-in that ends at
// a stand does not.
func leadsOn(g *airport.Graph, from, to airport.NodeID) bool {
	prev, cur, walked := from, to, 0.0
	for step := 0; step < 20; step++ {
		walked += localDist(g.Nodes[prev].Position, g.Nodes[cur].Position)
		if walked > crossroadsBranchMeters {
			return true
		}
		var next []airport.NodeID
		for _, e := range g.Adj[cur] {
			if e.To != prev && pushEdge(g, e) {
				next = append(next, e.To)
			}
		}
		switch len(next) {
		case 0:
			return false // a dead end: a stand's lead-in
		case 1:
			prev, cur = cur, next[0]
		default:
			return true // another junction
		}
	}
	return true
}

// crossroadsBranchMeters: a branch this long is a way on, not a stub.
const crossroadsBranchMeters = 80.0

// cleanPushTurn reports whether a push-and-turn fits the stand cleanly: a
// tug can turn it (pushPathFits' radius) and it keeps clear of the
// neighbours — not the least-bad one pushTurnPlan falls back to.
func (c *TaxiController) cleanPushTurn(gear airport.LatLon, pushDir float64) bool {
	g, prof := c.req.Graph, c.profile()
	pts := pushTurnPlan(g, c.req.Parking, gear, pushDir, c.route.Points[1:], prof)
	if pts == nil {
		return false
	}
	base := standIntrusion(g, c.req.Parking, []airport.LatLon{offsetHeading(gear, pushDir+180, 1), gear}, prof)
	return tightestTurn(pts) >= PushbackMinArcMeters-3 && standIntrusion(g, c.req.Parking, pts, prof) <= base+pushClearanceSlackMeters
}

// misalignedAfterPush reports a push that leaves the nose off the first
// leg of the taxi-out: the nose points from the branch it was pushed onto
// (from) at the junction, and the taxi-out's first pushMisalignMeters turn
// more than pushMisalignDeg off that line — a turn from a standstill right
// at the end of the push, where the tug should have swung the tail (LKPR
// B14, B15; LFPG, KJFK).
func misalignedAfterPush(out *airport.Route, from airport.LatLon) bool {
	if len(out.Points) < 2 {
		return false
	}
	k := out.Points[0]
	ahead, walked := out.Points[len(out.Points)-1], 0.0
	for i := 1; i < len(out.Points); i++ {
		walked += localDist(out.Points[i-1], out.Points[i])
		if walked >= pushMisalignMeters {
			ahead = out.Points[i]
			break
		}
	}
	return math.Abs(headingDiff(localBearing(from, k), localBearing(k, ahead))) > pushMisalignDeg
}

// hairpinAfterPush reports a taxi-out that turns back sharply (at least
// pushHairpinDeg) within pushHairpinMeters of the push: the push left the
// nose facing away from the way out (LKPR A5, live: KLM594 pushed onto B1
// facing south-east, taxied 80 m and turned 127° back onto B2).
func hairpinAfterPush(out *airport.Route) bool {
	pts := out.Points
	walked := 0.0
	for i := 1; i+1 < len(pts) && walked < pushHairpinMeters; i++ {
		walked += localDist(pts[i-1], pts[i])
		if localDist(pts[i-1], pts[i]) < 0.5 || localDist(pts[i], pts[i+1]) < 0.5 {
			continue
		}
		if math.Abs(headingDiff(localBearing(pts[i-1], pts[i]), localBearing(pts[i], pts[i+1]))) >= pushHairpinDeg {
			return true
		}
	}
	return false
}

// laneName is the taxiway a push onto branch e at k goes along: e's name,
// or where e is unnamed, the first name going on straightest from it.
func laneName(g *airport.Graph, k airport.NodeID, e airport.Edge) string {
	prev, cur, name := k, e.To, e.Name
	for step := 0; name == "" && step < 8; step++ {
		in := localBearing(g.Nodes[prev].Position, g.Nodes[cur].Position)
		var next *airport.Edge
		best := math.Inf(1)
		for i, a := range g.Adj[cur] {
			if a.To == prev || g.Nodes[a.To].Kind == airport.NodeParking {
				continue
			}
			if d := math.Abs(headingDiff(in, localBearing(g.Nodes[cur].Position, g.Nodes[a.To].Position))); d < best {
				best, next = d, &g.Adj[cur][i]
			}
		}
		if next == nil {
			break
		}
		prev, cur, name = cur, next.To, next.Name
	}
	return name
}

// alleyPush is the main gear path of a push up an alley: straight back off
// the stand, back along the route to r.Points[i], then onto the branch
// starting at node to, rounded with arcs (NewArcPath); nil if it turns
// tighter than PushbackMinArcMeters or swings into a neighbouring stand or
// the terminal.
func (c *TaxiController) alleyPush(gear airport.LatLon, i int, to airport.NodeID) []airport.LatLon {
	g, r, prof := c.req.Graph, c.route, c.profile()
	stand := g.Layout.Parking[c.req.Parking]
	pushDir := stand.Heading + 180
	tail := walkTaxiway(g, r.Nodes[i], to, prof.WheelbaseMeters+PushTailMeters)
	if len(tail) == 0 {
		return nil
	}
	// The lane's general line: a tug does not trace metre-long wiggles of
	// the centreline.
	lane := simplifyLine(append(slices.Clone(r.Points[1:i+1]), tail...), pushAlleySimplifyMeters)
	base := standIntrusion(g, c.req.Parking, []airport.LatLon{offsetHeading(gear, stand.Heading, 1), gear}, prof)
	pv := pavementAround(g, gear, pushAlleyMeters)
	ok := func(pts []airport.LatLon) bool {
		return tightestTurn(pts) >= PushbackMinArcMeters-1 && offPavement(pv, pts) <= pushOffPavementMeters &&
			standIntrusion(g, c.req.Parking, pts, prof) <= base+pushClearanceSlackMeters
	}
	// Onto the lane: the widest turn from the stand onto one of its straight
	// stretches (bends right by the stand, LKPR B9, leave no room for arcs
	// cut into the short segments), then back along the lane and onto the
	// branch, rounded with arcs.
	cum := make([]float64, len(lane))
	for n := 1; n < len(lane); n++ {
		cum[n] = cum[n-1] + localDist(lane[n-1], lane[n])
	}
	for rad := PushbackArcMeters; rad >= PushbackMinArcMeters-0.01; rad -= 4 {
		for n := 1; n < len(lane); n++ {
			seg := localDist(lane[n-1], lane[n])
			if seg < pushAlleyStraightMeters || cum[n] > cum[len(cum)-1]-1 && n == len(lane)-1 {
				continue
			}
			dir := localBearing(lane[n-1], lane[n])
			for s := 10.0; s <= seg-5; s += 10 {
				p := offsetHeading(lane[n-1], dir, s)
				entry := dubins(offsetHeading(gear, pushDir, PushStraightMeters), pushDir, p, dir, rad, 0.5)
				if entry == nil {
					continue
				}
				rest, err := NewArcPath(append([]airport.LatLon{p}, lane[n:]...), prof, PushbackArcMeters)
				if err != nil {
					continue
				}
				pts := append(append([]airport.LatLon{gear}, entry...), rest.Points()[1:]...)
				if ok(pts) {
					return pts
				}
			}
			break // only the first straight stretch
		}
	}
	return nil
}

// tightestTurn is the smallest turn radius along a polyline, from the
// heading change over about 4 m (+Inf when straight).
func tightestTurn(pts []airport.LatLon) float64 {
	r := math.Inf(1)
	for i := 1; i+1 < len(pts); i++ {
		j := i
		for j+1 < len(pts) && localDist(pts[i], pts[j]) < 4 {
			j++
		}
		if turn := math.Abs(headingDiff(localBearing(pts[i-1], pts[i]), localBearing(pts[j-1], pts[j]))); turn > 1 {
			r = math.Min(r, localDist(pts[i], pts[j])/(turn*math.Pi/180))
		}
	}
	return r
}

func (c *TaxiController) behindJunction(pushDir float64) []airport.LatLon {
	g, route := c.req.Graph, c.route
	j := route.Nodes[c.pushJunction]
	jp := g.Nodes[j].Position
	// Where the route really goes from the junction: the next node can be a
	// short connector pointing elsewhere (LKPR C17: 34° to the next node,
	// the route heads 316°), which pushed the tail the wrong way.
	far, walked := route.Points[c.pushJunction+1], 0.0
	for i := c.pushJunction + 1; i < len(route.Points) && walked < pushRouteLookMeters; i++ {
		walked += localDist(route.Points[i-1], route.Points[i])
		far = route.Points[i]
	}
	taxiDir := localBearing(jp, far)
	usableEdge := func(e airport.Edge) bool { return pushEdge(g, e) }
	first, bestScore := airport.NodeID(-1), maxNoseOffRouteDeg
	if c.havePushBranch {
		first = c.pushBranch // the taxi-out was planned from it (planPushback)
	}
	for _, e := range g.Adj[j] {
		if first >= 0 && c.havePushBranch {
			break
		}
		if !usableEdge(e) {
			continue
		}
		tailDir := localBearing(jp, g.Nodes[e.To].Position)
		if math.Abs(headingDiff(pushDir, tailDir)) > maxPushSwingDeg {
			continue
		}
		// The nose ends up facing away from the tail.
		if score := math.Abs(headingDiff(tailDir+180, taxiDir)); score < bestScore {
			first, bestScore = e.To, score
		}
	}
	if first < 0 {
		return nil
	}
	return walkTaxiway(g, j, first, pushWalkMeters)
}

// walkTaxiway follows the taxiway from node from over first for up to
// meters, taking the straightest continuation (at most a 60° bend) at each
// node, and returns the points passed, ending exactly meters along.
func walkTaxiway(g *airport.Graph, from, first airport.NodeID, meters float64) []airport.LatLon {
	prev, cur := from, first
	left := meters
	var pts []airport.LatLon
	for left > 0 {
		pp, cp := g.Nodes[prev].Position, g.Nodes[cur].Position
		if d := localDist(pp, cp); d >= left {
			pts = append(pts, offsetHeading(pp, localBearing(pp, cp), left))
			break
		} else {
			pts = append(pts, cp)
			left -= d
		}
		in := localBearing(pp, cp)
		next, bestTurn := airport.NodeID(-1), 60.0 // at most a 60° bend
		for _, e := range g.Adj[cur] {
			if e.To == prev || !pushEdge(g, e) {
				continue
			}
			if turn := math.Abs(headingDiff(in, localBearing(cp, g.Nodes[e.To].Position))); turn < bestTurn {
				next, bestTurn = e.To, turn
			}
		}
		if next < 0 {
			break
		}
		prev, cur = cur, next
	}
	return pts
}

// Pushback geometry: the tail can swing at most maxPushSwingDeg off the
// straight push, and a branch leaving the nose more than maxNoseOffRouteDeg
// off the taxi route is not worth the swing. A wider swing, up to
// maxPushSwingWideDeg, costs pushWideSwingPenalty: taken where the others
// would leave the aircraft blocking other taxiways. Each junction of
// another taxiway the pushed aircraft would sit on costs pushBlockPenalty
// (LKPR A4: pushed onto the B1 lane north, it stood across H; south, in
// the alley, it blocks nothing).
const (
	maxPushSwingDeg      = 100.0
	maxPushSwingWideDeg  = 125.0
	minPushSwingDeg      = 45.0
	pushWideSwingPenalty = 150.0
	pushBlockPenalty     = 400.0
	// A taxi-out turning back by pushHairpinDeg or more within
	// pushHairpinMeters of the push costs pushHairpinPenalty: the push faced
	// the wrong way.
	pushHairpinDeg     = 110.0
	pushHairpinMeters  = 200.0
	pushHairpinPenalty = 1000.0
	// The taxi-out's first pushMisalignMeters more than pushMisalignDeg off
	// the pushed aircraft's nose cost pushMisalignPenalty: a turn from a
	// standstill is impossible, holding a lane for a minute is not.
	pushMisalignDeg     = 60.0
	pushMisalignMeters  = 20.0
	pushMisalignPenalty = 2000.0
	maxNoseOffRouteDeg  = 150.0
	// pushWalkMeters is how much taxiway behind the junction pushPlan may use;
	// pushLineToleranceMeters how far the taxiway may bend from its first
	// direction and still count as straight; pushClearanceSlackMeters how
	// much deeper than the parked aircraft the swing may reach into a
	// neighbouring stand.
	pushWalkMeters           = 120.0
	pushLineToleranceMeters  = 1.5
	pushClearanceSlackMeters = 1.0
	// pushOffAxisMeters: a junction further off the stand axis than this is
	// pushed to abeam, straight, when no arc fits.
	pushOffAxisMeters = 3.0
	// A push-and-turn (pushTurnPlan) stops at most pushTurnBackMeters short
	// of the junction and is at most pushTurnMaxMeters long.
	pushTurnBackMeters = 40.0
	pushTurnMaxMeters  = 160.0
	// A push-and-turn may end up to pushTurnPastMeters past the junction on
	// the taxi-out; ending short of it costs pushTurnShortPenalty meters.
	pushTurnPastMeters   = 100.0
	pushTurnShortPenalty = 40.0
	// The main gear of a push-and-turn stays within pushOffPavementMeters of
	// the pavement (stand circles, taxi path strips).
	pushOffPavementMeters = 3.0
	// pushOffPavementWideMeters: the tolerance tried when no push-and-turn
	// fits within pushOffPavementMeters.
	pushOffPavementWideMeters = 8.0
	// A pushback may continue straight back past the first junction to a
	// later one, while the route stays within pushCorridorDeg of the push
	// direction, up to pushCorridorMeters from the stand.
	pushCorridorMeters = 150.0
	pushCorridorDeg    = 25.0
	// Up an alley the push follows the route up to pushAlleyMeters with no
	// bend over pushAlleyTurnDeg; each meter pushed costs pushCostFactor
	// meters of taxiing (a push is slow).
	pushAlleyMeters         = 250.0
	pushAlleyTurnDeg        = 45.0
	pushCostFactor          = 3.0
	pushAlleySimplifyMeters = 2.0  // the lane's line, within this of its centreline
	pushAlleyStraightMeters = 25.0 // a straight stretch of the lane the push turns onto
	// A push keeps pushTerminalMarginMeters short of the terminal line ahead
	// of the gates' parked noses; the nose is pushNoseFactor wheelbases ahead
	// of the main gear.
	pushTerminalMarginMeters = 5.0
	pushNoseFactor           = 1.35
	pushTerminalDepthMeters  = 40.0 // how deep the terminal zone reaches
	// pushRouteLookMeters is how far along the route from the junction its
	// direction is judged, to pick the side the tail goes.
	pushRouteLookMeters = 40.0
)

// startTaxiOut builds the taxi path from the nose gear to the hold-short
// of the departure runway, with holds short of runway crossings.
func (c *TaxiController) startTaxiOut() error {
	// A runway changed since the push (ChangeRunway): the route from here.
	if c.reroute {
		c.reroute = false
		if err := c.routeFromHere(); err != nil {
			return err
		}
	}
	prof, route := c.profile(), c.route
	pose := c.mover.Pose()
	nose := NoseGear(pose.Position, pose.Heading, prof)
	pts := []airport.LatLon{nose}
	var holds []holdOnPath
	// Start at the first of the next few route points ahead of the nose; after
	// a straight push from a dead-end stand none is, and the path starts with
	// a sharp turn onto the taxiway (after the junction).
	start := min(c.pushJunction+1, len(route.Points)-1)
	for i := c.pushJunction; i < min(c.pushJunction+3, len(route.Points)); i++ {
		if alongHeading(nose, pose.Heading, route.Points[i]) > 1 {
			start = i
			break
		}
	}
	// A push-and-turn ends on the taxi-out itself, up to pushTurnPastMeters
	// past the junction: start on from the route segment nearest the nose,
	// not from a point behind it (LKPR B9, EDDF: a first leg backwards).
	if c.pushTurn || c.pushPose != nil || c.fromHere {
		best := math.Inf(1)
		for i := c.pushJunction + 1; i < len(route.Points); i++ {
			a, b := route.Points[i-1], route.Points[i]
			if localDist(nose, a) > pushTurnPastMeters+pushTurnMaxMeters {
				break
			}
			h := localBearing(a, b)
			along := math.Max(0, math.Min(localDist(a, b), alongHeading(a, h, nose)))
			if d := localDist(nose, offsetHeading(a, h, along)); d < best && alongHeading(nose, pose.Heading, b) > 1 {
				best, start = d, i
			}
		}
	}
	// Out under its own power: the loop round to the side and back past the
	// stand, then the route from its first junction, behind the stand.
	if len(c.powerOut) > 0 && !c.fromHere {
		pts = append(pts, c.powerOut...)
		start = c.pushJunction
		c.powerOut = nil
	}
	apron := apronSpans{g: c.req.Graph}
	for i := start; i < len(route.Points); i++ {
		d := pathLen(pts) + localDist(pts[len(pts)-1], route.Points[i])
		apron.add(route.Nodes[i], d)
		if hs := c.req.Graph.Nodes[route.Nodes[i]].HoldShort; hs != nil && hs.Runway != c.runway.Index {
			holds = append(holds, holdOnPath{runway: hs.Runway, index: i, dist: d})
		}
		pts = append(pts, route.Points[i])
	}
	c.crossZones = crossingZones(c.req.Graph, route.Points, holds)
	// End the path HoldShortStopMeters before the hold-short line, so the
	// stop there cannot be lost when crossing holds are cleared.
	if n := len(pts); n >= 2 {
		a, b := pts[n-2], pts[n-1]
		if l := localDist(a, b); l > HoldShortStopMeters+1 {
			pts[n-1] = offsetHeading(b, localBearing(b, a), HoldShortStopMeters)
		}
	}
	path, err := NewGroundPath(pts, prof)
	if err != nil {
		return err
	}
	apron.limit(path, c.req.Airport, prof.Decel)
	// Start where the aircraft is (not a wheelbase along the path), at the
	// speed it has: 0 from a stop, its taxi speed on a re-plan.
	c.mover = NewGroundMoverFrom(path, prof, pose.Heading, pose.GroundSpeedKts)
	c.holdNextCrossing()
	if c.padNode >= 0 && !c.deiced {
		// Stop on the de-icing pad, the nose gear on its node.
		at, off := path.DistanceTo(c.req.Graph.Nodes[c.padNode].Position)
		if off >= 15 {
			// Not on the way any more (passed during the pushback): de-iced
			// here before taxiing, never skipped.
			at = 0
			c.note("de-icing pad not on the taxi path: de-icing here", nil)
		}
		c.padStop, c.hasPad = at, true
		c.updateHold()
	}
	if c.hasPendingLimit {
		c.hasPendingLimit = false
		if err := c.applyPendingLimit(c.pendingLimit); err != nil {
			c.note("clearance limit", err)
			c.emit(err, true) // #337: the caller learns the limit was not applied
		}
	}
	c.lastStep = c.now()
	return nil
}

// startLineUp builds the path onto the runway: from the nose gear to the
// centreline abeam the hold-short, aligned LineUpAlignMeters down the
// runway and on to its end; it stops aligned unless cleared for take-off.
func (c *TaxiController) startLineUp() {
	prof := c.profile()
	pose := c.mover.Pose()
	nose := NoseGear(pose.Position, pose.Heading, prof)
	thr, hdg := c.end.Threshold, c.end.Heading
	// Onto the runway along the entry taxiway's own geometry (entries are
	// often angled, not 90°), then aligned down the centreline.
	pts := []airport.LatLon{nose}
	entry := c.entryPath()
	pts = append(pts, entry...)
	onRunway := nose
	if len(entry) > 0 {
		onRunway = entry[len(entry)-1]
	}
	along := math.Max(0, alongHeading(thr, hdg, onRunway))
	if len(entry) == 0 {
		pts = append(pts, offsetHeading(thr, hdg, along)) // abeam, straight across
	}
	align := offsetHeading(thr, hdg, along+LineUpAlignMeters)
	far := offsetHeading(thr, hdg, math.Max(along+LineUpAlignMeters+50, c.runwayLength))
	pts = append(pts, align, far)
	path, err := NewGroundPath(pts, prof)
	if err != nil {
		c.fail(err)
		return
	}
	// Taxi speed through the entry, LineUpSpeedKts over the alignment; a
	// rolling take-off (cleared already) aligns briskly and rolls on.
	c.alignDist = pathLen(pts[:len(pts)-2]) + LineUpAlignMeters // on the runway, then aligned
	alignKts := LineUpSpeedKts
	if c.takeoffCleared {
		alignKts = LineUpRollingKts
	}
	path.LimitRange(c.alignDist-LineUpAlignMeters, path.Length(), alignKts, prof.Decel)
	c.mover = NewGroundMoverFrom(path, prof, pose.Heading, pose.GroundSpeedKts) // rolling on, or from a stop
	if !c.takeoffCleared {
		c.mover.HoldAt(c.alignDist)
	}
	c.lastStep = c.now()
	c.ignoreRunway = c.runway.Index // the phase lights cover the runway now
	c.last.HoldingShortOf = ""
	c.setInjectedLights(lightsLineUp, "lights line-up (strobes)")
	c.setState(TaxiLiningUp, nil)
}

// PushTurnRadiusCost is what a meter of turn radius below
// PushbackArcMeters is worth in meters of push, choosing a push-and-turn
// (a variable: the review renders compare weights, tools/push-review).
//
// A push to a pose is planned with PushWideRadiusCost (wide turns started
// early, the aircraft ending aligned on the taxiway) unless that makes it
// more than PushWideMaxExtraMeters longer than with PushTurnRadiusCost.
// Reviewed 2026-10-02 at the ten test airports: 3 was better everywhere
// but where it chose a much farther pose (LKPR C21, LFPG B2 and F14, KJFK
// A10: 33–59 m longer). To go back to the round-7 pushes (accepted
// 2026-10-01), set PushWideRadiusCost = PushTurnRadiusCost.
var (
	PushTurnRadiusCost     = 1.5
	PushWideRadiusCost     = 3.0
	PushWideMaxExtraMeters = 25.0
)

// LineUpRollingKts is the alignment's speed for a rolling take-off
// (cleared before lining up): onto the centreline and straight on.
var LineUpRollingKts = 12.0

// rollOnMeters: cleared for take-off while taxiing, the aircraft turns
// onto the runway this far before its holding point instead of stopping.
const rollOnMeters = 40.0

// startTakeoff hands over from the ground mover to the take-off.
func (c *TaxiController) startTakeoff() {
	pose := c.mover.Pose()
	c.takeoff = NewTakeoffMover(pose.Position, c.end.Heading, pose.GroundSpeedKts, c.takeoffProfile())
	c.mover = nil
	c.lastStep = c.now()
	c.seq.begin(c.lastStep)
	c.seq.add(c.lastStep, "take-off roll, landing lights on", 0, pose.GroundSpeedKts)
	c.takeoffPhase, c.flapsUpNoted = TakeoffRoll, false
	c.setInjectedLights(lightsTakeoff, "lights take-off (landing)")
	c.setState(TaxiDeparting, nil)
}

// onTakeoffFrame flies the take-off one frame: gear up with a positive
// climb, then hands the aircraft to MSFS AI for the climb-out.
func (c *TaxiController) onTakeoffFrame() {
	now := c.now()
	dt := math.Max(0, math.Min(now.Sub(c.lastStep).Seconds(), MaxFrameStepSeconds))
	c.lastStep = now
	pose := c.takeoff.Step(dt)
	ap := pose.ApproachPose()
	ap.RunwayFt = c.runway.Altitude / 0.3048 // the climb over any terrain
	if err := c.inj.PlaceAir(c.objectID, ap); err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	if c.takeoff.Rejected() {
		c.last.Position, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pose.Position, pose.Heading, pose.GroundSpeedKts, true
		if c.takeoff.Stopped() {
			if err := c.vacateAfterReject(pose); err != nil {
				c.fail(err)
			}
			return
		}
		c.emit(nil, false)
		return
	}
	c.last.Position, c.last.Heading, c.last.GroundSpeed = pose.Position, pose.Heading, pose.GroundSpeedKts
	c.last.OnGround, c.last.HeightFt = pose.Phase != TakeoffAirborne, pose.HeightFt
	if pose.Phase != c.takeoffPhase {
		c.takeoffPhase = pose.Phase
		switch pose.Phase {
		case TakeoffRotate:
			c.seq.add(now, "rotate", pose.HeightFt, pose.GroundSpeedKts)
		case TakeoffAirborne:
			c.seq.add(now, "lift-off", pose.HeightFt, pose.GroundSpeedKts)
		}
	}
	// Past the acceleration altitude the flaps follow the speed schedule:
	// all out at the climb speed, up at the clean speed.
	if fl, tp := c.aircraft().Flaps, c.takeoffProfile(); pose.HeightFt > fl.RetractFt && c.flaps.target > 0 {
		to := fl.TakeoffPct * tp.FlapsShare(pose.GroundSpeedKts)
		if to < c.flaps.target {
			if c.flaps.target == fl.TakeoffPct {
				c.seq.add(now, "acceleration: flaps retracting", pose.HeightFt, pose.GroundSpeedKts)
			}
			c.flaps.target, c.flaps.rate = to, fl.TakeoffPct/(FlapsRetractClimbSeconds*f(c.timing.flaps))
		}
	}
	if !c.flapsUpNoted && c.takeoffPhase == TakeoffAirborne && c.flaps.target == 0 && c.flaps.pct == 0 {
		c.flapsUpNoted = true
		c.seq.add(now, "flaps up", pose.HeightFt, pose.GroundSpeedKts)
	}
	// Placed in the air the first time, the simulator sets the aircraft up
	// clean: live, the gear was up at the first airborne frame, 7–12 ft above
	// the runway, whatever the gear-up height (CSA1802, EZY1957, TVS1175,
	// 2026-10-03). Until the crew raises it, the gear is put down again:
	// every frame just after lift-off, then every GearHoldEvery, the gear
	// itself as well as the handle (HoldGearDown), so it is not seen to move.
	if pose.Phase == TakeoffAirborne && !c.gearUp &&
		(pose.AirborneSeconds < GearHoldFirstSeconds || now.Sub(c.gearDownSent) >= GearHoldEvery) {
		c.gearDownSent = now
		_ = c.inj.HoldGearDown(c.objectID) // not noted: every frame for a while
	}
	if c.gearUpAt == 0 {
		c.gearUpAt = GearUpFt + c.rng.Float64()*(GearUpMaxFt-GearUpFt)
	}
	if !c.gearUp && pose.HeightFt > c.gearUpAt && pose.AirborneSeconds >= GearUpDelaySeconds*f(c.timing.gearUp) && pose.VerticalFpm >= GearUpFpm { // positive climb
		c.gearUp = true
		c.takeoff.GearUp() // the pitch settles from the climb pitch
		c.note("gear up", c.inj.SetGear(c.objectID, false))
		c.setInjectedLights(lightsClimb, "lights taxi off (gear up)")
		c.seq.add(now, "gear up, taxi light off", pose.HeightFt, pose.GroundSpeedKts)
	}
	// To MSFS AI clean: above the hand-over height with the flaps up (or
	// well above it, whatever the speed).
	// A VFR departure goes at VFRHandoverFt whatever its flaps: a light
	// single climbs below its clean speed, and waited for the margin (live,
	// OKVFD handed over at 2900 ft after 4.5 minutes).
	if pose.HeightFt >= c.handoverFt() && (c.req.VFR || c.flaps.pct == 0 || pose.HeightFt >= c.handoverFt()+HandoverCleanMarginFt) {
		c.seq.add(now, "hand-over to MSFS AI", pose.HeightFt, pose.GroundSpeedKts)
		c.handOverClimb(pose)
		return
	}
	c.emit(nil, false)
}

// handOverClimb releases the aircraft to MSFS AI with climb waypoints.
func (c *TaxiController) handOverClimb(pose TakeoffPose) {
	if !c.gearUp { // handed over below the crew's gear-up height: up now
		c.gearUp = true
		c.note("gear up", c.inj.SetGear(c.objectID, false))
	}
	c.note("flaps up", c.inj.SetFlaps(c.objectID, 0)) // clean for MSFS AI
	c.note("release", c.inj.Release(c.objectID))
	wps := TakeoffClimb(pose.Position.Lat, pose.Position.Lon, pose.Heading)
	if c.req.VFR && len(c.req.Departure) > 0 {
		wps = VFRDepartureWaypoints(pose.Position, pose.Heading, c.req.Departure, MaxBankDeg(*c.aircraft()))
	} else if len(c.req.Departure) > 0 {
		alt := convert.MetersToFeet(c.req.Graph.Layout.Altitude) + pose.HeightFt
		wps = DepartureWaypoints(pose.Position, pose.Heading, alt, c.req.Departure)
		// Its corners are the aircraft's turns (roundCorners).
		here := types.SIMCONNECT_DATA_WAYPOINT{Latitude: pose.Position.Lat, Longitude: pose.Position.Lon, KtsSpeed: ProcedureSpeedKts}
		wps = roundCorners(append([]types.SIMCONNECT_DATA_WAYPOINT{here}, wps...), MaxBankDeg(*c.aircraft()))[1:]
	}
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+defOffWaypoints, wps); err != nil {
		c.emit(err, true)
	}
	c.climb = wps
	c.stopMonitor()
	c.setState(TaxiComplete, nil)
}

// ClearPushback clears an injected departure to push back.
// enginesReady reports that the engines are started and running.
func (c *TaxiController) enginesReady(now time.Time) bool {
	return c.enginesOn && !now.Before(c.enginesReadyAt)
}

// ClearStartUp approves the start-up (with HoldForClearances): once the
// pushback tug has disconnected, the engines start, EngineStartTime each,
// before the taxi. Given with the pushback, it is used once the tug has
// gone. A taxi clearance given without it covers the start-up too.
func (c *TaxiController) ClearStartUp() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startUpCleared = true
	if c.facesOut() {
		c.pushCleared = true // no pushback: off the stand under its own power
	}
}

// FacesOut reports a stand the aircraft taxis straight out of, without a
// pushback: its crew asks for start-up, then taxi.
func (c *TaxiController) FacesOut() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.facesOut()
}

func (c *TaxiController) ClearPushback() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pushCleared = true
}

// ClearPushbackFacing clears the pushback to end facing a compass
// direction ("north", "east", "south", "west", or "n", "e", "s", "w"):
// the push is planned again among those ending within 45° of it, if any
// does. Once the push has begun it is ErrTooLate.
func (c *TaxiController) ClearPushbackFacing(dir string) error {
	deg, ok := CompassHeading(dir)
	if !ok {
		return fmt.Errorf("facing %q: north, east, south or west", dir)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state > TaxiAwaitingPushback {
		return ErrTooLate
	}
	c.pushFacing, c.havePushFacing = deg, true
	if c.pushPose != nil || c.pushPlanned != nil {
		c.route = c.origRoute
		c.planPushback()
		c.track = newRouteTracker(c.route)
		c.note("pushback facing "+CompassName(deg), nil)
	}
	c.pushCleared = true
	c.emit(nil, true)
	return nil
}

// PushFacing is the compass direction the planned push ends facing
// ("east"); "" without a planned push to a pose.
func (c *TaxiController) PushFacing() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pushPose == nil {
		return ""
	}
	h := c.pushPose.heading
	if len(c.towPts) > 1 {
		n := len(c.towPts)
		h = localBearing(c.towPts[n-2], c.towPts[n-1])
	}
	return CompassName(h)
}

// PushFacingSaid is PushFacing as told to the crew: the nearest of eight
// points ("south-east"), so a push ending at 143° is not told "facing
// south" (live, UAE375 at LKPR C22).
func (c *TaxiController) PushFacingSaid() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pushPose == nil {
		return ""
	}
	h := c.pushPose.heading
	if len(c.towPts) > 1 {
		n := len(c.towPts)
		h = localBearing(c.towPts[n-2], c.towPts[n-1])
	}
	return CompassName8(h)
}

// CompassName8 is the nearest of the eight compass points to heading:
// "north", "north-east", "east" … "north-west".
func CompassName8(heading float64) string {
	i := int(math.Mod(math.Mod(heading, 360)+360+22.5, 360) / 45)
	return [...]string{"north", "north-east", "east", "south-east", "south", "south-west", "west", "north-west"}[i%8]
}

// CompassHeading is the heading of a compass direction: "north" or "n" 0,
// "east" or "e" 90, "south" or "s" 180, "west" or "w" 270.
func CompassHeading(dir string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(dir)) {
	case "north", "n":
		return 0, true
	case "east", "e":
		return 90, true
	case "south", "s":
		return 180, true
	case "west", "w":
		return 270, true
	}
	return 0, false
}

// CompassName is the nearest of north, east, south and west to heading.
func CompassName(heading float64) string {
	i := int(math.Mod(math.Mod(heading, 360)+360+45, 360) / 90)
	return [...]string{"north", "east", "south", "west"}[i%4]
}

// HoldPushback keeps an injected departure on its stand (on) — a ground
// stop, e.g. by a traffic manager's situation check — until released
// (off). It stops a pushback not yet begun (beacon not on), even a
// cleared one; once the beacon is on the push goes ahead.
func (c *TaxiController) HoldPushback(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pushStopped = on
}

// ClearToTaxi clears an injected departure to taxi to the runway, without a
// limit (removing one given with ClearUpTo).
func (c *TaxiController) ClearToTaxi() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taxiCleared, c.hasPendingLimit = true, false
	if c.facesOut() {
		c.pushCleared = true
	}
	if c.state == TaxiTaxiing || c.state == TaxiHoldingShort {
		c.clearLimit()
	}
}

// ClearToLineUp clears an injected departure holding short of the
// departure runway to line up and wait.
func (c *TaxiController) ClearToLineUp() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lineUpCleared = true
}

// ClearToCross clears an injected departure to cross the runway it holds
// short of (or, given earlier, the next crossing ahead).
func (c *TaxiController) ClearToCross() {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case c.state == TaxiHoldingShort && c.holdingCrossing:
		c.crossingCleared()
		c.holdingCrossing = false
		c.last.HoldingShortOf = ""
		c.setState(TaxiTaxiing, nil)
	case c.mover != nil && c.state == TaxiTaxiing && c.nextCross < len(c.crossZones):
		c.crossingCleared()
	default:
		c.crossClears++
	}
}

// entryPath is the taxiway from the departure hold-short onto the runway
// centreline: to the entry's first node off the runway and along the
// entry's path to its runway node. Nil when no entry of the runway end
// starts at the hold-short.
func (c *TaxiController) entryPath() []airport.LatLon {
	g := c.req.Graph
	hold := c.route.Nodes[len(c.route.Nodes)-1]
	entries, err := g.RunwayEntries(c.end.Name)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.HoldShort != hold {
			continue
		}
		var pts []airport.LatLon
		if e.Node != hold {
			opts := c.req.Options
			opts.Via, opts.Taxiways = nil, nil // the custom route ends at the hold-short
			r, err := g.Route(hold, e.Node, opts)
			if err != nil {
				continue
			}
			pts = append(pts, r.Points[1:]...)
		}
		for _, id := range e.Path[1:] {
			pts = append(pts, g.Nodes[id].Position)
		}
		return pts
	}
	// No listed entry starts here (a turn past MaxEntryAngle, e.g. a
	// taxiway meeting the runway end square or slightly back): the taxi
	// path itself from the hold-short onto the runway.
	return c.entryPathByGraph(hold)
}

// entryPathByGraph is the taxi path from the hold-short node to the first
// node on the departure runway's centreline (breadth first along the taxi
// graph, at most EntrySearchMeters), so the line-up follows the painted
// lead-in; nil when none is found.
func (c *TaxiController) entryPathByGraph(hold airport.NodeID) []airport.LatLon {
	g := c.req.Graph
	a, b := c.runway.Primary.Threshold, c.runway.Secondary.Threshold
	onRunway := func(p airport.LatLon) bool {
		along := calc.AlongTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon)
		cross := math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon))
		return along > -30 && along < c.runway.Length+30 && cross < 6 // on the centreline
	}
	type step struct {
		node airport.NodeID
		dist float64
	}
	prev := map[airport.NodeID]airport.NodeID{hold: -1}
	queue := []step{{hold, 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.node != hold && onRunway(g.Nodes[cur.node].Position) {
			var rev []airport.LatLon
			for n := cur.node; n != hold; n = prev[n] {
				rev = append(rev, g.Nodes[n].Position)
			}
			slices.Reverse(rev)
			return rev
		}
		for _, e := range g.Adj[cur.node] {
			if _, seen := prev[e.To]; seen || cur.dist+e.Length > EntrySearchMeters {
				continue
			}
			prev[e.To] = cur.node
			queue = append(queue, step{e.To, cur.dist + e.Length})
		}
	}
	return nil
}

// EntrySearchMeters bounds the search for a runway entry from a hold-short.
const EntrySearchMeters = 400.0

// ClearUpTo clears an injected departure to taxi up to a node of its route
// and hold there (progressive taxi, #322): before the taxi starts it is the
// taxi clearance with a limit, while taxiing it moves the limit. ClearToTaxi
// removes the limit.
func (c *TaxiController) ClearUpTo(node airport.NodeID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil {
		return ErrNotInjected
	}
	if !slices.Contains(c.route.Nodes[1:], node) {
		return ErrNotOnRoute
	}
	switch c.state {
	case TaxiAwaitingPushback, TaxiPushback, TaxiAwaitingTaxi:
		c.pendingLimit, c.hasPendingLimit, c.taxiCleared = node, true, true
		return nil
	case TaxiTaxiing, TaxiHoldingShort:
		if c.mover == nil {
			return ErrNotOnRoute
		}
		return c.setLimit(node)
	}
	return ErrNotOnRoute
}

// pushTurnPoints is the push-and-turn of a stand whose only taxiway at the
// junction is the way out (planPushback), nil otherwise.
func (c *TaxiController) pushTurnPoints(gear airport.LatLon) []airport.LatLon {
	if !c.pushTurn {
		return nil
	}
	stand := c.req.Graph.Layout.Parking[c.req.Parking]
	return pushTurnPlan(c.req.Graph, c.req.Parking, gear, stand.Heading+180, c.route.Points[1:], c.profile())
}

// ClimbRoute is what a departure handed to MSFS AI still flies — its SID
// and the climb out — from pos (where it is now: the controller no longer
// follows it), for a map; nil before the hand-over.
func (c *TaxiController) ClimbRoute(pos airport.LatLon) []airport.LatLon {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.climb) == 0 {
		return nil
	}
	var out []airport.LatLon
	for _, w := range c.climb[nextWaypoint(pos, c.climb):] {
		out = append(out, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude})
	}
	return out
}

// ClimbPlan is the rest of a departure handed to MSFS AI as a route with
// its altitudes (feet MSL) and speeds, from pos: for a conflict resolution
// (ResolvedRoute, then Reroute; #639) and its prediction (#657). nil
// before the hand-over.
func (c *TaxiController) ClimbPlan(pos airport.LatLon) []RoutePoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.climb) == 0 || c.state != TaxiComplete {
		return nil
	}
	var out []RoutePoint
	for _, w := range c.climb[nextWaypoint(pos, c.climb):] {
		alt := w.Altitude
		if w.Flags&uint32(types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL) != 0 && c.req.Graph != nil {
			// TakeoffClimb: above the ground under the waypoint, taken as
			// above the field (#657): near enough for a prediction.
			alt += convert.MetersToFeet(c.req.Graph.Layout.Altitude)
		}
		out = append(out, RoutePoint{Position: airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}, AltFt: alt, Kts: w.KtsSpeed})
	}
	return out
}

// DirectTo sends a departure handed to MSFS AI from pos straight to fix, a
// point of its climb route ahead, and on along the route from there (its
// crew asked, #621). The fix keeps the altitude and speed planned for the
// climb waypoint nearest it.
func (c *TaxiController) DirectTo(pos airport.LatLon, altFt, kts float64, fix airport.LatLon) error {
	plan := c.ClimbPlan(pos)
	at, best := -1, math.Inf(1)
	for i, p := range plan {
		if d := calc.HaversineNM(fix.Lat, fix.Lon, p.Position.Lat, p.Position.Lon); d < best {
			at, best = i, d
		}
	}
	if at < 0 || best > 2 {
		return errors.New("traffic: direct: the fix is not on the climb route")
	}
	route := []RoutePoint{{Position: pos, AltFt: altFt, Kts: kts}, {Position: fix, AltFt: plan[at].AltFt, Kts: plan[at].Kts}}
	return c.Reroute(append(route, plan[at+1:]...))
}

// Reroute sends a departure handed to MSFS AI on route (from where it is
// now, as ResolvedRoute gives it), flown as waypoints like an en route
// flight; ClimbPlan and ClimbRoute follow the new route (#639).
func (c *TaxiController) Reroute(route []RoutePoint) error {
	_, wps, err := EnrouteStart(route)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != TaxiComplete || c.objectID == 0 {
		return errors.New("traffic: reroute: not handed over to MSFS AI")
	}
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+defOffWaypoints, wps); err != nil {
		return err
	}
	c.climb = wps
	return nil
}

// Power-out (TaxiRequest.PowerOut): a turn of powerOutRadius (the
// arrival's turn-around scaled to the wheelbase, at least
// PowerOutMinRadiusMeters); the loop may lie PowerOutOffPavementMeters off
// the pavement at most.
const (
	PowerOutMinRadiusMeters   = 8.0
	PowerOutOffPavementMeters = 1.0
)

// planPowerOut plans the loop out of the stand under the aircraft's own
// power: forward, round to the side and back past the stand towards the
// route's first junction behind it, on whichever side stays on the
// pavement (taxiways and its own stand) and clear of the stands StandOccupied
// reports taken. False when neither side fits.
func (c *TaxiController) planPowerOut() ([]airport.LatLon, bool) {
	if len(c.route.Points) < 2 || c.inj == nil {
		return nil, false
	}
	g, prof := c.req.Graph, c.profile()
	stand := g.Layout.Parking[c.req.Parking]
	h := stand.Heading
	nose := NoseGear(StandPoint(stand, c.req.NoseOffset), h, prof)
	r := math.Max(PowerOutMinRadiusMeters, TurnAroundMeters*prof.WheelbaseMeters/DefaultMotionProfile().WheelbaseMeters)
	all := pavementAround(g, nose, 4*r)
	own := pavement{segs: all.segs, stands: []airport.Parking{stand}}
	half := c.halfSpan()
	var best []airport.LatLon
	bestOff := math.Inf(1)
	for _, side := range []float64{1, -1} {
		at := func(u, v float64) airport.LatLon { return offsetHeading(offsetHeading(nose, h, u*r), h+90, v*r*side) }
		loop := []airport.LatLon{at(0.5, 0), at(1.4, 0.4), at(1.9, 1.2), at(1.6, 2.0), at(0.8, 2.3), at(0, 2.2)}
		off := offPavement(own, loop)
		if off > PowerOutOffPavementMeters || off >= bestOff {
			continue
		}
		clear := true
		for _, p := range g.Layout.Parking {
			if p.Index == stand.Index || c.req.StandOccupied == nil || !c.req.StandOccupied(p.Index) {
				continue
			}
			for _, q := range loop {
				if localDist(q, p.Position) < p.Radius+half {
					clear = false
				}
			}
		}
		if clear {
			best, bestOff = loop, off
		}
	}
	return best, best != nil
}
