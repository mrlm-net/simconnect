package calc

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/convert"
)

// DisplaceByHeading returns the latitude and longitude of the point reached by
// travelling distanceMeters along hdgDeg (degrees true, clockwise from north)
// from (lat, lon).
//
// hdgDeg is normalised internally; values outside [0, 360) are accepted.
// distanceMeters may be negative to displace in the reverse heading direction.
//
// Uses the WGS84 ellipsoid via convert.OffsetToLatLon; accurate to sub-metre
// for distances up to approximately 50 km from the reference point.
func DisplaceByHeading(lat, lon, hdgDeg, distanceMeters float64) (float64, float64) {
	rad := hdgDeg * math.Pi / 180.0
	return convert.OffsetToLatLon(lat, lon,
		distanceMeters*math.Sin(rad), // east component (X)
		distanceMeters*math.Cos(rad), // north component (Z)
	)
}
