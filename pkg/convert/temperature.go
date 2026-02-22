package convert

// celsiusKelvinOffset is the exact SI offset between Celsius and Kelvin.
const celsiusKelvinOffset = 273.15

func CelsiusToFahrenheit(c float64) float64 {
	return c*9.0/5.0 + 32.0
}

func FahrenheitToCelsius(f float64) float64 {
	return (f - 32.0) * 5.0 / 9.0
}

func CelsiusToKelvin(c float64) float64 {
	return c + celsiusKelvinOffset
}

func KelvinToCelsius(k float64) float64 {
	return k - celsiusKelvinOffset
}

func FahrenheitToKelvin(f float64) float64 {
	return CelsiusToKelvin(FahrenheitToCelsius(f))
}

func KelvinToFahrenheit(k float64) float64 {
	return CelsiusToFahrenheit(KelvinToCelsius(k))
}
