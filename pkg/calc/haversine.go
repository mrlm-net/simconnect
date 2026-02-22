package calc

import "math"

// HaversineMeters calculates the great-circle distance in meters between two
// geographic coordinates using the haversine formula. Input coordinates are in
// decimal degrees. Uses Earth mean radius of 6,371,000 meters.
// earthRadiusM is the mean Earth radius in metres used across all great-circle calculations.
const earthRadiusM = 6371000.0

func HaversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	toRad := func(deg float64) float64 { return deg * math.Pi / 180.0 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

// HaversineNM calculates the great-circle distance in nautical miles between
// two geographic coordinates using the haversine formula.
func HaversineNM(lat1, lon1, lat2, lon2 float64) float64 {
	return HaversineMeters(lat1, lon1, lat2, lon2) / 1852.0
}

// HaversineKM calculates the great-circle distance in kilometres between
// two geographic coordinates using the haversine formula.
func HaversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	return HaversineMeters(lat1, lon1, lat2, lon2) / 1000.0
}
