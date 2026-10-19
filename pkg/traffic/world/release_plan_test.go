package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A level arrival is judged staying level; a descending one as planned.
func TestHeldPlan(t *testing.T) {
	plan := []traffic.RoutePoint{{AltFt: 9000}, {AltFt: 6000}, {AltFt: 4000}}
	level := traffic.TrackedAircraft{Observation: traffic.Observation{AltFt: 9500, VSFpm: 50}}
	for i, p := range heldPlan(plan, level) {
		if p.AltFt != 9500 {
			t.Fatalf("level arrival point %d at %.0f ft, want 9500", i, p.AltFt)
		}
	}
	down := traffic.TrackedAircraft{Observation: traffic.Observation{AltFt: 9500, VSFpm: -1500}}
	if got := heldPlan(plan, down); got[2].AltFt != 4000 {
		t.Fatalf("descending arrival point at %.0f ft, want its plan's 4000", got[2].AltFt)
	}
	if plan[0].AltFt != 9000 {
		t.Fatal("the plan itself changed")
	}
}

// A climb cleared to FL240 is predicted at the rate it climbs, not at
// FL240 by its next point (live, KLM216 into SWR811 at FL100).
func TestReachable(t *testing.T) {
	me := traffic.TrackedAircraft{Observation: traffic.Observation{AltFt: 5000, VSFpm: 2000, GroundKts: 240}}
	me.Position.Lat, me.Position.Lon = 50, 14
	route := []traffic.RoutePoint{
		{Position: airport.LatLon{Lat: 50, Lon: 14.0622}, AltFt: 24000}, // 2.4 NM: 36 s
		{Position: airport.LatLon{Lat: 50, Lon: 14.6220}, AltFt: 24000}, // 24 NM: 6 min, 17000 ft
		{Position: airport.LatLon{Lat: 50, Lon: 14.0300}, AltFt: 3000},  // below it: kept at 5000
	}
	got := reachable(me, route)
	if got[0].AltFt > 7000 {
		t.Errorf("at 2.4 NM %.0f ft, want about 6200", got[0].AltFt)
	}
	if got[1].AltFt < 16000 || got[1].AltFt > 18000 {
		t.Errorf("at 24 NM %.0f ft, want about 17000", got[1].AltFt)
	}
	if got[2].AltFt < 5000 {
		t.Errorf("predicted down to %.0f ft", got[2].AltFt)
	}
	if route[0].AltFt != 24000 {
		t.Error("the route itself changed")
	}
}
