package traffic

import "math"

// SaidSpeed is a speed as a controller gives it (Doc 4444 4.6.1.6): at or
// above FL250 a Mach number in multiples of 0.01, below it an indicated
// airspeed in multiples of 10 kt. One of the two is set.
type SaidSpeed struct {
	Mach   float64 `json:"mach,omitempty"`
	IASKts float64 `json:"iasKts,omitempty"`
}

// MachFromFt: from this altitude speeds are given as Mach (FL250).
const MachFromFt = 25000.0

// SpeedSaidAt is a true airspeed of tasKts at altFt as a controller gives
// it, rounded to its step, and the true airspeed that is (ISA, no wind:
// the traffic's ground speed is taken as its true airspeed). Live: OKEAE at
// FL360 was told "reduce speed to 396 knots", a ground speed.
func SpeedSaidAt(altFt, tasKts float64) (SaidSpeed, float64) {
	a := soundKts(altFt)
	if altFt >= MachFromFt {
		m := math.Round(tasKts/a*100) / 100
		return SaidSpeed{Mach: m}, m * a
	}
	ias := math.Round(tasKts*math.Sqrt(densityRatio(altFt))/10) * 10
	return SaidSpeed{IASKts: ias}, ias / math.Sqrt(densityRatio(altFt))
}

// IASAt is the indicated airspeed of tasKts at altFt (ISA, no wind).
func IASAt(altFt, tasKts float64) float64 { return tasKts * math.Sqrt(densityRatio(altFt)) }

// isaKelvin is the ISA temperature at altFt: 288.15 K at sea level,
// 1.98 K less per 1000 ft up to the tropopause (36 089 ft), 216.65 K above.
func isaKelvin(altFt float64) float64 {
	return math.Max(216.65, 288.15-0.0019812*math.Max(0, altFt))
}

// soundKts is the speed of sound at altFt (ISA): 661.47 kt at sea level.
func soundKts(altFt float64) float64 { return 661.47 * math.Sqrt(isaKelvin(altFt)/288.15) }

// densityRatio is the ISA air density at altFt over that at sea level.
func densityRatio(altFt float64) float64 {
	const tropopauseFt = 36089.0
	if altFt <= tropopauseFt {
		return math.Pow(isaKelvin(altFt)/288.15, 4.2559)
	}
	return math.Pow(216.65/288.15, 4.2559) * math.Exp(-(altFt-tropopauseFt)/20806)
}
