//go:build windows
// +build windows

package nav

import "math"

// Descent speeds below the speed limit level: 250 kt below FL100
// (SERA.6001 airspace classes C–G; the US 14 CFR 91.117), and the rule of
// thumb pilots plan the slow-down by: about a mile per 10 kt lost.
const (
	SpeedLimitFt       = 10000.0
	SpeedLimitKts      = 250.0
	DecelNMPer10Kts    = 1.0
	minDescentGSKts    = 120.0
	defaultDescentRate = 1800.0
)

// TopOfDescent is how far before the point where an aircraft should be at
// toFt it starts down from fromFt: at its descent rate (p.DescentFPM) and
// ground speed gsKts above FL100, at the 250 kt limit below it (its ground
// speed if lower), plus the miles to slow to 250 kt (DecelNMPer10Kts) when
// it crosses FL100 faster, plus extraNM (the caller's margin: the slowing to
// approach speed, a level segment before the STAR's first constraint).
// 0 when it is at or below toFt already.
func TopOfDescent(p Performance, fromFt, toFt, gsKts, extraNM float64) float64 {
	if fromFt <= toFt {
		return 0
	}
	rate := p.DescentFPM
	if rate <= 0 {
		rate = defaultDescentRate
	}
	gs := math.Max(gsKts, minDescentGSKts)
	nm := func(feet, kts float64) float64 { return feet / rate / 60 * kts }
	d := extraNM
	high := math.Max(0, fromFt-math.Max(toFt, SpeedLimitFt)) // above FL100
	low := math.Max(0, math.Min(fromFt, SpeedLimitFt)-toFt)   // at or below it
	d += nm(high, gs)
	if low > 0 {
		slow := math.Min(gs, SpeedLimitKts)
		d += nm(low, slow)
		if high > 0 && gs > SpeedLimitKts {
			d += (gs - SpeedLimitKts) / 10 * DecelNMPer10Kts
		}
	}
	return d
}
