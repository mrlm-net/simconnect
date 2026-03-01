package calc

import "math"

// BearingDegrees returns the initial great-circle bearing in degrees [0, 360)
// from the point (lat1, lon1) toward (lat2, lon2). Input in decimal degrees.
func BearingDegrees(lat1, lon1, lat2, lon2 float64) float64 {
	toRad := func(deg float64) float64 { return deg * math.Pi / 180.0 }
	lat1R := toRad(lat1)
	lat2R := toRad(lat2)
	dLon := toRad(lon2 - lon1)
	x := math.Sin(dLon) * math.Cos(lat2R)
	y := math.Cos(lat1R)*math.Sin(lat2R) - math.Sin(lat1R)*math.Cos(lat2R)*math.Cos(dLon)
	theta := math.Atan2(x, y) * 180.0 / math.Pi
	return math.Mod(theta+360.0, 360.0)
}

// BearingFromOffsets returns the bearing in degrees [0, 360) from the local
// coordinate origin toward (xEast, zNorth). Uses SimConnect convention: X=east,
// Z=north. Returns 0 for the zero vector (both inputs are 0).
func BearingFromOffsets(xEast, zNorth float64) float64 {
	return math.Mod(math.Atan2(xEast, zNorth)*180.0/math.Pi+360.0, 360.0)
}
