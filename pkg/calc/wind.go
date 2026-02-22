package calc

import "math"

// HeadwindCrosswind decomposes wind into headwind and crosswind components
// relative to a runway heading. Uses the meteorological convention: windDir is
// the direction FROM which the wind blows, in degrees true [0, 360).
// runwayHeading is the runway magnetic/true heading in degrees [0, 360).
// windSpeed is the wind speed in any consistent unit (knots, m/s, etc.).
//
// Returns:
//   - headwind: positive = into the runway (headwind), negative = tailwind
//   - crosswind: positive = from the right, negative = from the left
func HeadwindCrosswind(windDir, windSpeed, runwayHeading float64) (headwind, crosswind float64) {
	angle := (windDir - runwayHeading) * math.Pi / 180.0
	headwind = windSpeed * math.Cos(angle)
	crosswind = windSpeed * math.Sin(angle)
	return
}

// CrosswindComponent returns the crosswind component (perpendicular to runway).
// Positive means wind from the right.
func CrosswindComponent(windDir, windSpeed, runwayHeading float64) float64 {
	_, crosswind := HeadwindCrosswind(windDir, windSpeed, runwayHeading)
	return crosswind
}

// HeadwindComponent returns the headwind component (parallel to runway).
// Negative values indicate a tailwind.
func HeadwindComponent(windDir, windSpeed, runwayHeading float64) float64 {
	headwind, _ := HeadwindCrosswind(windDir, windSpeed, runwayHeading)
	return headwind
}
