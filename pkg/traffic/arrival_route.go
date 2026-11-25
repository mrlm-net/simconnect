package traffic

import (
	"errors"
	"math"
	"slices"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ErrNotStandRoute is returned for a taxi-in route that does not end on a
// stand's PARKING path.
var ErrNotStandRoute = errors.New("traffic: taxi-in route must end at a parking spot")

// RequiredRollout returns the distance from the landing threshold to where an
// arriving aircraft has slowed to exitKts: touchdown at TouchdownMeters and
// TouchdownSpeedKts, then braking at RolloutDecel.
func RequiredRollout(exitKts float64) float64 {
	return exitReach{}.rollout(exitKts)
}

// exitReach is the aircraft an exit is chosen for (bestExit): its
// touchdown speed (kt) and planned braking (m/s²). A light single (touching
// down below exitLightTouchdownKts) has its own: it touches down at half an
// airliner's speed and turns off at the first exits. Any other aircraft,
// and zero, use an airliner's (TouchdownSpeedKts, RolloutDecel).
type exitReach struct{ touchdownKts, decel float64 }

// exitLightTouchdownKts: below this touchdown speed an aircraft is light
// (exitReach). A light aircraft is planned to touch down
// exitLightTouchdownMeters past the threshold (its injected landing,
// crossing the threshold at LightThresholdHeightFt, does at about 220 m,
// then its spread)
// and brakes at its own BrakeDecel.
const (
	exitLightTouchdownKts    = 90.0
	exitLightTouchdownMeters = 220.0 + TouchdownSpreadMeters
)

func (x exitReach) light() bool {
	return x.touchdownKts > 0 && x.touchdownKts < exitLightTouchdownKts
}

// exitTouchdownMeters is where an airliner's injected landing is planned
// to touch down for its exit: its usual point (about 490 m past the
// threshold, measured live) at the far end of its spread. Planned at the
// MSFS AI touchdown waypoint (TouchdownMeters) the exit was one it could
// make only braking hard.
const exitTouchdownMeters = 490.0 + TouchdownSpreadMeters

// exitSlowTouchdownKts: below this touchdown speed (not light) an aircraft
// is planned at its own speed too, touching down where an airliner does: a
// business jet or turboprop makes the earlier exits (the user, live: OKKST,
// a PC-24 at 103 kt, could have left 06 at L). Airliners, at this or more,
// keep the airliner's plan.
const exitSlowTouchdownKts = 115.0

// exitHeavyDecel: an aircraft faster than an airliner's plan
// (TouchdownSpeedKts) braking at this or less is a widebody (exitReach).
// Narrowbodies brake at 2.5 and keep the airliner's plan, though most touch
// down faster than it (an A320 at 130 kt, a B738 at 140): planned at their
// own speed every airliner took a later exit and held the runway longer.
const exitHeavyDecel = 2.0

func (x exitReach) heavy() bool {
	return x.touchdownKts > TouchdownSpeedKts && x.decel > 0 && x.decel <= exitHeavyDecel
}

func (x exitReach) speeds() (td, decel float64) {
	if x.heavy() {
		// At its own speed, braking as planned for an airliner, so a later
		// exit (the user, live: a 777 planned onto LKPR D as an A320,
		// braking at its limit).
		return x.touchdownKts, RolloutDecel
	}
	if x.light() || x.touchdownKts > 0 && x.touchdownKts < exitSlowTouchdownKts {
		decel = RolloutDecel
		if x.decel > 0 {
			decel = x.decel
		}
		return x.touchdownKts, decel
	}
	return TouchdownSpeedKts, RolloutDecel
}

// rollout is the distance from the landing threshold to where the aircraft
// has slowed to exitKts. An exit's Along is from the pavement end: compare
// it less the end's DisplacedM (landingAlong).
func (x exitReach) rollout(exitKts float64) float64 {
	td, decel := x.speeds()
	v0, v1 := td*knot, math.Min(exitKts, td)*knot
	at := exitTouchdownMeters
	if x.light() {
		at = exitLightTouchdownMeters
	}
	return at + (v0*v0-v1*v1)/(2*decel)
}

// landingAlong is how far exit e is past the landing threshold of
// runwayEnd: its Along (from the pavement end) less the end's DisplacedM.
func landingAlong(g *airport.Graph, runwayEnd string, e airport.RunwayExit) float64 {
	if _, end, ok := g.Layout.RunwayEnd(runwayEnd); ok && end.DisplacedM > 0 {
		return e.Along - end.DisplacedM
	}
	return e.Along
}

// runwayCostPerM is what a meter of rollout costs in taxi-in meters:
// exitRunwayCostPerM for an airliner; for a light single it grows with the
// square of how much slower it lands (about six times at 51 kt). It holds
// the runway longer for every meter and taxis slowly anyway, so it takes
// the first exit it can make even for a longer taxi-in.
func (x exitReach) runwayCostPerM() float64 {
	td, _ := x.speeds()
	k := math.Max(TouchdownSpeedKts/td, 1) // a faster one: an airliner's
	return exitRunwayCostPerM * k * k
}

// ExitSpeed is the speed to reach a runway exit at: ExitHighSpeedKts for a
// high-speed exit, ExitSpeedKts otherwise; SharpTurnSpeedKts for a turnoff
// leading back at the far end (endTurnoff, leadsBack).
func ExitSpeed(e airport.RunwayExit) float64 {
	if leadsBack(e) {
		return SharpTurnSpeedKts
	}
	if e.HighSpeed {
		return ExitHighSpeedKts
	}
	return ExitSpeedKts
}

// TaxiInWaypoints converts a taxi-in route from airport.Graph.RouteFromRunway
// or RouteToParking into ground waypoints after its first point: taxi legs
// (see groundLegs), then nose-in along the stand's PARKING path at
// StandApproachSpeedKts to the stand, and a last waypoint StandOvershootMeters
// past it, because MSFS AI stops short of its last ground waypoint.
func TaxiInWaypoints(g *airport.Graph, r *airport.Route) ([]types.SIMCONNECT_DATA_WAYPOINT, error) {
	return taxiIn(g, r, groundAlt{feet: convert.MetersToFeet(g.Layout.Altitude)}, DefaultNoseOffsetMeters, 0, 0)
}

// StandStop returns where an aircraft's reference point should stop on the
// stand at the end of route r: MSFS stands are circles sized for the largest
// aircraft allowed, and aircraft park with the nose at the front of the
// circle, so the stop point is RADIUS minus noseOffset (the distance from
// the aircraft's reference point to its nose) ahead of the centre along the
// stand heading.
func StandStop(g *airport.Graph, r *airport.Route, noseOffset float64) (airport.LatLon, error) {
	return standStop(g, r, noseOffset, 0)
}

func standStop(g *airport.Graph, r *airport.Route, noseOffset, side float64) (airport.LatLon, error) {
	n := len(r.Nodes)
	if n < 2 || g.Nodes[r.Nodes[n-1]].Kind != airport.NodeParking {
		return airport.LatLon{}, ErrNotStandRoute
	}
	return StandPointSide(g.Layout.Parking[g.Nodes[r.Nodes[n-1]].Index], noseOffset, side), nil
}

// StandPoint is where an aircraft stands on a parking spot: its reference
// point with the nose at the front of the parking circle (by the jetway and
// stop mark), noseOffset meters from the nose — not the circle's centre.
// Arrivals park and departures spawn there.
func StandPoint(p airport.Parking, noseOffset float64) airport.LatLon {
	if noseOffset <= 0 {
		noseOffset = DefaultNoseOffsetMeters
	}
	lat, lon := calc.DisplaceByHeading(p.Position.Lat, p.Position.Lon, p.Heading, math.Max(0, p.Radius-noseOffset))
	return airport.LatLon{Lat: lat, Lon: lon}
}

// StandPointSide is StandPoint side meters to the right of the parking
// line (left when negative).
func StandPointSide(p airport.Parking, noseOffset, side float64) airport.LatLon {
	at := StandPoint(p, noseOffset)
	if side == 0 {
		return at
	}
	lat, lon := calc.DisplaceByHeading(at.Lat, at.Lon, math.Mod(p.Heading+90, 360), side)
	return airport.LatLon{Lat: lat, Lon: lon}
}

func taxiIn(g *airport.Graph, r *airport.Route, alt groundAlt, noseOffset, side float64, from int) ([]types.SIMCONNECT_DATA_WAYPOINT, error) {
	n := len(r.Nodes)
	if n < 3 || g.Nodes[r.Nodes[n-1]].Kind != airport.NodeParking {
		return nil, ErrNotStandRoute
	}
	pts := r.Points
	junction := pts[n-2]
	stand, err := standStop(g, r, noseOffset, side)
	if err != nil {
		return nil, err
	}

	wps := groundLegs(thin(simplify(pts[from:n-1]), MinWaypointSpacingMeters), alt, legOptions{
		maxKts: TaxiSpeedKts, endKts: TurnSpeedKts, endMeters: HoldShortApproachMeters,
	})
	in := calc.BearingDegrees(junction.Lat, junction.Lon, stand.Lat, stand.Lon)
	if l := calc.HaversineMeters(junction.Lat, junction.Lon, stand.Lat, stand.Lon); l > StandSlowMeters+5 {
		// Taxi onto the stand at StandTaxiSpeedKts, then slow for the last meters.
		sLat, sLon := calc.DisplaceByHeading(stand.Lat, stand.Lon, math.Mod(in+180, 360), StandSlowMeters)
		wps = append(wps, alt.waypoint(sLat, sLon, StandTaxiSpeedKts))
	}
	wps = append(wps, alt.waypoint(stand.Lat, stand.Lon, StandApproachSpeedKts))
	oLat, oLon := calc.DisplaceByHeading(stand.Lat, stand.Lon, in, StandOvershootMeters)
	return append(wps, alt.waypoint(oLat, oLon, StandApproachSpeedKts)), nil
}

// bestExit chooses the runway exit for an arrival to parking: the first
// exit the aircraft reaches at its exit speed slowing down as planned
// (Along ≥ its rollout, exitReach), as a crew takes it — not one further
// down nearer its stand (the user: they slow down and should join the
// first exit smoothly, not roll on to another). One whose taxi-in crosses
// back over the runway vacated is passed while another avoids it. If none
// is reachable, the last exit is used.
func bestExit(g *airport.Graph, runwayEnd string, parking int, opts airport.RouteOptions, reach exitReach) (airport.RunwayExit, *airport.Route, error) {
	exits, err := g.RunwayExits(runwayEnd)
	if err != nil {
		return airport.RunwayExit{}, nil, err
	}
	var (
		best      airport.RunwayExit
		bestRoute *airport.Route
		bestCost        = math.Inf(1)
		lastErr   error = airport.ErrNoExit
	)
	for i, e := range exits {
		if landingAlong(g, runwayEnd, e) < reach.rollout(ExitSpeed(e)) && i < len(exits)-1 {
			continue
		}
		r, err := g.RouteFromRunway(e, parking, opts)
		if err != nil {
			lastErr = err
			continue
		}
		c := exitCost(e, r, reach.runwayCostPerM())
		// Runways crossed on the way in count as in the route search; back
		// over the one vacated, never if another exit avoids it (EGLL 09R
		// S5W once the routes past fewer stands were taken).
		for _, x := range r.RunwayCrossings {
			c += airport.DefaultRunwayCrossingPenalty
			if rwy, _, ok := g.Layout.RunwayEnd(runwayEnd); ok && x == rwy.Name() {
				c += exitCrossBackPenalty
			}
		}
		if c < bestCost {
			best, bestRoute, bestCost = e, r, c
		}
		if bestCost < exitCrossBackPenalty {
			break // the first it makes
		}
	}
	if bestRoute == nil {
		// None past the rollout reaches the stand (#1567): one braking
		// firmly that does, before a backtrack.
		if x, r, ok := firmExit(g, runwayEnd, exits, parking, opts, reach); ok {
			return x, r, nil
		}
		if x, r, ok := backtrackExit(g, runwayEnd, exits, parking, opts, reach); ok {
			return x, r, nil
		}
		return airport.RunwayExit{}, nil, lastErr
	}
	return best, bestRoute, nil
}

// firmExitDecel: braking (m/s²) an exit short of the planned rollout is
// still made with, when no later exit reaches the stand (#1567): an
// airliner's firm braking, not its limit.
const firmExitDecel = 2.5

// firmRollout is the distance from the landing threshold to where the
// aircraft has slowed to exitKts braking firmly (#1631): from its own
// touchdown speed, the airliner's planned one at least, at its own
// BrakeDecel (firmExitDecel when not given).
func (x exitReach) firmRollout(exitKts float64) float64 {
	td, _ := x.speeds()
	td = math.Max(td, x.touchdownKts)
	decel := firmExitDecel
	if x.decel > 0 {
		decel = x.decel
	}
	at := exitTouchdownMeters
	if x.light() {
		at = exitLightTouchdownMeters
	}
	v0, v1 := td*knot, math.Min(exitKts, td)*knot
	return at + (v0*v0-v1*v1)/(2*decel)
}

// makes reports whether the aircraft makes exit e off runwayEnd at its
// turn-off speed braking firmly (firmRollout).
func (x exitReach) makes(g *airport.Graph, runwayEnd string, e airport.RunwayExit) bool {
	return landingAlong(g, runwayEnd, e) >= x.firmRollout(ExitSpeed(e))
}

// firmExit is bestExit's next try when no exit past the rollout reaches
// the stand: the cheapest exit that does, reached braking at
// firmExitDecel (the planned rollout's braking otherwise); false none.
func firmExit(g *airport.Graph, runwayEnd string, exits []airport.RunwayExit, parking int, opts airport.RouteOptions, reach exitReach) (airport.RunwayExit, *airport.Route, bool) {
	var (
		best      airport.RunwayExit
		bestRoute *airport.Route
		bestCost  = math.Inf(1)
	)
	for _, e := range exits {
		if !reach.makes(g, runwayEnd, e) {
			continue
		}
		r, err := g.RouteFromRunway(e, parking, opts)
		if err != nil {
			continue
		}
		c := exitCost(e, r, reach.runwayCostPerM())
		for _, x := range r.RunwayCrossings {
			c += airport.DefaultRunwayCrossingPenalty
			if rwy, _, ok := g.Layout.RunwayEnd(runwayEnd); ok && x == rwy.Name() {
				c += exitCrossBackPenalty
			}
		}
		if c < bestCost {
			best, bestRoute, bestCost = e, r, c
		}
	}
	return best, bestRoute, bestRoute != nil
}

// backtrackExit is bestExit's way to a stand no exit reaches by taxiways
// (#1563: TNCM 10, no parallel taxiway to stands 0–20): back along the
// landing runway, turning round off an exit (a turning bay) or at its end,
// to an exit that does. Never one that turns on the spot (TurnsBack) or
// runs along another runway; the cheapest of those past the rollout.
func backtrackExit(g *airport.Graph, runwayEnd string, exits []airport.RunwayExit, parking int, opts airport.RouteOptions, reach exitReach) (airport.RunwayExit, *airport.Route, bool) {
	rwy, _, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return airport.RunwayExit{}, nil, false
	}
	opts.UseRunwayPaths = true
	var (
		best      airport.RunwayExit
		bestRoute *airport.Route
		bestCost  = math.Inf(1)
	)
	for i, e := range exits {
		if landingAlong(g, runwayEnd, e) < reach.rollout(ExitSpeed(e)) && i < len(exits)-1 {
			continue
		}
		r, err := g.RouteFromRunway(e, parking, opts)
		if err != nil || g.TurnsBack(r) {
			continue
		}
		if _, ok := backtrackClear(g, r, e, rwy); !ok {
			continue
		}
		c := exitCost(e, r, reach.runwayCostPerM()) + float64(len(r.RunwayCrossings))*airport.DefaultRunwayCrossingPenalty
		if c < bestCost {
			best, bestRoute, bestCost = e, r, c
		}
	}
	return best, bestRoute, bestRoute != nil
}

// backtrackClear: route r from exit x backtracks along runway rwy (a
// RUNWAY path after leaving it by x), and the route point where it is off
// rwy's surface for good; false when it does not backtrack, or runs along
// another runway.
func backtrackClear(g *airport.Graph, r *airport.Route, x airport.RunwayExit, rwy airport.Runway) (int, bool) {
	start := len(x.Path) - 1
	back := false
	for i := start; i < len(r.Edges); i++ {
		if r.Edges[i].Type != types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY {
			continue
		}
		if !onRunwaySurface(rwy, r.Points[i]) || !onRunwaySurface(rwy, r.Points[i+1]) {
			return 0, false
		}
		back = true
	}
	if !back {
		return 0, false
	}
	clear := start
	for i := start; i < len(r.Points); i++ {
		if onRunwaySurface(rwy, r.Points[i]) {
			clear = i + 1
		}
	}
	return min(clear, len(r.Points)-1), true
}

// onRunwaySurface reports whether p lies on runway r's surface.
func onRunwaySurface(r airport.Runway, p airport.LatLon) bool {
	a, b := r.Primary.Threshold, r.Secondary.Threshold
	along := calc.AlongTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon)
	off := math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon))
	return along >= -1 && along <= r.Length+1 && off <= r.Width/2+1
}

// backtrackHold is the hold-short node for runway rwy route r passes at
// or after its point from (the way off a backtrack), -1 none.
func backtrackHold(g *airport.Graph, r *airport.Route, from int, rwy airport.Runway) airport.NodeID {
	for i := from; i < len(r.Nodes)-2; i++ {
		if hs := g.Nodes[r.Nodes[i]].HoldShort; hs != nil && hs.Runway == rwy.Index {
			return r.Nodes[i]
		}
	}
	return -1
}

// Exit choice weights: meters of taxi-in equivalent per degree of turn above
// TurnAngleDeg, and per meter of extra rollout on the runway.
const (
	exitTurnCostPerDeg = 3.0
	exitRunwayCostPerM = 0.3
	// exitCrossBackPenalty: a taxi-in back across the runway just vacated.
	exitCrossBackPenalty = 100000.0
)

func exitCost(e airport.RunwayExit, r *airport.Route, perM float64) float64 {
	cost := r.Length + perM*e.Along
	pts := thin(simplify(r.Points), MinWaypointSpacingMeters)
	for i := 1; i+1 < len(pts); i++ {
		if a := turnAngle(pts[i-1], pts[i], pts[i+1]); a > TurnAngleDeg {
			cost += exitTurnCostPerDeg * (a - TurnAngleDeg)
		}
	}
	return cost
}

// vacateIndex returns the route point where an arriving aircraft stops clear
// of the runway: just past the hold-short behind the exit when the route
// passes it, else the first point VacateOffsetMeters from the centreline. It
// leaves at least two route points (the PARKING path) for the taxi-in.
func vacateIndex(g *airport.Graph, r *airport.Route, x airport.RunwayExit, rwy airport.Runway) int {
	last := len(r.Points) - 3
	start := len(x.Path) - 1
	for i := start; i <= last; i++ {
		if r.Nodes[i] == x.HoldShort {
			return min(i+1, last)
		}
	}
	for i := start; i <= last; i++ {
		p := r.Points[i]
		off := math.Abs(calc.CrossTrackMeters(rwy.Primary.Threshold.Lat, rwy.Primary.Threshold.Lon, rwy.Secondary.Threshold.Lat, rwy.Secondary.Threshold.Lon, p.Lat, p.Lon))
		if off >= VacateOffsetMeters {
			return i
		}
	}
	return max(start, min(start+1, last))
}

// thin drops points closer than minMeters to the last kept point, keeping
// both ends. Closely spaced waypoints make MSFS AI overshoot one at speed and
// loop back to it.
func thin(pts []airport.LatLon, minMeters float64) []airport.LatLon {
	if len(pts) <= 2 {
		return pts
	}
	out := []airport.LatLon{pts[0]}
	for _, p := range pts[1 : len(pts)-1] {
		last := out[len(out)-1]
		if calc.HaversineMeters(last.Lat, last.Lon, p.Lat, p.Lon) >= minMeters {
			out = append(out, p)
		}
	}
	end := pts[len(pts)-1]
	if last := out[len(out)-1]; len(out) > 1 && calc.HaversineMeters(last.Lat, last.Lon, end.Lat, end.Lon) < minMeters/2 {
		out = out[:len(out)-1] // keep the real end, drop the point crowding it
	}
	return append(out, end)
}

// ArrivalOptions shape an arrival plan.
type ArrivalOptions struct {
	// SpawnNm is how far out on final to start; 0 means DefaultSpawnNm.
	SpawnNm float64
	// Exit forces a runway exit; nil chooses one (see bestExit).
	Exit *airport.RunwayExit
	// Route controls taxi routing.
	Route airport.RouteOptions
	// GroundAGL sends ground waypoints at 0 ft above ground.
	GroundAGL bool
	// NoseOffset is the distance from the aircraft reference point to its
	// nose, used to stop on the stand (StandStop); 0 means DefaultNoseOffsetMeters.
	NoseOffset float64
	// StandSide moves the stand stop sideways, meters to the right of the
	// parking line (a gate whose simulator parking sits off it, so the
	// jetway meets the door); 0 on the line.
	StandSide float64
	// TouchdownKts and BrakeDecel (m/s²) are the aircraft's, for the exits
	// it can make and what its runway time costs (bestExit); 0: an
	// airliner's (TouchdownSpeedKts, RolloutDecel).
	TouchdownKts, BrakeDecel float64
	// endTurn: Exit is the exit back of a roll to the end planned already
	// (ArrivalPlan.EndTurn, #1658), planned so again.
	endTurn bool
}

// ArrivalPlan is a complete arrival for one aircraft.
type ArrivalPlan struct {
	Runway  airport.Runway
	End     airport.RunwayEnd
	Exit    airport.RunwayExit
	Route   *airport.Route // exit → stand
	SpawnNm float64
	Spawn   types.SIMCONNECT_DATA_INITPOSITION
	// Waypoints is the landing chain: approach, touchdown, rollout, exit and
	// the roll clear of the runway to the vacate stop.
	Waypoints []types.SIMCONNECT_DATA_WAYPOINT
	// TaxiWaypoints is the taxi-in chain from the vacate stop to the stand,
	// sent once the aircraft is cleared to taxi.
	TaxiWaypoints []types.SIMCONNECT_DATA_WAYPOINT
	// VacateIndex is the Route point where the aircraft stops clear of the
	// runway after landing.
	VacateIndex int
	// VacateBackMeters: the stop lies this far before route point
	// VacateIndex (along the route), short of the junction there.
	VacateBackMeters float64
	// Stop is where the aircraft stops on the stand (StandStop).
	Stop airport.LatLon
	// Backtrack: Route turns round off Exit (a turning bay, or the runway's
	// end) and taxis back along the landing runway to leave it further
	// back (#1563: no taxiway reaches the stand from any exit). The runway
	// stays taken until it is off it, at Route point ClearIndex.
	Backtrack  bool
	ClearIndex int
	// EndTurn: no exit ahead is in reach of the touchdown (#1631), so it
	// rolls past Exit to the runway's far end, turns round there and comes
	// back to leave by Exit, an exit of the opposite direction (Backtrack,
	// arrival_endturn.go); Exit.Along is from End's threshold. EndTurnMeters
	// is its way on the runway past Exit and back.
	EndTurn       bool
	EndTurnMeters float64
	// EndTurnoff: no exit ahead is in reach of the touchdown either, so it
	// rolls to the far end at taxi speed and leaves by Exit, a turnoff
	// there leading back (endTurnoff, #1674), with no backtrack.
	EndTurnoff bool
}

// ClearAt is the Route point where the aircraft is off the landing
// runway's surface: Exit's first taxiway node, or after a backtrack
// ClearIndex.
func (p *ArrivalPlan) ClearAt() int {
	if p.Backtrack && p.ClearIndex > 0 {
		return p.ClearIndex
	}
	return len(p.Exit.Path) - 1
}

// BacktrackMeters is how far the aircraft taxis on the runway after
// leaving it by Exit (Backtrack): to ClearAt; 0 none.
func (p *ArrivalPlan) BacktrackMeters() float64 {
	if !p.Backtrack || p.Route == nil {
		return 0
	}
	if p.EndTurn {
		return p.EndTurnMeters
	}
	m := 0.0
	for i := len(p.Exit.Path); i <= p.ClearAt() && i < len(p.Route.Points); i++ {
		m += localDist(p.Route.Points[i-1], p.Route.Points[i])
	}
	return m
}

// BacktrackOccupancy is the extra time the runway stays taken by a
// backtrack of meters at BacktrackKts, its turn round included.
func BacktrackOccupancy(meters float64) time.Duration {
	if meters <= 0 {
		return 0
	}
	return time.Duration((meters/(BacktrackKts*knot) + backtrackTurnSeconds) * float64(time.Second))
}

// backtrackTurnSeconds: turning round for a backtrack, slowing and on.
const backtrackTurnSeconds = 30.0

// PlanArrival builds the waypoint chain from spawnNm out on final for
// runwayEnd to the parking spot:
//
//   - approach waypoints every nautical mile on a 3° path, crossing the
//     threshold at 50 ft (ALTITUDE_IS_AGL, COMPUTE_VERTICAL_SPEED);
//   - an ON_GROUND touchdown waypoint TouchdownMeters past the threshold;
//   - rollout waypoints every RolloutSpacingMeters along the centreline,
//     slowing at RolloutDecel to the exit speed at the exit's runway node;
//   - TaxiInWaypoints from the exit to the stand.
//
// exit may be nil to choose the cheapest exit for the stand (bestExit). The
// aircraft must have its gear down (SetDataOnSimObject GEAR HANDLE POSITION = 1)
// or MSFS AI never touches down.
func PlanArrival(g *airport.Graph, runwayEnd string, parking int, o ArrivalOptions) (*ArrivalPlan, error) {
	spawnNm, exit, opts, groundAGL := o.SpawnNm, o.Exit, o.Route, o.GroundAGL
	if o.NoseOffset <= 0 {
		o.NoseOffset = DefaultNoseOffsetMeters
	}
	rwy, end, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return nil, airport.ErrUnknownRunway
	}
	if spawnNm < 2 {
		spawnNm = DefaultSpawnNm
	}
	var (
		x     airport.RunwayExit
		route *airport.Route
		err   error
		turnM = -1.0 // the end turn's runway meters (endTurnPlan); -1 none
		// turnoff: by a turnoff leading back at the far end (endTurnoff).
		turnoff bool
	)
	if exit != nil {
		x = *exit
		if route, err = g.RouteFromRunway(x, parking, opts); err != nil {
			return nil, err
		}
		// Planned again (#1658): still to the end and back, by another exit
		// back where this one's way to the new stand turns back.
		if o.endTurn {
			turnM = endTurnMeters(rwy, rwy.Length-x.Along)
			if g.TurnsBack(route) {
				if tx, tr, m, ok := endTurnPlan(g, runwayEnd, parking, o, exitReach{o.TouchdownKts, o.BrakeDecel}); ok {
					x, route, turnM = tx, tr, m
					x.Along = rwy.Length - tx.Along
				}
			}
		}
		// Planned again onto a turnoff leading back at the end: still so.
		turnoff = turnM < 0 && leadsBack(x)
	} else {
		// Never an exit the type only makes too fast (#1631: the last one
		// ahead, mid-length, taken at 100 kt): out of its reach, the roll
		// to the end and back to an exit, planned from the start.
		reach := exitReach{o.TouchdownKts, o.BrakeDecel}
		x, route, err = bestExit(g, runwayEnd, parking, opts, reach)
		if err != nil || !reach.makes(g, runwayEnd, x) {
			// A crew rolling out to the end at taxi speed takes the taxiway
			// there, whatever its angle (#1674); round on the runway and back
			// only when none reaches the stand.
			if tx, tr, ok := endTurnoff(g, runwayEnd, parking, opts, reach); ok {
				x, route, err, turnoff = tx, tr, nil, true
			} else if tx, tr, m, ok := endTurnPlan(g, runwayEnd, parking, o, reach); ok {
				x, route, err, turnM = tx, tr, nil, m
				x.Along = rwy.Length - tx.Along
			}
		}
		if err != nil {
			return nil, err
		}
	}
	elev := convert.MetersToFeet(g.Layout.Altitude)
	ground := groundAlt{feet: elev, agl: groundAGL}
	t := end.LandingThreshold()
	back := math.Mod(end.Heading+180, 360)

	p := &ArrivalPlan{Runway: rwy, End: end, Exit: x, Route: route, SpawnNm: spawnNm, EndTurnoff: turnoff}
	sLat, sLon := calc.DisplaceByHeading(t.Lat, t.Lon, back, spawnNm*1852)
	p.Spawn = types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: sLat, Longitude: sLon, Altitude: elev + GlidePathFtPerNm*spawnNm,
		Heading: end.Heading, Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(ApproachSpeedKts),
	}

	air := uint32(types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED | types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL | types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED)
	for nm := math.Ceil(spawnNm) - 1; nm >= 1; nm-- {
		lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, back, nm*1852)
		p.Waypoints = append(p.Waypoints, types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: GlidePathFtPerNm * nm, Flags: air, KtsSpeed: ApproachSpeedKts})
	}
	p.Waypoints = append(p.Waypoints, types.SIMCONNECT_DATA_WAYPOINT{Latitude: t.Lat, Longitude: t.Lon, Altitude: ThresholdCrossingFt, Flags: air, KtsSpeed: ApproachSpeedKts})

	// No flare waypoints: the flare matrix (#295) measured the softest
	// touchdown (about -100 fpm) with the threshold at 50 ft followed directly
	// by the ground waypoint; extra low-height points make the AI level off
	// and then drop.

	// Touchdown and rollout along the centreline to the exit's runway node.
	// The braking curve is planned backwards from the exit, so the aircraft
	// keeps its speed after touchdown and reaches the exit at exit speed
	// instead of crawling down the runway.
	exitKts := ExitSpeed(x)
	vx, v0 := exitKts*knot, TouchdownSpeedKts*knot
	toExit := x.Along - end.DisplacedM // Along is from the pavement end, s from t
	if turnM >= 0 {
		// Rolling to the end: slowing to the turn round there.
		r, a0 := endTurnRadius(rwy)
		toExit, exitKts = math.Min(rwy.Length-end.DisplacedM, end.LandingDistance(rwy.Length))-(a0+2*r), TurnSpeedKts
		vx = exitKts * knot
	}
	// At the exit speed by the end of a declared landing distance short of
	// the exit (#1660), on to it at that.
	slowBy := toExit
	if end.LDAM > 0 {
		slowBy = math.Min(toExit, end.LandingDistance(rwy.Length))
	}
	for s := TouchdownMeters; s < toExit; s += RolloutSpacingMeters {
		lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, end.Heading, s)
		v := math.Min(v0, math.Sqrt(vx*vx+2*RolloutDecel*math.Max(0, slowBy-s))) / knot
		p.Waypoints = append(p.Waypoints, ground.waypoint(lat, lon, math.Max(v, exitKts)))
	}
	rn := route.Points[0]
	p.Waypoints = append(p.Waypoints, ground.waypoint(rn.Lat, rn.Lon, exitKts))

	// Roll clear of the runway to the vacate stop, then taxi-in from there.
	// After a backtrack the stop is behind the way off the runway, not the
	// exit turned round on.
	vacx, maxKts := x, ExitSpeed(x)
	if clear, ok := backtrackClear(g, route, x, rwy); ok {
		p.Backtrack, p.ClearIndex = true, clear
		vacx.Path, vacx.HoldShort = route.Nodes[:clear+1], backtrackHold(g, route, clear, rwy)
		maxKts = math.Min(maxKts, BacktrackKts)
	}
	if turnM >= 0 {
		p.Backtrack, p.ClearIndex, p.EndTurn, p.EndTurnMeters = true, len(x.Path)-1, true, turnM
		maxKts = math.Min(maxKts, BacktrackKts)
	}
	p.VacateIndex, p.VacateBackMeters = vacateShort(g, route, vacx, vacateIndex(g, route, vacx, rwy))
	toStop := slices.Clone(route.Points[:p.VacateIndex+1])
	if p.VacateBackMeters > 0 {
		toStop[len(toStop)-1] = pointBack(toStop, p.VacateBackMeters)
	}
	p.Waypoints = append(p.Waypoints, groundLegs(thin(simplify(toStop), MinWaypointSpacingMeters), ground, legOptions{
		maxKts: maxKts, endKts: VacateStopKts, endMeters: 40,
	})...)
	in, err := taxiIn(g, route, ground, o.NoseOffset, o.StandSide, p.VacateIndex)
	if err != nil {
		return nil, err
	}
	p.TaxiWaypoints = in
	p.Stop, _ = standStop(g, route, o.NoseOffset, o.StandSide)
	return p, nil
}

// knot is one knot in meters per second.
const knot = 1852.0 / 3600

// endTurnoffM: a turnoff this far at most before the runway's far end is
// one an arrival rolling out to the end at taxi speed takes, whatever its
// angle up to MaxEntryAngle (#1674).
const endTurnoffM = 400.0

// leadsBack: x bends back sharper than MaxExitAngle, beyond the mapping
// noise of a right-angle stub (airport.ExitAngleTolerance, #1686: one at
// 92° is the 90° exit it is, taken as one).
func leadsBack(x airport.RunwayExit) bool {
	return x.Angle > airport.MaxExitAngle+airport.ExitAngleTolerance
}

// endTurnoff is PlanArrival's plan for an arrival on runwayEnd with no exit
// ahead in its reach (#1674): the cheapest turnoff in the last endTurnoffM
// that leads back sharper than MaxExitAngle (the exits up to it were
// bestExit's), reached braking firmly at its ExitSpeed, whose way on
// reaches the stand without a U-turn. False none.
func endTurnoff(g *airport.Graph, runwayEnd string, parking int, opts airport.RouteOptions, reach exitReach) (airport.RunwayExit, *airport.Route, bool) {
	rwy, _, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return airport.RunwayExit{}, nil, false
	}
	turnoffs, err := g.RunwayTurnoffs(runwayEnd, airport.MaxEntryAngle)
	if err != nil {
		return airport.RunwayExit{}, nil, false
	}
	var (
		best      airport.RunwayExit
		bestRoute *airport.Route
		bestCost  = math.Inf(1)
	)
	for _, x := range turnoffs {
		if !leadsBack(x) || x.Along < rwy.Length-endTurnoffM || !reach.makes(g, runwayEnd, x) {
			continue
		}
		r, err := g.RouteFromRunway(x, parking, opts)
		if err != nil || g.TurnsBack(r) {
			continue
		}
		if c := exitCost(x, r, reach.runwayCostPerM()); c < bestCost {
			best, bestRoute, bestCost = x, r, c
		}
	}
	return best, bestRoute, bestRoute != nil
}

// Stopping clear of the runway short of a junction, not on it (live: an
// arrival stopped where its exit met the parallel, blocking both).
const (
	// VacateJunctionMeters: the nose stops this far short of a taxiway
	// junction's centre, clear of the crossing taxiway.
	VacateJunctionMeters = 25.0
	// VacateTailMeters: the stop lies at least this far past the hold-short
	// line behind the exit, so the whole aircraft is clear of the runway.
	VacateTailMeters = 45.0
	// VacateMaxMeters: junctions this far past the exit at most are looked
	// at; beyond, the stop stays where it was.
	VacateMaxMeters = 400.0
)

// vacateShort places the vacate stop: just past the hold-short line behind
// the exit (VacateTailMeters), not rolling on to the next junction (live,
// OKZYC off 06 at L: 280 m down L to the D junction, then back). The stop is
// the route point returned less back meters (towards the runway). A junction
// at the stop is passed and the stop goes past it, short of the next; with
// no hold-short, the stop stays at v.
func vacateShort(g *airport.Graph, r *airport.Route, x airport.RunwayExit, v int) (int, float64) {
	last := len(r.Points) - 3
	cum := make([]float64, len(r.Points))
	for i := 1; i < len(r.Points); i++ {
		cum[i] = cum[i-1] + localDist(r.Points[i-1], r.Points[i])
	}
	from := len(x.Path) - 1
	minAt := cum[min(v, len(cum)-1)]
	for i := from; i <= last; i++ {
		if r.Nodes[i] == x.HoldShort {
			from, minAt = i, cum[i]+VacateTailMeters
			break
		}
	}
	// A junction: three or more taxiways meet (stands left out).
	junction := func(i int) bool {
		seen := map[airport.NodeID]bool{}
		for _, e := range g.Adj[r.Nodes[i]] {
			if g.Nodes[e.To].Kind != airport.NodeParking {
				seen[e.To] = true
			}
		}
		return len(seen) >= 3
	}
	for k := from + 1; k <= last && cum[k]-cum[from] <= VacateMaxMeters; k++ {
		if !junction(k) {
			continue
		}
		if cum[k]-VacateJunctionMeters >= minAt {
			break // short of it already
		}
		// Too close to stop short of: clear of it, short of the next.
		minAt = math.Max(minAt, cum[k]+VacateTailMeters)
	}
	// The first route point at or past minAt, back to minAt along the route.
	for k := from; k <= last; k++ {
		if cum[k] >= minAt {
			return k, cum[k] - minAt
		}
	}
	return v, 0
}

// pointBack is the point back meters before the end of pts, along them.
func pointBack(pts []airport.LatLon, back float64) airport.LatLon {
	for i := len(pts) - 1; i > 0; i-- {
		d := localDist(pts[i-1], pts[i])
		if back <= d {
			return offsetHeading(pts[i], localBearing(pts[i], pts[i-1]), back)
		}
		back -= d
	}
	return pts[0]
}

// TaxiRoute is the Route from the edge the vacate stop lies on to the stand:
// the taxi-in as ground clears it, without the exit off the runway.
func (p *ArrivalPlan) TaxiRoute() *airport.Route {
	k := p.VacateIndex
	if p.VacateBackMeters > 0 {
		k = max(k-1, 0) // the stop lies on the edge before
	}
	if p.Route == nil || k >= len(p.Route.Edges) {
		return p.Route
	}
	r := *p.Route
	r.Nodes, r.Points, r.Edges = r.Nodes[k:], r.Points[k:], r.Edges[k:]
	return &r
}

// VacateStop is where the aircraft stops clear of the runway.
func (p *ArrivalPlan) VacateStop() airport.LatLon {
	return pointBack(p.Route.Points[:p.VacateIndex+1], p.VacateBackMeters)
}
