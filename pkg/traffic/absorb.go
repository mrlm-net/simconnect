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
	if v := starNM / (starNM/speedKts + h); v >= minKts {
		return Absorption{SpeedKts: math.Round(v)}
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
// delay: slower on the rest of the STAR and, if need be, a dog-leg on its
// longest leg ahead; the waypoints are sent again. It returns how, with
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
	if a.SpeedKts == 0 && a.ExtraNM == 0 {
		return a, nil
	}
	// The rest of the STAR at the new speed (never faster than it was).
	var out []types.SIMCONNECT_DATA_WAYPOINT
	for _, w := range wps[next:final] {
		if a.SpeedKts > 0 {
			w.KtsSpeed = math.Min(w.KtsSpeed, a.SpeedKts)
		}
		out = append(out, w)
	}
	// The dog-leg on the longest leg ahead (from here, or between STAR
	// points), off to the side away from the runway's centreline.
	if a.ExtraNM > 0 {
		longest, at := 0.0, -1
		for i := 1; i < len(pts)-1; i++ { // not the leg into the align point
			if l := calc.HaversineNM(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon); l > longest {
				longest, at = l, i
			}
		}
		if at < 0 || longest < MinStretchLegNM {
			a.Left += time.Duration(a.ExtraNM / a.SpeedKts * float64(time.Hour))
			a.ExtraNM = 0
		} else {
			from, to := pts[at-1], pts[at]
			side := 1.0
			thr := c.plan.End.Threshold
			if calc.CrossTrackMeters(from.Lat, from.Lon, to.Lat, to.Lon, thr.Lat, thr.Lon) > 0 {
				side = -1 // the runway is to the right: stretch to the left
			}
			apex := StretchLeg(from, to, a.ExtraNM, side)
			alt := out[at-1].Altitude
			if at >= 2 {
				alt = (out[at-2].Altitude + out[at-1].Altitude) / 2
			}
			wp := procedureWaypoint(apex, alt, math.Min(out[at-1].KtsSpeed, a.SpeedKts))
			out = append(out[:at-1], append([]types.SIMCONNECT_DATA_WAYPOINT{wp}, out[at-1:]...)...)
		}
	}
	out = append(out, wps[final:]...)
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, out); err != nil {
		return Absorption{}, err
	}
	c.proc.Waypoints, c.procNext = out, 0
	if a.SpeedKts > 0 {
		c.procSpeed = a.SpeedKts
	}
	c.note(fmt.Sprintf("absorbing %s: %s", delay.Round(time.Second), a), nil)
	return a, nil
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
