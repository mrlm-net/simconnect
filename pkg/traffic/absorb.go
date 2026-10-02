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
			at = 1
		}
		{
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil {
		return nil
	}
	wps := c.proc.Waypoints
	if h := c.holding; h != nil { // from the fix on, where it will go on
		out := []airport.LatLon{h.hold.Fix}
		for _, w := range wps[min(h.resume, len(wps)):] {
			out = append(out, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude})
		}
		return out
	}
	next := c.procWaypoint(wps)
	var out []airport.LatLon
	for _, w := range wps[next:] {
		out = append(out, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude})
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
