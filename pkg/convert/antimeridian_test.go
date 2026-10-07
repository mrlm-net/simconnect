package convert

import (
	"math"
	"testing"
)

// TestOffsetsAcrossAntimeridian: offsets take the short way across 180°
// and positions come back as valid longitudes.
func TestOffsetsAcrossAntimeridian(t *testing.T) {
	x, _ := LatLonToOffset(0, 179.9, 0, -179.9)
	if x < 0 || math.Abs(x-22264) > 50 {
		t.Errorf("east 179.9 → -179.9 = %.0f m, want ~+22264", x)
	}
	x, _ = LatLonToOffset(0, -179.9, 0, 179.9)
	if x > 0 || math.Abs(x+22264) > 50 {
		t.Errorf("east -179.9 → 179.9 = %.0f m, want ~-22264", x)
	}
	_, lon := OffsetToLatLon(0, 179.9, 22264, 0)
	if math.Abs(lon+179.9) > 1e-3 {
		t.Errorf("lon = %v, want -179.9", lon)
	}
	_, lon = OffsetToLatLon(0, -179.9, -22264, 0)
	if math.Abs(lon-179.9) > 1e-3 {
		t.Errorf("lon = %v, want 179.9", lon)
	}
	// Normal cases keep the exact old values.
	if _, lon = OffsetToLatLon(0, 180, 0, 0); lon != 180 {
		t.Errorf("lon = %v, want 180", lon)
	}
	if _, lon = OffsetToLatLon(0, -180, 0, 0); lon != -180 {
		t.Errorf("lon = %v, want -180", lon)
	}
}
