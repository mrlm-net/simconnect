package calc

import (
	"math"
	"testing"
)

// TestDubins: the path ends at the target on its heading, and a charted
// turn direction is kept (a right-hand teardrop instead of a left turn).
func TestDubins(t *testing.T) {
	lat, lon := 50.1, 14.26
	// Outbound north, back south to a point 3 km east: either way works;
	// forced right, the first turn is clockwise.
	blat, blon := DisplaceByHeading(lat, lon, 90, 3000)
	for _, dir := range []int{0, 1, -1} {
		pts := Dubins(lat, lon, 0, blat, blon, 180, 1500, 50, dir)
		if pts == nil {
			t.Fatalf("dir %d: no path", dir)
		}
		end := pts[len(pts)-1]
		if d := HaversineMeters(end[0], end[1], blat, blon); d > 5 {
			t.Errorf("dir %d: ends %.1f m off", dir, d)
		}
		if dir != 0 {
			h0 := BearingDegrees(pts[0][0], pts[0][1], pts[1][0], pts[1][1])
			h1 := BearingDegrees(pts[1][0], pts[1][1], pts[2][0], pts[2][1])
			turn := math.Mod(h1-h0+540, 360) - 180
			if turn*float64(dir) <= 0 {
				t.Errorf("dir %d: first turn %.2f°", dir, turn)
			}
		}
	}
}
