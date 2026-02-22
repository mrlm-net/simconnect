package convert

import (
	"math"
	"testing"
)

func TestOffsetToLatLon(t *testing.T) {
	tests := []struct {
		name      string
		latRef    float64
		lonRef    float64
		xEast     float64
		zNorth    float64
		wantLat   float64
		wantLon   float64
		tolerance float64
	}{
		{
			name:   "zero offset at origin",
			latRef: 0, lonRef: 0,
			xEast: 0, zNorth: 0,
			wantLat: 0, wantLon: 0,
			tolerance: epsilon,
		},
		{
			name:   "1000m north at equator",
			latRef: 0, lonRef: 0,
			xEast: 0, zNorth: 1000,
			wantLat:   0.009043,
			wantLon:   0,
			tolerance: epsilonDeg,
		},
		{
			name:   "1000m east at equator",
			latRef: 0, lonRef: 0,
			xEast: 1000, zNorth: 0,
			wantLat:   0,
			wantLon:   0.008983,
			tolerance: epsilonDeg,
		},
		{
			name:   "1000m north at 60N",
			latRef: 60, lonRef: 10,
			xEast: 0, zNorth: 1000,
			wantLat:   60.008992,
			wantLon:   10,
			tolerance: epsilonDeg,
		},
		{
			name:   "1000m east at 60N",
			latRef: 60, lonRef: 10,
			xEast: 1000, zNorth: 0,
			wantLat:   60,
			wantLon:   10.017966,
			tolerance: epsilonDeg,
		},
		{
			name:   "negative offsets at equator",
			latRef: 0, lonRef: 0,
			xEast: -1000, zNorth: -1000,
			wantLat:   -0.009043,
			wantLon:   -0.008983,
			tolerance: epsilonDeg,
		},
		{
			name:   "combined offset at mid-latitude",
			latRef: 45, lonRef: -90,
			xEast: 5000, zNorth: 5000,
			wantLat:   45.04499,
			wantLon:   -89.93659,
			tolerance: epsilonDeg,
		},
		{
			name:   "pole guard — east offset at north pole returns deltaLon=0",
			latRef: 90, lonRef: 0,
			xEast: 1000, zNorth: 0,
			wantLat: 90, wantLon: 0,
			tolerance: epsilon,
		},
		{
			name:   "pole guard — east offset at south pole returns deltaLon=0",
			latRef: -90, lonRef: 0,
			xEast: 1000, zNorth: 0,
			wantLat: -90, wantLon: 0,
			tolerance: epsilon,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLat, gotLon := OffsetToLatLon(tt.latRef, tt.lonRef, tt.xEast, tt.zNorth)
			if math.Abs(gotLat-tt.wantLat) > tt.tolerance {
				t.Errorf("lat: got %v, want %v (diff %v)", gotLat, tt.wantLat, math.Abs(gotLat-tt.wantLat))
			}
			if math.Abs(gotLon-tt.wantLon) > tt.tolerance {
				t.Errorf("lon: got %v, want %v (diff %v)", gotLon, tt.wantLon, math.Abs(gotLon-tt.wantLon))
			}
		})
	}
}
