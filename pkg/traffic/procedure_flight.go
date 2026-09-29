//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Procedure flying (#315): after the injected take-off MSFS AI flies the
// SID (TaxiRequest.Departure); an arrival with ArrivalRequest.Procedure
// appears at the STAR's first fix, MSFS AI flies the STAR and the approach
// transition to a join point on the extended centreline, and the injected
// approach takes over there.

// Procedure flying tunables.
var (
	// ProcedureSpeedKts is the speed on SIDs and STARs (below FL100).
	ProcedureSpeedKts = 250.0
	// ProcedureApproachSpeedKts is the speed from the approach transition
	// to the join point.
	ProcedureApproachSpeedKts = 180.0
	// ProcedureClimbFtPerNm is the planned climb gradient of a SID
	// (about 5 %) where the procedure gives no altitude.
	ProcedureClimbFtPerNm = 300.0
	// ProcedureDescentFtPerNm is the planned descent (3°) of a STAR.
	ProcedureDescentFtPerNm = 318.0
	// ProcedureTopFt is the altitude a SID climbs to when nothing higher
	// is asked for (a SID ends below the first cruise level ATC gives).
	ProcedureTopFt = 10000.0
	// ProcedureContinueMeters extends a SID beyond its last point on the
	// same track, so MSFS AI does not turn back after the last waypoint.
	ProcedureContinueMeters = 60000.0
	// ProcedureJoinNm is how far out on the extended centreline the
	// injected approach takes over from MSFS AI (at least the spawn
	// distance); the procedure's own points closer than JoinNm plus
	// ProcedureAlignNm are replaced by two centreline points.
	ProcedureJoinNm  = 8.0
	ProcedureAlignNm = 3.0
	// JoinCaptureMeters: the takeover happens once the aircraft is this
	// close to the join point, or abeam it on the centreline.
	JoinCaptureMeters = 1500.0
	// JoinBlendSeconds: the offset between where MSFS AI flew the aircraft
	// and the injected approach path fades out over this time.
	JoinBlendSeconds = 20.0
)

// procedureWaypoint is an airborne waypoint at altFt MSL.
func procedureWaypoint(p airport.LatLon, altFt, kts float64) types.SIMCONNECT_DATA_WAYPOINT {
	return types.SIMCONNECT_DATA_WAYPOINT{
		Latitude: p.Lat, Longitude: p.Lon, Altitude: altFt,
		Flags:    uint32(types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED | types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED),
		KtsSpeed: kts,
	}
}

const ftPerMeter = 1 / 0.3048

// constrain applies a point's altitude constraints (meters) to altFt.
func constrain(n airport.NavPoint, altFt float64) float64 {
	if n.AltMax > 0 {
		altFt = math.Min(altFt, n.AltMax*ftPerMeter)
	}
	if n.AltMin > 0 {
		altFt = math.Max(altFt, n.AltMin*ftPerMeter)
	}
	return altFt
}

// DepartureWaypoints is the chain MSFS AI flies after the injected
// take-off: the points of route ahead of the aircraft (at pos, heading hdg,
// altFt MSL), climbing ProcedureClimbFtPerNm up to ProcedureTopFt (or
// higher where the route asks), within each point's constraints, then on
// along the last track.
func DepartureWaypoints(pos airport.LatLon, hdg, altFt float64, route []airport.NavPoint) []types.SIMCONNECT_DATA_WAYPOINT {
	var out []types.SIMCONNECT_DATA_WAYPOINT
	cur, alt := pos, altFt
	last := pos
	top := ProcedureTopFt
	for _, n := range route {
		if n.AltMin*ftPerMeter > top {
			top = n.AltMin * ftPerMeter
		}
	}
	for _, n := range route {
		d := calc.HaversineMeters(cur.Lat, cur.Lon, n.Position.Lat, n.Position.Lon)
		brg := calc.BearingDegrees(pos.Lat, pos.Lon, n.Position.Lat, n.Position.Lon)
		if len(out) == 0 && (d < 1852 || math.Abs(headingDiff(brg, hdg)) > 90) {
			continue // behind, or passed during the injected climb
		}
		alt = constrain(n, math.Min(top, alt+d/1852*ProcedureClimbFtPerNm))
		out = append(out, procedureWaypoint(n.Position, alt, ProcedureSpeedKts))
		last, cur = cur, n.Position
	}
	// On along the last track (or the take-off heading).
	track := hdg
	if len(out) > 0 && last != cur {
		track = calc.BearingDegrees(last.Lat, last.Lon, cur.Lat, cur.Lon)
	}
	lat, lon := calc.DisplaceByHeading(cur.Lat, cur.Lon, track, ProcedureContinueMeters)
	return append(out, procedureWaypoint(airport.LatLon{Lat: lat, Lon: lon}, math.Max(alt, top), ProcedureSpeedKts))
}

// ArrivalProcedure is the airborne part of an arrival flown by MSFS AI:
// where the aircraft appears, the waypoints to the join point and the join
// point itself, JoinMeters out on the extended centreline, where the
// injected approach takes over.
type ArrivalProcedure struct {
	Spawn      types.SIMCONNECT_DATA_INITPOSITION
	Waypoints  []types.SIMCONNECT_DATA_WAYPOINT
	Join       airport.LatLon
	JoinMeters float64
}

// PlanArrivalProcedure plans the STAR and approach of route to a join
// point joinMeters out on the final of end (threshold elevation fieldFt),
// where the approach is at joinFt MSL: the route's points up to the final,
// then two centreline points (aligned ProcedureAlignNm before the join,
// and the join). Altitudes descend ProcedureDescentFtPerNm back from the
// join point, at most ProcedureTopFt (unless a constraint is higher) and
// within each point's constraints.
func PlanArrivalProcedure(route []airport.NavPoint, end airport.RunwayEnd, joinMeters, joinFt float64) (*ArrivalProcedure, error) {
	t, hdg := end.Threshold, end.Heading
	outbound := math.Mod(hdg+180, 360)
	fLat, fLon := calc.DisplaceByHeading(t.Lat, t.Lon, outbound, 30*1852)
	// On the final: out on the extended centreline, within a mile of it.
	onFinal := func(p airport.LatLon, within float64) bool {
		along := calc.AlongTrackMeters(t.Lat, t.Lon, fLat, fLon, p.Lat, p.Lon)
		cross := math.Abs(calc.CrossTrackMeters(t.Lat, t.Lon, fLat, fLon, p.Lat, p.Lon))
		return along > 0 && along < within && cross < 1852
	}
	alignMeters := joinMeters + ProcedureAlignNm*1852
	var pts []airport.NavPoint
	for _, n := range route {
		if n.Vectors || n.MAP || onFinal(n.Position, alignMeters+1852) {
			break // the rest is the final: flown on the centreline
		}
		pts = append(pts, n)
	}
	if len(pts) == 0 {
		return nil, fmt.Errorf("%w: the procedure has no point before the final", ErrBadTaxiRequest)
	}
	aLat, aLon := calc.DisplaceByHeading(t.Lat, t.Lon, outbound, alignMeters)
	jLat, jLon := calc.DisplaceByHeading(t.Lat, t.Lon, outbound, joinMeters)
	align, join := airport.LatLon{Lat: aLat, Lon: aLon}, airport.LatLon{Lat: jLat, Lon: jLon}
	// Altitudes backwards from the join point, no higher than
	// ProcedureTopFt unless the procedure asks for more.
	top := ProcedureTopFt
	for _, n := range pts {
		top = math.Max(top, n.AltMin*ftPerMeter)
	}
	alts := make([]float64, len(pts))
	next, alt := align, joinFt+ProcedureAlignNm*ProcedureDescentFtPerNm
	for i := len(pts) - 1; i >= 0; i-- {
		d := calc.HaversineMeters(pts[i].Position.Lat, pts[i].Position.Lon, next.Lat, next.Lon)
		alt = constrain(pts[i], math.Min(top, alt+d/1852*ProcedureDescentFtPerNm))
		alts[i], next = alt, pts[i].Position
	}
	ap := &ArrivalProcedure{Join: join, JoinMeters: joinMeters}
	for i, n := range pts {
		kts := ProcedureSpeedKts
		if n.IAF || i == len(pts)-1 {
			kts = ProcedureApproachSpeedKts
		}
		if n.SpeedMax > 0 {
			kts = math.Min(kts, n.SpeedMax)
		}
		ap.Waypoints = append(ap.Waypoints, procedureWaypoint(n.Position, alts[i], kts))
	}
	ap.Waypoints = append(ap.Waypoints,
		procedureWaypoint(align, joinFt+ProcedureAlignNm*ProcedureDescentFtPerNm, ProcedureApproachSpeedKts),
		procedureWaypoint(join, joinFt, ProcedureApproachSpeedKts))
	first, toward := pts[0].Position, align
	if len(pts) > 1 {
		toward = pts[1].Position
	}
	ap.Spawn = types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: first.Lat, Longitude: first.Lon, Altitude: alts[0],
		Heading:  calc.BearingDegrees(first.Lat, first.Lon, toward.Lat, toward.Lon),
		Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(ProcedureSpeedKts),
	}
	// The waypoints start after the spawn point.
	ap.Waypoints = ap.Waypoints[1:]
	return ap, nil
}

// joinBlend fades out the offset between where MSFS AI flew the aircraft
// and the injected approach at the takeover.
type joinBlend struct {
	dLat, dLon, dFt, dHdg float64
	left                  float64 // seconds
}

func (b *joinBlend) apply(p ApproachPose, dt float64) ApproachPose {
	if b.left <= 0 {
		return p
	}
	b.left = math.Max(0, b.left-dt)
	k := b.left / JoinBlendSeconds
	k = k * k * (3 - 2*k) // smooth: no jerk at either end
	p.Position.Lat += b.dLat * k
	p.Position.Lon += b.dLon * k
	p.HeightFt += b.dFt * k
	p.Heading = math.Mod(p.Heading+b.dHdg*k+360, 360)
	return p
}

// startProcedure hands the arrival to MSFS AI for the STAR and approach:
// gear up, arrival lights, the procedure waypoints, position every second.
func (c *ArrivalController) startProcedure() error {
	if err := c.fleet.ReleaseControl(c.objectID, c.reqBase+arrReqRelease); err != nil {
		return err
	}
	c.setLights(false, false, true, true, true, "lights arrival")
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, c.proc.Waypoints); err != nil {
		return err
	}
	c.monitorEvery(types.SIMCONNECT_PERIOD_SECOND)
	c.flyingProc = true
	c.note("flying the STAR and approach (MSFS AI)", nil)
	c.setState(ArrivalApproaching, nil)
	return nil
}

// onProcedureFrame follows MSFS AI on the procedure and takes over with
// the injected approach at the join point (or abeam it on the centreline),
// fading out the offset from the injected path.
func (c *ArrivalController) onProcedureFrame(m arrivalMonitor) {
	pos := airport.LatLon{Lat: m.Latitude, Lon: m.Longitude}
	c.last.Position, c.last.AGL, c.last.Heading, c.last.GroundSpeed, c.last.OnGround = pos, m.AGL, m.Heading, m.GroundKts, false
	if c.holding != nil { // in a hold: no join until it leaves (#392)
		c.holdFrame(pos)
		c.emit(nil, false)
		return
	}
	t, far := c.plan.End.Threshold, c.proc.Join
	along := calc.AlongTrackMeters(t.Lat, t.Lon, far.Lat, far.Lon, pos.Lat, pos.Lon)
	cross := math.Abs(calc.CrossTrackMeters(t.Lat, t.Lon, far.Lat, far.Lon, pos.Lat, pos.Lon))
	near := calc.HaversineMeters(pos.Lat, pos.Lon, far.Lat, far.Lon) < JoinCaptureMeters
	established := along > 2*1852 && along <= c.proc.JoinMeters+300 && cross < 2000 && math.Abs(headingDiff(m.Heading, c.plan.End.Heading)) < 45
	if !near && !established {
		c.emit(nil, false)
		return
	}
	c.flyingProc = false
	start := math.Max(2*1852, math.Min(along, c.proc.JoinMeters+JoinCaptureMeters))
	if err := c.startInjectedApproach(start); err != nil {
		c.fail(err)
		return
	}
	p := c.approach.Pose()
	// Heights above the runway on both sides: MSL (ground under it plus its
	// height above it) less the runway's elevation.
	above := m.AGL
	if g, ok := c.inj.GroundFt(c.objectID); ok {
		above = g + m.AGL - c.plan.Runway.Altitude/0.3048
	}
	c.blend = joinBlend{dLat: pos.Lat - p.Position.Lat, dLon: pos.Lon - p.Position.Lon, dFt: above - p.HeightFt,
		dHdg: headingDiff(p.Heading, m.Heading), left: JoinBlendSeconds}
	c.note("joined the final: injected approach", nil)
}
