package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// virtualCrossing is a crossing of 200 NM from a start rotated by deg round
// a centre: 20 NM legs bending 2° each, at FL370 and 450 kt.
func virtualCrossing(deg float64) []RoutePoint {
	lat, lon := calc.DisplaceByHeading(50.1, 14.26, deg, 100*1852)
	trk := math.Mod(deg+180, 360)
	out := []RoutePoint{{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: 37000, Kts: 450}}
	for i := 0; i < 10; i++ {
		trk += 2
		lat, lon = calc.DisplaceByHeading(lat, lon, trk, 20*1852)
		out = append(out, RoutePoint{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: 37000, Kts: 450})
	}
	return out
}

// TestVirtualFlightContinues: a virtual flight split at any moment and
// flown on from its Pose along Ahead (as an aircraft created for it is)
// goes where the unsplit one goes, and both finish.
func TestVirtualFlightContinues(t *testing.T) {
	spawn, wps, err := EnrouteStart(virtualCrossing(30))
	if err != nil {
		t.Fatal(err)
	}
	whole := NewVirtualFlight(spawn, wps)
	for sec := 0; sec < 600; sec++ {
		whole.Step(1)
	}
	split := NewVirtualFlight(spawn, wps)
	for sec := 0; sec < 300; sec++ {
		split.Step(1)
	}
	if p := split.PoseAfter(300); calc.HaversineMeters(p.Position.Lat, p.Position.Lon, whole.Pose().Position.Lat, whole.Pose().Position.Lon) > 1 {
		t.Errorf("PoseAfter: %v, flown %v", p.Position, whole.Pose().Position)
	}
	on := NewVirtualFlight(split.Pose().InitPosition(), split.Ahead())
	for sec := 0; sec < 300; sec++ {
		on.Step(1)
	}
	a, b := whole.Pose(), on.Pose()
	if d := calc.HaversineMeters(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon); d > 50 || math.Abs(math.Remainder(a.Heading-b.Heading, 360)) > 1 {
		t.Errorf("flown on from its pose: %.1f m, %.2f° from the whole flight", d, math.Remainder(a.Heading-b.Heading, 360))
	}
	if math.Abs(a.AltFt-37000) > 1 || math.Abs(a.Kts-450) > 1 {
		t.Errorf("at %.0f ft %.0f kt, want FL370 at 450 kt", a.AltFt, a.Kts)
	}
	for sec := 0; sec < 3600 && !whole.Done(); sec++ {
		whole.Step(1)
	}
	if !whole.Done() || len(whole.Ahead()) != 0 {
		t.Errorf("not through its waypoints after an hour: %d ahead", len(whole.Ahead()))
	}
}

// BenchmarkVirtualTracks200 is one step of 200 virtual tracks (each
// second, as the World's tiers advance them): flown on a second, and each
// one's distance from a viewer (#1482: well under 0.5 ms).
func BenchmarkVirtualTracks200(b *testing.B) {
	var flights []*VirtualFlight
	for i := range 200 {
		spawn, wps, err := EnrouteStart(virtualCrossing(float64(i) * 1.8))
		if err != nil {
			b.Fatal(err)
		}
		flights = append(flights, NewVirtualFlight(spawn, wps))
	}
	viewer := airport.LatLon{Lat: 50.1, Lon: 14.26}
	near := 0
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, f := range flights {
			f.Step(1)
			if p := f.Pose().Position; calc.HaversineNM(viewer.Lat, viewer.Lon, p.Lat, p.Lon) < 40 {
				near++
			}
		}
		if n%1500 == 1499 { // through the crossing: from the start again
			b.StopTimer()
			for i, f := range flights {
				spawn, wps, _ := EnrouteStart(virtualCrossing(float64(i) * 1.8))
				*f = *NewVirtualFlight(spawn, wps)
			}
			b.StartTimer()
		}
	}
	_ = near
}
