package convert

// mach1Knots is the speed of sound at sea level ISA conditions in knots.
const mach1Knots = 661.4788

// mach1KPH is derived from mach1Knots to ensure cross-unit consistency.
const mach1KPH = mach1Knots * 1.852

// knotsToMS is the exact SI conversion factor (1852 m/NM ÷ 3600 s/hr).
const knotsToMS = 1852.0 / 3600.0

func KnotsToKilometersPerHour(knots float64) float64 {
	return knots * 1.852
}

func KilometersPerHourToKnots(kph float64) float64 {
	return kph / 1.852
}

func KnotsToMach(knots float64) float64 {
	return knots / mach1Knots
}

func MachToKnots(mach float64) float64 {
	return mach * mach1Knots
}

func KilometersPerHourToMach(kph float64) float64 {
	return kph / mach1KPH
}

func MachToKilometersPerHour(mach float64) float64 {
	return mach * mach1KPH
}

func KnotsToMetersPerSecond(knots float64) float64 {
	return knots * knotsToMS
}

func MetersPerSecondToKnots(ms float64) float64 {
	return ms / knotsToMS
}

// FeetPerMinuteToMetersPerSecond converts vertical speed from ft/min to m/s.
// Exact factor: 0.3048 m/ft ÷ 60 s/min = 0.00508 m/s per ft/min.
func FeetPerMinuteToMetersPerSecond(fpm float64) float64 {
	return fpm * 0.00508
}

func MetersPerSecondToFeetPerMinute(ms float64) float64 {
	return ms / 0.00508
}

// knotsToFPS is derived from the exact SI definitions:
// 1 knot = 1852 m/hr, 1 ft = 0.3048 m, 1 hr = 3600 s
// → 1 knot = 1852 / (3600 × 0.3048) ft/s
const knotsToFPS = 1852.0 / (3600.0 * 0.3048)

func KnotsToFeetPerSecond(knots float64) float64 {
	return knots * knotsToFPS
}

func FeetPerSecondToKnots(fps float64) float64 {
	return fps / knotsToFPS
}
