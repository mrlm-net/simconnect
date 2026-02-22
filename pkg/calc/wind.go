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
