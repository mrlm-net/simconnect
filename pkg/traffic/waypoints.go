//go:build windows
// +build windows

package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// PushbackWaypoint creates a waypoint for a reverse pushback manoeuvre.
//
// Flags: ON_GROUND | REVERSE | SPEED_REQUESTED.
//
// Set lat/lon to the taxiway centreline node where the pushback ends — typically
// one or two hops from the gate spur entry node toward the main taxiway.
// A speed of 2–4 kts matches a realistic pushback truck pace.
func PushbackWaypoint(lat, lon, altFt, ktsSpeed float64) types.SIMCONNECT_DATA_WAYPOINT {
	return types.SIMCONNECT_DATA_WAYPOINT{
		Latitude:  lat,
		Longitude: lon,
		Altitude:  altFt,
		Flags: uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND |
			types.SIMCONNECT_WAYPOINT_REVERSE |
			types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED),
		KtsSpeed: ktsSpeed,
	}
}

// TaxiWaypoint creates a ground taxi waypoint.
//
// Flags: ON_GROUND | SPEED_REQUESTED.
//
// Typical taxi speed is 12–18 kts. Use this for each node along the taxi route
// from the taxiway centreline to the runway threshold.
func TaxiWaypoint(lat, lon, altFt, ktsSpeed float64) types.SIMCONNECT_DATA_WAYPOINT {
	return types.SIMCONNECT_DATA_WAYPOINT{
		Latitude:  lat,
		Longitude: lon,
		Altitude:  altFt,
		Flags: uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND |
			types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED),
		KtsSpeed: ktsSpeed,
	}
}

// LineupWaypoint creates the final ground waypoint at the runway threshold.
//
// Flags: ON_GROUND | SPEED_REQUESTED at 5 kts for precise lineup.
//
// Placing this as the last ON_GROUND waypoint immediately before a ClimbWaypoint
// sequence triggers the simulator's takeoff roll when the AI transitions to the
// first airborne waypoint.
func LineupWaypoint(lat, lon, altFt float64) types.SIMCONNECT_DATA_WAYPOINT {
	return TaxiWaypoint(lat, lon, altFt, 5)
}

// ClimbWaypoint creates an airborne climb waypoint.
//
// Flags: SPEED_REQUESTED | THROTTLE_REQUESTED | COMPUTE_VERTICAL_SPEED | ALTITUDE_IS_AGL.
//
// altAGL is height above ground level in feet. throttlePct is 0–100.
// COMPUTE_VERTICAL_SPEED lets the simulator derive the required vertical speed
// to reach each altitude at the waypoint. ALTITUDE_IS_AGL makes altitudes
// terrain-relative rather than MSL.
//
// The transition from the last ON_GROUND waypoint to the first ClimbWaypoint
// triggers the takeoff roll and rotation sequence.
func ClimbWaypoint(lat, lon, altAGL, ktsSpeed, throttlePct float64) types.SIMCONNECT_DATA_WAYPOINT {
	return types.SIMCONNECT_DATA_WAYPOINT{
		Latitude:  lat,
		Longitude: lon,
		Altitude:  altAGL,
		Flags: uint32(
			types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED |
				types.SIMCONNECT_WAYPOINT_THROTTLE_REQUESTED |
				types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED |
				types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL,
		),
		KtsSpeed:        ktsSpeed,
		PercentThrottle: throttlePct,
	}
}

// TakeoffClimb builds the standard 3-waypoint climb chain from a runway threshold.
//
// rwyLat/rwyLon is the primary runway threshold. hdgDeg is the primary runway
// heading (the direction the aircraft will climb toward).
//
// Returns 3 ClimbWaypoints:
//   - 1.5 nm out / 1 500 ft AGL / 200 kts / 100 % throttle
//   - 5.0 nm out / 4 000 ft AGL / 240 kts /  90 % throttle
//   - 12  nm out / 9 000 ft AGL / 280 kts /  85 % throttle
//
// Append these directly after a LineupWaypoint to complete a full departure
// sequence: [PushbackWaypoint] [TaxiWaypoint...] [LineupWaypoint] [TakeoffClimb...]
func TakeoffClimb(rwyLat, rwyLon, hdgDeg float64) []types.SIMCONNECT_DATA_WAYPOINT {
	c1Lat, c1Lon := ahead(rwyLat, rwyLon, hdgDeg, 2778)  // 1.5 nm
	c2Lat, c2Lon := ahead(rwyLat, rwyLon, hdgDeg, 9260)  // 5 nm
	c3Lat, c3Lon := ahead(rwyLat, rwyLon, hdgDeg, 22224) // 12 nm
	return []types.SIMCONNECT_DATA_WAYPOINT{
		ClimbWaypoint(c1Lat, c1Lon, 1500, 200, 100),
		ClimbWaypoint(c2Lat, c2Lon, 4000, 240, 90),
		ClimbWaypoint(c3Lat, c3Lon, 9000, 280, 85),
	}
}

// ahead returns the lat/lon displaced from (lat, lon) by the given distance in
// meters along heading hdgDeg. Uses convert.OffsetToLatLon under the hood.
func ahead(lat, lon, hdgDeg, meters float64) (float64, float64) {
	rad := hdgDeg * math.Pi / 180
	return convert.OffsetToLatLon(lat, lon,
		meters*math.Sin(rad), // east component (X)
		meters*math.Cos(rad), // north component (Z)
	)
}
