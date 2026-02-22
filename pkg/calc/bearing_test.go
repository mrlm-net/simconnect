package calc

import (
	"math"
	"testing"
)

func TestBearingDegrees(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		wantBearing            float64
		tolerance              float64 // degrees
	}{
		{
			name: "due north",
			lat1: 0, lon1: 0, lat2: 1, lon2: 0,
			wantBearing: 0,
			tolerance:   0.01,
		},
		{
			name: "due east",
			lat1: 0, lon1: 0, lat2: 0, lon2: 1,
			wantBearing: 90,
			tolerance:   0.01,
		},
		{
			name: "due south",
			lat1: 1, lon1: 0, lat2: 0, lon2: 0,
			wantBearing: 180,
			tolerance:   0.01,
		},
		{
			name: "due west",
			lat1: 0, lon1: 1, lat2: 0, lon2: 0,
			wantBearing: 270,
			tolerance:   0.01,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BearingDegrees(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if got < 0 || got >= 360 {
				t.Errorf("BearingDegrees() = %v, not in [0,360)", got)
			}
			if math.Abs(got-tt.wantBearing) > tt.tolerance {
				t.Errorf("BearingDegrees() = %v, want ~%v", got, tt.wantBearing)
			}
		})
	}
}

func TestBearingDegreesRange(t *testing.T) {
	// Result must always be in [0, 360) regardless of direction.
	pairs := [][4]float64{
		{51.5, 0, 40.7, -74},       // London → NYC (SW)
		{0, 0, -1, 179},            // SW antipodal
		{-33.9, 151.2, 59.9, 30.3}, // Sydney → Helsinki (NW)
	}
	for _, p := range pairs {
		got := BearingDegrees(p[0], p[1], p[2], p[3])
		if got < 0 || got >= 360 {
			t.Errorf("BearingDegrees(%v,%v,%v,%v) = %v, not in [0,360)", p[0], p[1], p[2], p[3], got)
		}
	}
}
