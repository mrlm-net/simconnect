//go:build windows
// +build windows

package convert

func FeetToMeters(feet float64) float64 {
	return feet * 0.3048
}

func MetersToFeet(meters float64) float64 {
	return meters / 0.3048
}
