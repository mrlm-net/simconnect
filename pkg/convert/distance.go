package convert

// statuteMileToMeters is the exact SI definition of the international statute mile.
const statuteMileToMeters = 1609.344

func NMToMeters(nm float64) float64 {
	return nm * 1852.0
}

func MetersToNM(meters float64) float64 {
	return meters / 1852.0
}

func NMToKilometers(nm float64) float64 {
	return nm * 1.852
}

func KilometersToNM(km float64) float64 {
	return km / 1.852
}

func KilometersToMeters(km float64) float64 {
	return km * 1000.0
}

func MetersToKilometers(meters float64) float64 {
	return meters / 1000.0
}

func NMToStatuteMiles(nm float64) float64 {
	return nm * 1852.0 / statuteMileToMeters
}

func StatuteMilesToNM(mi float64) float64 {
	return mi * statuteMileToMeters / 1852.0
}

func KilometersToStatuteMiles(km float64) float64 {
	return km * 1000.0 / statuteMileToMeters
}

func StatuteMilesToKilometers(mi float64) float64 {
	return mi * statuteMileToMeters / 1000.0
}

func StatuteMilesToMeters(mi float64) float64 {
	return mi * statuteMileToMeters
}

func MetersToStatuteMiles(m float64) float64 {
	return m / statuteMileToMeters
}
