//go:build windows
// +build windows

package convert

func KnotsToKilometersPerHour(knots float64) float64 {
	return knots * 1.852
}

func KilometersPerHourToKnots(kph float64) float64 {
	return kph / 1.852
}

func KnotsToMach(knots float64) float64 {
	return knots / 661.4788
}

func MachToKnots(mach float64) float64 {
	return mach * 661.4788
}

func KilometersPerHourToMach(kph float64) float64 {
	return kph / 1225.044
}

func MachToKilometersPerHour(mach float64) float64 {
	return mach * 1225.044
}
