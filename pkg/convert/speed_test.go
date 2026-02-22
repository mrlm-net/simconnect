package convert

import (
	"math"
	"testing"
)

func TestKnotsToKilometersPerHour(t *testing.T) {
	tests := []struct {
		name  string
		knots float64
		want  float64
	}{
		{name: "zero", knots: 0, want: 0},
		{name: "250 kts cruise", knots: 250, want: 463.0},
		{name: "one knot", knots: 1, want: 1.852},
		{name: "high speed", knots: 500, want: 926.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KnotsToKilometersPerHour(tt.knots)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KnotsToKilometersPerHour(%v) = %v, want %v", tt.knots, got, tt.want)
			}
		})
	}
}

func TestKilometersPerHourToKnots(t *testing.T) {
	tests := []struct {
		name string
		kph  float64
		want float64
	}{
		{name: "zero", kph: 0, want: 0},
		{name: "463 kph", kph: 463.0, want: 250.0},
		{name: "1.852 kph", kph: 1.852, want: 1.0},
		{name: "high speed", kph: 926.0, want: 500.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KilometersPerHourToKnots(tt.kph)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KilometersPerHourToKnots(%v) = %v, want %v", tt.kph, got, tt.want)
			}
		})
	}
}

func TestKnotsKPHRoundtrip(t *testing.T) {
	values := []float64{0, 1, 100, 250, 500}
	for _, v := range values {
		got := KilometersPerHourToKnots(KnotsToKilometersPerHour(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip KnotsToKPH->KPHToKnots(%v) = %v", v, got)
		}
	}
}

func TestKnotsToMach(t *testing.T) {
	tests := []struct {
		name  string
		knots float64
		want  float64
	}{
		{name: "zero", knots: 0, want: 0},
		{name: "Mach 1", knots: mach1Knots, want: 1.0},
		{name: "Mach 0.82 cruise", knots: mach1Knots * 0.82, want: 0.82},
		{name: "subsonic 250 kts", knots: 250, want: 250.0 / mach1Knots},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KnotsToMach(tt.knots)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KnotsToMach(%v) = %v, want %v", tt.knots, got, tt.want)
			}
		})
	}
}

func TestMachToKnots(t *testing.T) {
	tests := []struct {
		name string
		mach float64
		want float64
	}{
		{name: "zero", mach: 0, want: 0},
		{name: "Mach 1", mach: 1.0, want: mach1Knots},
		{name: "Mach 0.82", mach: 0.82, want: mach1Knots * 0.82},
		{name: "Mach 2", mach: 2.0, want: mach1Knots * 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MachToKnots(tt.mach)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("MachToKnots(%v) = %v, want %v", tt.mach, got, tt.want)
			}
		})
	}
}

func TestKnotsMachRoundtrip(t *testing.T) {
	values := []float64{0, 100, 250, mach1Knots, 1000}
	for _, v := range values {
		got := MachToKnots(KnotsToMach(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip KnotsToMach->MachToKnots(%v) = %v", v, got)
		}
	}
}

func TestKilometersPerHourToMach(t *testing.T) {
	tests := []struct {
		name string
		kph  float64
		want float64
	}{
		{name: "zero", kph: 0, want: 0},
		{name: "Mach 1", kph: mach1KPH, want: 1.0},
		{name: "Mach 0.82 cruise", kph: mach1KPH * 0.82, want: 0.82},
		{name: "low speed", kph: 300, want: 300.0 / mach1KPH},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KilometersPerHourToMach(tt.kph)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KilometersPerHourToMach(%v) = %v, want %v", tt.kph, got, tt.want)
			}
		})
	}
}

func TestMachToKilometersPerHour(t *testing.T) {
	tests := []struct {
		name string
		mach float64
		want float64
	}{
		{name: "zero", mach: 0, want: 0},
		{name: "Mach 1", mach: 1.0, want: mach1KPH},
		{name: "Mach 0.82", mach: 0.82, want: mach1KPH * 0.82},
		{name: "Mach 2", mach: 2.0, want: mach1KPH * 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MachToKilometersPerHour(tt.mach)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("MachToKilometersPerHour(%v) = %v, want %v", tt.mach, got, tt.want)
			}
		})
	}
}

func TestKPHMachRoundtrip(t *testing.T) {
	values := []float64{0, 300, 900, mach1KPH, 2500}
	for _, v := range values {
		got := MachToKilometersPerHour(KilometersPerHourToMach(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip KPHToMach->MachToKPH(%v) = %v", v, got)
		}
	}
}

// TestMachCrossChainConsistency verifies that knots→mach→kph equals knots→kph directly.
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
