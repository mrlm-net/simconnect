//go:build windows
// +build windows

package traffic

import (
	"errors"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// VFR circuits (#567): the traffic pattern round a runway that light
// aircraft fly to land, for touch-and-goes and training, and to join from a
// reporting point. Each airport (each runway end) can have its own: side,
// height and leg distances (CircuitConfig); what is not set takes the
// defaults below, the downwind spacing from the aircraft's own turns.

// CircuitSide is the side the circuit turns to: left-hand (the default)
// or right-hand.
type CircuitSide string

const (
	CircuitLeft  CircuitSide = "left"
	CircuitRight CircuitSide = "right"
)

// Circuit defaults, used where a CircuitConfig leaves a value at zero.
var (
	// CircuitHeightFt is the circuit height above the airfield.
	CircuitHeightFt = 1000.0
	// CircuitUpwindNM is how far past the departure end the crosswind turn
	// starts.
	CircuitUpwindNM = 0.5
	// CircuitBaseNM is how far before the threshold the base leg joins the
	// final: the final's length.
	CircuitBaseNM = 1.0
	// CircuitMinDownwindNM is the least distance of the downwind from the
	// centreline (a slower aircraft's turns would put it closer).
	CircuitMinDownwindNM = 0.8
	// CircuitSpeedFactor is the circuit speed over the approach speed
	// (an estimate: a C172 flies its circuit at about 75 kt).
	CircuitSpeedFactor = 1.25
	// CircuitGlideFtPerNM is the final's descent: a 3° path.
	CircuitGlideFtPerNM = 318.0
)

// CircuitConfig is one runway end's circuit as an airport publishes it or a
// user sets it; zero values take the defaults.
type CircuitConfig struct {
	Side CircuitSide `json:"side,omitempty"`
	// HeightFt above the airfield.
	HeightFt float64 `json:"heightFt,omitempty"`
	// DownwindNM: the downwind's distance from the centreline (0: from the
	// aircraft's turns, at least CircuitMinDownwindNM).
	DownwindNM float64 `json:"downwindNM,omitempty"`
	// UpwindNM past the departure end, then crosswind.
	UpwindNM float64 `json:"upwindNM,omitempty"`
	// BaseNM before the threshold, then final.
	BaseNM float64 `json:"baseNM,omitempty"`
}

// CircuitLeg names a circuit's points, as pilots report them.
type CircuitLeg string

const (
	LegUpwind    CircuitLeg = "upwind"
	LegCrosswind CircuitLeg = "crosswind"
	LegDownwind  CircuitLeg = "downwind" // abeam the threshold: where "downwind" is reported
	LegBase      CircuitLeg = "base"
	LegFinal     CircuitLeg = "final"
	LegRunway    CircuitLeg = "runway"
)

// CircuitPoint is a corner of the circuit: the end of the leg it names
// (the downwind point is abeam the threshold), at its altitude (feet MSL)
// and speed.
type CircuitPoint struct {
	Leg      CircuitLeg     `json:"leg"`
	Position airport.LatLon `json:"position"`
	AltFt    float64        `json:"altFt"`
	Kts      float64        `json:"kts"`
}

// Circuit is a runway end's circuit for one aircraft: from the climb-out
// round to the threshold.
type Circuit struct {
	Runway     string         `json:"runway"`
	Side       CircuitSide    `json:"side"`
	HeightFt   float64        `json:"heightFt"` // MSL
	DownwindNM float64        `json:"downwindNM"`
	Points     []CircuitPoint `json:"points"`
	// heading is the runway's; maxBank the aircraft's.
	heading, maxBank float64
}

// ErrNoRunway is returned for a runway end the layout does not have.
var ErrNoRunway = errors.New("traffic: no such runway end")

// NewCircuit is the circuit of runway end rwy of l ("24"), set by cfg, for
// an aircraft of profile p.
func NewCircuit(l *airport.Layout, rwy string, cfg CircuitConfig, p AircraftProfile) (Circuit, error) {
	r, end, ok := l.RunwayEnd(rwy)
	if !ok {
		return Circuit{}, ErrNoRunway
	}
	far := r.Secondary.Threshold
	if end.Name == r.Secondary.Name {
		far = r.Primary.Threshold
	}
	side := cfg.Side
	if side != CircuitRight {
		side = CircuitLeft
	}
	height := firstNonZero(cfg.HeightFt, CircuitHeightFt)
	upwind := firstNonZero(cfg.UpwindNM, CircuitUpwindNM)
	base := firstNonZero(cfg.BaseNM, CircuitBaseNM)
	kts := CircuitKts(p)
	maxBank := MaxBankDeg(p)
	downwind := cfg.DownwindNM
	if downwind == 0 {
		// Two standard turns of 90° need two radii between the legs.
		downwind = math.Max(CircuitMinDownwindNM, 2*turnRadiusMeters(kts, StandardBankDeg(kts, maxBank))/1852)
		downwind = math.Round(downwind*10) / 10
	}
	hdg := end.Heading
	turn := hdg - 90 // the side the circuit turns to
	if side == CircuitRight {
		turn = hdg + 90
	}
	at := func(from airport.LatLon, alongNM, sideNM float64) airport.LatLon {
		return offsetHeading(offsetHeading(from, hdg, alongNM*1852), turn, sideNM*1852)
	}
	field := convert.MetersToFeet(l.Altitude)
	top := field + height
	approach := p.Approach.ApproachKts
	c := Circuit{Runway: end.Name, Side: side, HeightFt: top, DownwindNM: downwind, heading: hdg, maxBank: maxBank}
	c.Points = []CircuitPoint{
		// Climbing on the upwind; the crosswind turn about two thirds up.
		{Leg: LegUpwind, Position: at(far, upwind, 0), AltFt: field + height*2/3, Kts: kts},
		{Leg: LegCrosswind, Position: at(far, upwind, downwind), AltFt: top, Kts: kts},
		{Leg: LegDownwind, Position: at(end.Threshold, 0, downwind), AltFt: top, Kts: kts},
		// Down to the base turn; the final from base at the glide path.
		{Leg: LegBase, Position: at(end.Threshold, -base, downwind), AltFt: math.Max(field+base*CircuitGlideFtPerNM, top-300), Kts: (kts + approach) / 2},
		{Leg: LegFinal, Position: at(end.Threshold, -base, 0), AltFt: field + base*CircuitGlideFtPerNM, Kts: approach},
		{Leg: LegRunway, Position: end.Threshold, AltFt: field, Kts: approach},
	}
	return c, nil
}

// CircuitKts is the speed p flies a circuit at.
func CircuitKts(p AircraftProfile) float64 {
	return math.Round(p.Approach.ApproachKts * CircuitSpeedFactor)
}

// Point is the circuit's point of leg (false: none).
func (c Circuit) Point(leg CircuitLeg) (CircuitPoint, bool) {
	for _, p := range c.Points {
		if p.Leg == leg {
			return p, true
		}
	}
	return CircuitPoint{}, false
}

// From is the circuit from leg on (the leg's own point first): a circuit
// joined downwind starts at LegDownwind, one joined on base at LegBase.
func (c Circuit) From(leg CircuitLeg) []CircuitPoint {
	for i, p := range c.Points {
		if p.Leg == leg {
			return c.Points[i:]
		}
	}
	return nil
}

// JoinDownwind is the 45° entry to the downwind: a point a mile out from
// midfield on the downwind, outside the circuit and towards its upwind end,
// at circuit height; flown to midfield it meets the downwind at 45° (a
// left-hand circuit is joined with a right turn onto the downwind) and
// goes on from there (From(LegDownwind)).
func (c Circuit) JoinDownwind() (entry, midfield CircuitPoint) {
	cw, _ := c.Point(LegCrosswind)
	dw, _ := c.Point(LegDownwind)
	mid := airport.LatLon{Lat: (cw.Position.Lat + dw.Position.Lat) / 2, Lon: (cw.Position.Lon + dw.Position.Lon) / 2}
	out := c.heading - 45 // outside a left-hand circuit, towards its upwind end
	if c.Side == CircuitRight {
		out = c.heading + 45
	}
	midfield = CircuitPoint{Leg: LegDownwind, Position: mid, AltFt: c.HeightFt, Kts: dw.Kts}
	entry = CircuitPoint{Leg: LegDownwind, Position: offsetHeading(mid, out, 1852), AltFt: c.HeightFt, Kts: dw.Kts}
	return entry, midfield
}

// Waypoints are the points as MSFS AI waypoints, from leg on, the corners
// rounded to the aircraft's turns.
func (c Circuit) Waypoints(from CircuitLeg) []types.SIMCONNECT_DATA_WAYPOINT {
	pts := c.From(from)
	wps := make([]types.SIMCONNECT_DATA_WAYPOINT, len(pts))
	for i, p := range pts {
		wps[i] = procedureWaypoint(p.Position, p.AltFt, p.Kts)
	}
	return roundCorners(wps, c.maxBank)
}

func firstNonZero(v, def float64) float64 {
	if v != 0 {
		return v
	}
	return def
}

// PlanCircuitArrival is a VFR arrival through the circuit c (#568): it
// appears at the 45° entry (JoinDownwind) at circuit height and speed;
// MSFS AI flies to midfield, the downwind abeam the threshold and the base
// turn, then onto the final, where the injected approach takes over (the
// final's point, c's BaseNM out).
func PlanCircuitArrival(c Circuit) *ArrivalProcedure {
	entry, mid := c.JoinDownwind()
	dw, _ := c.Point(LegDownwind)
	base, _ := c.Point(LegBase)
	fin, _ := c.Point(LegFinal)
	rwy, _ := c.Point(LegRunway)
	wp := func(p CircuitPoint) types.SIMCONNECT_DATA_WAYPOINT { return procedureWaypoint(p.Position, p.AltFt, p.Kts) }
	return &ArrivalProcedure{
		Spawn: types.SIMCONNECT_DATA_INITPOSITION{
			Latitude: entry.Position.Lat, Longitude: entry.Position.Lon, Altitude: entry.AltFt,
			Heading:  localBearing(entry.Position, mid.Position),
			Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(entry.Kts),
		},
		Waypoints:     []types.SIMCONNECT_DATA_WAYPOINT{wp(mid), wp(dw), wp(base), wp(fin)},
		Join:          fin.Position,
		JoinMeters:    localDist(fin.Position, rwy.Position),
		MinJoinMeters: 0.5 * 1852,
	}
}
