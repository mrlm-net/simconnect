package traffic

import (
	"errors"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
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
	// OverheadJoin: VFR arrivals join by the standard overhead join
	// (CircuitJoinFor, PlanCircuitArrivalVia), as airfields without a
	// tower ask; else the tower assigns the join.
	OverheadJoin bool `json:"overheadJoin,omitempty"`
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
	// LegOverhead names the standard overhead join (CircuitJoinFor).
	LegOverhead CircuitLeg = "overhead"
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
	wp := func(p CircuitPoint) types.SIMCONNECT_DATA_WAYPOINT {
		return procedureWaypoint(p.Position, p.AltFt, p.Kts)
	}
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

// VFR departures (#568): out of the circuit to an exit point VFRExitNM from
// the field, VFRExitAboveFt above circuit height; the injected take-off
// hands over to MSFS AI at VFRHandoverFt above the runway, so MSFS AI
// flies the circuit's turns.
var (
	VFRExitNM      = 5.0
	VFRExitAboveFt = 1000.0
	VFRHandoverFt  = 400.0
)

// Departure is the way out of the circuit towards exitBearing (true
// degrees from the field), as the circuit's points then the exit point:
//
//   - ahead (within 45° of the runway heading): straight out, from the
//     upwind;
//   - to the circuit's side: by its crosswind leg;
//   - to the other side: turning away from the circuit after the upwind;
//   - behind, on the circuit's side: by its crosswind and downwind (a
//     downwind departure); behind on the other side, by the mirror of
//     those legs, never across the circuit.
//
// The points are at circuit height, the exit point VFRExitAboveFt above
// it; each at the circuit speed.
func (c Circuit) Departure(exitBearing float64) []airport.NavPoint {
	up, _ := c.Point(LegUpwind)
	cw, _ := c.Point(LegCrosswind)
	dw, _ := c.Point(LegDownwind)
	rwy, _ := c.Point(LegRunway)
	field := airport.LatLon{Lat: (rwy.Position.Lat + up.Position.Lat) / 2, Lon: (rwy.Position.Lon + up.Position.Lon) / 2}
	rel := headingDiff(c.heading, exitBearing) // exit relative to the runway heading, + to the right
	circuitSide := rel < 0
	if c.Side == CircuitRight {
		circuitSide = rel > 0
	}
	// The mirror of a circuit point on the other side of the centreline.
	mirror := func(p CircuitPoint) CircuitPoint {
		away := c.heading + 90
		if c.Side == CircuitRight {
			away = c.heading - 90
		}
		p.Position = offsetHeading(p.Position, away, 2*c.DownwindNM*1852)
		return p
	}
	top := up
	top.AltFt = c.HeightFt
	pts := []CircuitPoint{top}
	switch abs := math.Abs(rel); {
	case abs <= 45:
	case abs <= 135 && circuitSide:
		pts = append(pts, cw)
	case abs <= 135:
	case circuitSide:
		pts = append(pts, cw, dw)
	default:
		pts = append(pts, mirror(cw), mirror(dw))
	}
	exit := CircuitPoint{Position: offsetHeading(field, exitBearing, VFRExitNM*1852), AltFt: c.HeightFt + VFRExitAboveFt, Kts: up.Kts}
	pts = append(pts, exit)
	out := make([]airport.NavPoint, len(pts))
	prev := rwy.Position
	for i, p := range pts {
		alt := p.AltFt * 0.3048
		out[i] = airport.NavPoint{Position: p.Position, AltMin: alt, AltMax: alt, SpeedMax: p.Kts, Course: localBearing(prev, p.Position)}
		prev = p.Position
	}
	out[len(out)-1].Ident = "VFR exit"
	return out
}

// VFRDepartureWaypoints are a VFR departure's points (Circuit.Departure) as
// MSFS AI waypoints from pos (heading hdg) on: each at its altitude and
// speed, the points already passed or behind left out, then on along the
// last leg; the corners rounded to the aircraft's turns (maxBank).
func VFRDepartureWaypoints(pos airport.LatLon, hdg float64, route []airport.NavPoint, maxBank float64) []types.SIMCONNECT_DATA_WAYPOINT {
	var wps []types.SIMCONNECT_DATA_WAYPOINT
	here := procedureWaypoint(pos, 0, 0)
	kts, alt := 0.0, 0.0
	if n := len(route); n > 0 {
		kts, alt = route[n-1].SpeedMax, route[n-1].AltMax/0.3048
	}
	cur, last := pos, pos
	for _, n := range route {
		d := localDist(cur, n.Position)
		if len(wps) == 0 && (d < 0.3*1852 || math.Abs(headingDiff(localBearing(pos, n.Position), hdg)) > 100) {
			continue // passed during the injected climb, or behind
		}
		kts, alt = n.SpeedMax, n.AltMax/0.3048
		wps = append(wps, procedureWaypoint(n.Position, alt, kts))
		last, cur = cur, n.Position
	}
	track := hdg
	if len(wps) > 0 && last != cur {
		track = localBearing(last, cur)
	}
	wps = append(wps, procedureWaypoint(offsetHeading(cur, track, 20*1852), alt, kts))
	here.KtsSpeed = kts
	return roundCorners(append([]types.SIMCONNECT_DATA_WAYPOINT{here}, wps...), maxBank)[1:]
}

// ReportingPoint is a VFR reporting point (#566): where VFR traffic enters
// and leaves the control zone, as an airport publishes it or a user sets
// it ("NOVEMBER").
type ReportingPoint struct {
	Name     string         `json:"name"`
	Position airport.LatLon `json:"position"`
}

// DepartureVia is Departure towards reporting point p, ending over it.
func (c Circuit) DepartureVia(p ReportingPoint) []airport.NavPoint {
	up, _ := c.Point(LegUpwind)
	rwy, _ := c.Point(LegRunway)
	field := airport.LatLon{Lat: (rwy.Position.Lat + up.Position.Lat) / 2, Lon: (rwy.Position.Lon + up.Position.Lon) / 2}
	route := c.Departure(localBearing(field, p.Position))
	last := &route[len(route)-1]
	prev := rwy.Position
	if len(route) > 1 {
		prev = route[len(route)-2].Position
	}
	last.Position, last.Ident, last.Course = p.Position, p.Name, localBearing(prev, p.Position)
	return route
}

// CircuitJoinFor is how the tower of a controlled aerodrome joins a VFR
// arrival coming from p into runway rwy's traffic (Doc 4444 12.3.4.13: it
// assigns the direction of the circuit and where to join, or a
// straight-in approach), so it never crosses the runway:
//
//   - from the final's sector (within StraightInSectorDeg of the extended
//     centreline, beyond the threshold): a straight-in approach (LegFinal);
//   - from within BaseJoinSectorDeg of the extended centreline on the
//     approach side: the base leg (LegBase) of the circuit on its side;
//   - else the downwind (LegDownwind) of the circuit on p's side of the
//     centreline, cfg's side turned round where it is the other.
func CircuitJoinFor(l *airport.Layout, rwy string, cfg CircuitConfig, p airport.LatLon) (CircuitConfig, CircuitLeg, error) {
	r, end, ok := l.RunwayEnd(rwy)
	if !ok {
		return cfg, "", ErrNoRunway
	}
	if cfg.OverheadJoin {
		return cfg, LegOverhead, nil // whatever side it comes from: overhead first
	}
	if math.Abs(headingDiff(end.Heading+180, localBearing(end.Threshold, p))) <= StraightInSectorDeg {
		return cfg, LegFinal, nil
	}
	far := r.Secondary.Threshold
	if end.Name == r.Secondary.Name {
		far = r.Primary.Threshold
	}
	right := calc.CrossTrackMeters(end.Threshold.Lat, end.Threshold.Lon, far.Lat, far.Lon, p.Lat, p.Lon) > 0
	if right {
		cfg.Side = CircuitRight
	} else {
		cfg.Side = CircuitLeft
	}
	if math.Abs(headingDiff(end.Heading+180, localBearing(end.Threshold, p))) <= BaseJoinSectorDeg {
		return cfg, LegBase, nil
	}
	return cfg, LegDownwind, nil
}

// StraightInSectorDeg: a VFR arrival this close to the extended
// centreline, out on the final's side, is given a straight-in approach.
const StraightInSectorDeg = 30.0

// BaseJoinSectorDeg: a VFR arrival from within this of the extended
// centreline on the approach side (but not in the final's sector) joins
// on base.
const BaseJoinSectorDeg = 60.0

// StraightInNM: a straight-in approach lines up this far out.
const StraightInNM = 3.0

// PlanCircuitArrivalFrom is PlanCircuitArrival entering over reporting
// point from (nil: at the 45° entry): it appears there VFRExitAboveFt above
// circuit height, flies to the 45° entry and on as PlanCircuitArrival; with
// join LegFinal, onto the extended centreline StraightInNM out and down
// the final instead (a straight-in approach).
func PlanCircuitArrivalVia(c Circuit, from *ReportingPoint, join CircuitLeg) *ArrivalProcedure {
	proc := PlanCircuitArrivalFrom(c, from)
	if from == nil || join != LegFinal && join != LegBase && join != LegOverhead {
		return proc
	}
	if join == LegOverhead {
		proc.Waypoints = c.overheadJoin()
		first := proc.Waypoints[0]
		proc.Spawn.Heading = localBearing(from.Position, airport.LatLon{Lat: first.Latitude, Lon: first.Longitude})
		return proc
	}
	if join == LegBase {
		// Straight to the base turn and round onto the final.
		base, _ := c.Point(LegBase)
		fin, _ := c.Point(LegFinal)
		proc.Spawn.Heading = localBearing(from.Position, base.Position)
		proc.Waypoints = []types.SIMCONNECT_DATA_WAYPOINT{procedureWaypoint(base.Position, base.AltFt, base.Kts), procedureWaypoint(fin.Position, fin.AltFt, fin.Kts)}
		return proc
	}
	fin, _ := c.Point(LegFinal)
	rwy, _ := c.Point(LegRunway)
	out := offsetHeading(rwy.Position, c.heading+180, StraightInNM*1852)
	alt := math.Min(c.HeightFt, fin.AltFt+(StraightInNM-CircuitBaseNM)*CircuitGlideFtPerNM)
	proc.Spawn.Heading = localBearing(from.Position, out)
	proc.Waypoints = []types.SIMCONNECT_DATA_WAYPOINT{procedureWaypoint(out, alt, fin.Kts), procedureWaypoint(fin.Position, fin.AltFt, fin.Kts)}
	return proc
}

// PlanCircuitArrivalFrom is PlanCircuitArrival entering over reporting
// point from (nil: at the 45° entry): it appears there VFRExitAboveFt above
// circuit height, flies to the 45° entry and on as PlanCircuitArrival.
func PlanCircuitArrivalFrom(c Circuit, from *ReportingPoint) *ArrivalProcedure {
	proc := PlanCircuitArrival(c)
	if from == nil {
		return proc
	}
	entry, _ := c.JoinDownwind()
	alt := c.HeightFt + VFRExitAboveFt
	proc.Spawn.Latitude, proc.Spawn.Longitude, proc.Spawn.Altitude = from.Position.Lat, from.Position.Lon, alt
	proc.Spawn.Heading = localBearing(from.Position, entry.Position)
	join := procedureWaypoint(entry.Position, c.HeightFt, entry.Kts)
	proc.Waypoints = append([]types.SIMCONNECT_DATA_WAYPOINT{join}, proc.Waypoints...)
	return proc
}

// overheadJoin is the standard overhead join into circuit c, the way UK
// airfields without a tower publish it (UK Airprox Board report 2025183,
// quoting the Sherburn-in-Elmet AIP entry: "join overhead at 2000 FT QFE
// and descend in accordance with the 'Standard Overhead Join' procedure",
// circuits at 1000 FT QFE; the pilots descend "on the deadside"): overhead
// the field OverheadAboveFt above circuit height, down on the dead side
// (the side away from the circuit) to circuit height, across the upwind
// end of the runway at circuit height onto the crosswind leg, then the
// downwind, base and final.
func (c Circuit) overheadJoin() []types.SIMCONNECT_DATA_WAYPOINT {
	up, _ := c.Point(LegUpwind)
	cw, _ := c.Point(LegCrosswind)
	dw, _ := c.Point(LegDownwind)
	base, _ := c.Point(LegBase)
	fin, _ := c.Point(LegFinal)
	rwy, _ := c.Point(LegRunway)
	field := airport.LatLon{Lat: (rwy.Position.Lat + up.Position.Lat) / 2, Lon: (rwy.Position.Lon + up.Position.Lon) / 2}
	dead := c.heading + 90 // the dead side: away from a left-hand circuit
	if c.Side == CircuitRight {
		dead = c.heading - 90
	}
	// Down on the dead side, abeam the upwind end, as wide as the downwind.
	deadside := offsetHeading(up.Position, dead, c.DownwindNM*1852)
	wp := func(p airport.LatLon, alt, kts float64) types.SIMCONNECT_DATA_WAYPOINT {
		return procedureWaypoint(p, alt, kts)
	}
	return []types.SIMCONNECT_DATA_WAYPOINT{
		wp(field, c.HeightFt+OverheadAboveFt, up.Kts),
		wp(deadside, c.HeightFt, up.Kts),
		wp(up.Position, c.HeightFt, up.Kts), // across the upwind end
		wp(cw.Position, c.HeightFt, cw.Kts),
		wp(dw.Position, dw.AltFt, dw.Kts),
		wp(base.Position, base.AltFt, base.Kts),
		wp(fin.Position, fin.AltFt, fin.Kts),
	}
}

// OverheadAboveFt is how far above circuit height the overhead join
// crosses the field: 1000 ft (2000 ft above it for a 1000 ft circuit).
const OverheadAboveFt = 1000.0

// AnotherCircuit has a VFR circuit arrival make another circuit (Doc 4444
// 12.3.4.17 c, "make another circuit"; #569): on round its circuit from
// where it is to the final, over the runway at circuit height, and once
// more round from the upwind to the final, where the injected approach
// takes over. It returns about how long that takes. ErrNotOnProcedure when
// it is not flying its circuit.
func (c *ArrivalController) AnotherCircuit() (time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.anotherCircuit()
}

// anotherCircuit is AnotherCircuit under c.mu.
func (c *ArrivalController) anotherCircuit() (time.Duration, error) {
	if c.req.Circuit == nil || !c.flyingProc || c.proc == nil || len(c.proc.Waypoints) < 2 {
		return 0, ErrNotOnProcedure
	}
	ci := c.req.Circuit
	pos := c.last.Position
	if pos == (airport.LatLon{}) {
		return 0, ErrNotOnProcedure
	}
	thr, _ := ci.Point(LegRunway)
	up, _ := ci.Point(LegUpwind)
	var wps []types.SIMCONNECT_DATA_WAYPOINT
	next := c.procWaypoint(c.proc.Waypoints)
	wps = append(wps, c.proc.Waypoints[next:len(c.proc.Waypoints)-1]...) // round to the final, not into the join
	wps = append(wps, procedureWaypoint(thr.Position, ci.HeightFt, up.Kts), procedureWaypoint(up.Position, ci.HeightFt, up.Kts))
	for _, leg := range []CircuitLeg{LegCrosswind, LegDownwind, LegBase, LegFinal} {
		p, _ := ci.Point(leg)
		wps = append(wps, procedureWaypoint(p.Position, p.AltFt, p.Kts))
	}
	rounded := roundedChain(pos, wps, MaxBankDeg(*c.aircraft()))
	if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, rounded); err != nil {
		return 0, err
	}
	c.proc.Waypoints, c.procNext = rounded, 0
	c.corners, c.cornerNames, c.cornerNext = nil, nil, -1
	c.baseCall = false // round again: its own base turn
	c.note("another circuit", nil)
	nm := pathNMOf(pos, rounded[:len(rounded)-1], rounded[len(rounded)-1])
	return time.Duration(nm / CircuitKts(*c.aircraft()) * float64(time.Hour)), nil
}
