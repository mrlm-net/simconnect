package traffic

import (
	"errors"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Enroute aircraft (#369) appear airborne where their flight is now and
// fly the rest of it as MSFS AI on a waypoint chain: create them with
// Fleet.RequestNonATC at EnrouteStart's position, then ReleaseControl and
// SetWaypoints with its waypoints. (MSFS 2024 places an enroute ATC
// aircraft — AICreateEnrouteATCAircraft — on the ground at its plan's
// departure airport whatever the phase, and refuses one whose departure
// airport it has not loaded; a flight plan cannot start it mid-route.)

// RoutePoint is a point of a flight: where, at what altitude (feet MSL)
// and speed (knots).
type RoutePoint struct {
	Position airport.LatLon
	AltFt    float64
	Kts      float64
}

// EnrouteSpeedKts is the speed to fly at an altitude: 250 kt below
// 10 000 ft, the cruise speed above (default 420 kt).
func EnrouteSpeedKts(altFt, cruiseKts float64) float64 {
	if cruiseKts <= 0 {
		cruiseKts = 420
	}
	if altFt < 10000 {
		return math.Min(250, cruiseKts)
	}
	return cruiseKts
}

// EnrouteContinueMeters: the chain ends this far on along the last track,
// so the aircraft does not circle its last point.
const EnrouteContinueMeters = 60 * 1852

// EnrouteStart is where an aircraft flying route appears — at the first
// point, heading for the second, at its altitude and speed — and the
// waypoint chain it flies from there, ending EnrouteContinueMeters on
// along the last track.
func EnrouteStart(route []RoutePoint) (types.SIMCONNECT_DATA_INITPOSITION, []types.SIMCONNECT_DATA_WAYPOINT, error) {
	spawn, wps, err := enrouteChain(route)
	if err != nil || len(wps) == 0 {
		return spawn, wps, err
	}
	// Its corners are turns (from the spawn on), a jet's (TurnBankDeg).
	here := types.SIMCONNECT_DATA_WAYPOINT{Latitude: spawn.Latitude, Longitude: spawn.Longitude, KtsSpeed: wps[0].KtsSpeed}
	return spawn, roundCorners(append([]types.SIMCONNECT_DATA_WAYPOINT{here}, wps...), TurnBankDeg)[1:], nil
}

func enrouteChain(route []RoutePoint) (types.SIMCONNECT_DATA_INITPOSITION, []types.SIMCONNECT_DATA_WAYPOINT, error) {
	if len(route) < 2 {
		return types.SIMCONNECT_DATA_INITPOSITION{}, nil, errors.New("traffic: an enroute flight needs two points")
	}
	a, b := route[0], route[1]
	spawn := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: a.Position.Lat, Longitude: a.Position.Lon, Altitude: a.AltFt,
		Heading:  calc.BearingDegrees(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon),
		Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(a.Kts),
	}
	var wps []types.SIMCONNECT_DATA_WAYPOINT
	for _, p := range route[1:] {
		wps = append(wps, procedureWaypoint(p.Position, p.AltFt, p.Kts))
	}
	last, prev := route[len(route)-1], route[len(route)-2]
	track := calc.BearingDegrees(prev.Position.Lat, prev.Position.Lon, last.Position.Lat, last.Position.Lon)
	lat, lon := calc.DisplaceByHeading(last.Position.Lat, last.Position.Lon, track, EnrouteContinueMeters)
	wps = append(wps, procedureWaypoint(airport.LatLon{Lat: lat, Lon: lon}, last.AltFt, last.Kts))
	return spawn, wps, nil
}
