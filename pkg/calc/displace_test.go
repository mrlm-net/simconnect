package calc

import (
	"math"
	"testing"
)

func TestDisplaceByHeading(t *testing.T) {
	const tol = 1e-4

	tests := []struct {
		name           string
		lat, lon       float64
		hdgDeg         float64
		distanceMeters float64
		wantLat        float64
		wantLon        float64
		// when checkDirection is true, only direction of change is asserted
		checkDirection bool
		wantLatInc     bool
		wantLonInc     bool
	}{
		{
			name:           "due north 1852m from equator",
			lat:            0, lon: 0, hdgDeg: 0, distanceMeters: 1852,
			wantLat: 0.016667, wantLon: 0,
		},
		{
			name:           "due east 1852m at equator",
			lat:            0, lon: 0, hdgDeg: 90, distanceMeters: 1852,
			wantLat: 0, wantLon: 0.016667,
		},
		{
			name:           "due south 1852m",
			lat:            1, lon: 0, hdgDeg: 180, distanceMeters: 1852,
			wantLat: 0.983333, wantLon: 0,
		},
		{
			name:           "due west 1852m at equator",
			lat:            0, lon: 1, hdgDeg: 270, distanceMeters: 1852,
			wantLat: 0, wantLon: 0.983333,
		},
		{
			name:           "zero distance",
			lat:            51.5, lon: -0.1, hdgDeg: 135, distanceMeters: 0,
			wantLat: 51.5, wantLon: -0.1,
		},
		{
			name:           "45 degree heading 1000m",
			lat:            0, lon: 0, hdgDeg: 45, distanceMeters: 1000,
			checkDirection: true,
			wantLatInc:     true,
			wantLonInc:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLat, gotLon := DisplaceByHeading(tt.lat, tt.lon, tt.hdgDeg, tt.distanceMeters)

			if tt.checkDirection {
				if tt.wantLatInc && gotLat <= tt.lat {
					t.Errorf("DisplaceByHeading() lat %v, expected increase from %v", gotLat, tt.lat)
				}
				if !tt.wantLatInc && gotLat >= tt.lat {
					t.Errorf("DisplaceByHeading() lat %v, expected decrease from %v", gotLat, tt.lat)
				}
				if tt.wantLonInc && gotLon <= tt.lon {
					t.Errorf("DisplaceByHeading() lon %v, expected increase from %v", gotLon, tt.lon)
				}
				if !tt.wantLonInc && gotLon >= tt.lon {
					t.Errorf("DisplaceByHeading() lon %v, expected decrease from %v", gotLon, tt.lon)
				}
				return
			}

			if math.Abs(gotLat-tt.wantLat) > tol {
				t.Errorf("DisplaceByHeading() lat = %v, want ~%v (tol %v)", gotLat, tt.wantLat, tol)
			}
			if math.Abs(gotLon-tt.wantLon) > tol {
				t.Errorf("DisplaceByHeading() lon = %v, want ~%v (tol %v)", gotLon, tt.wantLon, tol)
			}
		})
	}
}

// TestDisplaceByHeading_360EqualsZero verifies that hdgDeg=360 produces the
// same result as hdgDeg=0 within floating-point precision.
func TestDisplaceByHeading_360EqualsZero(t *testing.T) {
	const tol = 1e-9
	lat0, lon0 := DisplaceByHeading(0, 0, 0, 1852)
	lat360, lon360 := DisplaceByHeading(0, 0, 360, 1852)
	if math.Abs(lat0-lat360) > tol || math.Abs(lon0-lon360) > tol {
		t.Errorf("hdg=360 (%v, %v) != hdg=0 (%v, %v)", lat360, lon360, lat0, lon0)
	}
}

// TestDisplaceByHeading_NegativeDistance verifies that a negative distance on
// hdg=0 is equivalent to a positive distance on hdg=180.
func TestDisplaceByHeading_NegativeDistance(t *testing.T) {
	const tol = 1e-4
	latFwd, lonFwd := DisplaceByHeading(1, 0, 180, 1852)
	latRev, lonRev := DisplaceByHeading(1, 0, 0, -1852)
	if math.Abs(latFwd-latRev) > tol {
		t.Errorf("negative distance lat %v != reverse heading lat %v", latRev, latFwd)
	}
	if math.Abs(lonFwd-lonRev) > tol {
		t.Errorf("negative distance lon %v != reverse heading lon %v", lonRev, lonFwd)
	}
}

// TestDisplaceByHeading_RoundTrip verifies that displacing by a heading and
// then displacing by the reverse heading recovers the original point within
// a tolerance appropriate to the distance and heading.
//
// Cardinal-heading round trips (north/east) stay within 1e-4 degrees.
// Diagonal round trips (e.g. 135°/225°) at 10–20 km accumulate a slightly
// larger longitude error (~2e-4) due to the WGS84 first-order linear
// approximation in OffsetToLatLon; the tolerance for those cases is 5e-4.
func TestDisplaceByHeading_RoundTrip(t *testing.T) {
	cases := []struct {
		name           string
		lat, lon       float64
		hdgDeg         float64
		distanceMeters float64
		tol            float64
	}{
		{name: "north from equator", lat: 0, lon: 0, hdgDeg: 0, distanceMeters: 1852, tol: 1e-4},
		{name: "east from equator", lat: 0, lon: 0, hdgDeg: 90, distanceMeters: 5000, tol: 1e-4},
		// Diagonal trips at mid-latitudes accumulate a small ellipsoid approximation
		// error in the longitude component; 5e-4 deg ≈ 35 m at these latitudes.
		{name: "SE from London", lat: 51.5, lon: -0.1, hdgDeg: 135, distanceMeters: 10000, tol: 5e-4},
		{name: "SW from Sydney", lat: -33.9, lon: 151.2, hdgDeg: 225, distanceMeters: 20000, tol: 5e-4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			midLat, midLon := DisplaceByHeading(tc.lat, tc.lon, tc.hdgDeg, tc.distanceMeters)
			gotLat, gotLon := DisplaceByHeading(midLat, midLon, tc.hdgDeg+180, tc.distanceMeters)
			if math.Abs(gotLat-tc.lat) > tc.tol {
				t.Errorf("round-trip lat = %v, want ~%v (tol %v)", gotLat, tc.lat, tc.tol)
			}
			if math.Abs(gotLon-tc.lon) > tc.tol {
				t.Errorf("round-trip lon = %v, want ~%v (tol %v)", gotLon, tc.lon, tc.tol)
			}
		})
	}
}
