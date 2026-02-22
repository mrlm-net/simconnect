package convert

import (
	"math"
	"testing"
)

func TestInHgToMillibar(t *testing.T) {
	tests := []struct {
		name      string
		inHg      float64
		want      float64
		tolerance float64
	}{
		// Standard atmosphere: 29.92 inHg × 33.8639 = 1013.208 mbar
		// (exact ISA is 29.9213 inHg; 29.92 is the commonly cited rounded value)
		{name: "standard atmosphere", inHg: 29.92, want: 1013.21, tolerance: 0.01},
		{name: "zero", inHg: 0, want: 0, tolerance: epsilon},
		{name: "one inHg", inHg: 1, want: 33.8639, tolerance: epsilon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InHgToMillibar(tt.inHg)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("InHgToMillibar(%v) = %v, want ~%v", tt.inHg, got, tt.want)
			}
		})
	}
}

func TestMillibarToInHg(t *testing.T) {
	tests := []struct {
		name      string
		mbar      float64
		want      float64
		tolerance float64
	}{
		// Standard atmosphere: 1013.25 mbar ≈ 29.92 inHg
		{name: "standard atmosphere", mbar: 1013.25, want: 29.92, tolerance: 0.01},
		{name: "zero", mbar: 0, want: 0, tolerance: epsilon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MillibarToInHg(tt.mbar)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("MillibarToInHg(%v) = %v, want ~%v", tt.mbar, got, tt.want)
			}
		})
	}
}

func TestInHgToHectopascalEqualsMillibar(t *testing.T) {
	values := []float64{0, 1, 29.92, 30.0}
	for _, v := range values {
		mbar := InHgToMillibar(v)
		hPa := InHgToHectopascal(v)
		if mbar != hPa {
			t.Errorf("InHgToHectopascal(%v) = %v != InHgToMillibar(%v) = %v", v, hPa, v, mbar)
		}
	}
}

func TestHectopascalToInHgEqualsMillibarToInHg(t *testing.T) {
	values := []float64{0, 1, 1013.25, 1030.0}
	for _, v := range values {
		fromMbar := MillibarToInHg(v)
		fromHPa := HectopascalToInHg(v)
		if fromMbar != fromHPa {
			t.Errorf("HectopascalToInHg(%v) = %v != MillibarToInHg(%v) = %v", v, fromHPa, v, fromMbar)
		}
	}
}

func TestInHgToPascal(t *testing.T) {
	tests := []struct {
		name      string
		inHg      float64
		want      float64
		tolerance float64
	}{
		// Standard atmosphere: 29.92 × 33.8639 × 100 = 101320.8 Pa
		// (exact ISA is 29.9213 inHg; 29.92 is the commonly cited rounded value)
		{name: "standard atmosphere", inHg: 29.92, want: 101320.8, tolerance: 1.0},
		{name: "zero", inHg: 0, want: 0, tolerance: epsilon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InHgToPascal(tt.inHg)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("InHgToPascal(%v) = %v, want ~%v", tt.inHg, got, tt.want)
			}
		})
	}
}

func TestPressureRoundTrips(t *testing.T) {
	values := []float64{0, 29.92, 30.0, 31.0}
	for _, v := range values {
		// inHg → mbar → inHg
		if got := MillibarToInHg(InHgToMillibar(v)); math.Abs(got-v) > epsilon {
			t.Errorf("inHg→mbar→inHg round-trip for %v: got %v", v, got)
		}
		// inHg → Pa → inHg
		if got := PascalToInHg(InHgToPascal(v)); math.Abs(got-v) > epsilon {
			t.Errorf("inHg→Pa→inHg round-trip for %v: got %v", v, got)
		}
	}
}
