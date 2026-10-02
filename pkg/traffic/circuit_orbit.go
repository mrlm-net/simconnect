//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// CircuitMaxExtendNM: a circuit's downwind is extended at most this far
// (AbsorbDelay); more delay is lost in an orbit (Orbit, #569).
const CircuitMaxExtendNM = 2.0

// Orbit has a VFR circuit arrival fly an orbit where it is (Doc 4444
// 12.3.4.17: "orbit left/right"): a full turn at circuit height and speed,
// to the side its circuit turns, standard rate for its speed, then on with
// the circuit. It returns the time the orbit takes. ErrNotOnProcedure when
// it is not flying its circuit (on the final, or no circuit arrival).
func (c *ArrivalController) Orbit() (time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.req.Circuit == nil || !c.flyingProc || c.proc == nil || len(c.proc.Waypoints) < 3 {
		return 0, ErrNotOnProcedure
	}
	pos, hdg := c.last.Position, c.last.Heading
	if pos == (airport.LatLon{}) {
		return 0, ErrNotOnProcedure
	}
	ci := c.req.Circuit
	kts := CircuitKts(*c.aircraft())
	r := turnRadiusMeters(kts, StandardBankDeg(kts, MaxBankDeg(*c.aircraft())))
	turn := -1.0 // left
	if ci.Side == CircuitRight {
		turn = 1
	}
	centre := offsetHeading(pos, hdg+90*turn, r)
	from := localBearing(centre, pos)
	var orbit []types.SIMCONNECT_DATA_WAYPOINT
	for k := 1; k <= 8; k++ {
		orbit = append(orbit, procedureWaypoint(offsetHeading(centre, from+turn*45*float64(k), r), ci.HeightFt, kts))
	}
	next := c.procWaypoint(c.proc.Waypoints)
	all := append(orbit, c.proc.Waypoints[next:]...)
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, all); err != nil {
		return 0, err
	}
	c.proc.Waypoints, c.procNext = all, 0
	c.corners, c.cornerNames, c.cornerNext = nil, nil, -1 // re-planned from the flown points from now on
	c.note("orbit", nil)
	return time.Duration(2 * math.Pi * r / (kts * ktsToMS) * float64(time.Second)), nil
}

// absorbInCircuit is AbsorbDelay for a VFR circuit arrival: on along its
// downwind and the base turn further out, up to CircuitMaxExtendNM in all
// (each mile on adds two), at circuit speed; what that cannot take is
// Left, for an orbit. ErrNotOnProcedure once it turns base. c.mu held.
func (c *ArrivalController) absorbInCircuit(delay time.Duration) (Absorption, error) {
	ci := c.req.Circuit
	pos := c.last.Position
	if pos == (airport.LatLon{}) {
		return Absorption{}, ErrNotOnProcedure
	}
	thr, _ := ci.Point(LegRunway)
	base, _ := ci.Point(LegBase)
	fin, _ := ci.Point(LegFinal)
	dw, _ := ci.Point(LegDownwind)
	along := alongHeading(thr.Position, ci.heading, pos) // ahead of the threshold: + on the upwind side
	baseAlong := alongHeading(thr.Position, ci.heading, base.Position) - c.tromboneNM*1852
	if along <= baseAlong+300 {
		return Absorption{}, ErrNotOnProcedure // turning base, or on the final
	}
	kts := CircuitKts(*c.aircraft())
	a := Absorption{Left: delay}
	x := math.Min(delay.Hours()*kts/2, CircuitMaxExtendNM-c.tromboneNM)
	if x < 0.1 {
		return a, nil // extended as far as it goes: the rest for an orbit
	}
	away := ci.heading + 180
	out := (c.tromboneNM + x) * 1852
	var wps []types.SIMCONNECT_DATA_WAYPOINT
	// Not yet abeam the threshold: by the downwind's point there.
	if along > 300 {
		wps = append(wps, procedureWaypoint(dw.Position, dw.AltFt, dw.Kts))
	}
	baseExt := offsetHeading(base.Position, away, out)
	finExt := offsetHeading(fin.Position, away, out)
	wps = append(wps, procedureWaypoint(baseExt, base.AltFt, base.Kts),
		procedureWaypoint(finExt, fin.AltFt+out/1852*CircuitGlideFtPerNM, fin.Kts),
		procedureWaypoint(fin.Position, fin.AltFt, fin.Kts))
	rounded := roundedChain(pos, wps, MaxBankDeg(*c.aircraft()))
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, rounded); err != nil {
		return Absorption{}, err
	}
	c.proc.Waypoints, c.procNext = rounded, 0
	c.corners, c.cornerNames, c.cornerNext = nil, nil, -1
	c.tromboneNM += x
	a.ExtraNM = 2 * x
	a.Left -= time.Duration(a.ExtraNM / kts * float64(time.Hour))
	if a.Left < 0 {
		a.Left = 0
	}
	c.note(fmt.Sprintf("circuit: downwind extended %.1f NM, %s left", x, a.Left.Round(time.Second)), nil)
	return a, nil
}
