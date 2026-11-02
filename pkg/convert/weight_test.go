package convert

import (
	"math"
	"testing"
)

func TestKilogramsToPounds(t *testing.T) {
	tests := []struct {
		name      string
		kg        float64
		want      float64
		tolerance float64
	}{
		// 1 kg ≈ 2.20462 lbs
		{name: "one kilogram", kg: 1, want: 1.0 / 0.45359237, tolerance: 1e-6},
		{name: "zero", kg: 0, want: 0, tolerance: epsilon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KilogramsToPounds(tt.kg)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("KilogramsToPounds(%v) = %v, want ~%v", tt.kg, got, tt.want)
			}
		})
	}
}

func TestLitersToUSGallons(t *testing.T) {
	tests := []struct {
		name      string
		liters    float64
		want      float64
		tolerance float64
	}{
		// 1 liter ≈ 0.264172 US gallons
		{name: "one liter", liters: 1, want: 1.0 / 3.785411784, tolerance: 1e-6},
		{name: "zero", liters: 0, want: 0, tolerance: epsilon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LitersToUSGallons(tt.liters)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("LitersToUSGallons(%v) = %v, want ~%v", tt.liters, got, tt.want)
			}
		})
	}
}

