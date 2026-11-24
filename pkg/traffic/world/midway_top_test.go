package world

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// An arrival on its way in at the start is no higher than a 3° path for
// the way the arrival controller flies it from where it appears: to its
// join, then along the rest of the STAR and approach (a leg with no fix
// of its own left out). Not for its plan's way in, which may be far
// longer (#1681 follow-up: DLH1691 at FL289 for 90 NM of plan with 36 NM
// in from DF520, joined its final thousands of feet high, and its crew
// went round as soon as it was cleared to land).
func TestMidwayTopFt(t *testing.T) {
	const field = 364.0
	pos := airport.LatLon{Lat: 50.30, Lon: 8.90}
	pts := []airport.NavPoint{
		{Ident: "DF520", Position: airport.LatLon{Lat: 50.20, Lon: 8.80}},
		{Ident: "VM"},
		{Ident: "DF521", Position: airport.LatLon{Lat: 50.10, Lon: 8.85}},
		{Ident: "RW25C", Position: airport.LatLon{Lat: 50.04, Lon: 8.59}},
	}
	nm, at := 0.0, pos
	for _, p := range pts {
		if p.Position != (airport.LatLon{}) {
			nm += calc.HaversineNM(at.Lat, at.Lon, p.Position.Lat, p.Position.Lon)
			at = p.Position
		}
	}
	top := midwayTopFt(field, pos, pts)
	if want := field + nm*midwayFtPerNM; math.Abs(top-want) > 1 {
		t.Errorf("%.0f NM in: %.0f ft at most, want %.0f", nm, top, want)
	}
	if planned := field + 90*midwayFtPerNM; top >= planned-10000 {
		t.Errorf("%.0f NM in: %.0f ft at most, not far below the %.0f ft of its plan's 90 NM", nm, top, planned)
	}
}
