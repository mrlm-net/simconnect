package convert

// inHgToMillibar is the standard conversion factor for pressure.
// 1 inHg = 33.8639 mbar (hPa).
const inHgToMillibar = 33.8639

func InHgToMillibar(inHg float64) float64 {
	return inHg * inHgToMillibar
}

func MillibarToInHg(mbar float64) float64 {
	return mbar / inHgToMillibar
}

// InHgToHectopascal is an alias for InHgToMillibar since 1 mbar = 1 hPa.
func InHgToHectopascal(inHg float64) float64 {
	return InHgToMillibar(inHg)
}

// HectopascalToInHg is an alias for MillibarToInHg since 1 hPa = 1 mbar.
func HectopascalToInHg(hPa float64) float64 {
	return MillibarToInHg(hPa)
}

func InHgToPascal(inHg float64) float64 {
	return inHg * inHgToMillibar * 100.0
}

func PascalToInHg(pa float64) float64 {
	return pa / (inHgToMillibar * 100.0)
}
