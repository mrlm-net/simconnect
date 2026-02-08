//go:build windows
// +build windows

package calc

import (
	"math"
	"testing"
)

func TestHaversineMeters(t *testing.T) {
	tests := []struct {
		name      string
		lat1      float64
		lon1      float64
		lat2      float64
		lon2      float64
		wantDist  float64
		tolerance float64 // fraction, e.g. 0.01 = 1%
	}{
		{
			name: "same point",
			lat1: 0, lon1: 0,
			lat2: 0, lon2: 0,
			wantDist:  0.0,
			tolerance: 0,
		},
		{
			name: "NYC to London",
			lat1: 40.7128, lon1: -74.0060,
			lat2: 51.5074, lon2: -0.1278,
			wantDist:  5570000,
			tolerance: 0.01,
		},
		{
			name: "1 degree longitude at equator",
			lat1: 0, lon1: 0,
			lat2: 0, lon2: 1,
			wantDist:  111195,
			tolerance: 0.01,
		},
		{
			name: "antipodal points",
			lat1: 0, lon1: 0,
			lat2: 0, lon2: 180,
			wantDist:  20015000,
			tolerance: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HaversineMeters(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if tt.wantDist == 0 {
				if got != 0 {
					t.Errorf("HaversineMeters() = %v, want 0", got)
				}
				return
			}
			diff := math.Abs(got-tt.wantDist) / tt.wantDist
			if diff > tt.tolerance {
				t.Errorf("HaversineMeters() = %v, want ~%v (got %.2f%% error, max %.2f%%)",
					got, tt.wantDist, diff*100, tt.tolerance*100)
			}
		})
	}
}
