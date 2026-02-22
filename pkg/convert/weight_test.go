package convert

import (
	"math"
	"testing"
)

func TestPoundsToKilograms(t *testing.T) {
	tests := []struct {
		name      string
		lbs       float64
		want      float64
		tolerance float64
	}{
		{name: "one pound", lbs: 1, want: 0.45359237, tolerance: epsilon},
		{name: "zero", lbs: 0, want: 0, tolerance: epsilon},
		{name: "100 lbs", lbs: 100, want: 45.359237, tolerance: epsilon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PoundsToKilograms(tt.lbs)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("PoundsToKilograms(%v) = %v, want %v", tt.lbs, got, tt.want)
			}
		})
	}
}

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

func TestUSGallonsToLiters(t *testing.T) {
	tests := []struct {
		name      string
		gal       float64
		want      float64
		tolerance float64
	}{
		{name: "one US gallon", gal: 1, want: 3.785411784, tolerance: epsilon},
		{name: "zero", gal: 0, want: 0, tolerance: epsilon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := USGallonsToLiters(tt.gal)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("USGallonsToLiters(%v) = %v, want %v", tt.gal, got, tt.want)
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

func TestWeightRoundTrips(t *testing.T) {
	lbsValues := []float64{0, 1, 100, 2500}
	for _, v := range lbsValues {
		if got := KilogramsToPounds(PoundsToKilograms(v)); math.Abs(got-v) > epsilon {
			t.Errorf("lbs→kg→lbs round-trip for %v: got %v", v, got)
		}
	}

	galValues := []float64{0, 1, 10, 100}
	for _, v := range galValues {
		if got := LitersToUSGallons(USGallonsToLiters(v)); math.Abs(got-v) > epsilon {
			t.Errorf("gal→L→gal round-trip for %v: got %v", v, got)
		}
	}
}
