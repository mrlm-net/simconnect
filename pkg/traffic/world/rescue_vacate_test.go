package world

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// planArrival is an arrival on approach with its landing plan.
type planArrival struct {
	arrivalCtl
	plan *traffic.ArrivalPlan
}

func (a *planArrival) State() traffic.ArrivalState { return traffic.ArrivalApproaching }
func (a *planArrival) Plan() *traffic.ArrivalPlan  { return a.plan }

// rescueOneExitField is a runway 09/27 of 1900 m with one exit, taxiway C
// half way along to the south, a parallel taxiway A 200 m south and a fire
// station with 2 vehicles beyond it.
func rescueOneExitField(t *testing.T) *airport.Layout {
	t.Helper()
	var pts []airport.RawTaxiPoint
	var paths []airport.RawTaxiPath
	pt := func(x, z float32, typ int32) int32 {
		pts = append(pts, airport.RawTaxiPoint{Type: typ, BiasX: x, BiasZ: z})
		return int32(len(pts) - 1)
	}
	link := func(a, b, typ int32, name uint32, width float32, rwy int32) {
		paths = append(paths, airport.RawTaxiPath{Type: typ, Width: width, RunwayNumber: rwy, Start: a, End: b, NameIndex: name})
	}
	west, r, east := pt(-950, 0, 1), pt(0, 0, 1), pt(950, 0, 1)
	link(west, r, 2, 0, 45, 9)
	link(r, east, 2, 0, 45, 9)
	h := pt(0, -90, 2)
	link(r, h, 1, 3, 23, 0)
	a := pt(0, -200, 1)
	link(h, a, 1, 3, 23, 0)
	aw, ae := pt(-950, -200, 1), pt(950, -200, 1)
	link(aw, a, 1, 1, 23, 0)
	link(a, ae, 1, 1, 23, 0)
	s := pt(0, -400, 1)
	link(a, s, 1, 5, 15, 0)
	lead := pt(0, -520, 1)
	link(s, lead, 1, 5, 15, 0)
	link(lead, 0, 3, 0, 15, 0)
	raw := airport.RawAirport{ICAO: "XONE", Name: "One exit", Latitude: 50.12, Longitude: 13.81, Altitude: 300,
		Runways:    []airport.RawRunway{{Latitude: 50.12, Longitude: 13.81, Altitude: 300, Heading: 90, Length: 1900, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking:    []airport.RawParking{{Name: 14, Number: 1, Type: 13, Radius: 10, BiasX: 0, BiasZ: -600, RescueVehicles: 2}},
		TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "B", "C", "D", "E"}}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// TestRescueStandbyOffVacateWay (#1632): a drill on a runway with one
// exit. The arrival plans to vacate by it and taxi east on A; the fire
// trucks wait at standby off that way: none within the arrival's half-span
// and VehicleClearMeters of it (they stood on the exit's centreline and it
// drove through both: aircraft do not see the vehicles).
func TestRescueStandbyOffVacateWay(t *testing.T) {
	l := rescueOneExitField(t)
	cc, c, w := rescueBed(t, l, rescueTestTitles)
	g, _ := cc.graph("XONE")
	_, end, ok := l.RunwayEnd("09")
	if !ok {
		t.Fatal("no runway 09")
	}
	exits, err := g.RunwayExits("09")
	if err != nil || len(exits) != 1 {
		t.Fatalf("exits %v, %v: want 1", exits, err)
	}
	x := exits[0]
	var vacate []airport.LatLon
	for _, n := range x.Path {
		vacate = append(vacate, g.Nodes[n].Position)
	}
	// On from the exit: down C to A and east along it.
	cA := vacate[len(vacate)-1]
	for _, d := range []float64{0, 40, 110} {
		lat, lon := calc.DisplaceByHeading(cA.Lat, cA.Lon, 180, d)
		vacate = append(vacate, airport.LatLon{Lat: lat, Lon: lon})
	}
	a := vacate[len(vacate)-1]
	for _, d := range []float64{100, 300, 500} {
		lat, lon := calc.DisplaceByHeading(a.Lat, a.Lon, 90, d)
		vacate = append(vacate, airport.LatLon{Lat: lat, Lon: lon})
	}
	plan := &traffic.ArrivalPlan{End: end, Exit: x, Route: &airport.Route{Points: vacate[len(x.Path)-1:]}}
	it := &controlled{ID: 1, Kind: "arrival", Tail: "CSA1", ICAO: "XONE", arr: &planArrival{plan: plan}}
	it.view.Runway = "09"
	cc.items[1] = it
	w.PriorityFor("CSA1", traffic.EmergencyFire)
	prio := map[string]bool{"CSA1": true}
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, -(rescueDispatchNM-1)*1852)
	ac := traffic.TrackedAircraft{Airport: "XONE"}
	ac.Tail, ac.Position, ac.Heading, ac.GroundKts, ac.AGLFt = "CSA1", airport.LatLon{Lat: lat, Lon: lon}, end.Heading, 140, 2000
	now := time.Now()
	ac.Updated = now
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{ac})
	crew := cc.rescue.by["CSA1"]
	if crew == nil || len(crew.trucks) == 0 {
		t.Fatal("no crew sent")
	}
	handled := 0
	for step := 0; step < 10*60*10; step++ {
		for ; handled < len(c.req); handled++ {
			cc.rescueHandle(assignedObject(c.req[handled], 800+c.req[handled]))
		}
		now = now.Add(100 * time.Millisecond)
		ac.Updated = now
		if step%10 == 0 {
			cc.rescueStep(now, prio, []traffic.TrackedAircraft{ac})
		}
		cc.rescueFrame(now)
	}
	keep := traffic.DefaultHalfSpanMeters + traffic.VehicleClearMeters
	for i, tr := range crew.trucks {
		p, route, ok := tr.Track()
		if !ok {
			t.Errorf("vehicle %d not out", i)
			continue
		}
		if len(route) >= 2 {
			t.Errorf("vehicle %d still on its way to standby after 10 min", i)
		}
		near := math.Inf(1)
		for k := 1; k < len(vacate); k++ {
			near = math.Min(near, segDist(p.Position, vacate[k-1], vacate[k]))
		}
		if near <= keep {
			t.Errorf("vehicle %d at standby %.1f m from the arrival's way off the runway (keep %.1f m)", i, near, keep)
		}
	}
}
