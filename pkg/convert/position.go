package convert

import "math"

// OffsetToLatLon calculates the latitude and longitude of a point
// given a reference point (latRef, lonRef in degrees) and offsets
// X (east, meters) and Z (north, meters).
// At the geographic poles (latRef = ±90) eastward offset is undefined;
// deltaLon is forced to zero to avoid a division-by-zero singularity.
func OffsetToLatLon(latRef, lonRef, xEast, zNorth float64) (lat, lon float64) {
	// WGS84 ellipsoid
	const a = 6378137.0
	const b = 6356752.314245
	e2 := (a*a - b*b) / (a * a)

	latRefRad := latRef * math.Pi / 180.0
	sinLat := math.Sin(latRefRad)
	W := math.Sqrt(1 - e2*sinLat*sinLat)

	// Meridian radius of curvature (M) and prime vertical radius (N)
	M := a * (1 - e2) / (W * W * W)
	N := a / W

	deltaLat := (zNorth / M) * (180.0 / math.Pi)

	var deltaLon float64
	if math.Abs(latRef) < 90.0 {
		deltaLon = (xEast / (N * math.Cos(latRefRad))) * (180.0 / math.Pi)
	}

	lat = latRef + deltaLat
	lon = lonRef + deltaLon
	return
}
