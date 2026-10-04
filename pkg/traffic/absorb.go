//go:build windows
// +build windows

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

// Delay absorption (#391): an arrival the sequencer delays loses the time
// in the air before it would hold, the way approach control does it —
// first by flying slower on the STAR, then by a longer path (a dog-leg
// off the route, as vectors would give), and what still remains goes to
// the hold (#392). The final part of the approach (align and join points)
// is never touched.

// Speeds and limits of the absorption.
const (
	// MinProcedureSpeedKts is the slowest an arrival is asked to fly on the
	// STAR, clean (jets); MinProcedureSpeedTurbopropKts for turboprops.
	MinProcedureSpeedKts          = 210.0
	MinProcedureSpeedTurbopropKts = 170.0
	// MaxStretchNM is the most track path stretching adds.
	MaxStretchNM = 30.0
	// MinStretchLegNM: a leg shorter than this is not stretched.
	MinStretchLegNM = 3.0
)

// Absorption is how an arrival loses a delay.
type Absorption struct {
	// SpeedKts is the speed assigned on the STAR (0: unchanged).
	SpeedKts float64 `json:"speedKts,omitempty"`
	// ExtraNM is the track added by path stretching.
	ExtraNM float64 `json:"extraNM,omitempty"`
	// Orbit: the stretch is a 360 where it is (near the end of the STAR),
	// the way it turns.
	Orbit string `json:"orbit,omitempty"` // "left" or "right"
		// Left is what neither absorbs: for the hold.
	Left time.Duration `json:"left,omitempty"`
}

func (a Absorption) String() string {
	s := ""
	if a.SpeedKts > 0 {
		s = fmt.Sprintf("%.0f kt", a.SpeedKts)
	}
	if a.ExtraNM > 0 {
		if s != "" {
			s += ", "
		}
		s += fmt.Sprintf("+%.1f NM", a.ExtraNM)
		if a.Orbit != "" {
			s += " (360)"
		}
	}
	if a.Left > 0 {
		if s != "" {
			s += ", "
		}
		s += fmt.Sprintf("%s left to hold", a.Left.Round(time.Second))
	}
	return s
}

// PlanAbsorption plans losing delay over starNM still to fly at speedKts:
// a speed down to minKts that loses it all, else minKts and a path
// stretch at it of at most MaxStretchNM, and what is left.
func PlanAbsorption(delay time.Duration, starNM, speedKts, minKts float64) Absorption {
	if delay <= 0 || starNM <= 0 || speedKts <= 0 {
		return Absorption{}
	}
	h := delay.Hours()
	minKts = math.Min(minKts, speedKts)
	// Speeds as ATC gives them, in tens of knots ("210", not "239"): down
	// to the ten below, which loses a little more; the rest is asked again.
	if v := math.Floor(starNM/(starNM/speedKts+h)/10) * 10; v >= minKts {
		return Absorption{SpeedKts: v}
	}
	a := Absorption{SpeedKts: minKts}
	rest := h - (starNM/minKts - starNM/speedKts) // hours after slowing down
	a.ExtraNM = rest * minKts
	if a.ExtraNM > MaxStretchNM {
		a.Left = time.Duration((a.ExtraNM - MaxStretchNM) / minKts * float64(time.Hour))
		a.ExtraNM = MaxStretchNM
	}
	return a
}

// StretchLeg is the apex of a dog-leg on the leg a → b that makes it
// extraNM longer: abeam the middle of the leg, off to the given side
// (+1 right, -1 left of the track).
func StretchLeg(a, b airport.LatLon, extraNM float64, side float64) airport.LatLon {
	l := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
	half, hyp := l/2, (l+extraNM)/2
	x := math.Sqrt(math.Max(0, hyp*hyp-half*half))
	brg := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
	mLat, mLon := calc.DisplaceByHeading(a.Lat, a.Lon, brg, half*1852)
	lat, lon := calc.DisplaceByHeading(mLat, mLon, math.Mod(brg+90*side+360, 360), x*1852)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// ErrNotOnProcedure is returned when an arrival is not flying its STAR;
// ErrHolding when it is in a hold (it loses its delay there).
var (
	ErrNotOnProcedure = errors.New("traffic: not flying the STAR")
	ErrHolding        = errors.New("traffic: holding")
)

// AbsorbDelay has an arrival on its STAR (MSFS AI, before the final) lose
// delay: slower on the rest of the STAR and, if need be, a longer downwind —
// on along it past the last STAR point and onto the final that much further
// out (a trombone, once an approach) — or where the STAR does not end on a
// downwind, a dog-leg on its longest leg ahead; the waypoints are sent again. It returns how, with
// what is left for the hold. Call it with the sequencer's delay; a later
// call adds to what was absorbed (the sequencer sees the slower, longer
// flight and asks for the rest).
func (c *ArrivalController) AbsorbDelay(delay time.Duration) (Absorption, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil || len(c.proc.Waypoints) < 3 {
		return Absorption{}, ErrNotOnProcedure
	}
	if c.holding != nil {
		return Absorption{}, ErrHolding
	}
	if c.req.Circuit != nil {
		return c.absorbInCircuit(delay) // a VFR circuit: its downwind, then an orbit (#569)
	}
	pos := c.last.Position
	wps := c.proc.Waypoints
	final := len(wps) - 2 // align and join: never touched
	next := c.procWaypoint(wps[:final])
	if next >= final || pos == (airport.LatLon{}) {
		return Absorption{}, ErrNotOnProcedure // on the final
	}
	// The track still to fly as flown, its turns rounded.
	before := pathNMOf(pos, wps[next:final], wps[final])
	names := make([]string, len(wps))
	if len(c.corners) >= 3 { // re-planned from the corners, not the rounded turns
		wps, names, final = c.corners, c.cornerNames, len(c.corners)-2
		if next = c.cornerAhead(); next >= final {
			return Absorption{}, ErrNotOnProcedure
		}
		// Measured as the stretched track will be: rounded from here.
		r := roundedChain(pos, wps[next:], MaxBankDeg(*c.aircraft()))
		before = pathNMOf(pos, r[:len(r)-2], r[len(r)-2])
	}
	// The STAR still to fly: to the next point, then on to the align point.
	pts := []airport.LatLon{pos}
	for _, w := range wps[next : final+1] {
		pts = append(pts, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude})
	}
	starNM := 0.0
	for i := 1; i < len(pts); i++ {
		starNM += calc.HaversineNM(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
	}
	speed := c.procSpeed
	if speed == 0 {
		speed = ProcedureSpeedKts
	}
	minKts := MinProcedureSpeedKts
	if c.aircraft().Category == CategoryTurboprop {
		minKts = MinProcedureSpeedTurbopropKts
	}
	if c.req.Circuit != nil {
		// In the circuit (#569): at circuit speed throughout; the delay all
		// in a longer downwind, as a tower extends it.
		speed = CircuitKts(*c.aircraft())
		minKts = speed
	}
	a := PlanAbsorption(delay, starNM, speed, minKts)
	if a.SpeedKts >= speed {
		a.SpeedKts = 0 // flying it already: nothing to say about the speed
	}
	if a.SpeedKts == 0 && a.ExtraNM == 0 {
		return a, nil
	}
	// The rest of the STAR at the new speed (never faster than it was).
	var out []types.SIMCONNECT_DATA_WAYPOINT
	outNames := append([]string(nil), names[next:final]...)
	for _, w := range wps[next:final] {
		if a.SpeedKts > 0 {
			w.KtsSpeed = math.Min(w.KtsSpeed, a.SpeedKts)
		}
		out = append(out, w)
	}
	stretched := false
	// A longer downwind, the way a controller extends it: on along the
	// downwind past its last point, the base turn and the final that much
	// further out (each mile on adds two), again as more is asked, up to
	// MaxStretchNM an approach; beyond that the hold.
	if a.ExtraNM > 0 && len(out) > 0 {
		x := math.Min(a.ExtraNM/2, MaxStretchNM/2-c.tromboneNM)
		if x > 0.2 {
			if ext, k, f, ok := extendDownwindAt(pos, out, wps[final], wps[final+1], x); ok {
				out, c.tromboneNM, stretched = ext, c.tromboneNM+x, true
				// The old base turn is on the downwind now; the new one after it.
				kept := append([]string(nil), outNames[:k+1]...)
				for i := range kept {
					if kept[i] == "BASE" {
						kept[i] = ""
					}
				}
				outNames = append(append(kept, "BASE", ""), outNames[f:]...)
				if lost := a.ExtraNM - 2*x; lost > 0 {
					a.Left += time.Duration(lost / math.Max(a.SpeedKts, speed) * float64(time.Hour))
				}
			}
		}
		if !stretched && c.tromboneNM > 0 { // extended already, as far as it goes
			a.Left += time.Duration(a.ExtraNM / math.Max(a.SpeedKts, speed) * float64(time.Hour))
			a.ExtraNM = 0
		}
	}
	// Under a mile is not worth a turn: the hold, or the sequencer asks again.
	if a.ExtraNM > 0 && a.ExtraNM < 1 && !stretched {
		a.Left += time.Duration(a.ExtraNM / math.Max(a.SpeedKts, speed) * float64(time.Hour))
		a.ExtraNM = 0
	}
	// The dog-leg on the longest leg ahead (from here, or between STAR
	// points), off to the side away from the runway's centreline: only where
	// the STAR has no downwind to extend.
	if a.ExtraNM > 0 && !stretched {
		longest, at := 0.0, -1
		for i := 1; i < len(pts)-1; i++ { // not the leg into the align point
			if l := calc.HaversineNM(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon); l > longest {
				longest, at = l, i
			}
		}
		if at < 0 || longest < MinStretchLegNM {
			// No leg long enough (near the end of the STAR): vectors from
			// where it is, out and back to its next point — a hold is for
			// long delays only (live, LOT775 held at PR532 for a minute).
			// About one turn's worth or more: a 360 where it is, smoother
			// than out and back on a short leg (live, OKYDV).
			at = 1
			if orbit, nm, side, ok := c.orbitHere(pts, a, speed); ok && a.ExtraNM >= OrbitFromShare*nm {
				if lost := a.ExtraNM - nm; lost > 0 {
					a.Left += time.Duration(lost / math.Max(a.SpeedKts, speed) * float64(time.Hour))
				}
				out = append(orbit, out...)
				outNames = append(make([]string, len(orbit)), outNames...)
				a.Orbit, at = side, -1
			}
		}
		if at > 0 {
			from, to := pts[at-1], pts[at]
			side := 1.0
			thr := c.plan.End.Threshold
			if calc.CrossTrackMeters(from.Lat, from.Lon, to.Lat, to.Lon, thr.Lat, thr.Lon) > 0 {
				side = -1 // the runway is to the right: stretch to the left
			}
			apex := StretchLeg(from, to, a.ExtraNM, side)
			ref := wps[final] // no STAR point left: the align point's
			if at-1 < len(out) {
				ref = out[at-1]
			}
			alt := ref.Altitude
			if at >= 2 {
				alt = (out[at-2].Altitude + out[at-1].Altitude) / 2
			}
			kts := ref.KtsSpeed
			if a.SpeedKts > 0 {
				kts = math.Min(kts, a.SpeedKts) // never 0 kt: unchanged keeps its own
			}
			wp := procedureWaypoint(apex, alt, kts)
			out = append(out[:at-1], append([]types.SIMCONNECT_DATA_WAYPOINT{wp}, out[at-1:]...)...)
			outNames = append(outNames[:at-1], append([]string{""}, outNames[at-1:]...)...)
		}
	}
	plain := append(append([]types.SIMCONNECT_DATA_WAYPOINT(nil), out...), wps[final:]...)
	plainNames := append(outNames, names[final:]...)
	out = roundedChain(pos, plain, MaxBankDeg(*c.aircraft()))
	if a.ExtraNM > 0 {
		// The track added as flown: the rounded turns included.
		a.ExtraNM = pathNMOf(pos, out[:len(out)-2], out[len(out)-2]) - before
	}
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, out); err != nil {
		return Absorption{}, err
	}
	c.proc.Waypoints, c.procNext = out, 0
	c.corners, c.cornerNames, c.cornerNext = plain, plainNames, 0
	if a.SpeedKts > 0 {
		c.procSpeed = a.SpeedKts
	}
	c.note(fmt.Sprintf("absorbing %s: %s", delay.Round(time.Second), a), nil)
	return a, nil
}

// extendDownwind is out (the STAR ahead, up to the align point) with its
// downwind extended by x NM: on along it past its last point, the base turn
// that much further out and onto the centreline x NM beyond where the STAR
// joined it (its own base turn may be well beyond the align point: LKPR
// VLM6T turns base some 16 NM out), on the glide path's height there. The
// downwind is the last point ahead more than a mile beside the centreline
// reached flying away from the runway; the base turn after it is replaced.
// False where the STAR does not end on a downwind.
func extendDownwind(pos airport.LatLon, out []types.SIMCONNECT_DATA_WAYPOINT, align, join types.SIMCONNECT_DATA_WAYPOINT, x float64) ([]types.SIMCONNECT_DATA_WAYPOINT, bool) {
	ext, _, _, ok := extendDownwindAt(pos, out, align, join, x)
	return ext, ok
}

// extendDownwindAt is extendDownwind, with the index in out of the last
// downwind point kept (k) and of the first point after the new base turn
// and final (f): the result is out[:k+1], the base turn, the point on the
// final, out[f:].
func extendDownwindAt(pos airport.LatLon, out []types.SIMCONNECT_DATA_WAYPOINT, align, join types.SIMCONNECT_DATA_WAYPOINT, x float64) ([]types.SIMCONNECT_DATA_WAYPOINT, int, int, bool) {
	ll := func(w types.SIMCONNECT_DATA_WAYPOINT) airport.LatLon { return airport.LatLon{Lat: w.Latitude, Lon: w.Longitude} }
	a, j := ll(align), ll(join)
	hOut := calc.BearingDegrees(j.Lat, j.Lon, a.Lat, a.Lon) // away from the runway
	x = math.Min(x, MaxStretchNM/2)
	for k := len(out) - 1; k >= 0 && k >= len(out)-16; k-- { // (a rounded base turn has several points)
		d, from := ll(out[k]), pos
		if k > 0 {
			from = ll(out[k-1])
		}
		leg := calc.BearingDegrees(from.Lat, from.Lon, d.Lat, d.Lon)
		cross := math.Abs(calc.CrossTrackMeters(j.Lat, j.Lon, a.Lat, a.Lon, d.Lat, d.Lon))
		if cross < 1852 || math.Abs(headingDiff(leg, hOut)) > 60 {
			continue
		}
		// Where the STAR reaches the centreline after the downwind (else the
		// align point): the new base turn joins x NM beyond it.
		f, onto := len(out), align
		for i := k + 1; i < len(out); i++ {
			p := ll(out[i])
			if math.Abs(calc.CrossTrackMeters(j.Lat, j.Lon, a.Lat, a.Lon, p.Lat, p.Lon)) < 0.3*1852 {
				f, onto = i, out[i]
				break
			}
		}
		dLat, dLon := calc.DisplaceByHeading(d.Lat, d.Lon, hOut, x*1852)
		eLat, eLon := calc.DisplaceByHeading(onto.Latitude, onto.Longitude, hOut, x*1852)
		d2, e := out[k], onto
		d2.Latitude, d2.Longitude = dLat, dLon
		e.Latitude, e.Longitude = eLat, eLon
		e.Altitude = onto.Altitude + x*ProcedureDescentFtPerNm
		ext := append(append([]types.SIMCONNECT_DATA_WAYPOINT(nil), out[:k+1]...), d2, e)
		return append(ext, out[f:]...), k, f, true
	}
	return nil, 0, 0, false
}

// pathNMOf is the track from pos along wps to the point to, in NM.
func pathNMOf(pos airport.LatLon, wps []types.SIMCONNECT_DATA_WAYPOINT, to types.SIMCONNECT_DATA_WAYPOINT) float64 {
	d, at := 0.0, pos
	for _, w := range append(append([]types.SIMCONNECT_DATA_WAYPOINT(nil), wps...), to) {
		d += calc.HaversineNM(at.Lat, at.Lon, w.Latitude, w.Longitude)
		at = airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
	}
	return d
}

// setCorners keeps wps, a procedure's points before its turns are rounded,
// with their names (nil: none). c.mu held.
func (c *ArrivalController) setCorners(wps []types.SIMCONNECT_DATA_WAYPOINT, names []string) {
	c.corners = append([]types.SIMCONNECT_DATA_WAYPOINT(nil), wps...)
	c.cornerNames = make([]string, len(wps))
	copy(c.cornerNames, names)
	c.cornerNext = -1
}

// cornerAhead is the index of the corner the aircraft flies to, tracked
// forward as procWaypoint tracks the rounded points. c.mu held.
func (c *ArrivalController) cornerAhead() int {
	all, pos := c.corners, c.last.Position
	i := c.cornerNext
	if i < 0 {
		i = nextWaypoint(pos, all)
	}
	for i+1 < len(all) {
		n, m := all[i], all[i+1]
		d := calc.HaversineNM(pos.Lat, pos.Lon, n.Latitude, n.Longitude)
		if d >= 1.5 && calc.HaversineNM(pos.Lat, pos.Lon, m.Latitude, m.Longitude) >= calc.HaversineNM(n.Latitude, n.Longitude, m.Latitude, m.Longitude) {
			break // not passed yet
		}
		i++
	}
	c.cornerNext = i
	return i
}

// TurningFinal reports whether the arrival flies the last leg of its
// procedure before the final — the base, into the turn onto the final —
// where approach clears it for the approach.
func (c *ArrivalController) TurningFinal() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.flyingProc && len(c.corners) >= 3 && c.last.Position != (airport.LatLon{}) && c.cornerAhead() >= len(c.corners)-2
}

// CircuitFixes are the named points still ahead of a go-around's circuit
// back to the final ("CROSSWIND", "DOWNWIND", "BASE", "FINAL", or the
// published missed approach's fixes), with their altitude in meters
// (AltMin); none when not flying one.
func (c *ArrivalController) CircuitFixes() []airport.NavPoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.circuit || !c.flyingProc || len(c.corners) == 0 {
		return nil
	}
	var out []airport.NavPoint
	for i := c.cornerAhead(); i < len(c.corners); i++ {
		if n := c.cornerNames[i]; n != "" {
			w := c.corners[i]
			out = append(out, airport.NavPoint{Ident: n, Position: airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}, AltMin: w.Altitude / ftPerMeter})
		}
	}
	return out
}

// ProcedureRoute is the rest of the arrival's STAR and approach as it
// flies it now (with any dog-leg), up to the join point.
func (c *ArrivalController) ProcedureRoute() []airport.LatLon {
	var out []airport.LatLon
	for _, p := range c.ProcedurePlan() {
		out = append(out, p.Position)
	}
	return out
}

// mslAltitude is w's altitude in feet MSL, 0 (none) for one above ground.
func mslAltitude(w types.SIMCONNECT_DATA_WAYPOINT) float64 {
	if w.Flags&uint32(types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL) != 0 {
		return 0
	}
	return w.Altitude
}

// ProcedurePlan is ProcedureRoute with the altitude and speed of each
// point: the vertical profile ahead, for predicting conflicts (#657).
func (c *ArrivalController) ProcedurePlan() []RoutePoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil {
		return nil
	}
	wps := c.proc.Waypoints
	var out []RoutePoint
	if h := c.holding; h != nil { // from the fix on, where it will go on
		// Holding: at the hold's altitude until cleared out of it, not
		// down the STAR (two stacked a level apart are no conflict).
		out = append(out, RoutePoint{Position: h.hold.Fix, AltFt: h.altFt})
		for _, w := range wps[min(h.resume, len(wps)):] {
			out = append(out, RoutePoint{Position: airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}, AltFt: h.altFt, Kts: w.KtsSpeed})
		}
		return out
	}
	for _, w := range wps[c.procWaypoint(wps):] {
		out = append(out, RoutePoint{Position: airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}, AltFt: mslAltitude(w), Kts: w.KtsSpeed})
	}
	return out
}

// procWaypoint is the index in wps (the procedure's waypoints, or the
// first of them) of the one the aircraft flies to: tracked forward from
// procNext, else nextWaypoint. c.mu held.
func (c *ArrivalController) procWaypoint(wps []types.SIMCONNECT_DATA_WAYPOINT) int {
	if c.procNext < 0 || len(wps) == 0 {
		return nextWaypoint(c.last.Position, wps)
	}
	pos, all := c.last.Position, c.proc.Waypoints
	i := c.procNext
	for i+1 < len(all) {
		n, m := all[i], all[i+1]
		d := calc.HaversineNM(pos.Lat, pos.Lon, n.Latitude, n.Longitude)
		if d >= 1.5 && calc.HaversineNM(pos.Lat, pos.Lon, m.Latitude, m.Longitude) >= calc.HaversineNM(n.Latitude, n.Longitude, m.Latitude, m.Longitude) {
			break // not passed yet
		}
		i++
	}
	c.procNext = i
	return min(i, len(wps))
}

// nextWaypoint is the index of the waypoint the aircraft at pos flies to:
// the nearest one, or the one after it once that is passed.
func nextWaypoint(pos airport.LatLon, wps []types.SIMCONNECT_DATA_WAYPOINT) int {
	if len(wps) == 0 {
		return 0
	}
	nearest, best := 0, math.Inf(1)
	for i, w := range wps {
		if d := calc.HaversineNM(pos.Lat, pos.Lon, w.Latitude, w.Longitude); d < best {
			nearest, best = i, d
		}
	}
	if nearest+1 < len(wps) {
		n, m := wps[nearest], wps[nearest+1]
		// Passed: nearer to the next than the nearest is, or on top of it.
		if best < 1.5 || calc.HaversineNM(pos.Lat, pos.Lon, m.Latitude, m.Longitude) < calc.HaversineNM(n.Latitude, n.Longitude, m.Latitude, m.Longitude) {
			return nearest + 1
		}
	}
	return nearest
}

// DirectToJoin sends an arrival on its procedure straight to the join point
// on the final, leaving out the rest of its STAR (a shortcut a controller
// gives to fill a gap): its waypoints become the align and join points.
// It returns ErrNotOnProcedure on the final or off a procedure and
// ErrHolding while holding (LeaveHold first).
func (c *ArrivalController) DirectToJoin() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil || len(c.proc.Waypoints) < 2 {
		return ErrNotOnProcedure
	}
	if c.holding != nil {
		return ErrHolding
	}
	wps := c.proc.Waypoints
	final := wps[len(wps)-2:]
	if c.procWaypoint(wps) >= len(wps)-2 {
		return nil // already on its way to the final
	}
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, final); err != nil {
		return err
	}
	c.proc.Waypoints, c.procNext = append([]types.SIMCONNECT_DATA_WAYPOINT(nil), final...), 0
	c.note("direct to the join point", nil)
	return nil
}

// StopDescent has an arrival on its STAR stop its descent at altFt (no
// lower) for the next forNM of its route, then descend as planned: a level
// that keeps it above traffic merging below it (live, LKPR: AUA529 on VLM6T
// and CSA1871 descending to the same level at the merge, held instead).
// The align and join points are never raised. ErrNotOnProcedure on the
// final, ErrHolding in the hold.
func (c *ArrivalController) StopDescent(altFt, forNM float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil || len(c.proc.Waypoints) < 3 {
		return ErrNotOnProcedure
	}
	if c.holding != nil {
		return ErrHolding
	}
	pos := c.last.Position
	wps := c.proc.Waypoints
	final := len(wps) - 2
	next := c.procWaypoint(wps[:final])
	if next >= final || pos == (airport.LatLon{}) {
		return ErrNotOnProcedure
	}
	raise := func(chain []types.SIMCONNECT_DATA_WAYPOINT, end int) []types.SIMCONNECT_DATA_WAYPOINT {
		out := append([]types.SIMCONNECT_DATA_WAYPOINT(nil), chain...)
		prev, gone := pos, 0.0
		for i := range out[:end] {
			gone += calc.HaversineNM(prev.Lat, prev.Lon, out[i].Latitude, out[i].Longitude)
			prev = airport.LatLon{Lat: out[i].Latitude, Lon: out[i].Longitude}
			if gone > forNM {
				break
			}
			out[i].Altitude = math.Max(out[i].Altitude, altFt)
		}
		return out
	}
	out := raise(wps[next:], final-next)
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, out); err != nil {
		return err
	}
	c.proc.Waypoints, c.procNext = out, 0
	if len(c.corners) >= 3 {
		if k := c.cornerAhead(); k < len(c.corners)-2 {
			c.corners = append(append([]types.SIMCONNECT_DATA_WAYPOINT(nil), c.corners[:k]...), raise(c.corners[k:], len(c.corners)-2-k)...)
		}
	}
	c.note(fmt.Sprintf("stop descent at %.0f ft for %.0f NM", altFt, forNM), nil)
	return nil
}

// OrbitFromShare: near the end of the STAR, a stretch of at least this
// share of a 360's track is flown as the 360 (AbsorbDelay).
const OrbitFromShare = 0.7

// orbitHere is a 360 where the arrival is, at kts (a.SpeedKts when slower),
// standard rate for that speed, turning away from the final (pts: from its
// position on along the STAR), at the altitude it flies to next; and the
// track it adds (NM). c.mu held.
func (c *ArrivalController) orbitHere(pts []airport.LatLon, a Absorption, kts float64) ([]types.SIMCONNECT_DATA_WAYPOINT, float64, string, bool) {
	if len(pts) < 2 || c.proc == nil || len(c.proc.Waypoints) == 0 {
		return nil, 0, "", false
	}
	if a.SpeedKts > 0 && a.SpeedKts < kts {
		kts = a.SpeedKts
	}
	pos, hdg := pts[0], c.last.Heading
	r := turnRadiusMeters(kts, StandardBankDeg(kts, MaxBankDeg(*c.aircraft())))
	turn := 1.0 // right
	thr := c.plan.End.Threshold
	if calc.CrossTrackMeters(pts[0].Lat, pts[0].Lon, pts[1].Lat, pts[1].Lon, thr.Lat, thr.Lon) > 0 {
		turn = -1 // the runway to the right: turn left, away from it
	}
	alt := c.proc.Waypoints[c.procWaypoint(c.proc.Waypoints)].Altitude
	centre := offsetHeading(pos, hdg+90*turn, r)
	from := localBearing(centre, pos)
	var orbit []types.SIMCONNECT_DATA_WAYPOINT
	for k := 1; k <= 8; k++ {
		orbit = append(orbit, procedureWaypoint(offsetHeading(centre, from+turn*45*float64(k), r), alt, kts))
	}
	side := "right"
	if turn < 0 {
		side = "left"
	}
	return orbit, 2 * math.Pi * r / 1852, side, true
}

// ShortcutDescentFtPerNM is the steepest descent a shortcut may leave an
// arrival to the fix it goes direct to: about a 3° path (318 ft/NM), so
// the descent profile still works.
const ShortcutDescentFtPerNM = 320.0

// ShortcutAirportClearNM: a shortcut's straight leg keeps this far from the
// runway (both thresholds and its middle): never across the field (live,
// LKPR: CSA1909 direct PR532 over the airport).
const ShortcutAirportClearNM = 4.0

// ShortcutMinKts: no shortcut before the aircraft is reported this fast
// (flying, its heading known).
const ShortcutMinKts = 100.0

// ShortcutMinNM: a shortcut saving less is not worth the call.
const ShortcutMinNM = 2.0

// Shortcut sends an arrival on its STAR direct to a named fix further on,
// saving up to maxSaveNM of track (the room ahead of it in the sequence)
// where it can still descend to that fix's altitude at
// ShortcutDescentFtPerNM or less, the longest such saving. It returns the
// fix and the track saved; "" when none fits (too high, too little room,
// none named). ErrNotOnProcedure on the final or in a circuit, ErrHolding
// in the hold.
func (c *ArrivalController) Shortcut(maxSaveNM float64) (string, float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil || c.req.Circuit != nil || len(c.corners) < 3 {
		return "", 0, ErrNotOnProcedure
	}
	if c.holding != nil {
		return "", 0, ErrHolding
	}
	pos := c.last.Position
	final := len(c.corners) - 2 // align and join: never left out
	k := c.cornerAhead()
	if pos == (airport.LatLon{}) || k >= final-1 {
		return "", 0, ErrNotOnProcedure
	}
	if c.last.GroundSpeed < ShortcutMinKts {
		return "", 0, nil // not reported flying yet: no heading to turn from
	}
	// Only with a long way to go: near the end of the STAR a direct only
	// muddles the join.
	togo := 0.0
	for j, q := k, pos; j < len(c.corners); j++ {
		w := c.corners[j]
		togo += calc.HaversineNM(q.Lat, q.Lon, w.Latitude, w.Longitude)
		q = airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
	}
	if togo < ShortcutMinToGoNM {
		return "", 0, nil
	}
	altFt := c.last.AGL + convert.MetersToFeet(c.req.Graph.Layout.Altitude)
	best, bestSave := -1, 0.0
	along := 0.0
	prev := pos
	for j := k; j < final; j++ {
		w := c.corners[j]
		p := airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
		along += calc.HaversineNM(prev.Lat, prev.Lon, p.Lat, p.Lon)
		prev = p
		if j == k || c.cornerName(j) == "" || c.pastIAF(c.cornerName(j)) {
			continue // at least one point left out; to a named fix, the IAF at the latest
		}
		direct := calc.HaversineNM(pos.Lat, pos.Lon, p.Lat, p.Lon)
		save := along - direct
		if save <= bestSave || save > maxSaveNM || save < ShortcutMinNM {
			continue
		}
		if c.overAirport(pos, p) || !c.shortcutSensible(pos, p) {
			continue // across the field or the final, too sharp a turn, too close in
		}
		if altFt-w.Altitude > direct*ShortcutDescentFtPerNM {
			continue // too high to make its altitude
		}
		best, bestSave = j, save
	}
	if best < 0 {
		return "", 0, nil
	}
	plain := append([]types.SIMCONNECT_DATA_WAYPOINT(nil), c.corners[best:]...)
	names := append([]string(nil), c.cornerNames[best:]...)
	out := roundedChain(pos, plain, MaxBankDeg(*c.aircraft()))
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, out); err != nil {
		return "", 0, err
	}
	fix := c.cornerName(best)
	c.proc.Waypoints, c.procNext = out, 0
	c.corners, c.cornerNames, c.cornerNext = plain, names, 0
	c.note(fmt.Sprintf("direct %s: %.1f NM shorter", fix, bestSave), nil)
	return fix, bestSave, nil
}

// cornerName is corner j's fix: its name, else the procedure's fix within
// 0.3 NM of it ("" none: a computed point). c.mu held.
func (c *ArrivalController) cornerName(j int) string {
	if j < len(c.cornerNames) && c.cornerNames[j] != "" {
		return c.cornerNames[j]
	}
	w := c.corners[j]
	for _, n := range c.req.Procedure {
		if n.Ident != "" && calc.HaversineNM(w.Latitude, w.Longitude, n.Position.Lat, n.Position.Lon) < 0.3 {
			return n.Ident
		}
	}
	return ""
}

// overAirport reports a straight leg a→b passing within
// ShortcutAirportClearNM of the runway (its thresholds and its middle).
// c.mu held.
func (c *ArrivalController) overAirport(a, b airport.LatLon) bool {
	r := c.plan.Runway
	mid := airport.LatLon{Lat: (r.Primary.Threshold.Lat + r.Secondary.Threshold.Lat) / 2, Lon: (r.Primary.Threshold.Lon + r.Secondary.Threshold.Lon) / 2}
	for _, p := range []airport.LatLon{r.Primary.Threshold, r.Secondary.Threshold, mid} {
		if legDistNM(p, a, b) < ShortcutAirportClearNM {
			return true
		}
	}
	return false
}

// legDistNM is the distance from p to the leg a→b, in NM (flat earth: legs
// of tens of miles).
func legDistNM(p, a, b airport.LatLon) float64 {
	k := math.Cos(a.Lat * math.Pi / 180)
	ax, ay := 0.0, 0.0
	bx, by := (b.Lon-a.Lon)*k*60, (b.Lat-a.Lat)*60
	px, py := (p.Lon-a.Lon)*k*60, (p.Lat-a.Lat)*60
	l2 := bx*bx + by*by
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, (px*bx+py*by)/l2))
	}
	dx, dy := px-(ax+t*bx), py-(ay+t*by)
	return math.Hypot(dx, dy)
}

// Where a shortcut makes sense (shortcutSensible), besides keeping off the
// field (ShortcutAirportClearNM): a turn of ShortcutMaxTurnDeg at most to
// the fix, the fix ShortcutFixFromThresholdNM or more from the threshold,
// and the leg not across the final within ShortcutFinalClearNM of the
// threshold; and only with ShortcutMinToGoNM or more of the STAR left.
const (
	ShortcutMaxTurnDeg         = 60.0
	ShortcutFixFromThresholdNM = 10.0
	ShortcutFinalClearNM       = 20.0
	ShortcutMinToGoNM          = 20.0
)

// shortcutSensible reports whether direct from pos to fix makes sense
// (see the constants). c.mu held.
func (c *ArrivalController) shortcutSensible(pos, fix airport.LatLon) bool {
	thr := c.plan.End.Threshold
	if calc.HaversineNM(fix.Lat, fix.Lon, thr.Lat, thr.Lon) < ShortcutFixFromThresholdNM {
		return false
	}
	brg := calc.BearingDegrees(pos.Lat, pos.Lon, fix.Lat, fix.Lon)
	if math.Abs(headingDiff(c.last.Heading, brg)) > ShortcutMaxTurnDeg {
		return false
	}
	// The final: from the threshold out against the landing direction.
	out := offsetHeading(thr, c.plan.End.Heading+180, ShortcutFinalClearNM*1852)
	return !segmentsCross(pos, fix, thr, out)
}

// segmentsCross reports whether legs a1→a2 and b1→b2 cross (flat earth).
func segmentsCross(a1, a2, b1, b2 airport.LatLon) bool {
	k := math.Cos(a1.Lat * math.Pi / 180)
	xy := func(p airport.LatLon) (float64, float64) { return (p.Lon - a1.Lon) * k, p.Lat - a1.Lat }
	ax1, ay1 := xy(a1)
	ax2, ay2 := xy(a2)
	bx1, by1 := xy(b1)
	bx2, by2 := xy(b2)
	side := func(x1, y1, x2, y2, px, py float64) float64 { return (x2-x1)*(py-y1) - (y2-y1)*(px-x1) }
	d1, d2 := side(bx1, by1, bx2, by2, ax1, ay1), side(bx1, by1, bx2, by2, ax2, ay2)
	d3, d4 := side(ax1, ay1, ax2, ay2, bx1, by1), side(ax1, ay1, ax2, ay2, bx2, by2)
	return d1*d2 < 0 && d3*d4 < 0
}

// pastIAF reports a fix after the procedure's initial approach fix: a
// shortcut goes to the IAF at the latest (live, LKPR: TVS1442 sent direct
// PR532, a point on the approach, where ERASU was the one). False when the
// procedure marks no IAF. c.mu held.
func (c *ArrivalController) pastIAF(fix string) bool {
	iaf := -1
	at := -1
	for i, n := range c.req.Procedure {
		if n.IAF && iaf < 0 {
			iaf = i
		}
		if n.Ident == fix && at < 0 {
			at = i
		}
	}
	return iaf >= 0 && at > iaf
}
