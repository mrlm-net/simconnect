package airport

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// dmeArc is an IF at bearing from of a navaid and an AF leg round it to
// bearing to, as the facility data gives them: no arc centre.
func dmeArc(from, to float64, right bool) (nav LatLon, legs []Leg) {
	nav = LatLon{Lat: 41.8, Lon: 12.24}
	const rho = 20000.0
	return nav, []Leg{
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_IF, Fix: "START", FixKind: "W", Position: onSphere(nav, from, rho)},
		{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_AF, Fix: "END", FixKind: "W", Position: onSphere(nav, to, rho),
			TurnRight: right, TurnLeft: !right, OriginFix: "OST", Origin: nav, Rho: rho},
	}
}

// legShapes are legs as drawn (ProcedurePath) and as flown (resolveLegs).
func legShapes(legs []Leg) map[string][]LatLon {
	out := map[string][]LatLon{"path": ProcedurePath(legs, LatLon{}, 0, 0, TurnRadiusApproach)}
	for _, n := range resolveLegs(legs, LatLon{}, 0, 0) {
		out["points"] = append(out["points"], n.Position)
	}
	return out
}

// An AF leg is an arc about its navaid at rho, the way it turns (#1646).
func TestAFLegIsAnArcAboutItsNavaid(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to float64
		right    bool
		sweep    float64
	}{
		{"right", 20, 110, true, 90},
		{"left", 110, 20, false, 90},
		{"right through north", 330, 40, true, 70},
		{"left through north", 40, 330, false, 70},
		{"right, the long way", 100, 20, true, 280},
	} {
		nav, legs := dmeArc(tc.from, tc.to, tc.right)
		for how, pts := range legShapes(legs) {
			if len(pts) < 3 {
				t.Errorf("%s, %s: straight (%d points)", tc.name, how, len(pts))
				continue
			}
			turned := 0.0
			for i, p := range pts {
				if d := fixDistM(nav, p); math.Abs(d-20000) > 5 {
					t.Errorf("%s, %s: point %d is %.0f m from the navaid, want 20000", tc.name, how, i, d)
				}
				if i == 0 {
					continue
				}
				step := math.Mod(bearingOf(nav, p)-bearingOf(nav, pts[i-1])+540, 360) - 180
				if (step > 0) != tc.right || math.Abs(step) > 35 {
					t.Errorf("%s, %s: point %d steps %.1f°", tc.name, how, i, step)
				}
				turned += math.Abs(step)
			}
			if math.Abs(turned-tc.sweep) > 0.5 {
				t.Errorf("%s, %s: %.1f° round, want %.0f°", tc.name, how, turned, tc.sweep)
			}
			if end := pts[len(pts)-1]; end != legs[1].Position {
				t.Errorf("%s, %s: ends %.0f m from the fix", tc.name, how, fixDistM(end, legs[1].Position))
			}
		}
	}
}

// An AF leg whose numbers don't make an arc stays straight.
func TestAFLegStraightWhereInconsistent(t *testing.T) {
	for name, spoil := range map[string]func(nav LatLon, legs []Leg){
		"no turn":       func(_ LatLon, l []Leg) { l[1].TurnLeft, l[1].TurnRight = false, false },
		"both turns":    func(_ LatLon, l []Leg) { l[1].TurnLeft, l[1].TurnRight = true, true },
		"no navaid":     func(_ LatLon, l []Leg) { l[1].OriginFix, l[1].Origin = "", LatLon{} },
		"no rho":        func(_ LatLon, l []Leg) { l[1].Rho = 0 },
		"rho wrong":     func(_ LatLon, l []Leg) { l[1].Rho = 24000 },
		"start off":     func(n LatLon, l []Leg) { l[0].Position = fixAlong(n, 20, 17000) },
		"end off":       func(n LatLon, l []Leg) { l[1].Position = fixAlong(n, 50, 23000) },
		"the wrong way": func(_ LatLon, l []Leg) { l[1].TurnRight, l[1].TurnLeft = false, true },
	} {
		nav, legs := dmeArc(20, 50, true)
		spoil(nav, legs)
		for how, pts := range legShapes(legs) {
			if len(pts) != 2 {
				t.Errorf("%s, %s: %d points, want the straight leg", name, how, len(pts))
			}
		}
	}
}

// A course or heading to intercept before a DME arc goes as far as the arc
// (#1646): out to it from inside, in to it from outside.
func TestInterceptBeforeAFReachesTheArc(t *testing.T) {
	for _, tc := range []struct {
		name   string
		radius float64
		course float64
		leg    types.SIMCONNECT_FACILITY_LEG_TYPE
	}{
		{"out", 4000, 20, types.SIMCONNECT_FACILITY_LEG_TYPE_VI},
		{"in", 40000, 200, types.SIMCONNECT_FACILITY_LEG_TYPE_CI},
	} {
		nav, arc := dmeArc(20, 110, true)
		legs := []Leg{
			{Type: types.SIMCONNECT_FACILITY_LEG_TYPE_IF, Fix: "FROM", FixKind: "W", Position: fixAlong(nav, 20, tc.radius)},
			{Type: tc.leg, Course: tc.course},
			arc[1],
		}
		for how, pts := range legShapes(legs) {
			var on []LatLon
			for _, p := range pts {
				if math.Abs(fixDistM(nav, p)-20000) < 300 {
					on = append(on, p)
				}
			}
			if len(on) < 4 {
				t.Errorf("%s, %s: %d points on the arc", tc.name, how, len(on))
				continue
			}
			if d := fixDistM(on[0], arc[0].Position); d > 500 {
				t.Errorf("%s, %s: meets the arc %.0f m from the intercept", tc.name, how, d)
			}
		}
	}
}

// The arcs reported in #1646, from the dumped data: every point along them
// is at rho from the navaid.
func TestAFLegsOffline(t *testing.T) {
	for _, tc := range []struct {
		icao, approach, transition, fix string
	}{
		{"LIRF", "VOR/DME 07", "CMP", "CD07"},
		{"KASE", "LOC 15", "IVKEF", "D309D"},
		{"SBRJ", "VOR/DME 20L A", "EVRIR", "PROTE"},
	} {
		t.Run(tc.icao, func(t *testing.T) {
			_, p := offlineProcedures(t, tc.icao)
			pts, err := p.ResolveApproach(tc.approach, tc.transition)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := p.findApproach(tc.approach)
			tr, _ := namedTransition(a.Transitions, tc.transition)
			var leg Leg
			for _, l := range tr.Legs {
				if l.Type == types.SIMCONNECT_FACILITY_LEG_TYPE_AF && l.Fix == tc.fix {
					leg = l
				}
			}
			if leg.Rho == 0 {
				t.Fatalf("no AF leg to %s", tc.fix)
			}
			n := 0
			for _, np := range pts {
				if np.LegType != types.SIMCONNECT_FACILITY_LEG_TYPE_AF || !np.Computed() {
					continue
				}
				if d := fixDistM(leg.Origin, np.Position); math.Abs(d-leg.Rho) > arcTestTolM {
					t.Errorf("arc point %.0f m from %s, rho %.0f", d, leg.OriginFix, leg.Rho)
				}
				n++
			}
			if n == 0 {
				t.Errorf("%s via %s: straight to %s", tc.approach, tc.transition, tc.fix)
			}
		})
	}
}

// arcTestTolM is how far the charted fixes lie off their arc's radius.
const arcTestTolM = 500.0

// onSphere is the point d meters from p that calc's bearing and distance
// give back (fixAlong's flat earth is 70 m off at 20 km).
func onSphere(p LatLon, bearing, d float64) LatLon {
	lat, b, a := p.Lat*math.Pi/180, bearing*math.Pi/180, d/6371000
	to := math.Asin(math.Sin(lat)*math.Cos(a) + math.Cos(lat)*math.Sin(a)*math.Cos(b))
	dLon := math.Atan2(math.Sin(b)*math.Sin(a)*math.Cos(lat), math.Cos(a)-math.Sin(lat)*math.Sin(to))
	return LatLon{Lat: to * 180 / math.Pi, Lon: p.Lon + dLon*180/math.Pi}
}

func bearingOf(a, b LatLon) float64 { return calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon) }
