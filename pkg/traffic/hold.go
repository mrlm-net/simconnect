//go:build windows
// +build windows

package traffic

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Holding patterns (#392), ours: the simulator's HOLDING_PATTERN facility
// data is unusable (probing it crashed MSFS 2024). A hold is a racetrack
// on a fix — inbound course, turn direction, legs by time — entered the
// ICAO way (direct, teardrop or parallel, from the heading), flown by MSFS
// AI as a waypoint chain that wraps to its first point, and stacked at
// 1000 ft levels, left from the bottom.

// Hold is a holding pattern.
type Hold struct {
	Ident       string         `json:"ident"`
	Fix         airport.LatLon `json:"fix"`
	InboundTrue float64        `json:"inboundTrue"` // the inbound course, degrees true
	LeftTurns   bool           `json:"leftTurns"`   // standard is right
}

// HoldSpeedKts is the ICAO maximum holding speed at an altitude (Doc 8168):
// 230 kt up to FL140, 240 to FL200, 265 to FL340, 280 above.
func HoldSpeedKts(altFt float64) float64 {
	switch {
	case altFt <= 14000:
		return 230
	case altFt <= 20000:
		return 240
	case altFt <= 34000:
		return 265
	}
	return 280
}

// HoldLegTime is the outbound leg: 1 min up to FL140, 1.5 min above.
func HoldLegTime(altFt float64) time.Duration {
	if altFt <= 14000 {
		return time.Minute
	}
	return 90 * time.Second
}

// TurnRadiusNM is the radius of a rate-one turn (3°/s) at a speed.
func TurnRadiusNM(kts float64) float64 { return kts / (60 * math.Pi) }

// HoldEntry is how an aircraft enters a hold.
type HoldEntry uint8

const (
	EntryDirect HoldEntry = iota
	EntryTeardrop
	EntryParallel
)

var holdEntryNames = [...]string{"direct", "teardrop", "parallel"}

func (e HoldEntry) String() string {
	if int(e) < len(holdEntryNames) {
		return holdEntryNames[e]
	}
	return "unknown"
}

// side is +1 for right turns, -1 for left.
func (h Hold) side() float64 {
	if h.LeftTurns {
		return -1
	}
	return 1
}

// Entry is the entry for an aircraft heading headingTrue to the fix: the
// ICAO sectors from the inbound course (right turns; mirrored for left) —
// direct from 70° on the non-holding side round to 110° on the holding
// side, teardrop the next 70°, parallel the remaining 110°.
func (h Hold) Entry(headingTrue float64) HoldEntry {
	// The heading off the inbound course in (-180, 180]: flying at the fix
	// against the inbound course is +180, a teardrop.
	d := math.Mod(headingTrue-h.InboundTrue+720, 360)
	if d > 180 {
		d -= 360
	}
	d *= h.side()
	switch {
	case d >= -70 && d <= 110:
		return EntryDirect
	case d > 110:
		return EntryTeardrop
	}
	return EntryParallel
}

// at displaces p by nm along bearing.
func at(p airport.LatLon, bearing, nm float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, math.Mod(bearing+720, 360), nm*1852)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// Racetrack is the hold flown at an altitude: from the fix, the turn
// outbound (three points), the outbound leg, the turn inbound (three
// points) and back to the fix, which comes last.
func (h Hold) Racetrack(altFt float64) []airport.LatLon {
	c, s := h.InboundTrue, h.side()
	v := HoldSpeedKts(altFt)
	r := TurnRadiusNM(v)
	leg := v * HoldLegTime(altFt).Hours()
	o1 := at(h.Fix, c+90*s, r) // centre of the turn outbound
	arc := func(o airport.LatLon, from float64) []airport.LatLon {
		var out []airport.LatLon
		for _, a := range []float64{45, 90, 135, 180} {
			out = append(out, at(o, from+a*s, r))
		}
		return out
	}
	pts := arc(o1, c-90*s) // from the fix (behind the centre) round to abeam
	endOut := at(pts[len(pts)-1], c+180, leg)
	o2 := at(endOut, c-90*s, r) // centre of the turn inbound
	pts = append(pts, endOut)
	pts = append(pts, arc(o2, c+90*s)...) // round onto the inbound course
	return append(pts, h.Fix)
}

// EntryPoints are the points an entry flies from the fix before the
// racetrack (none for a direct entry): teardrop, out 30° off the outbound
// course into the holding side for a leg, then back; parallel, out along
// the non-holding side for a leg, turning back through the holding side.
func (h Hold) EntryPoints(e HoldEntry, altFt float64) []airport.LatLon {
	c, s := h.InboundTrue, h.side()
	v := HoldSpeedKts(altFt)
	leg := v * HoldLegTime(altFt).Hours()
	r := TurnRadiusNM(v)
	switch e {
	case EntryTeardrop:
		out := at(h.Fix, c+180-30*s, leg)
		return []airport.LatLon{h.Fix, out, at(out, c+90*s, r)}
	case EntryParallel:
		out := at(at(h.Fix, c-90*s, r/2), c+180, leg)
		return []airport.LatLon{h.Fix, out, at(out, c+90*s, 2*r)}
	}
	return nil
}

// holdWaypoints turns points into waypoints at the holding altitude and
// speed; wrap makes the last lead back to the first.
func holdWaypoints(pts []airport.LatLon, altFt float64, wrap bool) []types.SIMCONNECT_DATA_WAYPOINT {
	v := HoldSpeedKts(altFt)
	out := make([]types.SIMCONNECT_DATA_WAYPOINT, len(pts))
	for i, p := range pts {
		out[i] = procedureWaypoint(p, altFt, v)
	}
	if wrap && len(out) > 0 {
		out[len(out)-1].Flags |= uint32(types.SIMCONNECT_WAYPOINT_WRAP_TO_FIRST)
	}
	return out
}

// HoldStack is the stack at one hold: aircraft at 1000 ft levels from
// Base up, leaving from the bottom; when one leaves, those above step down.
type HoldStack struct {
	Hold Hold
	// BaseFt is the lowest holding altitude; StepFt between levels (1000).
	BaseFt, StepFt float64

	mu     sync.Mutex
	levels []string // bottom first
}

// Assign takes an aircraft into the stack at the lowest free level and
// returns its altitude; one already in keeps its level.
func (s *HoldStack) Assign(callsign string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, cs := range s.levels {
		if cs == callsign {
			return s.alt(i)
		}
	}
	s.levels = append(s.levels, callsign)
	return s.alt(len(s.levels) - 1)
}

func (s *HoldStack) alt(level int) float64 {
	step := s.StepFt
	if step == 0 {
		step = 1000
	}
	return s.BaseFt + float64(level)*step
}

// Release takes an aircraft out; the ones above step down. It returns the
// new altitudes of those that moved.
func (s *HoldStack) Release(callsign string) map[string]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	moved := map[string]float64{}
	for i, cs := range s.levels {
		if cs != callsign {
			continue
		}
		s.levels = append(s.levels[:i], s.levels[i+1:]...)
		for j := i; j < len(s.levels); j++ {
			moved[s.levels[j]] = s.alt(j)
		}
		break
	}
	return moved
}

// Aircraft are the aircraft in the stack, bottom first, with altitudes.
func (s *HoldStack) Aircraft() []StackLevel {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StackLevel, len(s.levels))
	for i, cs := range s.levels {
		out[i] = StackLevel{Callsign: cs, AltFt: s.alt(i)}
	}
	return out
}

// StackLevel is an aircraft's place in a stack.
type StackLevel struct {
	Callsign string  `json:"callsign"`
	AltFt    float64 `json:"altFt"`
}

// holdState is an arrival in a hold.
type holdState struct {
	hold    Hold
	altFt   float64
	entry   HoldEntry
	looping bool // the wrapped racetrack is flying (the entry is done)
	resume  int  // the procedure waypoint after the fix, to go on with
	since   time.Time
}

// ErrNotHolding is returned by hold commands for an arrival not in a hold.
var ErrNotHolding = errors.New("traffic: not holding")

// HoldFix is where an arrival on its STAR can hold: the first STAR point
// ahead at least minFromThresholdNM (along the route) from the threshold,
// with the inbound course the STAR's track into it.
func (c *ArrivalController) HoldFix(minFromThresholdNM float64) (Hold, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil {
		return Hold{}, false
	}
	wps := c.proc.Waypoints
	final := len(wps) - 2
	next := c.procWaypoint(wps[:max(final, 1)])
	thr := c.plan.End.Threshold
	for i := next; i < final; i++ {
		var route []airport.LatLon
		for _, w := range wps[i:] {
			route = append(route, airport.LatLon{Lat: w.Latitude, Lon: w.Longitude})
		}
		fix := route[0]
		if DistanceToGo(fix, route[1:], thr) < minFromThresholdNM {
			break
		}
		from := c.last.Position
		if i > 0 && i > next {
			from = airport.LatLon{Lat: wps[i-1].Latitude, Lon: wps[i-1].Longitude}
		}
		// Named after the STAR fix there, when it has one.
		ident := fmt.Sprintf("WP%d", i)
		for _, n := range c.req.Procedure {
			if n.Ident != "" && calc.HaversineNM(n.Position.Lat, n.Position.Lon, fix.Lat, fix.Lon) < 0.3 {
				ident = n.Ident
				break
			}
		}
		return Hold{Ident: ident, Fix: fix, InboundTrue: calc.BearingDegrees(from.Lat, from.Lon, fix.Lat, fix.Lon)}, true
	}
	return Hold{}, false
}

// EnterHold sends an arrival on its STAR into a hold at altFt: to the fix,
// the entry for its heading and a lap, then the racetrack wrapped (sent
// when it is back over the fix) until LeaveHold.
func (c *ArrivalController) EnterHold(h Hold, altFt float64) (HoldEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.flyingProc || c.proc == nil {
		return 0, ErrNotOnProcedure
	}
	// Where the STAR goes on after the fix.
	resume := len(c.proc.Waypoints) - 2
	best := math.Inf(1)
	for i, w := range c.proc.Waypoints {
		if d := calc.HaversineNM(h.Fix.Lat, h.Fix.Lon, w.Latitude, w.Longitude); d < best {
			best, resume = d, i+1
		}
	}
	hdg := calc.BearingDegrees(c.last.Position.Lat, c.last.Position.Lon, h.Fix.Lat, h.Fix.Lon)
	e := h.Entry(hdg)
	pts := append(h.EntryPoints(e, altFt), h.Racetrack(altFt)...)
	if len(h.EntryPoints(e, altFt)) == 0 {
		pts = append([]airport.LatLon{h.Fix}, pts...)
	}
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, holdWaypoints(pts, altFt, false)); err != nil {
		return 0, err
	}
	c.holding = &holdState{hold: h, altFt: altFt, entry: e, resume: resume, since: c.now()}
	c.note(fmt.Sprintf("holding at %s, %s entry, %.0f ft", h.Ident, e, altFt), nil)
	return e, nil
}

// HoldAltitude moves a holding arrival to another level (the stack steps
// down); the racetrack is sent again at it.
func (c *ArrivalController) HoldAltitude(altFt float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.holding == nil {
		return ErrNotHolding
	}
	c.holding.altFt = altFt
	c.holding.looping = true
	return c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, holdWaypoints(c.holding.hold.Racetrack(altFt), altFt, true))
}

// LeaveHold sends a holding arrival on along its STAR from the fix.
func (c *ArrivalController) LeaveHold() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.holding == nil {
		return ErrNotHolding
	}
	h := c.holding
	rest := c.proc.Waypoints[min(h.resume, len(c.proc.Waypoints)-2):]
	chain := append(holdWaypoints([]airport.LatLon{h.hold.Fix}, h.altFt, false), rest...)
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, chain); err != nil {
		return err
	}
	c.proc.Waypoints, c.procNext = chain, 0
	c.holding = nil
	c.note(fmt.Sprintf("leaving the hold at %s after %s", h.hold.Ident, c.now().Sub(h.since).Round(time.Second)), nil)
	return nil
}

// Holding reports whether the arrival is in a hold, and which.
func (c *ArrivalController) Holding() (Hold, float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.holding == nil {
		return Hold{}, 0, false
	}
	return c.holding.hold, c.holding.altFt, true
}

// holdFrame runs a holding arrival each second: once the entry and its
// first lap are flown (back at the fix), the wrapped racetrack takes over.
func (c *ArrivalController) holdFrame(pos airport.LatLon) {
	h := c.holding
	if h == nil || h.looping || c.now().Sub(h.since) < time.Minute {
		return
	}
	if calc.HaversineNM(pos.Lat, pos.Lon, h.hold.Fix.Lat, h.hold.Fix.Lon) < 1.5 {
		h.looping = true
		c.note("holding: racetrack", c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, holdWaypoints(h.hold.Racetrack(h.altFt), h.altFt, true)))
	}
}
