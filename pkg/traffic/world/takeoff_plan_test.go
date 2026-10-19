package world

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestTakeoffMeetsLongProcedure: a take-off is judged along the
// departure's SID against an arrival's STAR and approach as planned, not
// by its distance to the threshold. Live, ENT1744 off 06 on ARTU 5E into
// TVS1113 flying VLM 5S and the approach to 24 (held under the arrivals at
// 3700 ft); the same arrival still at the start of its STAR is no reason
// to wait.
func TestTakeoffMeetsLongProcedure(t *testing.T) {
	l := lkprLayout(t)
	b, err := os.ReadFile("../../airport/testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	fieldFt := l.Altitude / 0.3048
	_, dep, _ := l.RunwayEnd("06")
	_, arr, _ := l.RunwayEnd("24")
	sid, err := p.ResolveSID("ARTU5E", "06", "", dep.Threshold, l.Altitude)
	if err != nil {
		t.Fatal(err)
	}
	in, err := p.Arrival("24", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	way := releaseProfile(in)
	if len(way) < 4 {
		t.Fatalf("arrival %d points", len(way))
	}
	// The arrival at the first point of its way within nm of the 24
	// threshold, flying on along the rest.
	at := func(nm float64) (traffic.TrackedAircraft, []traffic.RoutePoint) {
		i := 0
		for i < len(way)-2 && calc.HaversineNM(way[i].Position.Lat, way[i].Position.Lon, arr.Threshold.Lat, arr.Threshold.Lon) > nm {
			i++
		}
		w := way[i]
		a := traffic.TrackedAircraft{Observation: traffic.Observation{Tail: "TVS1113", Position: w.Position, AltFt: w.AltFt, GroundKts: 220, VSFpm: -800,
			Heading: calc.BearingDegrees(w.Position.Lat, w.Position.Lon, way[i+1].Position.Lat, way[i+1].Position.Lon)}, Phase: traffic.PhaseArriving, Ours: true}
		return a, way[i:]
	}
	judge := func(nm float64) string {
		me, plan := takeoffPlan(dep, fieldFt, sid)
		me.Tail = "ENT1744"
		a, rest := at(nm)
		return takeoffConflict([]traffic.TrackedAircraft{me, a}, map[string][]traffic.RoutePoint{me.Tail: plan, a.Tail: rest})
	}
	if got := judge(15); got != "TVS1113" {
		t.Errorf("arrival 15 NM out on the 24 approach: take-off clear (%q), want held for TVS1113", got)
	}
	if got := judge(1000); got != "" {
		t.Errorf("arrival at the start of its STAR: held for %q, want clear", got)
	}
}

// TestTakeoffCapClears: a departure whose climb meets an arrival level at
// 6000 ft across its way is clear with "after departure climb to altitude
// 4000 feet" (the user: a restriction up front, not a hold or a level-off
// later).
func TestTakeoffCapClears(t *testing.T) {
	l := lkprLayout(t)
	_, dep, _ := l.RunwayEnd("06")
	fieldFt := l.Altitude / 0.3048
	me, plan := takeoffPlan(dep, fieldFt, nil) // straight out
	me.Tail = "OKETU"
	// The arrival 15 NM out along the climb, crossing it level at 6000 ft.
	lat, lon := calc.DisplaceByHeading(dep.Threshold.Lat, dep.Threshold.Lon, dep.Heading, 15*1852)
	sLat, sLon := calc.DisplaceByHeading(lat, lon, dep.Heading-90, 13*1852)
	eLat, eLon := calc.DisplaceByHeading(lat, lon, dep.Heading+90, 13*1852)
	start, endP := airport.LatLon{Lat: sLat, Lon: sLon}, airport.LatLon{Lat: eLat, Lon: eLon}
	arr := traffic.TrackedAircraft{Observation: traffic.Observation{Tail: "OOILE", Position: start, AltFt: 6000, GroundKts: 220,
		Heading: calc.BearingDegrees(sLat, sLon, eLat, eLon)}, Phase: traffic.PhaseArriving, Ours: true}
	way := []traffic.RoutePoint{{Position: start, AltFt: 6000, Kts: 220}, {Position: endP, AltFt: 6000, Kts: 220}}
	// Climbing as planned (to its SID top or on): through 9000 ft there.
	for i := range plan {
		plan[i].AltFt = math.Max(plan[i].AltFt, 11000)
	}
	if got := takeoffConflict([]traffic.TrackedAircraft{me, arr}, map[string][]traffic.RoutePoint{me.Tail: plan, arr.Tail: way}); got != "OOILE" {
		t.Skipf("no conflict climbing unrestricted (%q): geometry", got)
	}
	capped := append([]traffic.RoutePoint(nil), plan...)
	for i := range capped {
		capped[i].AltFt = math.Min(capped[i].AltFt, 4000)
	}
	if got := takeoffConflict([]traffic.TrackedAircraft{me, arr}, map[string][]traffic.RoutePoint{me.Tail: capped, arr.Tail: way}); got != "" {
		t.Errorf("after departure climb to 4000 ft: still meets %q", got)
	}
}
