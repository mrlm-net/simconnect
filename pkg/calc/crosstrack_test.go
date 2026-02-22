package calc

import (
	"math"
	"testing"
)

func TestCrossTrackMeters(t *testing.T) {
	const epsilon = 1e-9
	// 1 NM in metres (used for right/left offset cases)
	const oneNM = 1852.0

	// Helper: offset a point 1 NM perpendicular to a due-north track.
	// Track A→B runs due north along lon=0 from lat=0 to lat=1.
	// A point directly on that track has XTD = 0.
	// A point displaced 1 NM east has XTD ≈ +1852 m (to the right).
	// A point displaced 1 NM west has XTD ≈ -1852 m (to the left).
	//
	// 1 NM ≈ 1/60 degree of latitude, so a perpendicular (east-west)
	// offset at the equator is also ≈ 1/60 degree of longitude.
	const nmInDeg = 1.0 / 60.0

	tests := []struct {
		name                   string
		latA, lonA, latB, lonB float64
		latD, lonD             float64
		wantSign               float64 // +1, -1, or 0
		wantApprox             float64 // expected magnitude (metres)
		tolerance              float64 // acceptable absolute error (metres)
	}{
		{
			name: "point exactly on track",
			latA: 0, lonA: 0, latB: 1, lonB: 0,
			latD: 0.5, lonD: 0,
			wantSign:   0,
			wantApprox: 0,
			tolerance:  epsilon,
		},
		{
			name: "point 1 NM to the right of track",
			latA: 0, lonA: 0, latB: 1, lonB: 0,
			latD: 0.5, lonD: nmInDeg,
			wantSign:   1,
			wantApprox: oneNM,
			tolerance:  5.0, // metres — small error due to great-circle geometry at equator
		},
		{
			name: "point 1 NM to the left of track",
			latA: 0, lonA: 0, latB: 1, lonB: 0,
			latD: 0.5, lonD: -nmInDeg,
			wantSign:   -1,
			wantApprox: -oneNM,
			tolerance:  5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CrossTrackMeters(tt.latA, tt.lonA, tt.latB, tt.lonB, tt.latD, tt.lonD)

			if tt.wantSign == 0 {
				if math.Abs(got) > tt.tolerance {
					t.Errorf("CrossTrackMeters() = %v, want ~0", got)
				}
				return
			}

			if math.Abs(got-tt.wantApprox) > tt.tolerance {
				t.Errorf("CrossTrackMeters() = %v, want ~%v (tolerance %v)", got, tt.wantApprox, tt.tolerance)
			}
		})
	}
}
