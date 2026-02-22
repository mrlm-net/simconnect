package convert

import "math"

func DegreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}

func RadiansToDegrees(rad float64) float64 {
	return rad * 180.0 / math.Pi
}

// NormalizeHeading returns a heading in [0, 360).
func NormalizeHeading(deg float64) float64 {
	h := math.Mod(deg, 360.0)
	if h < 0 {
		h += 360.0
	}
	return h
}

// NormalizeAngle normalises an angle to (-180, 180].
// Useful for signed bearing differences and track deviation calculations.
func NormalizeAngle(deg float64) float64 {
	deg = math.Mod(deg, 360.0)
	if deg > 180.0 {
		deg -= 360.0
	} else if deg <= -180.0 {
		deg += 360.0
	}
	return deg
}

// AngleDifference returns the shortest signed angular difference from → to,
// in degrees, in the range (-180, 180].
// Positive = clockwise (to is to the right of from).
// Negative = counter-clockwise (to is to the left of from).
func AngleDifference(from, to float64) float64 {
	return NormalizeAngle(to - from)
}
