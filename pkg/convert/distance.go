package convert

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
