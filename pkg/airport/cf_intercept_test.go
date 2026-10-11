package airport

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// LKKB ILS 24 via SULOV (the sim's legs, 2026-10-11): from D140J heading
// 140°, CF24 is a course to fix (237°, turn right) 3.5 km on, and the
// heading crosses the 237° final 2.6 km before it. The path turns right
// onto the final there (about 97°); it drew a loop of -264° (MyCrew).
func TestCFInterceptsAhead(t *testing.T) {
	ll := func(lat, lon float64) LatLon { return LatLon{Lat: lat, Lon: lon} }
	cf := types.SIMCONNECT_FACILITY_LEG_TYPE_CF
	legs := []Leg{
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_FC, Fix: "SULOV", Position: ll(50.29562775045633, 14.568291753530502), Course: 60, Distance: 5000.4},
		{Type: cf, Fix: "D140I", Position: ll(50.24099987000227, 14.75835844874382), Course: 140},
		{Type: cf, Fix: "D140J", Position: ll(50.227344408631325, 14.773208498954773), Course: 140},
		{Type: cf, Fix: "CF24", Position: ll(50.19599441438913, 14.766255766153336), Course: 237, TurnRight: true},
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_IF, Fix: "CF24", Position: ll(50.19599441438913, 14.766255766153336)},
		{Type: cf, Fix: "FF24", Position: ll(50.174469351768494, 14.7018501162529), Course: 237},
		{Type: cf, Fix: "RW24", Position: ll(50.12138742953539, 14.543643146753311), Course: 237},
	}
	raw := ProcedurePath(legs, ll(50.29562775045633, 14.568291753530502), 1000, 4, TurnRadiusApproach)
	pts := SmoothPath(raw, TurnRadiusApproach, nil)
	turn, worst := 0.0, 0.0
	prev, have := 0.0, false
	for i := 1; i < len(pts); i++ {
		if calc.HaversineMeters(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon) < 0.5 {
			continue
		}
		b := calc.BearingDegrees(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
		if !have {
			prev, have = b, true
			continue
		}
		d := math.Mod(b-prev+540, 360) - 180
		prev = b
		turn += d
		if math.Abs(d) > 20 {
			t.Logf("corner %.0f at %d %.5f,%.5f", d, i-1, pts[i-1].Lat, pts[i-1].Lon)
		}
		worst = math.Max(worst, math.Abs(d))
	}
	// From SULOV (060°) round to the final (237°): right turns of about
	// 80° and 97°, no loop.
	if turn < 150 || turn > 200 {
		t.Errorf("turns %.0f° in all from SULOV to the runway, want about +177°", turn)
	}
	if worst > 20 {
		t.Errorf("a corner of %.0f°", worst)
	}
}
