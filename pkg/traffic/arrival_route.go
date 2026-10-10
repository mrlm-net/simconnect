package traffic

import (
	"errors"
	"math"
	"slices"

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
// crossing the threshold at LightThresholdHeightFt, does at about 220 m)
// and brakes at its own BrakeDecel.
const (
	exitLightTouchdownKts    = 90.0
	exitLightTouchdownMeters = 250.0
)

func (x exitReach) light() bool {
	return x.touchdownKts > 0 && x.touchdownKts < exitLightTouchdownKts
}

func (x exitReach) speeds() (td, decel float64) {
	if x.light() {
		decel = RolloutDecel
		if x.decel > 0 {
			decel = x.decel
		}
		return x.touchdownKts, decel
	}
	return TouchdownSpeedKts, RolloutDecel
}

// rollout is the distance from the threshold to where the aircraft has
// slowed to exitKts.
func (x exitReach) rollout(exitKts float64) float64 {
	td, decel := x.speeds()
	v0, v1 := td*knot, math.Min(exitKts, td)*knot
	at := TouchdownMeters
	if x.light() {
		at = exitLightTouchdownMeters
	}
	return at + (v0*v0-v1*v1)/(2*decel)
}

// runwayCostPerM is what a meter of rollout costs in taxi-in meters:
// exitRunwayCostPerM for an airliner; for a light single it grows with the
// square of how much slower it lands (about six times at 51 kt). It holds
// the runway longer for every meter and taxis slowly anyway, so it takes
// the first exit it can make even for a longer taxi-in.
func (x exitReach) runwayCostPerM() float64 {
	td, _ := x.speeds()
	k := TouchdownSpeedKts / td
	return exitRunwayCostPerM * k * k
}

// ExitSpeed is the speed to reach a runway exit at: ExitHighSpeedKts for a
// high-speed exit, ExitSpeedKts otherwise.
func ExitSpeed(e airport.RunwayExit) float64 {
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
	return taxiIn(g, r, groundAlt{feet: convert.MetersToFeet(g.Layout.Altitude)}, DefaultNoseOffsetMeters, 0)
}

// StandStop returns where an aircraft's reference point should stop on the
// stand at the end of route r: MSFS stands are circles sized for the largest
// aircraft allowed, and aircraft park with the nose at the front of the
// circle, so the stop point is RADIUS minus noseOffset (the distance from
// the aircraft's reference point to its nose) ahead of the centre along the
// stand heading.
func StandStop(g *airport.Graph, r *airport.Route, noseOffset float64) (airport.LatLon, error) {
	n := len(r.Nodes)
	if n < 2 || g.Nodes[r.Nodes[n-1]].Kind != airport.NodeParking {
		return airport.LatLon{}, ErrNotStandRoute
	}
	return StandPoint(g.Layout.Parking[g.Nodes[r.Nodes[n-1]].Index], noseOffset), nil
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

func taxiIn(g *airport.Graph, r *airport.Route, alt groundAlt, noseOffset float64, from int) ([]types.SIMCONNECT_DATA_WAYPOINT, error) {
	n := len(r.Nodes)
	if n < 3 || g.Nodes[r.Nodes[n-1]].Kind != airport.NodeParking {
		return nil, ErrNotStandRoute
	}
	pts := r.Points
	junction := pts[n-2]
	stand, err := StandStop(g, r, noseOffset)
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

// bestExit chooses the runway exit for an arrival to parking: among exits the
// aircraft can reach at its exit speed (Along ≥ its rollout, exitReach), the
// one with the lowest taxi-in cost — route length, plus a penalty for every
// turn sharper than TurnAngleDeg, plus runway occupancy for exits further
// down (more for a slow aircraft). If none is reachable, the last exit is
// used.
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
		if e.Along < reach.rollout(ExitSpeed(e)) && i < len(exits)-1 {
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
	}
	if bestRoute == nil {
		return airport.RunwayExit{}, nil, lastErr
	}
	return best, bestRoute, nil
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
	// TouchdownKts and BrakeDecel (m/s²) are the aircraft's, for the exits
	// it can make and what its runway time costs (bestExit); 0: an
	// airliner's (TouchdownSpeedKts, RolloutDecel).
	TouchdownKts, BrakeDecel float64
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
}

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
	)
	if exit != nil {
		x = *exit
		if route, err = g.RouteFromRunway(x, parking, opts); err != nil {
			return nil, err
		}
	} else if x, route, err = bestExit(g, runwayEnd, parking, opts, exitReach{o.TouchdownKts, o.BrakeDecel}); err != nil {
		return nil, err
	}
	elev := convert.MetersToFeet(g.Layout.Altitude)
	ground := groundAlt{feet: elev, agl: groundAGL}
	t := end.Threshold
	back := math.Mod(end.Heading+180, 360)

	p := &ArrivalPlan{Runway: rwy, End: end, Exit: x, Route: route, SpawnNm: spawnNm}
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
	for s := TouchdownMeters; s < x.Along; s += RolloutSpacingMeters {
		lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, end.Heading, s)
		v := math.Min(v0, math.Sqrt(vx*vx+2*RolloutDecel*(x.Along-s))) / knot
		p.Waypoints = append(p.Waypoints, ground.waypoint(lat, lon, math.Max(v, exitKts)))
	}
	rn := route.Points[0]
	p.Waypoints = append(p.Waypoints, ground.waypoint(rn.Lat, rn.Lon, exitKts))

	// Roll clear of the runway to the vacate stop, then taxi-in from there.
	p.VacateIndex, p.VacateBackMeters = vacateShort(g, route, x, vacateIndex(g, route, x, rwy))
	toStop := slices.Clone(route.Points[:p.VacateIndex+1])
	if p.VacateBackMeters > 0 {
		toStop[len(toStop)-1] = pointBack(toStop, p.VacateBackMeters)
	}
	p.Waypoints = append(p.Waypoints, groundLegs(thin(simplify(toStop), MinWaypointSpacingMeters), ground, legOptions{
		maxKts: ExitSpeed(x), endKts: VacateStopKts, endMeters: 40,
	})...)
	in, err := taxiIn(g, route, ground, o.NoseOffset, p.VacateIndex)
	if err != nil {
		return nil, err
	}
	p.TaxiWaypoints = in
	p.Stop, _ = StandStop(g, route, o.NoseOffset)
	return p, nil
}

// knot is one knot in meters per second.
const knot = 1852.0 / 3600

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

// vacateShort moves the vacate stop at route point v to just short of the
// first taxiway junction after the runway: the stop is the route point
// returned less back meters (towards the runway). A junction too close to
// leave the aircraft clear of the runway is passed, and the stop goes short
// of the next one; with none, the stop stays at v.
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
			return k, VacateJunctionMeters
		}
		// Too close to stop short of: clear of it, short of the next.
		minAt = math.Max(minAt, cum[k]+VacateTailMeters)
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

// VacateStop is where the aircraft stops clear of the runway.
func (p *ArrivalPlan) VacateStop() airport.LatLon {
	return pointBack(p.Route.Points[:p.VacateIndex+1], p.VacateBackMeters)
}
