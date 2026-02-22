package calc

import (
	"math"
	"testing"
)

func TestTrueToMagnetic(t *testing.T) {
	const epsilon = 1e-9

	tests := []struct {
		name         string
		trueHeading  float64
		magVar       float64
		wantMagnetic float64
	}{
		{
			name:        "easterly variation — correct left",
			trueHeading: 360, magVar: 5,
			wantMagnetic: 355,
		},
		{
			name:        "westerly variation — correct right",
			trueHeading: 10, magVar: -5,
			wantMagnetic: 15,
		},
		{
			name:        "no variation",
			trueHeading: 90, magVar: 0,
			wantMagnetic: 90,
		},
		{
			name:        "wrap-around below zero",
			trueHeading: 5, magVar: 10,
			wantMagnetic: 355,
		},
		{
			name:        "wrap-around at 360",
			trueHeading: 355, magVar: -10,
			wantMagnetic: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrueToMagnetic(tt.trueHeading, tt.magVar)
			if got < 0 || got >= 360 {
				t.Errorf("TrueToMagnetic() = %v, not in [0,360)", got)
			}
			if math.Abs(got-tt.wantMagnetic) > epsilon {
				t.Errorf("TrueToMagnetic(%v, %v) = %v, want %v", tt.trueHeading, tt.magVar, got, tt.wantMagnetic)
			}
		})
	}
}

func TestMagneticToTrue(t *testing.T) {
	const epsilon = 1e-9

	tests := []struct {
		name            string
		magneticHeading float64
		magVar          float64
		wantTrue        float64
	}{
		{
			name:            "easterly variation — recover true (normalised to 0)",
			magneticHeading: 355, magVar: 5,
			wantTrue: 0,
		},
		{
			name:            "westerly variation — recover true",
			magneticHeading: 15, magVar: -5,
			wantTrue: 10,
		},
		{
			name:            "no variation",
			magneticHeading: 180, magVar: 0,
			wantTrue: 180,
		},
		{
			name:            "wrap-around upward",
			magneticHeading: 355, magVar: 10,
			wantTrue: 5,
		},
		{
			name:            "wrap-around downward",
			magneticHeading: 5, magVar: -10,
			wantTrue: 355,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MagneticToTrue(tt.magneticHeading, tt.magVar)
			if got < 0 || got >= 360 {
				t.Errorf("MagneticToTrue() = %v, not in [0,360)", got)
			}
			if math.Abs(got-tt.wantTrue) > epsilon {
				t.Errorf("MagneticToTrue(%v, %v) = %v, want %v", tt.magneticHeading, tt.magVar, got, tt.wantTrue)
			}
		})
	}
}

// TestTrueMagneticRoundTrip verifies that TrueToMagnetic and MagneticToTrue are inverses.
func TestTrueMagneticRoundTrip(t *testing.T) {
	const epsilon = 1e-9

	cases := [][2]float64{
		{0, 5},
		{90, -10},
		{180, 20},
		{270, -3},
		{359, 7},
	}

	for _, c := range cases {
		trueH, magVar := c[0], c[1]

		mag := TrueToMagnetic(trueH, magVar)
		recovered := MagneticToTrue(mag, magVar)

		// Both must be in [0, 360) and round-trip must match original.
		if recovered < 0 || recovered >= 360 {
			t.Errorf("round-trip result %v not in [0,360)", recovered)
		}
		if math.Abs(recovered-trueH) > epsilon {
			t.Errorf("round-trip true=%.1f magVar=%.1f: got %.10f, want %.10f", trueH, magVar, recovered, trueH)
		}
	}
}
