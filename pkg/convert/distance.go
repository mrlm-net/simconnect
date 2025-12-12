//go:build windows
// +build windows

package convert

func NMToMeters(nm float64) float64 {
	return nm * 1852.0
}

func MetersToNM(meters float64) float64 {
	return meters / 1852.0
}
