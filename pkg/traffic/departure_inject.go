//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
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
	if c.req.Profile != (MotionProfile{}) {
		return c.req.Profile
	}
	return DefaultMotionProfile()
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

// openGate starts a gate's automatic wait of about d.
func (c *TaxiController) openGate(d time.Duration) {
	c.gateAt = c.now().Add(time.Duration(float64(d) * (1 + DwellJitter*(2*c.rng.Float64()-1))))
}

// startInjectedDeparture takes the aircraft over on the stand.
func (c *TaxiController) startInjectedDeparture() error {
	if err := c.inj.Takeover(c.objectID); err != nil {
		return err
	}
	c.injector, c.object, c.graph, c.prof = c.inj, c.objectID, c.req.Graph, c.profile()
	c.holdAtCrossings = c.req.HoldForClearances
	client := c.fleet.clientOrNil()
	if client == nil {
		return ErrNotConnected
	}
	if err := client.RequestDataOnSimObject(c.reqBase+reqOffMonitor, c.defBase+defOffMonitor, c.objectID,
		types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
		return err
	}
	c.fast = true
	c.openGate(PushbackDelay)
	c.setState(TaxiAwaitingPushback, nil)
	return nil
}

// onDepartureFrame runs the injected departure one sim frame.
func (c *TaxiController) onDepartureFrame(m taxiMonitor) {
	now := c.now()
	if !c.lightsSet {
		// Parked: nav lights, logo and wing as the aircraft has them.
		c.lightsSet = true
		c.lights = m.currentLights()
		c.setInjectedLights(LightsParked, "lights parked")
	}
	c.last.Lights = m.currentLights()
	switch c.state {
	case TaxiAwaitingPushback:
		if c.pushAt.IsZero() && c.gate(c.pushCleared) {
			// Beacon on, and the push starts BeaconLeadTime later.
			c.lights.Logo = true // as MSFS AI shows it; aircraft spawn with it off
			c.setInjectedLights(LightsPushback, "lights beacon (pushback)")
			c.pushAt = now.Add(BeaconLeadTime)
		}
		if !c.pushAt.IsZero() && !now.Before(c.pushAt) {
			if err := c.startPushback(); err != nil {
				c.fail(err)
				return
			}
			c.setState(TaxiPushback, nil)
		}
		c.emit(nil, false)
		return
	case TaxiAwaitingTaxi:
		if c.moveAt.IsZero() && c.gate(c.taxiCleared) {
			// Taxi light on, then release the brakes TaxiLightDelay later.
			c.setInjectedLights(LightsTaxi, "lights taxi")
			c.moveAt = now.Add(TaxiLightDelay)
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
		if c.gate(c.takeoffCleared) {
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
	pose, err := c.advance()
	if err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	c.last.Position, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pose.Position, pose.Heading, pose.GroundSpeedKts, true
	c.last.Remaining = math.Max(0, c.mover.Path().Length()-pose.Distance)
	if c.state != TaxiPushback {
		c.checkCrossing(pose)
	}
	switch c.state {
	case TaxiPushback:
		if pose.Arrived {
			c.openGate(TaxiAfterPushDelay)
			c.setState(TaxiAwaitingTaxi, nil)
			return
		}
	case TaxiTaxiing:
		if rwy, ok := c.atCrossingHold(pose); ok {
			c.holdingCrossing = true
			c.last.HoldingShortOf = rwy
			c.setState(TaxiHoldingShort, nil)
			return
		}
		if pose.Arrived {
			c.holdingCrossing = false
			c.last.HoldingShortOf = c.runway.Name()
			c.openGate(LineUpDelay)
			c.setState(TaxiHoldingShort, nil)
			return
		}
	case TaxiHoldingShort:
		if !c.holdingCrossing && c.gate(c.lineUpCleared || c.takeoffCleared) {
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

// startPushback builds the push path: the main gear from the stand back to
// the taxiway junction and on along the taxiway, away from the taxi
// direction, so the aircraft ends up facing the way it will taxi.
func (c *TaxiController) startPushback() error {
	g, prof, route := c.req.Graph, c.profile(), c.route
	stand := g.Layout.Parking[c.req.Parking]
	gear := offsetHeading(stand.Position, stand.Heading, -prof.RefAheadMeters)
	pts := []airport.LatLon{gear}
	if len(route.Points) > 1 {
		pts = append(pts, route.Points[1])
	}
	if len(route.Nodes) > 2 {
		pts = append(pts, c.behindJunction()...)
	}
	push := prof
	push.CruiseKts, push.MinTurnKts, push.Accel, push.Decel = PushbackSpeedKts, 1, 0.15, 0.25
	path, err := NewGroundPath(pts, push)
	if err != nil {
		return err
	}
	c.mover = NewPushbackMover(path, push, stand.Heading)
	c.lastStep = c.now()
	return nil
}

// behindJunction walks the taxiway from the junction away from the taxi
// direction for a wheelbase plus PushTailMeters, taking the straightest
// continuation at each node (taxiway segments can be a few meters long),
// and returns the points the tail is pushed through; none when no taxiway
// leads away from the taxi direction.
func (c *TaxiController) behindJunction() []airport.LatLon {
	g, route, prof := c.req.Graph, c.route, c.profile()
	prev, cur := route.Nodes[2], route.Nodes[1] // as if arriving from the taxi direction
	left := prof.WheelbaseMeters + PushTailMeters
	var pts []airport.LatLon
	for left > 0 {
		cp := g.Nodes[cur].Position
		in := localBearing(g.Nodes[prev].Position, cp)
		next, bestTurn := airport.NodeID(-1), 60.0 // at most a 60° bend
		for _, e := range g.Adj[cur] {
			if e.To == prev || g.Nodes[e.To].Kind == airport.NodeParking || e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING || e.AlongRunway {
				continue
			}
			if turn := math.Abs(headingDiff(in, localBearing(cp, g.Nodes[e.To].Position))); turn < bestTurn {
				next, bestTurn = e.To, turn
			}
		}
		if next < 0 {
			break
		}
		np := g.Nodes[next].Position
		if d := localDist(cp, np); d >= left {
			pts = append(pts, offsetHeading(cp, localBearing(cp, np), left))
			break
		} else {
			pts = append(pts, np)
			left -= d
		}
		prev, cur = cur, next
	}
	return pts
}

// startTaxiOut builds the taxi path from the nose gear to the hold-short
// of the departure runway, with holds short of runway crossings.
func (c *TaxiController) startTaxiOut() error {
	prof, route := c.profile(), c.route
	pose := c.mover.Pose()
	nose := NoseGear(pose.Position, pose.Heading, prof)
	pts := []airport.LatLon{nose}
	var holds []holdOnPath
	for i := 1; i < len(route.Points); i++ {
		if len(pts) == 1 && alongHeading(nose, pose.Heading, route.Points[i]) <= 1 {
			continue // behind the nose after the push
		}
		d := pathLen(pts) + localDist(pts[len(pts)-1], route.Points[i])
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
	// Start where the aircraft stands (not a wheelbase along the path).
	c.mover = NewGroundMoverFrom(path, prof, pose.Heading, 0)
	c.holdNextCrossing()
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
	// Taxi speed through the entry, LineUpSpeedKts over the alignment.
	c.alignDist = pathLen(pts[:len(pts)-2]) + LineUpAlignMeters // on the runway, then aligned
	path.LimitRange(c.alignDist-LineUpAlignMeters, path.Length(), LineUpSpeedKts, prof.Decel)
	c.mover = NewGroundMoverFrom(path, prof, pose.Heading, 0)
	if !c.takeoffCleared {
		c.mover.HoldAt(c.alignDist)
	}
	c.lastStep = c.now()
	c.ignoreRunway = c.runway.Index // the phase lights cover the runway now
	c.last.HoldingShortOf = ""
	c.setInjectedLights(lightsLineUp, "lights line-up (strobes)")
	c.setState(TaxiLiningUp, nil)
}

// startTakeoff hands over from the ground mover to the take-off.
func (c *TaxiController) startTakeoff() {
	pose := c.mover.Pose()
	c.takeoff = NewTakeoffMover(pose.Position, c.end.Heading, pose.GroundSpeedKts, c.takeoffProfile())
	c.mover = nil
	c.lastStep = c.now()
	c.setInjectedLights(lightsTakeoff, "lights take-off (landing)")
	c.setState(TaxiDeparting, nil)
}

// onTakeoffFrame flies the take-off one frame: gear up with a positive
// climb, then hands the aircraft to MSFS AI for the climb-out.
func (c *TaxiController) onTakeoffFrame() {
	now := c.now()
	dt := math.Max(0, math.Min(now.Sub(c.lastStep).Seconds(), 0.25))
	c.lastStep = now
	pose := c.takeoff.Step(dt)
	if err := c.inj.PlaceAir(c.objectID, pose.ApproachPose()); err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	c.last.Position, c.last.Heading, c.last.GroundSpeed = pose.Position, pose.Heading, pose.GroundSpeedKts
	c.last.OnGround, c.last.HeightFt = pose.Phase != TakeoffAirborne, pose.HeightFt
	if !c.gearUp && pose.HeightFt > GearUpFt {
		c.gearUp = true
		c.note("gear up", c.inj.SetGear(c.objectID, false))
		c.setInjectedLights(lightsClimb, "lights taxi off (gear up)")
	}
	if pose.HeightFt >= ClimbHandoverFt {
		c.handOverClimb(pose)
		return
	}
	c.emit(nil, false)
}

// handOverClimb releases the aircraft to MSFS AI with climb waypoints.
func (c *TaxiController) handOverClimb(pose TakeoffPose) {
	c.note("release", c.inj.Release(c.objectID))
	wps := TakeoffClimb(pose.Position.Lat, pose.Position.Lon, pose.Heading)
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+defOffWaypoints, wps); err != nil {
		c.emit(err, true)
	}
	c.stopMonitor()
	c.setState(TaxiComplete, nil)
}

// ClearPushback clears an injected departure to push back.
func (c *TaxiController) ClearPushback() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pushCleared = true
}

// ClearToTaxi clears an injected departure to taxi to the runway.
func (c *TaxiController) ClearToTaxi() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taxiCleared = true
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
			r, err := g.Route(hold, e.Node, c.req.Options)
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
	return nil
}
