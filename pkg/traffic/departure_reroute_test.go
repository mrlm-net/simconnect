//go:build windows
// +build windows

package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A departure handed to MSFS AI gives its climb as a route and flies a
// conflict resolution sent back (#639: RYR1527 flew through OKCVY ahead on
// the same SID, neither steerable).
func TestDepartureReroute(t *testing.T) {
	ec := &eventClient{}
	ctl := NewTaxiController(NewFleet(ec))
	start := pictureLKPR.Position
	var pts []airport.LatLon
	for i := range 5 {
		pts = append(pts, offsetHeading(start, 65, float64(i+1)*10*1852))
	}
	if ctl.ClimbPlan(start) != nil {
		t.Fatal("a climb plan before the hand-over")
	}
	ctl.objectID, ctl.state = 77, TaxiComplete
	for i, p := range pts {
		ctl.climb = append(ctl.climb, procedureWaypoint(p, 6000+float64(i)*3000, 250))
	}
	if err := ctl.Reroute(nil); err == nil {
		t.Error("an empty route accepted")
	}
	at := offsetHeading(start, 65, 15*1852) // past the first point
	plan := ctl.ClimbPlan(at)
	if len(plan) != 4 || plan[0].AltFt != 9000 || plan[0].Kts != 250 {
		t.Fatalf("plan %+v, want the 4 points ahead from 9000 ft at 250 kt", plan)
	}
	a := TrackedAircraft{Observation: Observation{ObjectID: 77, Tail: "RYR1527", Position: at, AltFt: 8000, GroundKts: 300, Heading: 65}}
	r := Resolution{Callsign: "RYR1527", ObjectID: 77, Kind: ResolveLevel, AltFt: 7000}
	before := len(ec.waypoints)
	if err := ctl.Reroute(ResolvedRoute(plan, a, r, 5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(ec.waypoints) != before+1 {
		t.Fatal("no waypoints sent")
	}
	if got := ctl.ClimbPlan(at); len(got) == 0 || got[0].AltFt != 7000 {
		t.Errorf("after the level change the plan starts %+v, want 7000 ft", got)
	}
}
