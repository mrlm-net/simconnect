package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Smoothness benchmarks (docs/traffic-smoothness.md): what runs on the
// connection's goroutine between two simulator frames. Everything there
// holds back every injected aircraft, so one frame's work must stay far
// below a frame (16.7 ms at 60 fps); a step of 100 ms is a visible jump
// of every moving aircraft. Run them with scripts/bench-smoothness.sh.

// busyApron is a ground picture of LKPR with n departures on their way
// to runway 24 from the first gates, every other one waiting for its taxi
// clearance with its whole route planned (the live case that held every
// frame back), the rest taxiing with OncomingLookMeters of intent; and a
// path for the aircraft looking (id 1) along its own route.
func busyApron(b *testing.B, n int) (*GroundPicture, *GroundPath, time.Time) {
	g := lkprGraph(&testing.T{})
	now := time.Unix(1_700_000_000, 0)
	p := NewGroundPicture()
	prof := DefaultMotionProfile()
	var mine *GroundPath
	id := uint32(1)
	for stand := 0; stand < len(g.Layout.Parking) && int(id) <= n+1; stand++ {
		if !g.Layout.Parking[stand].IsGate() {
			continue
		}
		r, err := g.RouteToRunway(stand, "24", airport.RouteOptions{})
		if err != nil || len(r.Points) < 3 {
			continue
		}
		pts := r.Points
		if id == 1 {
			path, err := NewGroundPath(pts, prof)
			if err != nil {
				b.Fatal(err)
			}
			mine = path
		} else {
			p.Report(id, pts[1], localBearing(pts[1], pts[2]), prof, now)
			if id%2 == 0 {
				p.ReportPlanned(id, pts[1:], DefaultHalfSpanMeters)
			} else {
				p.ReportPath(id, pts[1:], DefaultHalfSpanMeters)
				p.ReportIntent(id, wayAhead(pts[1:], OncomingLookMeters))
			}
		}
		id++
	}
	if mine == nil || int(id) <= n+1 {
		b.Fatalf("only %d routes at LKPR", id-1)
	}
	return p, mine, now
}

// BenchmarkGroundChecksFrame is one taxiing aircraft's traffic checks for
// one frame (blocking and head-on) among 20 others on the apron.
func BenchmarkGroundChecksFrame(b *testing.B) {
	p, path, now := busyApron(b, 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.blocking(1, path, 0, 200, DefaultHalfSpanMeters, now)
		p.oncoming(1, path, 0, OncomingLookMeters, DefaultHalfSpanMeters, now)
	}
}

// BenchmarkSpawnArrivalPlan is an arrival's plan at its spawn: the exit,
// the taxi-in and the stand stop (PlanArrival), LKPR runway 24.
func BenchmarkSpawnArrivalPlan(b *testing.B) {
	g := lkprGraph(&testing.T{})
	stand := firstGate(g)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := PlanArrival(g, "24", stand, ArrivalOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSpawnDepartureRoute is a departure's taxi route at its spawn,
// stand to runway 24 at LKPR.
func BenchmarkSpawnDepartureRoute(b *testing.B) {
	g := lkprGraph(&testing.T{})
	stand := firstGate(g)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := g.RouteToRunway(stand, "24", airport.RouteOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func firstGate(g *airport.Graph) int {
	for i, p := range g.Layout.Parking {
		if p.IsGate() {
			return i
		}
	}
	return 0
}

// BenchmarkSpawnDepartureStart is a departure controller's start as the
// airport map spawns one (C22, runway 24, its push planned in the
// background): all of it runs on the connection's goroutine.
func BenchmarkSpawnDepartureStart(b *testing.B) {
	g := lkprGraph(&testing.T{})
	c22, _ := g.Layout.ParkingIndex("C22")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)), TaxiWithPushInBackground())
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: "FSLTL A320 Air France SL", Tail: "CSA8"}); err != nil {
			b.Fatal(err)
		}
		ctl.Cancel()
	}
}

// BenchmarkSpawnArrivalStart is an arrival controller's start (injected
// approach, runway 24 to C22), on the connection's goroutine too.
func BenchmarkSpawnArrivalStart(b *testing.B) {
	g := lkprGraph(&testing.T{})
	c22, _ := g.Layout.ParkingIndex("C22")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ec := &eventClient{}
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8", InjectApproach: true}); err != nil {
			b.Fatal(err)
		}
		go func() {
			for range ctl.Events() {
			}
		}()
	}
}

// BenchmarkPushPlan is one departure's pushback planning (C22, runway 24):
// in the background, but its CPU and garbage are the whole map's.
func BenchmarkPushPlan(b *testing.B) {
	g := lkprGraph(&testing.T{})
	c22, _ := g.Layout.ParkingIndex("C22")
	ec := &eventClient{}
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: "FSLTL A320 Air France SL", Tail: "CSA8"}); err != nil {
		b.Fatal(err)
	}
	route := ctl.route
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctl.route = route
		ctl.planPushback()
	}
}

// BenchmarkVehicleRoute is one ground vehicle's way (a tug from its depot
// to a gate), planned when a departure's services start and when its tug
// drives home: on the connection's goroutine.
func BenchmarkVehicleRoute(b *testing.B) {
	g := lkprGraph(&testing.T{})
	depots := g.Layout.VehicleDepots()
	if len(depots) == 0 {
		b.Skip("no depots")
	}
	depot := g.Layout.Parking[depots[0]].Position
	c22, _ := g.Layout.ParkingIndex("C22")
	stand := g.Layout.Parking[c22].Position
	g.Layout.VehicleRoute(depot, stand) // the vehicle network built once
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := g.Layout.VehicleRoute(depot, stand); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVehicleGates is the gates along a ground vehicle's way (a tug
// from its depot to a gate), worked out when it gets a new path: in the
// aircraft's frame.
func BenchmarkVehicleGates(b *testing.B) {
	g := lkprGraph(&testing.T{})
	depots := g.Layout.VehicleDepots()
	if len(depots) == 0 {
		b.Skip("no depots")
	}
	c22, _ := g.Layout.ParkingIndex("C22")
	pts, err := g.Layout.VehicleRoute(g.Layout.Parking[depots[0]].Position, g.Layout.Parking[c22].Position)
	if err != nil {
		b.Fatal(err)
	}
	path, err := NewGroundPath(pts, DefaultMotionProfile())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vehicleGates(g.Layout, path)
	}
	b.ReportMetric(path.Length(), "m")
}
