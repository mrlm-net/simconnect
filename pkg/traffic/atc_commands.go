package traffic

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
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
	// GoAroundGearUpFt: the gear comes up this far above the runway in the
	// go-around climb (it stayed down all round the circuit before).
	GoAroundGearUpFt = 400.0
	GoAroundClimbNm  = 3.0
	GoAroundOffsetNm = 3.5
	GoAroundHeightFt = 3000.0
)

// GoAround sends an injected arrival on final around: before touchdown it
// climbs out and MSFS AI flies the published missed approach
// (ArrivalRequest.MissedApproach), else a left-hand circuit, back to the
// join point, where the injected approach takes over again. On the runway
// it returns ErrTooLate.
func (c *ArrivalController) GoAround() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil {
		return ErrNotInjected
	}
	if c.flyingProc {
		// A circuit arrival on its base or final before the injected
		// approach takes over: over the runway and round again (live,
		// OKIMV sent around twice, landed: the order was dropped).
		if c.req.Circuit != nil {
			if _, err := c.anotherCircuit(); err != nil {
				return err
			}
			c.goArounds++
			c.note("go around (circuit, before the final)", nil)
			return nil
		}
		return nil // not on final yet: nothing to go around from
	}
	if c.approach == nil {
		return ErrNotApplicable
	}
	pose := c.approach.Pose()
	if pose.OnGround {
		return ErrTooLate
	}
	// A VFR circuit arrival goes round its own circuit, at circuit height
	// (#569): not the airliners' 3000 ft circuit 3.5 NM out.
	if c.req.Circuit != nil {
		c.approach = nil
		if err := c.circuitAgain(true); err != nil {
			return err
		}
		c.goArounds++
		c.gaGearUp = true // a light single's gear stays as it is
		c.note("go around (circuit)", nil)
		c.setState(ArrivalApproaching, nil)
		return nil
	}
	end := c.plan.End
	join := math.Max(c.plan.SpawnNm, ProcedureJoinNm) * 1852
	jp := NewApproachMover(end.Threshold, end.Heading, join, c.approachProfile()).Pose()
	fieldFt := convert.MetersToFeet(c.req.Graph.Layout.Altitude)
	joinFt := fieldFt + jp.HeightFt
	circuitFt := math.Max(joinFt, fieldFt+GoAroundHeightFt)
	// Up to the approach's own altitude: its last constraint before the
	// runway (the final approach fix's) where it is higher.
	for _, n := range c.req.Procedure {
		if n.Kind != "R" && math.Max(n.AltMin, n.AltMax) > 0 {
			joinAlt := math.Max(n.AltMin, n.AltMax) * ftPerMeter
			circuitFt = math.Max(math.Max(joinFt, fieldFt+GoAroundHeightFt), joinAlt)
		}
	}
	// The published missed approach where known: its points (none when it
	// climbs straight ahead for vectors, as at LKPR) at its altitude, which
	// the circuit keeps too; then round the circuit onto the final.
	missed, circuitFt := missedWaypoints(c.req.MissedApproach, circuitFt)
	// Without a published missed approach: out and back onto its own
	// STAR's downwind, sequenced again with the others there (live,
	// CSA1958 sent round a 3.5 NM circuit inside the 5 NM downwind of
	// LOMK8S and back in at 10 NM, cutting into the stream).
	if len(missed) == 0 {
		if ok, err := c.goAroundToDownwind(join, joinFt, circuitFt); ok || err != nil {
			return err
		}
	}
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
	// Its track points, named for the map and the radio.
	names := []string{"UPWIND", "CROSSWIND", "DOWNWIND", "BASE", "", "FINAL"}
	if len(missed) > 0 {
		wps = append(missed, wps[1:]...) // instead of the climb straight ahead
		var idents []string
		for _, n := range c.req.MissedApproach {
			if n.Position.Lat != 0 || n.Position.Lon != 0 {
				idents = append(idents, n.Ident)
			}
		}
		names = append(idents, names[1:]...)
	}
	c.setCorners(wps, names)
	// From the first corner on: the nearest may well be past the climb-out
	// (abeam, close to the final) and would skip the circuit.
	c.cornerNext = 0
	wps = roundedChain(c.last.Position, wps, MaxBankDeg(*c.aircraft()))
	c.note("go around", nil)
	c.note("release", c.inj.Release(c.objectID))
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, wps); err != nil {
		return err
	}
	c.approach = nil
	c.proc, c.procNext, c.circuit, c.tromboneNM = &ArrivalProcedure{Waypoints: wps, Join: joinAt, JoinMeters: join}, 0, true, 0
	c.flyingProc, c.blend, c.gaGearUp = true, joinBlend{}, false
	c.monitorEvery(types.SIMCONNECT_PERIOD_SECOND)
	c.goArounds++
	c.setState(ArrivalApproaching, nil)
	return nil
}

// ReduceToFinalSpeed has an arrival on its injected final fly its final
// approach speed from now on instead of slowing to it on the way: for
// spacing behind a slower leader. It returns the time that gains;
// ErrNotApplicable when it is not on the final.
func (c *ArrivalController) ReduceToFinalSpeed() (time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.approach == nil || c.flyingProc {
		return 0, ErrNotApplicable
	}
	return c.approach.slowNow(), nil
}

// FinalSlowGain is the time ReduceToFinalSpeed would gain now (0 when not
// on the final, or slowed already).
func (c *ArrivalController) FinalSlowGain() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.approach == nil || c.flyingProc {
		return 0
	}
	m := *c.approach
	return m.slowNow()
}

// missedWaypoints are the points of a published missed approach at its
// highest altitude (at least minFt), and that altitude.
func missedWaypoints(missed []airport.NavPoint, minFt float64) ([]types.SIMCONNECT_DATA_WAYPOINT, float64) {
	top := minFt
	for _, n := range missed {
		top = math.Max(top, math.Max(n.AltMin, n.AltMax)*ftPerMeter)
	}
	var wps []types.SIMCONNECT_DATA_WAYPOINT
	for _, n := range missed {
		if n.Position.Lat == 0 && n.Position.Lon == 0 {
			continue
		}
		wps = append(wps, procedureWaypoint(n.Position, top, ProcedureApproachSpeedKts))
	}
	return wps, top
}

// GoAroundRejoinMinNM: a STAR point this far or more off the extended
// centreline, abeam the runway or behind it, is on its downwind: a
// go-around rejoins there (goAroundToDownwind).
const GoAroundRejoinMinNM = 3.0

// goAroundToDownwind sends an IFR arrival going around (c.mu held) out
// ahead and across onto its own STAR's downwind, then along the rest of
// its STAR and approach to the final as before, at circuitFt at most on
// the way: back in the stream with the others, where the sequencer can
// stretch its downwind again. False when its STAR has no downwind (a
// straight-in): the go-around circuit then.
func (c *ArrivalController) goAroundToDownwind(join, joinFt, circuitFt float64) (bool, error) {
	end := c.plan.End
	t, hdg := end.Threshold, end.Heading
	ahead := offsetHeading(t, hdg, 10*1852)
	cross := func(p airport.LatLon) float64 { return calc.CrossTrackMeters(t.Lat, t.Lon, ahead.Lat, ahead.Lon, p.Lat, p.Lon) }
	route := c.req.Procedure
	k := -1
	for i, n := range route {
		if n.Kind == "R" || n.Position == (airport.LatLon{}) {
			continue
		}
		if math.Abs(cross(n.Position)) >= GoAroundRejoinMinNM*1852 && alongHeading(t, hdg, n.Position) <= GoAroundClimbNm*1852 {
			k = i
			break
		}
	}
	if k < 0 {
		return false, nil
	}
	proc, err := PlanArrivalProcedure(route[k:], end, join, joinFt)
	if err != nil || len(proc.Waypoints) < 3 {
		return false, nil
	}
	side := cross(route[k].Position)
	off := math.Abs(side)
	turn := hdg + 90
	if probe := offsetHeading(t, turn, off); (cross(probe) > 0) != (side > 0) {
		turn = hdg - 90
	}
	climb := offsetHeading(t, hdg, GoAroundClimbNm*1852)
	wps := []types.SIMCONNECT_DATA_WAYPOINT{
		procedureWaypoint(climb, circuitFt, ProcedureApproachSpeedKts),
		procedureWaypoint(offsetHeading(climb, turn, off), circuitFt, ProcedureApproachSpeedKts),
	}
	names := []string{"UPWIND", "CROSSWIND"}
	if alongHeading(t, hdg, route[k].Position) < 0 {
		wps = append(wps, procedureWaypoint(offsetHeading(t, turn, off), circuitFt, ProcedureApproachSpeedKts))
		names = append(names, "DOWNWIND")
	}
	star := proc.Waypoints
	for i := range star[:len(star)-2] { // the STAR's points no higher than the go-around's level
		star[i].Altitude = math.Min(star[i].Altitude, circuitFt)
	}
	wps = append(wps, star...)
	starNames := proc.Names
	if len(starNames) != len(star) {
		starNames = make([]string, len(star))
	}
	names = append(names, starNames...)
	c.setCorners(wps, names)
	c.cornerNext = 0
	rounded := roundedChain(c.last.Position, wps, MaxBankDeg(*c.aircraft()))
	c.note("go around: back to the downwind at "+route[k].Ident, nil)
	c.note("release", c.inj.Release(c.objectID))
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, rounded); err != nil {
		return true, err
	}
	c.approach = nil
	c.proc, c.procNext, c.circuit, c.tromboneNM = &ArrivalProcedure{Waypoints: rounded, Join: proc.Join, JoinMeters: proc.JoinMeters, MinJoinMeters: proc.MinJoinMeters}, 0, true, 0
	c.flyingProc, c.blend, c.gaGearUp = true, joinBlend{}, false
	c.monitorEvery(types.SIMCONNECT_PERIOD_SECOND)
	c.goArounds++
	c.setState(ArrivalApproaching, nil)
	return true, nil
}
