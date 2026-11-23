package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

var arcTestEnd = airport.RunwayEnd{Name: "18", Heading: 180, Threshold: airport.LatLon{Lat: 50, Lon: 14}}

func arcTestAt(bearing, nm float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(arcTestEnd.Threshold.Lat, arcTestEnd.Threshold.Lon, bearing, nm*1852)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// arcTestRoute resolves an approach to runway 18 round a DME arc of nm
// about the threshold, left from bearing from to the final (north): from
// its start, or from startNM out along that radial by an intercept (inbound
// from outside the arc, outbound from inside). arc
// false: the same legs without the arc's radius, as main flies them.
func arcTestRoute(t *testing.T, from, nm, startNM float64, arc bool) []airport.NavPoint {
	t.Helper()
	legs := []airport.Leg{{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_IF, Fix: "START", FixKind: "W", Position: arcTestAt(from, startNM)}}
	if startNM != nm {
		course := from
		if startNM > nm {
			course = math.Mod(from+180, 360)
		}
		legs = append(legs, airport.Leg{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_CI, Course: course})
	}
	af := airport.Leg{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_AF, Fix: "LEAD", FixKind: "W", Position: arcTestAt(0, nm),
		TurnLeft: true, OriginFix: "NAV", Origin: arcTestEnd.Threshold}
	if arc {
		af.Rho = nm * 1852
	}
	p := airport.Procedures{Approaches: []airport.Approach{{Name: "VOR 18", Runway: "18",
		Transitions: []airport.Transition{{Name: "START", Legs: append(legs, af)}},
		Final:       []airport.Leg{{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_TF, Fix: "RW18", FixKind: "R", Position: arcTestEnd.Threshold, MAP: true}}}}}
	pts, err := p.ResolveApproach("VOR 18", "START")
	if err != nil {
		t.Fatal(err)
	}
	return pts
}

// An arc that would turn the arrival back out at its end (onto the final
// inside the point where it lines up at 11 NM: CYPG VOR/DME 13R via SETPI)
// where the straight way doesn't is flown straight, and so is the intercept
// before it: exactly the plan without arcs (#1646).
func TestPlanArrivalReversingArcStraight(t *testing.T) {
	for _, tc := range []struct {
		name              string
		from, nm, startNM float64
	}{
		{"from its start", 70, 9.2, 9.2},
		{"by an intercept", 60, 6, 20},
	} {
		route, straight := arcTestRoute(t, tc.from, tc.nm, tc.startNM, true), arcTestRoute(t, tc.from, tc.nm, tc.startNM, false)
		if len(route) <= len(straight) {
			t.Errorf("%s: the arc isn't resolved", tc.name)
			continue
		}
		got, err := PlanArrivalProcedure(route, arcTestEnd, ProcedureJoinNm*1852, 2500)
		if err != nil {
			t.Fatal(err)
		}
		want, err := PlanArrivalProcedure(straight, arcTestEnd, ProcedureJoinNm*1852, 2500)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Waypoints) != len(want.Waypoints) {
			t.Errorf("%s: %d waypoints, straight %d", tc.name, len(got.Waypoints), len(want.Waypoints))
			continue
		}
		for i, w := range got.Waypoints {
			if d := calc.HaversineMeters(w.Latitude, w.Longitude, want.Waypoints[i].Latitude, want.Waypoints[i].Longitude); d > 0.01 {
				t.Errorf("%s: waypoint %d is %.0f m from the straight plan's", tc.name, i, d)
			}
		}
	}
}

// An arc outside that point is flown round (#1646).
func TestPlanArrivalFliesDMEArc(t *testing.T) {
	p, err := PlanArrivalProcedure(arcTestRoute(t, 70, 15, 15, true), arcTestEnd, ProcedureJoinNm*1852, 2500)
	if err != nil {
		t.Fatal(err)
	}
	onArc := 0
	for _, w := range p.Waypoints {
		if math.Abs(calc.HaversineNM(arcTestEnd.Threshold.Lat, arcTestEnd.Threshold.Lon, w.Latitude, w.Longitude)-15) < 0.3 {
			onArc++
		}
	}
	if onArc < 4 {
		t.Errorf("%d waypoints on the arc, want 4 or more", onArc)
	}
}
