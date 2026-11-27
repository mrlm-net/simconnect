package traffic

import (
	"fmt"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
	"unsafe"
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
	// same track, so MSFS AI does not turn back after the last waypoint
	// and keeps the level it was cleared to for as long as it is ours:
	// past its last waypoint MSFS AI climbs on to a level of its own (live,
	// departures cleared to FL220, FL240 and held at FL100 all met at
	// FL234, the conflict watch turning them apart). About 30 min
	// (ManagerOptions.RemoveDepartedAfter) at 250 kt with 100 kt of wind.
	ProcedureContinueMeters = 450000.0
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

// JoinEaseKtsPerSecond: the injected approach eases from the speed MSFS AI
// flew it at to its own this fast (#1682), faster where it would not be on
// its speeds by JoinEaseByFt (ApproachMover.EaseFrom), up to
// JoinEaseMaxKtsPerSecond. The maximum is an estimate (no published
// figure read): an airliner slowing on the glide path with drag out.
const (
	JoinEaseKtsPerSecond    = 1.5
	JoinEaseMaxKtsPerSecond = 3.0
	JoinEaseByFt            = 500.0
)

// airWaypoints fits a chain MSFS AI flies for the arrival to its type
// (#1682): no airborne point faster than its top speed
// (ApproachProfile.TransitionMaxKts), and a chain ending on the final, as
// to the join point, slows toward the approach: the last point at the
// approach's start speed (StartKts), the one before it on the final
// halfway from the transition speed (ProcedureApproachSpeedKts, no more
// than the top speed), so the injected approach takes over at about its
// own speed.
func (c *ArrivalController) airWaypoints(wps []types.SIMCONNECT_DATA_WAYPOINT) []types.SIMCONNECT_DATA_WAYPOINT {
	if c.plan == nil {
		return fitAirSpeeds(wps, c.approachProfile(), nil)
	}
	end := c.plan.End
	return fitAirSpeeds(wps, c.approachProfile(), &end)
}

// airHoldWaypoints is airWaypoints for a hold's racetrack: no faster than
// the type's top speed, never slowed as a chain to the final is (a hold
// near the final's extended centreline is not a join).
func (c *ArrivalController) airHoldWaypoints(wps []types.SIMCONNECT_DATA_WAYPOINT) []types.SIMCONNECT_DATA_WAYPOINT {
	return fitAirSpeeds(wps, c.approachProfile(), nil)
}

// fitAirSpeeds is airWaypoints for profile p and the landing end (nil: no
// final known).
func fitAirSpeeds(wps []types.SIMCONNECT_DATA_WAYPOINT, p ApproachProfile, end *airport.RunwayEnd) []types.SIMCONNECT_DATA_WAYPOINT {
	if len(wps) == 0 {
		return wps
	}
	out := make([]types.SIMCONNECT_DATA_WAYPOINT, len(wps))
	copy(out, wps)
	air := func(w types.SIMCONNECT_DATA_WAYPOINT) bool {
		return w.Flags&uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND) == 0
	}
	top := p.TransitionMaxKts
	for i := range out {
		if top > 0 && air(out[i]) && out[i].KtsSpeed > top {
			out[i].KtsSpeed = top
		}
	}
	if end == nil || p.StartKts <= 0 {
		return out
	}
	t := end.LandingThreshold()
	onFinal := func(w types.SIMCONNECT_DATA_WAYPOINT) bool {
		if !air(w) {
			return false
		}
		q := airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
		back := -alongHeading(t, end.Heading, q)
		cross := calc.HaversineMeters(t.Lat, t.Lon, q.Lat, q.Lon)
		off := math.Sqrt(math.Max(0, cross*cross-back*back))
		return back > 0 && back < airFinalMaxM && off < airFinalOffM
	}
	n := len(out)
	if !onFinal(out[n-1]) {
		return out
	}
	_, align, join := ChainSpeeds(p.StartKts, top)
	out[n-1].KtsSpeed = math.Min(out[n-1].KtsSpeed, join)
	if n >= 2 && onFinal(out[n-2]) {
		out[n-2].KtsSpeed = math.Min(out[n-2].KtsSpeed, align)
	}
	return out
}

// ChainSpeeds is the speeds a chain to the final is flown at (#1682), for
// an approach starting at startKts (ApproachProfile.StartKts) and a type
// whose top speed is topKts (ApproachProfile.TransitionMaxKts, 0: none):
// the transition at ProcedureApproachSpeedKts or the top speed if lower,
// the join at the approach's start speed, the align point halfway between.
// The chain (airWaypoints) and the sequencer's estimate (eta) both use it.
func ChainSpeeds(startKts, topKts float64) (trans, align, join float64) {
	trans = ProcedureApproachSpeedKts
	if topKts > 0 {
		trans = math.Min(trans, topKts)
	}
	join = math.Min(startKts, trans)
	align = math.Max(join, (trans+join)/2)
	return trans, align, join
}

// A chain's last point is on the final (airWaypoints) when it is out on the
// extended centreline, at most airFinalMaxM back and airFinalOffM off it.
const (
	airFinalMaxM = 15 * 1852.0
	airFinalOffM = 150.0
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
		// Passed during the injected climb: left out — at hand, or behind
		// on the track just flown. A fix behind off that track is where a
		// SID turns back to, and is flown (LKPR 24 ARTU 6M: climb to 1700
		// ft, then direct PR407 some 95° off the take-off; EGLL 09L CPT 4K
		// back west to D253K): left out, all of a SID's fixes went and it
		// flew the runway heading for 60 km. A direct-to-fix leg is flown
		// from wherever the aircraft is, never passed.
		if len(out) == 0 && (d < 1852 || n.LegType != types.SIMCONNECT_FACILITY_LEG_TYPE_DF && passedOnClimb(pos, hdg, n.Position)) {
			continue
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

// passedOnClimb reports whether p lies behind pos on the track hdg it
// climbed out on (within a mile of it): passed.
func passedOnClimb(pos airport.LatLon, hdg float64, p airport.LatLon) bool {
	if math.Abs(headingDiff(calc.BearingDegrees(pos.Lat, pos.Lon, p.Lat, p.Lon), hdg)) <= 90 {
		return false // ahead
	}
	bLat, bLon := calc.DisplaceByHeading(pos.Lat, pos.Lon, math.Mod(hdg+180, 360), 1852)
	return math.Abs(calc.CrossTrackMeters(pos.Lat, pos.Lon, bLat, bLon, p.Lat, p.Lon)) < 1852
}

// ArrivalProcedure is the airborne part of an arrival flown by MSFS AI:
// where the aircraft appears, the waypoints to the join point and the join
// point itself, JoinMeters out on the extended centreline, where the
// injected approach takes over.
type ArrivalProcedure struct {
	Spawn     types.SIMCONNECT_DATA_INITPOSITION
	Waypoints []types.SIMCONNECT_DATA_WAYPOINT
	// Names are the fixes of Waypoints ("" none), alike: the radar
	// vectors back onto the STAR name them (#661).
	Names      []string
	Join       airport.LatLon
	JoinMeters float64
	// MinJoinMeters is the nearest to the threshold the injected approach
	// takes over (0: 2 NM, an airliner's final; a circuit's is shorter).
	MinJoinMeters float64
}

// minJoin is p's MinJoinMeters, 2 NM when unset.
func (p *ArrivalProcedure) minJoin() float64 {
	if p.MinJoinMeters > 0 {
		return p.MinJoinMeters
	}
	return 2 * 1852
}

// PlanArrivalProcedure plans the STAR and approach of route to a join
// point joinMeters out on the final of end (threshold elevation fieldFt),
// where the approach is at joinFt MSL: the route's points up to the final,
// then two centreline points (aligned ProcedureAlignNm before the join,
// and the join), a DME arc's points only where they make that way no
// worse (soundArcRoute). Altitudes descend ProcedureDescentFtPerNm
// back from the join point, at most ProcedureTopFt (unless a constraint is higher) and
// within each point's constraints.
func PlanArrivalProcedure(route []airport.NavPoint, end airport.RunwayEnd, joinMeters, joinFt float64) (*ArrivalProcedure, error) {
	return planArrival(soundArcRoute(route, end, joinMeters, joinFt), end, joinMeters, joinFt)
}

// planArrival is PlanArrivalProcedure of route's points as they are.
func planArrival(route []airport.NavPoint, end airport.RunwayEnd, joinMeters, joinFt float64) (*ArrivalProcedure, error) {
	t, hdg := end.LandingThreshold(), end.Heading
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
	if len(pts) == 0 && len(route) > 0 {
		// All of it on the final (#1616: TNCM's RNAV 10 from AVAKI, the
		// intermediate fix, 10 NM out): a straight-in from farther out on
		// the centreline.
		sLat, sLon := calc.DisplaceByHeading(t.Lat, t.Lon, outbound, alignMeters+ProcedureAlignNm*1852)
		pts = append(pts, airport.NavPoint{Position: airport.LatLon{Lat: sLat, Lon: sLon}, Course: hdg})
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
	// A descent never climbs: no point above the one before, so an
	// at-or-below holds on from its fix (live sweep, EDDF 07C via EMPAX:
	// ADNIS at or below 10000 ft, then SPESA planned at 28031 ft, the
	// 3° profile back over a long transition).
	for i := 1; i < len(alts); i++ {
		alts[i] = math.Min(alts[i], alts[i-1])
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
		ap.Names = append(ap.Names, n.Ident)
	}
	ap.Waypoints = append(ap.Waypoints,
		procedureWaypoint(align, joinFt+ProcedureAlignNm*ProcedureDescentFtPerNm, ProcedureApproachSpeedKts),
		procedureWaypoint(join, joinFt, ProcedureApproachSpeedKts))
	ap.Names = append(ap.Names, "", "")
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
	ap.Waypoints, ap.Names = ap.Waypoints[1:], ap.Names[1:]
	return ap, nil
}

// joinBlend fades out the offset between where MSFS AI flew the aircraft
// and the injected approach at the takeover.
type joinBlend struct {
	dLat, dLon, dFt, dHdg float64
	left                  float64 // seconds
	// ftLeft of ftFor seconds: the height's own fade, as long as left's
	// unless that would change the height faster than joinBlendMaxFpm
	// (joinBlendFtSeconds, #1681).
	ftLeft, ftFor float64
	// gateFt: the stabilised-approach gate its approach is judged at
	// (gateFor; judgeGate), 0 none or judged.
	gateFt float64
	// dPitch, dBank: the attitude it had, faded out over attitudeLeft
	// seconds (#1636: rolled level from its base turn in one write).
	dPitch, dBank, attitudeLeft float64
	// attitudeFresh: not applied yet, so a newer answer may still replace it.
	attitudeFresh bool
	// liftDue: dFt is still to the reference point's altitude MSFS AI
	// reported, not yet less its height above the wheels (#1682).
	liftDue bool
}

// attitudeBlendSeconds: out of a 25° banked turn at 7.5°/s at most.
const attitudeBlendSeconds = 5.0

// joinBlendMaxFpm: the most the height changes at the takeover, the
// approach's own descent and the blend together: an airliner's descent
// (#1681: AAL432 put 7,000 ft down onto the final in 30 s). A var for the
// tests.
var joinBlendMaxFpm = 3000.0

// StabilisedGateFt: the stabilised-approach gate of an IFR approach, its
// height above the runway (the airlines' usual criterion): the
// hand-over's height not faded by there, the approach is not stable
// (ArrivalEvent.Unstable). StabilisedVisualGateFt: the visual one, for an
// approach joined too low for the first (a close-in join).
const (
	StabilisedGateFt       = 1000.0
	StabilisedVisualGateFt = 500.0
)

// stableOffsetFt: the hand-over's height still to fade at the gate that
// leaves the approach stable: about a dot of glide path there.
const stableOffsetFt = 100.0

// joinGateMarginFt: a gate is judged only for an approach joined this far
// above it, the descent JoinBlendSeconds of a normal hand-over takes: one
// joined just above it would still be fading a normal offset there.
const joinGateMarginFt = 300.0

// gateFor is the gate an approach joined at joinFt above the runway is
// judged at: StabilisedGateFt, StabilisedVisualGateFt joined too low for
// that, 0 none (a VFR circuit's final, joined at some 300 ft) (#1681).
func gateFor(joinFt float64) float64 {
	for _, g := range []float64{StabilisedGateFt, StabilisedVisualGateFt} {
		if joinFt >= g+joinGateMarginFt {
			return g
		}
	}
	return 0
}

// unstableTouchdownFt: an approach that lands not stable at the gate
// has its hand-over's height faded by this height (SetLandsUnstable).
const unstableTouchdownFt = 50.0

// secondsTo is how long the path p comes down to ft above the runway.
func secondsTo(p ApproachPose, ft float64) float64 {
	return math.Max(p.HeightFt-ft, 0) / math.Max(math.Abs(p.VerticalFpm), 100) * 60
}

// refit fades the height still left over the next seconds instead: on
// from where it is, never a step.
func (b *joinBlend) refit(seconds float64) {
	left := b.heightLeft()
	b.dFt, b.ftFor, b.ftLeft = left, math.Max(seconds, 1), math.Max(seconds, 1)
}

// SetLandsUnstable has the arrival land from an approach not stable at
// the gate, the hand-over's height faded by unstableTouchdownFt, its crew
// not going round: the user's own with held gates (#1096), or a crew with
// nowhere else to go (#1681). Off, its crew goes round where the height
// cannot fade before touchdown.
func (c *ArrivalController) SetLandsUnstable(on bool) {
	c.mu.Lock()
	c.landsUnstable = on
	c.mu.Unlock()
}

// SetLandsOnOccupied has the arrival land with another aircraft on its
// runway at its crew's last look, its crew not going round (#1680): the
// user's own with held gates (#1096). Off, the default, its crew goes
// round (RunwayTaken).
func (c *ArrivalController) SetLandsOnOccupied(on bool) {
	c.mu.Lock()
	c.landsOccupied = on
	c.mu.Unlock()
}

// heightLeft is the hand-over's height offset still shown (after apply).
func (b *joinBlend) heightLeft() float64 {
	if b.ftLeft <= 0 || b.ftFor <= 0 {
		return 0
	}
	k := b.ftLeft / b.ftFor
	return b.dFt * k * k * (3 - 2*k)
}

// judgeGate reports, once, on the frame its path passes its gate (pathFt
// above the runway), whether the approach is not stable there: the
// hand-over's height still more than stableOffsetFt off (#1681).
func (b *joinBlend) judgeGate(pathFt float64) bool {
	if b.gateFt <= 0 || pathFt > b.gateFt {
		return false
	}
	b.gateFt = 0
	return math.Abs(b.heightLeft()) > stableOffsetFt
}

// joinBlendMinRoomFpm: what the blend may take at least, however steep
// the approach is where it takes over.
const joinBlendMinRoomFpm = 1000.0

// joinBlendFtSeconds is how long the height offset dFt fades at the
// takeover onto an approach descending pathFpm: JoinBlendSeconds, or
// longer where that would be faster than joinBlendMaxFpm with the
// approach's descent (the fade's steepest is 1.5 times its average).
func joinBlendFtSeconds(dFt, pathFpm float64) float64 {
	room := math.Max(joinBlendMaxFpm-math.Abs(pathFpm), joinBlendMinRoomFpm)
	return math.Max(JoinBlendSeconds, 1.5*math.Abs(dFt)/room*60)
}

// ftShare is the share of dFt apply added last: the height fade's
// smoothed factor, 0 once faded.
func (b *joinBlend) ftShare() float64 {
	if b.ftLeft <= 0 || b.ftFor <= 0 {
		return 0
	}
	k := b.ftLeft / b.ftFor
	return k * k * (3 - 2*k)
}

func (b *joinBlend) apply(p ApproachPose, dt float64) ApproachPose {
	b.attitudeFresh = false
	if b.attitudeLeft > 0 {
		b.attitudeLeft = math.Max(0, b.attitudeLeft-dt)
		k := b.attitudeLeft / attitudeBlendSeconds
		k = k * k * (3 - 2*k)
		p.PitchDeg += b.dPitch * k
		p.BankDeg += b.dBank * k
	}
	if b.left > 0 {
		b.left = math.Max(0, b.left-dt)
		k := b.left / JoinBlendSeconds
		k = k * k * (3 - 2*k) // smooth: no jerk at either end
		p.Position.Lat += b.dLat * k
		p.Position.Lon += b.dLon * k
		p.Heading = math.Mod(p.Heading+b.dHdg*k+360, 360)
	}
	if b.ftLeft > 0 {
		b.ftLeft = math.Max(0, b.ftLeft-dt)
		k := b.ftLeft / b.ftFor
		k = k * k * (3 - 2*k)
		p.HeightFt += b.dFt * k
	}
	return p
}

// MissedJoinNM: an arrival flown by MSFS AI this much farther from the
// threshold than any point left on its route (its dog-legs, orbits and
// extended downwind included) is off its route and will not join the final
// (live, TVS1972 flew heading 065 off LOMK8S's end for 38 minutes until
// cancelled as stuck).
const MissedJoinNM = 5.0

// watchJoin reports an arrival off its route (MissedJoin), once until it
// is back within it.
func (c *ArrivalController) watchJoin(pos airport.LatLon, m arrivalMonitor) {
	if c.circuit || c.holding != nil {
		return
	}
	wps := c.proc.Waypoints
	thr := c.plan.End.LandingThreshold()
	// Where it appeared counts too while it flies to its first waypoint:
	// the waypoints start after it (live, every arrival flagged at its STAR
	// entry on its first frame).
	k := c.procWaypoint(wps)
	far := 0.0
	if k == 0 {
		far = calc.HaversineMeters(thr.Lat, thr.Lon, c.proc.Spawn.Latitude, c.proc.Spawn.Longitude)
	}
	for _, w := range wps[max(0, k-1):] {
		far = math.Max(far, calc.HaversineMeters(thr.Lat, thr.Lon, w.Latitude, w.Longitude))
	}
	d := calc.HaversineMeters(thr.Lat, thr.Lon, pos.Lat, pos.Lon)
	away := c.lastRunwayM > 0 && d > c.lastRunwayM // moving away, not just placed far
	c.lastRunwayM = d
	switch {
	case far == 0:
	case d > far+MissedJoinNM*1852+swingMeters(m.GroundKts) && away && c.joinMinM >= 0:
		c.last.MissedJoin = fmt.Sprintf("off its route: %.1f NM from the runway, its route %.1f NM at most; heading %03.0f, %.0f ft, %.0f kt",
			d/1852, far/1852, m.Heading, m.AltFt, m.GroundKts)
		c.joinMinM = -1
	case d <= far:
		c.joinMinM = 0 // back on it: a new report if it strays again
	}
}

// startProcedure hands the arrival to MSFS AI for the STAR and approach:
// gear up, arrival lights, the procedure waypoints, position every second.
func (c *ArrivalController) startProcedure() error {
	if err := c.fleet.ReleaseControl(c.objectID, c.reqBase+arrReqRelease); err != nil {
		return err
	}
	c.setLights(false, false, true, true, true, "lights arrival")
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, c.airWaypoints(c.proc.Waypoints)); err != nil {
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
	// Going around: the gear up once well clear of the runway.
	if c.circuit && !c.gaGearUp && m.AltFt != 0 && m.AltFt-c.plan.Runway.Altitude/0.3048 > GoAroundGearUpFt {
		c.gaGearUp = true
		if client := c.fleet.clientOrNil(); client != nil {
			gear := [1]float64{0}
			c.note("go-around: gear up", client.SetDataOnSimObject(c.defBase+arrDefGear, c.objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(gear)), unsafe.Pointer(&gear)))
		}
	}
	t, far := c.plan.End.LandingThreshold(), c.proc.Join
	along := calc.AlongTrackMeters(t.Lat, t.Lon, far.Lat, far.Lon, pos.Lat, pos.Lon)
	cross := math.Abs(calc.CrossTrackMeters(t.Lat, t.Lon, far.Lat, far.Lon, pos.Lat, pos.Lon))
	near := calc.HaversineMeters(pos.Lat, pos.Lon, far.Lat, far.Lon) < JoinCaptureMeters
	established := along > c.proc.minJoin() && along <= c.proc.JoinMeters+300 && cross < 2000 && math.Abs(headingDiff(m.Heading, c.plan.End.Heading)) < 45
	if c.circuit && c.procWaypoint(c.proc.Waypoints) < len(c.proc.Waypoints)-2 {
		near, established = false, false // still going around
	}
	if near || established || c.nearHandOver(pos, t) {
		c.askAttitude()
	}
	if !near && !established {
		c.watchJoin(pos, m)
		c.emit(nil, c.last.MissedJoin != "" && c.joinMinM < 0)
		return
	}
	start := math.Max(c.proc.minJoin(), math.Min(along, c.proc.JoinMeters+JoinCaptureMeters))
	above := c.aboveRunwayFt(m)
	// Too high to make the gate: not joined to go round from it, but given
	// the track miles to come down first (joinHighFt, extendForHeight).
	if c.heightExtends < maxHeightExtends {
		if high := c.joinHighFt(start, above); high > 0 {
			err := c.extendForHeight(m, along, above, high)
			if err == nil {
				c.emit(nil, true)
				return
			}
			c.note("extending for height", err)
		}
	}
	c.last.MissedJoin, c.joinMinM, c.last.Unstable, c.last.CrewGoAround, c.last.RunwayTaken = "", 0, false, false, false
	c.flyingProc, c.circuit, c.heightExtends = false, false, 0
	if err := c.startInjectedApproach(start); err != nil {
		c.fail(err)
		return
	}
	p := c.approach.Pose()
	// MSFS AI's altitude is its reference point's, the pose's height its
	// wheels': the reference point's height above them comes off before
	// the first write (liftDue, onApproachFrame), so it is written where
	// MSFS AI had it (#1682: 9 ft up in one write).
	c.blend = joinBlend{dLat: pos.Lat - p.Position.Lat, dLon: pos.Lon - p.Position.Lon, dFt: above - p.HeightFt,
		dHdg: headingDiff(p.Heading, m.Heading), left: JoinBlendSeconds,
		attitudeFresh: true, liftDue: true}
	// From the speed MSFS AI flew it at, eased to the approach's (#1682: a
	// DHC6 from 180 to 87 kt in one write).
	c.approach.EaseFrom(m.GroundKts)
	// The height at an airliner's rate at most (#1681): over JoinBlendSeconds
	// for every arrival MSFS AI brings in near its path.
	path := joinPathFpm(p)
	c.blend.ftFor = joinBlendFtSeconds(c.blend.dFt, path)
	// One that lands from it anyway (SetLandsUnstable) fades it by
	// unstableTouchdownFt from here: one fade over the whole final, not a
	// steeper one from the gate.
	if t := math.Max(p.HeightFt-unstableTouchdownFt, 0) / path * 60; c.landsUnstable && c.blend.ftFor > t {
		c.blend.ftFor = math.Max(t, JoinBlendSeconds)
	}
	c.blend.ftLeft = c.blend.ftFor
	c.blend.gateFt = gateFor(p.HeightFt)
	c.blendAttitude()
	c.note("joined the final: injected approach", nil)
}

// swingMeters is the radius of a turn at kts with MSFS AI's bank
// (TurnBankDeg): how wide a fast aircraft swings out of a turn without
// being off its route.
func swingMeters(kts float64) float64 {
	v := kts * 1852 / 3600
	return v * v / (9.80665 * math.Tan(TurnBankDeg*math.Pi/180))
}
