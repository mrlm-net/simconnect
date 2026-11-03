package convert

import (
	"math"
	"testing"
)

// TestConversions: each unit conversion against known values.
func TestConversions(t *testing.T) {
	for _, c := range []struct {
		name     string
		fn       func(float64) float64
		in, want float64
	}{
		{"KilometersToMeters", KilometersToMeters, 1.5, 1500},
		{"MetersToKilometers", MetersToKilometers, 1500, 1.5},
		{"StatuteMilesToMeters", StatuteMilesToMeters, 1, 1609.344},
		{"MetersToStatuteMiles", MetersToStatuteMiles, 1609.344, 1},
		{"PascalToInHg", PascalToInHg, 3386.3900, 1},
		{"FeetToMeters", FeetToMeters, 0, 0},                                                    // zero
		{"FeetToMeters", FeetToMeters, 35000, 10668.0},                                          // FL350 (35000 ft)
		{"FeetToMeters", FeetToMeters, 1, 0.3048},                                               // one foot
		{"FeetToMeters", FeetToMeters, -1000, -304.8},                                           // negative (below sea level)
		{"MetersToFeet", MetersToFeet, 0, 0},                                                    // zero
		{"MetersToFeet", MetersToFeet, 10668.0, 35000.0},                                        // 10668 m (FL350)
		{"MetersToFeet", MetersToFeet, 1, 1.0 / 0.3048},                                         // one meter
		{"MetersToFeet", MetersToFeet, -304.8, -1000.0},                                         // negative
		{"NormalizeHeading", NormalizeHeading, 0, 0},                                            // zero
		{"NormalizeHeading", NormalizeHeading, 180, 180},                                        // 180
		{"NormalizeHeading", NormalizeHeading, 359, 359},                                        // 359
		{"NormalizeHeading", NormalizeHeading, 360, 0},                                          // 360 wraps to 0
		{"NormalizeHeading", NormalizeHeading, 720, 0},                                          // 720 wraps to 0
		{"NormalizeHeading", NormalizeHeading, -90, 270},                                        // negative wraps to 270
		{"NormalizeHeading", NormalizeHeading, -10, 350},                                        // negative wraps to 350
		{"NormalizeAngle", NormalizeAngle, 0, 0},                                                // zero
		{"NormalizeAngle", NormalizeAngle, 180, 180},                                            // 180 stays 180
		{"NormalizeAngle", NormalizeAngle, 181, -179},                                           // 181 wraps to -179
		{"NormalizeAngle", NormalizeAngle, -180, 180},                                           // -180 wraps to 180
		{"NormalizeAngle", NormalizeAngle, 360, 0},                                              // 360 wraps to 0
		{"NormalizeAngle", NormalizeAngle, -361, -1},                                            // -361 wraps to -1
		{"NormalizeAngle", NormalizeAngle, 90, 90},                                              // 90 stays 90
		{"NormalizeAngle", NormalizeAngle, -90, -90},                                            // -90 stays -90
		{"NMToMeters", NMToMeters, 0, 0},                                                        // zero
		{"NMToMeters", NMToMeters, 1, 1852.0},                                                   // 1 NM
		{"NMToMeters", NMToMeters, 0.5, 926.0},                                                  // fractional
		{"NMToMeters", NMToMeters, 100, 185200.0},                                               // large
		{"MetersToNM", MetersToNM, 0, 0},                                                        // zero
		{"MetersToNM", MetersToNM, 1852.0, 1.0},                                                 // 1852 m
		{"MetersToNM", MetersToNM, 926.0, 0.5},                                                  // fractional
		{"MetersToNM", MetersToNM, 185200.0, 100.0},                                             // large
		{"NMToStatuteMiles", NMToStatuteMiles, 0, 0},                                            // zero
		{"NMToStatuteMiles", NMToStatuteMiles, 1, 1852.0 / statuteMileToMeters},                 // 1 NM ≈ 1.15078 statute miles
		{"NMToStatuteMiles", NMToStatuteMiles, 10, 10 * 1852.0 / statuteMileToMeters},           // 10 NM
		{"StatuteMilesToNM", StatuteMilesToNM, 0, 0},                                            // zero
		{"StatuteMilesToNM", StatuteMilesToNM, 1, statuteMileToMeters / 1852.0},                 // 1 statute mile ≈ 0.86898 NM
		{"StatuteMilesToNM", StatuteMilesToNM, 10, 10 * statuteMileToMeters / 1852.0},           // 10 statute miles
		{"KilometersToStatuteMiles", KilometersToStatuteMiles, 0, 0},                            // zero
		{"KilometersToStatuteMiles", KilometersToStatuteMiles, 1, 1000.0 / statuteMileToMeters}, // 1 km ≈ 0.62137 statute miles
		{"KilometersToStatuteMiles", KilometersToStatuteMiles, 1.609344, 1.0},                   // 1.609344 km = 1 statute mile
		{"StatuteMilesToKilometers", StatuteMilesToKilometers, 0, 0},                            // zero
		{"StatuteMilesToKilometers", StatuteMilesToKilometers, 1, 1.609344},                     // 1 statute mile = 1.609344 km
		{"StatuteMilesToKilometers", StatuteMilesToKilometers, 2, 2 * 1.609344},                 // 2 statute miles
		{"KnotsToKilometersPerHour", KnotsToKilometersPerHour, 0, 0},                            // zero
		{"KnotsToKilometersPerHour", KnotsToKilometersPerHour, 250, 463.0},                      // 250 kts cruise
		{"KnotsToKilometersPerHour", KnotsToKilometersPerHour, 1, 1.852},                        // one knot
		{"KnotsToKilometersPerHour", KnotsToKilometersPerHour, 500, 926.0},                      // high speed
		{"KilometersPerHourToKnots", KilometersPerHourToKnots, 0, 0},                            // zero
		{"KilometersPerHourToKnots", KilometersPerHourToKnots, 463.0, 250.0},                    // 463 kph
		{"KilometersPerHourToKnots", KilometersPerHourToKnots, 1.852, 1.0},                      // 1.852 kph
		{"KilometersPerHourToKnots", KilometersPerHourToKnots, 926.0, 500.0},                    // high speed
		{"KnotsToMach", KnotsToMach, 0, 0},                                                      // zero
		{"KnotsToMach", KnotsToMach, mach1Knots, 1.0},                                           // Mach 1
		{"KnotsToMach", KnotsToMach, mach1Knots * 0.82, 0.82},                                   // Mach 0.82 cruise
		{"KnotsToMach", KnotsToMach, 250, 250.0 / mach1Knots},                                   // subsonic 250 kts
		{"MachToKnots", MachToKnots, 0, 0},                                                      // zero
		{"MachToKnots", MachToKnots, 1.0, mach1Knots},                                           // Mach 1
		{"MachToKnots", MachToKnots, 0.82, mach1Knots * 0.82},                                   // Mach 0.82
		{"MachToKnots", MachToKnots, 2.0, mach1Knots * 2.0},                                     // Mach 2
		{"KilometersPerHourToMach", KilometersPerHourToMach, 0, 0},                              // zero
		{"KilometersPerHourToMach", KilometersPerHourToMach, mach1KPH, 1.0},                     // Mach 1
		{"KilometersPerHourToMach", KilometersPerHourToMach, mach1KPH * 0.82, 0.82},             // Mach 0.82 cruise
		{"KilometersPerHourToMach", KilometersPerHourToMach, 300, 300.0 / mach1KPH},             // low speed
		{"MachToKilometersPerHour", MachToKilometersPerHour, 0, 0},                              // zero
		{"MachToKilometersPerHour", MachToKilometersPerHour, 1.0, mach1KPH},                     // Mach 1
		{"MachToKilometersPerHour", MachToKilometersPerHour, 0.82, mach1KPH * 0.82},             // Mach 0.82
		{"MachToKilometersPerHour", MachToKilometersPerHour, 2.0, mach1KPH * 2.0},               // Mach 2
		{"KnotsToFeetPerSecond", KnotsToFeetPerSecond, 0, 0},                                    // zero
		{"KnotsToFeetPerSecond", KnotsToFeetPerSecond, 1, knotsToFPS},                           // 1 knot ≈ 1.68781 fps
		{"KnotsToFeetPerSecond", KnotsToFeetPerSecond, 100, 100 * knotsToFPS},                   // 100 knots ≈ 168.781 fps
		{"FeetPerSecondToKnots", FeetPerSecondToKnots, 0, 0},                                    // zero
		{"FeetPerSecondToKnots", FeetPerSecondToKnots, knotsToFPS, 1},                           // knotsToFPS → 1 knot
		{"FeetPerSecondToKnots", FeetPerSecondToKnots, 100 * knotsToFPS, 100},                   // 100 * knotsToFPS → 100 knots
		{"PoundsToKilograms", PoundsToKilograms, 1, 0.45359237},                                 // one pound
		{"PoundsToKilograms", PoundsToKilograms, 0, 0},                                          // zero
		{"PoundsToKilograms", PoundsToKilograms, 100, 45.359237},                                // 100 lbs
		{"USGallonsToLiters", USGallonsToLiters, 1, 3.785411784},                                // one US gallon
		{"USGallonsToLiters", USGallonsToLiters, 0, 0},                                          // zero
	} {
		if got := c.fn(c.in); math.Abs(got-c.want) > epsilon {
			t.Errorf("%s(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

// TestSpeedConstantsLiteral: the rows above take their wants from the
// package's own constants, so a wrong constant passes there. These are the
// published values, to the precision they are quoted at.
func TestSpeedConstantsLiteral(t *testing.T) {
	for _, c := range []struct {
		name      string
		fn        func(float64) float64
		in, want  float64
		tolerance float64
	}{
		{"MachToKnots", MachToKnots, 1, 661.47, 0.01},                          // Mach 1, sea level ISA
		{"MachToKilometersPerHour", MachToKilometersPerHour, 1, 1225.06, 0.01}, // Mach 1, sea level ISA
		{"KnotsToFeetPerSecond", KnotsToFeetPerSecond, 1, 1.68781, 1e-5},       // 1 kt
	} {
		if got := c.fn(c.in); math.Abs(got-c.want) > c.tolerance {
			t.Errorf("%s(%v) = %v, want %v ± %v", c.name, c.in, got, c.want, c.tolerance)
		}
	}
}
