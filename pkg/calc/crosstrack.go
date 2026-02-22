package calc

import "math"

// CrossTrackMeters returns the cross-track distance (in metres) from point D
// to the great-circle path defined by points A → B.
//
// Positive values indicate the point is to the right of the track;
// negative values indicate left of the track.
//
// All latitude/longitude arguments are in decimal degrees.
// earthRadiusM is the mean Earth radius used for distance calculations.
func CrossTrackMeters(latA, lonA, latB, lonB, latD, lonD float64) float64 {
	toRad := func(deg float64) float64 { return deg * math.Pi / 180.0 }

	φA, λA := toRad(latA), toRad(lonA)
	φB, λB := toRad(latB), toRad(lonB)
	φD, λD := toRad(latD), toRad(lonD)

	// Angular distance from A to D along great circle
	Δφ := φD - φA
	Δλ := λD - λA
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φA)*math.Cos(φD)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	δAD := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	// Initial bearing from A to D
	y := math.Sin(λD-λA) * math.Cos(φD)
	x := math.Cos(φA)*math.Sin(φD) - math.Sin(φA)*math.Cos(φD)*math.Cos(λD-λA)
	θAD := math.Atan2(y, x)

	// Initial bearing from A to B
	y2 := math.Sin(λB-λA) * math.Cos(φB)
	x2 := math.Cos(φA)*math.Sin(φB) - math.Sin(φA)*math.Cos(φB)*math.Cos(λB-λA)
	θAB := math.Atan2(y2, x2)

	// Cross-track distance = R * asin(sin(δAD) * sin(θAD − θAB))
	// Clamp to [-1, 1] to guard against floating-point overshoot near boundary conditions.
	arg := math.Sin(δAD) * math.Sin(θAD-θAB)
	arg = math.Max(-1.0, math.Min(1.0, arg))
	return earthRadiusM * math.Asin(arg)
}
