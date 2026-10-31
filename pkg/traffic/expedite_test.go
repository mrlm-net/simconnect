package traffic

import (
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestInstructionWords: what #1331 adds to the clearances, as said.
func TestInstructionWords(t *testing.T) {
	const cs = "CSA1"
	for _, tc := range []struct {
		name string
		got  Transmission
		want string
	}{
		{"expedite climb", Expedited(Climb(PosDeparture, cs, 12000, 5000), ""), "expedite climb to "},
		{"expedite descent", Expedited(Descend(PosApproach, cs, 4000, 5000), ""), "expedite descent to 4000 feet"},
		{"through a level", Expedited(Climb(PosDeparture, cs, 12000, 5000), "FL 80"), ", expedite through FL 80"},
		{"speed", Expedited(SpeedAssigned(cs, 180, 220), ""), "reduce speed to 180 knots, expedite"},
		{"immediate", Rushed(ClearedTakeoff(cs, "24", "")), "cleared for immediate take-off"},
		{"no delay, traffic", NoDelayed(ClearedTakeoff(cs, "24", ""), 2.6), "no delay, traffic on 3 mile final"},
		{"no delay, none", NoDelayed(ClearedTakeoff(cs, "24", ""), 0), "cleared for take-off, no delay"},
		{"departing ahead", LandingAhead(ClearedToLand(cs, "24", ""), "departing"), "cleared to land, traffic departing ahead"},
		{"landing ahead", LandingAhead(ClearedToLand(cs, "24", ""), "landing"), "cleared to land, traffic landing ahead"},
		{"nothing ahead", LandingAhead(ClearedToLand(cs, "24", ""), ""), "cleared to land"},
		{"limit by name", HoldShortOf(ClearedTaxiUpTo(cs, []string{"A"}, "C"), "B"), "taxi via A, hold short of B"},
	} {
		if !strings.Contains(tc.got.Text, tc.want) {
			t.Errorf("%s: said %q, want %q", tc.name, tc.got.Text, tc.want)
		}
	}
	// Only a climb or a descent: a stop is no hurry, nor is normal speed.
	if got := Expedited(SpeedAssigned(cs, 0, 220), ""); strings.Contains(got.Text, "expedite") {
		t.Errorf("resume normal speed: %q", got.Text)
	}
	if got := Expedited(ContinueTaxi(cs), ""); strings.Contains(got.Text, "expedite") {
		t.Errorf("a taxi clearance: %q", got.Text)
	}
}

// TestClimbRouteExpedited: expedited, the level is reached in a shorter
// distance; only through a level, that level first and on at the usual rate.
func TestClimbRouteExpedited(t *testing.T) {
	pos := airport.LatLon{Lat: 50, Lon: 14}
	lat, lon := calc.DisplaceByHeading(pos.Lat, pos.Lon, 90, 200000)
	plan := []RoutePoint{{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: 5000, Kts: 300}}
	to := func(r RoutePoint) float64 {
		return calc.HaversineMeters(pos.Lat, pos.Lon, r.Position.Lat, r.Position.Lon)
	}

	plain := climbRoute(pos, 5000, 11000, 0, plan)
	whole := climbRoute(pos, 5000, 11000, 11000, plan)
	if len(plain) != 3 || len(whole) != 3 {
		t.Fatalf("plain %d, whole %d points, want 3", len(plain), len(whole))
	}
	if a, b := to(plain[1]), to(whole[1]); b >= a*0.7 {
		t.Errorf("11000 ft after %.0f m, expedited after %.0f m", a, b)
	}

	part := climbRoute(pos, 5000, 11000, 8000, plan)
	if len(part) != 4 || part[1].AltFt != 8000 || part[2].AltFt != 11000 {
		t.Fatalf("through 8000 ft: %+v", part)
	}
	first, rest := to(part[1]), to(part[2])-to(part[1])
	if rest < first*1.4 {
		t.Errorf("3000 ft in %.0f m, the next 3000 ft in %.0f m: not slower after", first, rest)
	}
	if last := whole[len(whole)-1]; last.AltFt != 11000 {
		t.Errorf("the plan stays at %.0f ft", last.AltFt)
	}
}
