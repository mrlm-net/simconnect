package traffic

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

func lkprProcedures(t *testing.T) airport.Procedures {
	t.Helper()
	b, err := os.ReadFile("../airport/testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestPlanArrivalProcedure: GOLOP 4T and the ILS 06 via KUVIX flown to a
// join point 8 NM out on the 06 centreline: descending all the way, the
// approach's 4000 ft minimums kept, ending aligned on the final.
func TestPlanArrivalProcedure(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	_, end, _ := g.Layout.RunwayEnd("06")
	join := ProcedureJoinNm * 1852
	jp := NewApproachMover(end.Threshold, end.Heading, join, DefaultApproachProfile()).Pose()
	fieldFt := g.Layout.Altitude / 0.3048
	ap, err := PlanArrivalProcedure(route, end, join, fieldFt+jp.HeightFt)
	if err != nil {
		t.Fatal(err)
	}
	if d := calc.HaversineMeters(ap.Spawn.Latitude, ap.Spawn.Longitude, route[0].Position.Lat, route[0].Position.Lon); d > 1 {
		t.Errorf("spawn %.0f m from GOLOP", d)
	}
	prev := ap.Spawn.Altitude
	for i, w := range ap.Waypoints {
		if w.Altitude > prev+1 {
			t.Errorf("waypoint %d climbs: %.0f ft after %.0f", i, w.Altitude, prev)
		}
		prev = w.Altitude
	}
	last := ap.Waypoints[len(ap.Waypoints)-1]
	if d := calc.HaversineMeters(last.Latitude, last.Longitude, ap.Join.Lat, ap.Join.Lon); d > 1 {
		t.Errorf("last waypoint %.0f m from the join point", d)
	}
	for _, n := range route {
		if n.Ident != "PR741" && n.Ident != "PR742" {
			continue
		}
		for _, w := range ap.Waypoints {
			if calc.HaversineMeters(w.Latitude, w.Longitude, n.Position.Lat, n.Position.Lon) < 1 && w.Altitude < 3999 {
				t.Errorf("%s at %.0f ft, below its 4000 ft minimum", n.Ident, w.Altitude)
			}
		}
	}
	if len(ap.Waypoints) < 7 || ap.Spawn.Altitude > ProcedureTopFt+1 {
		t.Errorf("spawn %.0f ft, %d waypoints", ap.Spawn.Altitude, len(ap.Waypoints))
	}
	t.Logf("spawn %.0f ft, %d waypoints, join %.0f ft", ap.Spawn.Altitude, len(ap.Waypoints), last.Altitude)
}

// TestDepartureWaypoints: the SID after the injected climb, climbing and
// continuing on the last track.
func TestDepartureWaypoints(t *testing.T) {
	g := lkprGraph(t)
	p := lkprProcedures(t)
	rwy, end, _ := g.Layout.RunwayEnd("24")
	far := rwy.Primary.Threshold
	if end.Name == rwy.Primary.Name {
		far = rwy.Secondary.Threshold
	}
	sid, err := p.ResolveSID("VOZ4A", "24", "", far, g.Layout.Altitude)
	if err != nil {
		t.Fatal(err)
	}
	lat, lon := calc.DisplaceByHeading(far.Lat, far.Lon, end.Heading, 3000)
	wps := DepartureWaypoints(airport.LatLon{Lat: lat, Lon: lon}, end.Heading, 2800, sid)
	if len(wps) < 3 {
		t.Fatalf("%d waypoints", len(wps))
	}
	voz := sid[len(sid)-1].Position
	found := false
	for i, w := range wps {
		if i > 0 && w.Altitude < wps[i-1].Altitude {
			t.Errorf("descends at %d", i)
		}
		if calc.HaversineMeters(w.Latitude, w.Longitude, voz.Lat, voz.Lon) < 1 {
			found = true
		}
	}
	if !found || wps[len(wps)-1].Altitude < ProcedureTopFt {
		t.Errorf("VOZ in the chain %v, final altitude %.0f", found, wps[len(wps)-1].Altitude)
	}
}

// TestArrivalControllerFliesProcedure: the arrival appears at GOLOP, MSFS
// AI gets the procedure; abeam the join point on the centreline the
// injected approach takes over without a jump and the aircraft lands.
func TestArrivalControllerFliesProcedure(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", "GOLOP")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
		InjectApproach: true, Procedure: route, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	if inj.Driven(77) || !ctl.flyingProc || len(ec.released) == 0 {
		t.Fatal("not handed to MSFS AI for the procedure")
	}
	mon := DefaultArrivalRequestBase + arrReqMonitor
	p := ctl.Plan()
	// MSFS AI on the STAR: nothing happens.
	ctl.Handle(arrivalPositionMsg(mon, 77, route[1].Position, 6000, 200, 250, false))
	if !ctl.flyingProc {
		t.Fatal("took over on the STAR")
	}
	// MSFS AI established on the final, 50 m off the centreline.
	out := math.Mod(p.End.Heading+180, 360)
	lat, lon := calc.DisplaceByHeading(p.End.Threshold.Lat, p.End.Threshold.Lon, out, ProcedureJoinNm*1852-200)
	lat, lon = calc.DisplaceByHeading(lat, lon, out+90, 50)
	at := airport.LatLon{Lat: lat, Lon: lon}
	ctl.Handle(arrivalPositionMsg(mon, 77, at, 2400, p.End.Heading+3, 160, false))
	if ctl.flyingProc || !inj.Driven(77) || ctl.approach == nil {
		t.Fatal("no takeover on the final")
	}
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	start := len(placements(ec))
	for i := 0; i < 60*1500 && ctl.State() != ArrivalParked; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("stuck in %v", ctl.State())
	}
	placed := placements(ec)[start:]
	if d := calc.HaversineMeters(placed[0].Latitude, placed[0].Longitude, at.Lat, at.Lon); d > 10 {
		t.Errorf("first injected placement %.0f m from where MSFS AI flew it", d)
	}
	for i := 1; i < len(placed) && i < 60*60; i++ {
		a, b := placed[i-1], placed[i]
		if d := calc.HaversineMeters(a.Latitude, a.Longitude, b.Latitude, b.Longitude); d > 2 {
			t.Fatalf("jump of %.1f m at frame %d after the takeover", d, i)
		}
	}
}
