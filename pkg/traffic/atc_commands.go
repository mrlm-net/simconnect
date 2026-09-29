//go:build windows
// +build windows

package traffic

import (
	"errors"
	"fmt"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ATC commands beyond the clearances (injected traffic): hold position,
// go around, abort take-off — what a controller says when something is in
// the way.

// Errors of the ATC commands.
var (
	// ErrNotTaxiing: hold position is for an aircraft moving on the ground.
	ErrNotTaxiing = errors.New("traffic: the aircraft is not taxiing")
	// ErrTooLate: past V1 (abort take-off) or on the runway (go around).
	ErrTooLate = errors.New("traffic: too late for that")
	// ErrNotApplicable: the command does not fit the aircraft's phase.
	ErrNotApplicable = errors.New("traffic: not possible in this phase")
)

// HoldPositionDecel (m/s²) is how firmly an aircraft stops on "hold
// position": a prompt but normal stop.
var HoldPositionDecel = 1.2

// holdPosition stops the aircraft as soon as it comfortably can and keeps
// it there (a clearance limit at the stopping point) until the next taxi
// clearance.
func (d *groundDrive) holdPosition() {
	p := d.mover.Pose()
	v := p.GroundSpeedKts * ktsToMS
	d.limit, d.hasLimit, d.limitNode = p.Distance+v*v/(2*HoldPositionDecel)+0.5, true, -1
	d.updateHold()
}

// HoldPosition stops a taxiing injected departure where it is ("hold
// position"); ClearToTaxi or ClearUpTo lets it go on.
func (c *TaxiController) HoldPosition() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil {
		return ErrNotInjected
	}
	// Not while lining up: stopped part-way onto the runway it would have no
	// way on (the line-up clearances do not lift a limit); AbortTakeoff
	// cancels a take-off there.
	if c.mover == nil || c.state != TaxiTaxiing {
		return ErrNotTaxiing
	}
	c.holdPosition()
	c.note("hold position", nil)
	return nil
}

// HoldPosition stops a taxiing injected arrival where it is; ClearToTaxi
// or ClearUpTo lets it go on.
func (c *ArrivalController) HoldPosition() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil {
		return ErrNotInjected
	}
	if c.mover == nil || c.state != ArrivalTaxiing {
		return ErrNotTaxiing
	}
	c.holdPosition()
	c.note("hold position", nil)
	return nil
}

// AbortTakeoff rejects the take-off: before V1 the aircraft brakes to a
// stop on the runway, vacates at the next exit ahead and taxis back to the
// holding point, where it waits for a new line-up and take-off clearance.
// Past V1 it returns ErrTooLate and the take-off continues.
func (c *TaxiController) AbortTakeoff() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil {
		return ErrNotInjected
	}
	if c.state == TaxiLiningUp || c.state == TaxiLinedUp {
		// Not rolling yet: the clearance is cancelled and the aircraft holds
		// lined up (or lines up and holds) until the next ClearForTakeoff.
		c.takeoffCleared, c.takeoffHeld = false, true
		return nil
	}
	if c.state != TaxiDeparting || c.takeoff == nil {
		return ErrNotApplicable
	}
	if !c.takeoff.Reject() {
		return ErrTooLate
	}
	c.note("take-off rejected", nil)
	c.emit(nil, true)
	return nil
}

// vacateAfterReject taxis a rejected take-off, stopped on the runway at
// pose, off at the next exit ahead and back to the holding point.
func (c *TaxiController) vacateAfterReject(pose TakeoffPose) error {
	g, prof := c.req.Graph, c.profile()
	exits, err := g.RunwayExits(c.end.Name)
	if err != nil {
		return err
	}
	along := alongHeading(c.end.Threshold, c.end.Heading, pose.Position)
	var ex *airport.RunwayExit
	for i := range exits {
		e := &exits[i]
		if e.Angle > 120 || e.Along < along+prof.WheelbaseMeters || len(e.Path) < 2 {
			continue
		}
		if ex == nil || e.Along < ex.Along {
			ex = e
		}
	}
	if ex == nil {
		return fmt.Errorf("%w: no runway exit ahead", ErrNotApplicable)
	}
	opts := c.req.Options
	opts.Via, opts.Taxiways = nil, nil
	route, err := g.RouteToRunwayFrom(ex.Node, ex.Path[len(ex.Path)-2], c.end.Name, c.req.Entry, opts)
	if err != nil {
		return err
	}
	nose := NoseGear(pose.Position, pose.Heading, prof)
	pts := []airport.LatLon{nose}
	for _, n := range ex.Path {
		if q := g.Nodes[n].Position; alongHeading(nose, pose.Heading, q) > 1 || len(pts) > 1 {
			pts = append(pts, q)
		}
	}
	pts = append(pts, route.Points[1:]...)
	if n := len(pts); n >= 2 { // stop short of the hold-short line
		a, b := pts[n-2], pts[n-1]
		if l := localDist(a, b); l > HoldShortStopMeters+1 {
			pts[n-1] = offsetHeading(b, localBearing(b, a), HoldShortStopMeters)
		}
	}
	path, err := NewGroundPath(pts, prof)
	if err != nil {
		return err
	}
	c.takeoff = nil
	c.mover = NewGroundMoverFrom(path, prof, pose.Heading, 0)
	c.route, c.pushJunction = route, 0
	c.lineUpCleared, c.takeoffCleared, c.gearUp = false, false, false
	// The crossings and limits of the old taxi-out do not apply to this
	// path: its own crossings (not of the departure runway), held until
	// cleared, and no limit.
	var holds []holdOnPath
	for i, n := range route.Nodes {
		if hs := g.Nodes[n].HoldShort; hs != nil && hs.Runway != c.runway.Index {
			at, _ := path.DistanceTo(route.Points[i])
			holds = append(holds, holdOnPath{runway: hs.Runway, index: i, dist: at})
		}
	}
	c.crossZones, c.nextCross, c.crossClears = crossingZones(g, route.Points, holds), 0, 0
	c.hasLimit, c.limitNode, c.hasPad = false, -1, false
	c.holdNextCrossing()
	c.lastStep = c.now()
	c.setInjectedLights(LightsTaxi, "lights taxi (vacating after the rejected take-off)")
	c.note(fmt.Sprintf("vacating via %s", ex.Taxiway), nil)
	c.setState(TaxiTaxiing, nil)
	return nil
}

// Go-around circuit: the missed approach climbs straight ahead to
// GoAroundClimbNm past the threshold, turns crosswind and flies a
// downwind GoAroundOffsetNm from the runway at GoAroundHeightFt above it,
// then back to the join point for another approach.
var (
	GoAroundClimbNm  = 3.0
	GoAroundOffsetNm = 3.5
	GoAroundHeightFt = 3000.0
)

// GoAround sends an injected arrival on final around: before touchdown it
// climbs out and MSFS AI flies a left-hand circuit back to the join point,
// where the injected approach takes over again. On the runway it returns
// ErrTooLate.
func (c *ArrivalController) GoAround() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil {
		return ErrNotInjected
	}
	if c.flyingProc {
		return nil // not on final yet: nothing to go around from
	}
	if c.approach == nil {
		return ErrNotApplicable
	}
	pose := c.approach.Pose()
	if pose.OnGround {
		return ErrTooLate
	}
	end := c.plan.End
	join := math.Max(c.plan.SpawnNm, ProcedureJoinNm) * 1852
	jp := NewApproachMover(end.Threshold, end.Heading, join, c.approachProfile()).Pose()
	fieldFt := convert.MetersToFeet(c.req.Graph.Layout.Altitude)
	joinFt := fieldFt + jp.HeightFt
	circuitFt := math.Max(joinFt, fieldFt+GoAroundHeightFt)
	t, hdg := end.Threshold, end.Heading
	left := hdg - 90 // left-hand circuit
	at := func(alongNm, sideNm float64) airport.LatLon {
		return offsetHeading(offsetHeading(t, hdg, alongNm*1852), left, sideNm*1852)
	}
	climb := at(GoAroundClimbNm, 0)
	crosswind := at(GoAroundClimbNm, GoAroundOffsetNm)
	abeam := at(0, GoAroundOffsetNm)
	downwindEnd := at(-join/1852-1, GoAroundOffsetNm)
	align := at(-join/1852-ProcedureAlignNm, 0)
	joinAt := at(-join/1852, 0)
	wps := []types.SIMCONNECT_DATA_WAYPOINT{
		procedureWaypoint(climb, circuitFt, ProcedureApproachSpeedKts),
		procedureWaypoint(crosswind, circuitFt, ProcedureApproachSpeedKts),
		procedureWaypoint(abeam, circuitFt, ProcedureApproachSpeedKts),
		procedureWaypoint(downwindEnd, circuitFt, ProcedureApproachSpeedKts),
		procedureWaypoint(align, joinFt+ProcedureAlignNm*ProcedureDescentFtPerNm, ProcedureApproachSpeedKts),
		procedureWaypoint(joinAt, joinFt, ProcedureApproachSpeedKts),
	}
	c.note("go around", nil)
	c.note("release", c.inj.Release(c.objectID))
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, wps); err != nil {
		return err
	}
	c.approach = nil
	c.proc = &ArrivalProcedure{Waypoints: wps, Join: joinAt, JoinMeters: join}
	c.flyingProc, c.blend = true, joinBlend{}
	c.monitorEvery(types.SIMCONNECT_PERIOD_SECOND)
	c.goArounds++
	c.setState(ArrivalApproaching, nil)
	return nil
}
