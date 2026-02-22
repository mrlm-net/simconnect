package convert

func FeetToMeters(feet float64) float64 {
	return feet * 0.3048
}

func MetersToFeet(meters float64) float64 {
	return meters / 0.3048
}

func FeetPerMinuteToFeetPerSecond(fpm float64) float64 {
	return fpm / 60.0
}

func FeetPerSecondToFeetPerMinute(fps float64) float64 {
	return fps * 60.0
}
