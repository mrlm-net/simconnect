package calc

import "math"

// TrueToMagnetic converts a true heading to magnetic heading.
//
// magVar is the magnetic variation in degrees.
// Positive magVar = easterly variation (magnetic north is east of true north).
// Easterly variation: subtract from true to get magnetic.
// Westerly variation: add to true to get magnetic.
//
// Returns a heading normalised to [0, 360).
func TrueToMagnetic(trueHeading, magVar float64) float64 {
	return normalizeHeading(trueHeading - magVar)
}

// MagneticToTrue converts a magnetic heading to true heading.
//
// magVar is the magnetic variation in degrees (same sign convention as TrueToMagnetic).
// Positive magVar = easterly variation.
//
// Returns a heading normalised to [0, 360).
func MagneticToTrue(magneticHeading, magVar float64) float64 {
	return normalizeHeading(magneticHeading + magVar)
}

// normalizeHeading normalises an angle to [0, 360).
func normalizeHeading(h float64) float64 {
	h = math.Mod(h, 360.0)
	if h < 0 {
		h += 360.0
	}
	return h
}
