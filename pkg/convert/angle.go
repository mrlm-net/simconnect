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
