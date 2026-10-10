package airport

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// EDDM AKIN1N from 26R (the sim's legs, 2026-10-10): DM043 → DM044 is an RF
// arc to the right whose end lies a little off the radius of its start;
// the path ran out radially at DM044 (heading ~300°) and was drawn as a
// loop of about 330° before EMGEP, where the chart turns 4° right.
func TestRFArcEndsOnTrack(t *testing.T) {
	ll := func(lat, lon float64) LatLon { return LatLon{Lat: lat, Lon: lon} }
	legs := []Leg{
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_CF, Fix: "DM040", Position: ll(48.3581917360425, 11.708044409751892), Course: 261},
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_RF, Fix: "DM041", Position: ll(48.384472355246544, 11.653400212526321), TurnRight: true, ArcCenter: ll(48.39123621582985, 11.702300012111664)},
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_TF, Fix: "DM043", Position: ll(48.43356113880873, 11.63841962814331)},
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_RF, Fix: "DM044", Position: ll(48.50203324109316, 11.678122133016586), TurnRight: true, ArcCenter: ll(48.44708885997534, 11.738880425691605)},
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_TF, Fix: "EMGEP", Position: ll(48.7002444639802, 12.090266793966293)},
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_TF, Fix: "AKINI", Position: ll(48.74972216784954, 12.124166786670685)},
	}
	for _, smooth := range []bool{true} { // as drawn: the charted corners rounded
		pts := ProcedurePath(legs, ll(48.3627, 11.7675), 450, 356, 0)
		if smooth {
			pts = SmoothPath(pts, TurnRadiusEnroute, nil)
		}
		worst := 0.0
		for i := 2; i < len(pts); i++ {
			a := calc.BearingDegrees(pts[i-2].Lat, pts[i-2].Lon, pts[i-1].Lat, pts[i-1].Lon)
			b := calc.BearingDegrees(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
			if calc.HaversineMeters(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon) < 1 {
				continue
			}
			worst = math.Max(worst, math.Abs(math.Mod(b-a+540, 360)-180))
		}
		// The arcs are sampled every 5°; no corner of the path turns more.
		if worst > 20 {
			t.Errorf("smooth %v: a corner of %.0f° in the path", smooth, worst)
		}
	}
}
