package calc

import (
	"math"
	"testing"
)

// TestNearAntipodesNoNaN: rounding near antipodal points must not turn the
// haversine term past 1 into NaN.
func TestNearAntipodesNoNaN(t *testing.T) {
	for _, p := range [][4]float64{
		{0, 0, 0, 180},
		{0, 0, 0, -180},
		{45, 10, -45, -170},
		{50.1, 14.26, -50.1, -165.74},
		{89.999999, 0, -89.999999, 180},
		{1e-12, 0, -1e-12, 180},
	} {
		if d := HaversineMeters(p[0], p[1], p[2], p[3]); math.IsNaN(d) || d > math.Pi*earthRadiusM+1 {
			t.Errorf("HaversineMeters%v = %v", p, d)
		}
		if d := CrossTrackMeters(p[0], p[1], 10, 20, p[2], p[3]); math.IsNaN(d) {
			t.Errorf("CrossTrackMeters%v = NaN", p)
		}
		if d := AlongTrackMeters(p[0], p[1], 10, 20, p[2], p[3]); math.IsNaN(d) {
			t.Errorf("AlongTrackMeters%v = NaN", p)
		}
		if la, lo := IntermediatePoint(p[0], p[1], p[2], p[3], 0.5); math.IsNaN(la) || math.IsNaN(lo) {
			t.Errorf("IntermediatePoint%v = NaN", p)
		}
	}
}

// TestDubinsBadInput: a zero or negative step or radius gives no path
// instead of looping forever.
func TestDubinsBadInput(t *testing.T) {
	blat, blon := DisplaceByHeading(50.1, 14.26, 90, 3000)
	for _, rs := range [][2]float64{{1500, 0}, {1500, -5}, {0, 50}, {-1500, 50}, {math.NaN(), 50}, {1500, math.NaN()}} {
		if pts := Dubins(50.1, 14.26, 0, blat, blon, 180, rs[0], rs[1], 0); pts != nil {
			t.Errorf("r %v step %v: %d points, want nil", rs[0], rs[1], len(pts))
		}
	}
}

// TestDubinsAntimeridian: a path across 180° goes the short way east and
// every point stays a valid longitude.
func TestDubinsAntimeridian(t *testing.T) {
	lat, lon := -17.0, 179.99
	blat, blon := DisplaceByHeading(lat, lon, 90, 3000)
	if blon > 0 {
		t.Fatalf("target lon %v did not wrap", blon)
	}
	pts := Dubins(lat, lon, 90, blat, blon, 90, 1500, 50, 0)
	if pts == nil {
		t.Fatal("no path")
	}
	total := 0.0
	for i, p := range pts {
		if p[1] > 180 || p[1] < -180 {
			t.Fatalf("point %d lon %v", i, p[1])
		}
		if i > 0 {
			total += HaversineMeters(pts[i-1][0], pts[i-1][1], p[0], p[1])
		}
	}
	// Straight ahead: about 3 km, not a loop round the other way.
	if total > 3100 {
		t.Errorf("path %.0f m, want ~3000", total)
	}
}
