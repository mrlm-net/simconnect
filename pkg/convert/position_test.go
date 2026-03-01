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

func TestLatLonToOffset(t *testing.T) {
	// epsilonM is 0.01m tolerance for meter offset comparisons.
	const epsilonM = 0.01

	tests := []struct {
		name       string
		latRef     float64
		lonRef     float64
		lat        float64
		lon        float64
		wantXEast  float64
		wantZNorth float64
		tolerance  float64
	}{
		{
			name:       "zero delta — same point",
			latRef:     0, lonRef: 0,
			lat:        0, lon: 0,
			wantXEast:  0, wantZNorth: 0,
			tolerance: epsilon,
		},
		{
			// Exact lat computed from OffsetToLatLon(0,0,0,1000): 0.009043694770504°
			name:       "1000m north at equator",
			latRef:     0, lonRef: 0,
			lat:        0.009043694770504, lon: 0,
			wantXEast:  0, wantZNorth: 1000,
			tolerance: epsilonM,
		},
		{
			// Exact lon computed from OffsetToLatLon(0,0,1000,0): 0.008983152841195°
			name:       "1000m east at equator",
			latRef:     0, lonRef: 0,
			lat:        0, lon: 0.008983152841195,
			wantXEast:  1000, wantZNorth: 0,
			tolerance: epsilonM,
		},
		{
			// Exact lat computed from OffsetToLatLon(60,10,0,1000): 60.008975670662707°
			name:       "1000m north at 60N",
			latRef:     60, lonRef: 10,
			lat:        60.008975670662707, lon: 10,
			wantXEast:  0, wantZNorth: 1000,
			tolerance: epsilonM,
		},
		{
			// Exact lon computed from OffsetToLatLon(60,10,1000,0): 10.017921146448389°
			name:       "1000m east at 60N",
			latRef:     60, lonRef: 10,
			lat:        60, lon: 10.017921146448389,
			wantXEast:  1000, wantZNorth: 0,
			tolerance: epsilonM,
		},
		{
			// Exact values from OffsetToLatLon(0,0,-1000,-1000)
			name:       "negative offsets — south and west",
			latRef:     0, lonRef: 0,
			lat:        -0.009043694770504, lon: -0.008983152841195,
			wantXEast:  -1000, wantZNorth: -1000,
			tolerance: epsilonM,
		},
		{
			name:       "pole guard — xEast must be zero at north pole",
			latRef:     90, lonRef: 0,
			lat:        90, lon: 1,
			wantXEast:  0, wantZNorth: 0,
			tolerance: epsilon,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotXEast, gotZNorth := LatLonToOffset(tt.latRef, tt.lonRef, tt.lat, tt.lon)
			if math.Abs(gotXEast-tt.wantXEast) > tt.tolerance {
				t.Errorf("xEast: got %v, want %v (diff %v)", gotXEast, tt.wantXEast, math.Abs(gotXEast-tt.wantXEast))
			}
			if math.Abs(gotZNorth-tt.wantZNorth) > tt.tolerance {
				t.Errorf("zNorth: got %v, want %v (diff %v)", gotZNorth, tt.wantZNorth, math.Abs(gotZNorth-tt.wantZNorth))
			}
		})
	}
}

func TestLatLonToOffset_RoundTrip(t *testing.T) {
	// Round-trip: OffsetToLatLon → LatLonToOffset must recover the original
	// offsets within 0.01m.
	const roundTripTolerance = 0.01

	cases := []struct {
		name   string
		latRef float64
		lonRef float64
		xEast  float64
		zNorth float64
	}{
		{"equator zero offsets", 0, 0, 0, 0},
		{"equator north", 0, 0, 0, 1000},
		{"equator east", 0, 0, 1000, 0},
		{"equator combined", 0, 0, 500, 800},
		{"60N large north", 60, 10, 0, 5000},
		{"60N large east", 60, 10, 5000, 0},
		{"60N combined", 60, 10, 2000, 3000},
		{"mid-lat negative", 45, -90, -1500, -2500},
		{"southern hemisphere", -30, 20, 700, 1300},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lat, lon := OffsetToLatLon(tc.latRef, tc.lonRef, tc.xEast, tc.zNorth)
			gotXEast, gotZNorth := LatLonToOffset(tc.latRef, tc.lonRef, lat, lon)
			if math.Abs(gotXEast-tc.xEast) > roundTripTolerance {
				t.Errorf("xEast round-trip: got %v, want %v (diff %v)", gotXEast, tc.xEast, math.Abs(gotXEast-tc.xEast))
			}
			if math.Abs(gotZNorth-tc.zNorth) > roundTripTolerance {
				t.Errorf("zNorth round-trip: got %v, want %v (diff %v)", gotZNorth, tc.zNorth, math.Abs(gotZNorth-tc.zNorth))
			}
		})
	}
}
