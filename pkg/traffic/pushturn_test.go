//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
)

// TestDubins: the path starts and ends at the given poses, heading as asked,
// never turns tighter than the radius and is no longer than needed (a U-turn
// to a point 2r to the side is a half circle).
func TestDubins(t *testing.T) {
	for _, c := range []struct {
		name         string
		east, north  float64
		headA, headB float64
		r, maxLen    float64
	}{
		{"u-turn", 20, 0, 0, 180, 10, math.Pi*10 + 0.5},
		{"straight", 0, 50, 0, 0, 10, 50.5},
		{"quarter", 20, 20, 0, 90, 20, math.Pi*10 + 0.5},
		{"s-bend", 10, 60, 0, 0, 15, 70},
	} {
		b := offset(lkpr, c.east, c.north)
		pts := dubins(lkpr, c.headA, b, c.headB, c.r, 0.25)
		if pts == nil {
			t.Fatalf("%s: no path", c.name)
		}
		if d := localDist(pts[len(pts)-1], b); d > 0.2 {
			t.Errorf("%s: ends %.2f m from the target", c.name, d)
		}
		n := len(pts)
		if d := math.Abs(headingDiff(localBearing(pts[0], pts[2]), c.headA)); d > 3 {
			t.Errorf("%s: starts %.1f° off", c.name, d)
		}
		if d := math.Abs(headingDiff(localBearing(pts[n-3], pts[n-1]), c.headB)); d > 3 {
			t.Errorf("%s: ends %.1f° off", c.name, d)
		}
		if l := pathLen(pts); l > c.maxLen {
			t.Errorf("%s: %.1f m long, want at most %.1f", c.name, l, c.maxLen)
		}
		for i := 2; i+2 < n; i += 2 {
			turn := math.Abs(headingDiff(localBearing(pts[i-2], pts[i]), localBearing(pts[i], pts[i+2])))
			if turn > 1e-6 {
				if rad := localDist(pts[i-2], pts[i+2]) / (turn * math.Pi / 180); rad < c.r*0.95 {
					t.Fatalf("%s: turns on %.1f m", c.name, rad)
				}
			}
		}
	}
}
