//go:build windows
// +build windows

package traffic

import (
	"errors"
	"fmt"
	"math"
	"slices"
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
	c.last.LimitNode = -1
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
	// Flaps move on every frame, whatever the phase.
	if !c.frameAt.IsZero() && c.flaps.step(math.Min(now.Sub(c.frameAt).Seconds(), 0.25)) {
		c.note("flaps", c.inj.SetFlaps(c.objectID, c.flaps.pct))
	}
	if !c.frameAt.IsZero() {
		c.updateTug(math.Min(now.Sub(c.frameAt).Seconds(), 0.25))
	}
	c.frameAt = now
	switch c.state {
	case TaxiAwaitingPushback:
		if c.pushAt.IsZero() && c.gate(c.pushCleared) {
			// Beacon on, and the push starts BeaconLeadTime later.
			c.lights.Logo = true // as MSFS AI shows it; aircraft spawn with it off
			c.setInjectedLights(LightsPushback, "lights beacon (pushback)")
			c.pushAt = now.Add(BeaconLeadTime)
		}
		if !c.pushAt.IsZero() && !now.Before(c.pushAt) {
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
			// Take-off flaps set after engine start, while taxiing out.
			c.flaps = surfaceRamp{target: TakeoffFlapsPct, rate: TakeoffFlapsPct / FlapsSetSeconds}
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
			// Without held gates some departures get line-up and take-off in one
			// clearance and roll straight into the take-off.
			if chance := c.req.RollingTakeoffChance; !c.req.HoldForClearances && chance >= 0 {
				if chance == 0 {
					chance = DefaultRollingTakeoffChance
				}
				c.takeoffCleared = c.takeoffCleared || c.rng.Float64() < chance
			}
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

// facesOut reports a self-manoeuvring stand: the lead-in junction lies
// ahead of the parked aircraft, so it taxis out without a pushback.
func (c *TaxiController) facesOut() bool {
	return len(c.route.Points) > 1 && leadInAhead(c.req.Graph, c.req.Parking, c.route.Points[1])
}

// updateTug connects the tug while the aircraft waits for its pushback
// and moves it with the aircraft until it has driven off.
func (c *TaxiController) updateTug(dt float64) {
	t := c.req.Tug
	if t == nil || t.Done() {
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
		c.tugAttached = true
		c.tugErr(t.Attach(pose))
		return
	}
	c.tugErr(t.Update(pose, c.state == TaxiAwaitingPushback || c.state == TaxiPushback, dt))
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
	g, prof, route := c.req.Graph, c.profile(), c.route
	stand := g.Layout.Parking[c.req.Parking]
	gear := offsetHeading(StandPoint(stand, c.req.NoseOffset), stand.Heading, -prof.RefAheadMeters)
	pts := []airport.LatLon{gear}
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
	push := prof
	push.CruiseKts, push.MinTurnKts, push.Accel, push.Decel = PushbackSpeedKts, 1, 0.15, 0.25
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
		return err
	}
	c.mover = NewPushbackMover(path, push, stand.Heading)
	c.lastStep = c.now()
	return nil
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

// planPushback chooses the taxiway branch the tail is pushed onto by where
// the aircraft can go from there: for every branch at the stand's junction
// the push can swing onto, the taxi-out is planned from the junction facing
// away from it (RouteToRunwayFrom); the cheapest wins and the route becomes
// stand → junction → that taxi-out. Without it the pushback guessed from
// the route planned from the stand, which at LKPR C17 went on straight
// ahead of the push and left the aircraft facing away from its route.
func (c *TaxiController) planPushback() {
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
	branch, at, bestCost := airport.NodeID(-1), 1, math.Inf(1)
	for _, i := range cands {
		k, kp := r.Nodes[i], r.Points[i]
		in := localBearing(r.Points[i-1], kp) // the push arriving at k
		pushed := localDist(gear, jp)
		for n := 2; n <= i; n++ {
			pushed += localDist(r.Points[n-1], r.Points[n])
		}
		for _, e := range g.Adj[k] {
			if e.To == r.Nodes[i-1] || !pushEdge(g, e) || !g.Fits(e, c.req.Options) ||
				math.Abs(headingDiff(in, localBearing(kp, g.Nodes[e.To].Position))) > maxPushSwingDeg {
				continue
			}
			out, err := g.RouteToRunwayFrom(k, e.To, c.req.Runway, c.req.Entry, c.req.Options)
			if err != nil || len(out.Nodes) < 2 || out.Nodes[1] == e.To {
				continue // no way on, or only back over the branch it was pushed onto
			}
			// Pushing is slow: each meter costs pushCostFactor taxi meters.
			cost := out.Cost + pushed*pushCostFactor
			if cost >= bestCost {
				continue
			}
			var pts []airport.LatLon
			if alley[i] {
				if pts = c.alleyPush(gear, i, e.To); pts == nil {
					continue // too tight or into the neighbours
				}
			}
			best, bestPts, branch, at, bestCost = out, pts, e.To, i, cost
		}
	}
	if best == nil {
		// No branch to push the tail onto: the only taxiway at the junction
		// is the way out (LKPR A7, B9). Push and turn on the apron to face it.
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
// off the taxi route is not worth the swing.
const (
	maxPushSwingDeg    = 100.0
	maxNoseOffRouteDeg = 150.0
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
	// pushTurnRadiusCost is what a meter of turn radius below
	// PushbackArcMeters is worth in meters of push, choosing a push-and-turn.
	pushTurnRadiusCost = 1.5
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
	for i := start; i < len(route.Points); i++ {
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
	if c.hasPendingLimit {
		c.hasPendingLimit = false
		c.note("clearance limit", c.setLimit(c.pendingLimit))
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
	if pose.HeightFt > FlapsRetractFt && c.flaps.target > 0 {
		c.flaps.target, c.flaps.rate = 0, TakeoffFlapsPct/FlapsRetractClimbSeconds // flaps up in the climb
	}
	if !c.gearUp && pose.HeightFt > GearUpFt && pose.AirborneSeconds >= GearUpDelaySeconds && pose.VerticalFpm >= GearUpFpm { // positive climb
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
	c.note("flaps up", c.inj.SetFlaps(c.objectID, 0)) // clean for MSFS AI
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

// ClearToTaxi clears an injected departure to taxi to the runway, without a
// limit (removing one given with ClearUpTo).
func (c *TaxiController) ClearToTaxi() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taxiCleared, c.hasPendingLimit = true, false
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
