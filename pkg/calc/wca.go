package calc

import "math"

// WindCorrectionAngle returns the wind correction angle (WCA) in degrees
// needed to maintain a desired course, given wind conditions.
//
// Parameters:
//   - windDir: wind direction the wind is coming FROM, in degrees true (0-359)
//   - windSpeed: wind speed in knots
//   - tas: true airspeed in knots
//   - course: desired track/course in degrees true (0-359)
//
// Returns the WCA in degrees. Positive = correct right; negative = correct left.
// The heading to fly is course + WCA, into the wind: a wind from the left of
// the course gives a negative WCA.
// Returns 0 if tas is zero or near-zero (undefined).
//
// Formula: WCA = asin((windSpeed / tas) * sin(windDir - course))
func WindCorrectionAngle(windDir, windSpeed, tas, course float64) float64 {
	if tas < 1e-9 {
		return 0
	}

	toRad := func(deg float64) float64 { return deg * math.Pi / 180.0 }
	toDeg := func(rad float64) float64 { return rad * 180.0 / math.Pi }

	// A wind from the right of the course drifts the aircraft left: turn
	// right, into it.
	sinWCA := (windSpeed / tas) * math.Sin(toRad(windDir-course))

	// Clamp to [-1, 1] to guard against floating-point overshoot
	sinWCA = math.Max(-1.0, math.Min(1.0, sinWCA))

	return toDeg(math.Asin(sinWCA))
}
