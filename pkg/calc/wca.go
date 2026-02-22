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
// Returns 0 if tas is zero or near-zero (undefined).
//
// Formula: WCA = asin((windSpeed / tas) * sin(windDir + 180° - course))
func WindCorrectionAngle(windDir, windSpeed, tas, course float64) float64 {
	if tas < 1e-9 {
		return 0
	}

	toRad := func(deg float64) float64 { return deg * math.Pi / 180.0 }
	toDeg := func(rad float64) float64 { return rad * 180.0 / math.Pi }

	// Wind direction the wind blows TO (reciprocal)
	windTo := windDir + 180.0

	sinWCA := (windSpeed / tas) * math.Sin(toRad(windTo-course))

	// Clamp to [-1, 1] to guard against floating-point overshoot
	sinWCA = math.Max(-1.0, math.Min(1.0, sinWCA))

	return toDeg(math.Asin(sinWCA))
}
