package convert

// poundsToKilograms is the exact SI conversion factor (international avoirdupois pound).
const poundsToKilograms = 0.45359237

// usGallonToLiters is the exact conversion factor for US liquid gallons.
const usGallonToLiters = 3.785411784

func PoundsToKilograms(lbs float64) float64 {
	return lbs * poundsToKilograms
}

func KilogramsToPounds(kg float64) float64 {
	return kg / poundsToKilograms
}

func USGallonsToLiters(gal float64) float64 {
	return gal * usGallonToLiters
}

func LitersToUSGallons(l float64) float64 {
	return l / usGallonToLiters
}
