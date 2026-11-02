package convert

import (
	"math"
	"testing"
)

func TestMachCrossChainConsistency(t *testing.T) {
	values := []float64{0, 100, 250, 450, mach1Knots, 900}
	for _, kts := range values {
		viaKPH := KnotsToKilometersPerHour(kts)
		viaMach := MachToKilometersPerHour(KnotsToMach(kts))
		if math.Abs(viaKPH-viaMach) > epsilon {
			t.Errorf("kts %v: direct kph=%v, via mach kph=%v (diff=%v)",
				kts, viaKPH, viaMach, math.Abs(viaKPH-viaMach))
		}
	}
}

func TestKnotsMetersPerSecond(t *testing.T) {
	tests := []struct {
		name  string
		knots float64
		ms    float64
	}{
		{name: "zero", knots: 0, ms: 0},
		{name: "1 knot = 1852/3600 m/s", knots: 1, ms: 1852.0 / 3600.0},
		{name: "100 knots", knots: 100, ms: 100 * 1852.0 / 3600.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KnotsToMetersPerSecond(tt.knots); math.Abs(got-tt.ms) > epsilon {
				t.Errorf("KnotsToMetersPerSecond(%v) = %v, want %v", tt.knots, got, tt.ms)
			}
			if got := MetersPerSecondToKnots(tt.ms); math.Abs(got-tt.knots) > epsilon {
				t.Errorf("MetersPerSecondToKnots(%v) = %v, want %v", tt.ms, got, tt.knots)
			}
		})
	}
}

func TestFeetPerMinuteToMetersPerSecond(t *testing.T) {
	tests := []struct {
		name string
		fpm  float64
		ms   float64
	}{
		{name: "zero", fpm: 0, ms: 0},
		{name: "1000 fpm", fpm: 1000, ms: 5.08},
		{name: "negative (descent)", fpm: -500, ms: -2.54},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FeetPerMinuteToMetersPerSecond(tt.fpm); math.Abs(got-tt.ms) > 1e-6 {
				t.Errorf("FPMtoMS(%v) = %v, want %v", tt.fpm, got, tt.ms)
			}
			if got := MetersPerSecondToFeetPerMinute(tt.ms); math.Abs(got-tt.fpm) > 1e-6 {
				t.Errorf("MStoFPM(%v) = %v, want %v", tt.ms, got, tt.fpm)
			}
		})
	}
}
