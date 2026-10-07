package traffic

import (
	"errors"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Touch-and-goes (#569, Doc 4444 12.3.4.16 c): a VFR circuit arrival with
// ArrivalRequest.TouchAndGos left does not roll out once its nose wheel is
// down. It takes off again from where it is, at the speed it has, injected
// (TakeoffMover), and at VFRHandoverFt MSFS AI flies the circuit again,
// from its crosswind leg round to the final, where the injected approach
// takes over as on its first circuit. The last landing is a full stop.

// TouchAndGosLeft is how many touch-and-goes the arrival still makes before
// its full stop.
func (c *ArrivalController) TouchAndGosLeft() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tngLeft
}

// startTouchAndGo turns a landing into a touch-and-go at pose (nose wheel
// down on the runway): spoilers in, take-off flaps, the take-off roll
// from here at this speed.
func (c *ArrivalController) startTouchAndGo(pose ApproachPose) {
	c.tngLeft--
	c.approach = nil
	if c.req.StopAndGo {
		// Braking to a stop first; the take-off from a standstill after.
		c.sng = &stopAndGo{pos: pose.Position, hdg: pose.Heading, v: pose.GroundSpeedKts * ktsToMS, wait: StopAndGoWait.Seconds()}
	} else {
		c.tng = NewTakeoffMover(pose.Position, pose.Heading, pose.GroundSpeedKts, c.aircraft().Takeoff)
	}
	c.spoilers = surfaceRamp{target: 0, rate: 100 / SpoilerDeploySeconds, pct: c.spoilers.pct}
	c.flapsPct = c.aircraft().Flaps.TakeoffPct
	c.note("flaps", c.inj.SetFlaps(c.objectID, c.flapsPct))
	c.note("take-off thrust", c.inj.SetThrottle(c.objectID, c.aircraft().EngineCount(), TakeoffThrottlePct)) // off again
	c.lastStep = c.now()
	c.seq.add(c.lastStep, "touch and go", 0, pose.GroundSpeedKts)
	c.last.TouchAndGo = true
	c.emit(nil, true)
}

// onTouchAndGoFrame flies the take-off of a touch-and-go one frame; at
// VFRHandoverFt the circuit goes to MSFS AI (circuitAgain).
func (c *ArrivalController) onTouchAndGoFrame() {
	now := c.now()
	dt := math.Max(0, math.Min(now.Sub(c.lastStep).Seconds(), MaxFrameStepSeconds))
	c.lastStep = now
	if s := c.sng; s != nil {
		// A stop-and-go: braking on the runway, then a moment standing.
		if s.v > 0 {
			v := math.Max(0, s.v-c.rolloutProfile().BrakeDecel*dt)
			s.pos = offsetHeading(s.pos, s.hdg, (s.v+v)/2*dt)
			s.v = v
		} else if s.wait -= dt; s.wait <= 0 {
			c.tng, c.sng = NewTakeoffMover(s.pos, s.hdg, 0, c.aircraft().Takeoff), nil
			c.seq.add(now, "stop and go: take-off", 0, 0)
		}
		ap := TakeoffPose{Position: s.pos, Heading: s.hdg, GroundSpeedKts: s.v / ktsToMS}.ApproachPose()
		ap.RunwayFt = c.plan.Runway.Altitude / 0.3048
		if err := c.inj.PlaceAir(c.objectID, ap); err != nil && !errors.Is(err, ErrGroundUnknown) {
			c.emit(err, true)
		}
		c.stepSurfaces(dt)
		c.last.Position, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = s.pos, s.hdg, s.v/ktsToMS, true
		c.emit(nil, false)
		return
	}
	pose := c.tng.Step(dt)
	ap := pose.ApproachPose()
	ap.RunwayFt = c.plan.Runway.Altitude / 0.3048
	if err := c.inj.PlaceAir(c.objectID, ap); err != nil && !errors.Is(err, ErrGroundUnknown) {
		c.emit(err, true)
	}
	c.stepSurfaces(dt)
	c.last.Position, c.last.Heading, c.last.GroundSpeed = pose.Position, pose.Heading, pose.GroundSpeedKts
	c.last.AGL, c.last.OnGround = pose.HeightFt, pose.Phase != TakeoffAirborne
	if pose.Phase == TakeoffAirborne && c.state == ArrivalRollout {
		c.setState(ArrivalApproaching, nil) // off the runway, in the circuit again
		return
	}
	if pose.HeightFt >= VFRHandoverFt {
		if err := c.circuitAgain(false); err != nil {
			c.fail(err)
		}
		return
	}
	c.emit(nil, false)
}

// circuitAgain hands a touch-and-go's climb-out (or a circuit arrival's
// go-around, upwind: from the upwind leg) to MSFS AI: the circuit from its
// crosswind leg round to the final, the join point there.
func (c *ArrivalController) circuitAgain(upwind bool) error {
	ci := *c.req.Circuit
	plan := PlanCircuitArrival(ci)
	var wps []types.SIMCONNECT_DATA_WAYPOINT
	var names []string
	legs := []CircuitLeg{LegCrosswind, LegDownwind, LegBase, LegFinal}
	if upwind {
		legs = append([]CircuitLeg{LegUpwind}, legs...)
	}
	for _, leg := range legs {
		if p, ok := ci.Point(leg); ok {
			wps = append(wps, procedureWaypoint(p.Position, p.AltFt, p.Kts))
			names = append(names, string(leg))
		}
	}
	c.setCorners(wps, names)
	c.cornerNext = 0
	wps = roundedChain(c.last.Position, wps, MaxBankDeg(*c.aircraft()))
	c.flapsPct = c.aircraft().Flaps.ApproachPct
	c.note("flaps", c.inj.SetFlaps(c.objectID, c.flapsPct))
	c.note("release", c.inj.Release(c.objectID))
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, wps); err != nil {
		return err
	}
	c.tng = nil
	c.proc, c.procNext, c.tromboneNM = &ArrivalProcedure{Waypoints: wps, Join: plan.Join, JoinMeters: plan.JoinMeters, MinJoinMeters: plan.MinJoinMeters}, 0, 0
	c.flyingProc, c.blend = true, joinBlend{}
	c.approachLightsSet, c.landingFlaps, c.landingFlapsSet = false, false, false
	c.last.TouchAndGo = false
	c.monitorEvery(types.SIMCONNECT_PERIOD_SECOND)
	c.seq.add(c.now(), "circuit again (MSFS AI)", VFRHandoverFt, c.last.GroundSpeed)
	c.emit(nil, true)
	return nil
}

// StopAndGoWait is how long a stop-and-go stands on the runway before its
// take-off (an estimate: flaps reset, a check).
const StopAndGoWait = 10 * time.Second

// stopAndGo is a stop-and-go braking on the runway (v m/s along hdg from
// pos), then standing wait seconds.
type stopAndGo struct {
	pos    airport.LatLon
	hdg, v float64
	wait   float64
}
