package calc

import (
	"math"
	"testing"
)

func TestIntermediatePoint(t *testing.T) {
	// The ends, and the midpoint of two points on the equator.
	if lat, lon := IntermediatePoint(0, 0, 0, 90, 0.5); math.Abs(lat) > 1e-9 || math.Abs(lon-45) > 1e-9 {
		t.Errorf("equator midpoint %.6f, %.6f", lat, lon)
	}
	if lat, lon := IntermediatePoint(50, 14, 52, 21, 0); math.Abs(lat-50) > 1e-9 || math.Abs(lon-14) > 1e-9 {
		t.Errorf("start %.6f, %.6f", lat, lon)
	}
	// A great circle bulges poleward: halfway from Dublin to Seoul is far
	// north of the straight lat/lon midpoint (about 45.4N 66.3E).
	lat, _ := IntermediatePoint(53.4213, -6.2701, 37.4602, 126.4407, 0.5)
	if lat < 60 {
		t.Errorf("Dublin–Seoul midpoint at %.1fN, want well north of the lat/lon midpoint", lat)
	}
	// Every point is on the circle: distances add up.
	total := HaversineNM(53.4213, -6.2701, 37.4602, 126.4407)
	mlat, mlon := IntermediatePoint(53.4213, -6.2701, 37.4602, 126.4407, 0.3)
	d := HaversineNM(53.4213, -6.2701, mlat, mlon)
	if math.Abs(d-0.3*total) > 0.5 {
		t.Errorf("30%% along: %.1f NM, want %.1f", d, 0.3*total)
	}
}
