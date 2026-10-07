package calc

import "math"

// IntermediatePoint is the point a fraction f (0–1) of the way along the
// great circle from (lat1, lon1) to (lat2, lon2), in degrees: where a
// flight between them actually is, unlike a straight line in latitude and
// longitude (#468).
func IntermediatePoint(lat1, lon1, lat2, lon2, f float64) (lat, lon float64) {
	φ1, λ1 := lat1*math.Pi/180, lon1*math.Pi/180
	φ2, λ2 := lat2*math.Pi/180, lon2*math.Pi/180
	// The angular distance between the two points.
	δ := 2 * math.Asin(math.Sqrt(clamp01(math.Pow(math.Sin((φ2-φ1)/2), 2)+math.Cos(φ1)*math.Cos(φ2)*math.Pow(math.Sin((λ2-λ1)/2), 2))))
	if δ == 0 {
		return lat1, lon1
	}
	a := math.Sin((1-f)*δ) / math.Sin(δ)
	b := math.Sin(f*δ) / math.Sin(δ)
	x := a*math.Cos(φ1)*math.Cos(λ1) + b*math.Cos(φ2)*math.Cos(λ2)
	y := a*math.Cos(φ1)*math.Sin(λ1) + b*math.Cos(φ2)*math.Sin(λ2)
	z := a*math.Sin(φ1) + b*math.Sin(φ2)
	return math.Atan2(z, math.Hypot(x, y)) * 180 / math.Pi, math.Atan2(y, x) * 180 / math.Pi
}
