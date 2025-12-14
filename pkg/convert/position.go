//go:build windows
// +build windows

package convert

import "math"

// PositionFromBias returns a new latitude and longitude (degrees) when starting
// from latRef/lonRef and applying a forward (meters along airport heading)
// and right (meters to the right of the heading) offset.
// headingDeg is azimuth clockwise from north.
func PositionFromBias(latRef, lonRef, headingDeg, forwardMeters, rightMeters float64) (lat, lon float64) {
	theta := headingDeg * math.Pi / 180.0
	// heading unit: (sinθ east, cosθ north)
	// right unit:   (cosθ east, -sinθ north)
	east := forwardMeters*math.Sin(theta) + rightMeters*math.Cos(theta)
	north := forwardMeters*math.Cos(theta) - rightMeters*math.Sin(theta)

	metersPerDegLat := 110574.0
	metersPerDegLon := 111319.9 * math.Cos(latRef*math.Pi/180.0)

	lat = latRef + north/metersPerDegLat
	lon = lonRef + east/metersPerDegLon
	return
}
