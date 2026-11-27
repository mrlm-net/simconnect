package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// maxHeightExtends: the ways out and back an arrival too high at its join
// is given (extendForHeight) before it joins as it is, its crew going
// round if it is still not stable at the gate (#1681).
const maxHeightExtends = 2

// aboveRunwayFt is how high above the runway MSFS AI has it (m): MSL
// (ground under it plus its height above it) less the runway's
// elevation. From its MSL altitude: the ground the injector last saw
// under it may be from elsewhere while MSFS AI flew it (live, a 341 ft
// step at the takeover).
func (c *ArrivalController) aboveRunwayFt(m arrivalMonitor) float64 {
	runwayFt := c.plan.Runway.Altitude / 0.3048
	if m.AltFt != 0 {
		return m.AltFt - runwayFt
	}
	if g, ok := c.inj.GroundFt(c.objectID); ok {
		return g + m.AGL - runwayFt
	}
	return m.AGL
}

// joinPathFpm is the descent the hand-over's height fades against at a
// join onto the approach at p: its own, at least ProcedureDescentFtPerNm
// at its speed.
func joinPathFpm(p ApproachPose) float64 {
	return math.Max(math.Abs(p.VerticalFpm), ProcedureDescentFtPerNm*p.GroundSpeedKts/60)
}

// gateLeftFt is the hand-over's height still to fade at the gate of an
// approach joined heightFt above the runway, descending pathFpm, with
// MSFS AI dFt above it: the blend the join sets (joinBlendFtSeconds),
// as judgeGate finds it there. 0 with no gate (a VFR circuit's final).
func gateLeftFt(dFt, heightFt, pathFpm float64) float64 {
	gate := gateFor(heightFt)
	if gate <= 0 || pathFpm <= 0 {
		return 0
	}
	b := joinBlend{dFt: dFt, ftFor: joinBlendFtSeconds(dFt, pathFpm)}
	b.ftLeft = math.Max(b.ftFor-(heightFt-gate)/pathFpm*60, 0)
	return math.Abs(b.heightLeft())
}

// joinHighFt is how far above its final MSFS AI is (above, feet over the
// runway) at a join start metres out, where that is more than the
// hand-over could fade by the gate: 0 where it can, below the final, with
// no gate, or for one that lands from it anyway (SetLandsUnstable). It
// would join only to go round (#1681 follow-up: DLH1691 handed over at
// FL289 36 NM out, joined thousands of feet high, cleared to land and
// round at once). c.mu held.
func (c *ArrivalController) joinHighFt(start, above float64) float64 {
	if c.landsUnstable {
		return 0
	}
	end := c.plan.End
	p := NewApproachMover(end.LandingThreshold(), end.Heading, start, c.approachProfile()).Pose()
	dFt := above - p.HeightFt
	if dFt <= stableOffsetFt || gateLeftFt(dFt, p.HeightFt, joinPathFpm(p)) <= stableOffsetFt {
		return 0
	}
	return dFt
}

// heightExtendDescent is the way out and back of an arrival highFt above
// its final, from abeam it fromNM out to its join joinNM out at joinFt:
// the downwind on out ext NM further where the way at
// ProcedureDescentFtPerNm is too short, and the downwind's end and the
// align point at the altitudes of that descent (as goAroundDescent).
func heightExtendDescent(fromNM, joinNM, joinFt, fromFt float64) (ext, downwindFt, alignFt float64) {
	baseNM := math.Hypot(ProcedureAlignNm-1, GoAroundOffsetNm)
	need := (fromFt - joinFt) / ProcedureDescentFtPerNm
	have := math.Max(joinNM-fromNM, 0) + 1 + baseNM + ProcedureAlignNm
	ext = math.Max(0, (need-have)/2)
	alignFt = joinFt + (ProcedureAlignNm+ext)*ProcedureDescentFtPerNm
	downwindFt = math.Min(fromFt, alignFt+baseNM*ProcedureDescentFtPerNm)
	return ext, downwindFt, alignFt
}

// extendForHeight gives an arrival on its procedure, at its join along
// metres out but highFt above its final (joinHighFt), the track miles to
// come down instead of the final, as a controller does one high on the
// localizer: out to the side, along a downwind as long as its descent
// needs, and back onto the final at its join, the circuit a go-around
// flies (goAroundCircuitSide) but no go-around: MSFS AI flies it, and it
// joins as before once on its final again. above: its height over the
// runway now. c.mu held.
func (c *ArrivalController) extendForHeight(m arrivalMonitor, along, above, highFt float64) error {
	end := c.plan.End
	t, hdg := end.LandingThreshold(), end.Heading
	join := c.proc.JoinMeters
	runwayFt := c.plan.Runway.Altitude / 0.3048
	joinFt := runwayFt + NewApproachMover(t, hdg, join, c.approachProfile()).Pose().HeightFt
	fromFt := runwayFt + above
	fromNM := math.Min(along, join) / 1852
	ext, downwindFt, alignFt := heightExtendDescent(fromNM, join/1852, joinFt, fromFt)
	side := c.goAroundCircuitSide(t, hdg, join/1852)
	at := func(alongNm, sideNm float64) airport.LatLon {
		return offsetHeading(offsetHeading(t, hdg, alongNm*1852), side, sideNm*1852)
	}
	out := at(-fromNM, GoAroundOffsetNm)
	downwindEnd := at(-join/1852-1-ext, GoAroundOffsetNm)
	align := at(-join/1852-ProcedureAlignNm-ext, 0)
	joinAt := at(-join/1852, 0)
	outFt := math.Min(fromFt, downwindFt+(join/1852-fromNM+1+ext)*ProcedureDescentFtPerNm)
	wps := []types.SIMCONNECT_DATA_WAYPOINT{
		procedureWaypoint(out, outFt, ProcedureApproachSpeedKts),
		procedureWaypoint(downwindEnd, downwindFt, ProcedureApproachSpeedKts),
		procedureWaypoint(align, alignFt, ProcedureApproachSpeedKts),
		procedureWaypoint(joinAt, joinFt, ProcedureApproachSpeedKts),
	}
	c.setCorners(wps, []string{"DOWNWIND", "BASE", "", "FINAL"})
	c.cornerNext = 0
	wps = roundedChain(c.last.Position, wps, MaxBankDeg(*c.aircraft()))
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, c.airWaypoints(wps)); err != nil {
		return err
	}
	c.proc = &ArrivalProcedure{Waypoints: wps, Join: joinAt, JoinMeters: join, MinJoinMeters: c.proc.MinJoinMeters}
	c.procNext, c.circuit, c.gaGearUp, c.tromboneNM = 0, true, true, 0
	c.procSpeed = ProcedureApproachSpeedKts
	c.heightExtends++
	c.seq.add(c.now(), "too high for the final: out and back for the descent", above, m.GroundKts)
	c.note("too high for the final: extended for the descent", nil)
	return nil
}
