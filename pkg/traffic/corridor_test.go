package traffic

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The corridor's aircraft around a flight east at FL360: ahead the same
// way 2000 ft off, the other way 1000 ft off coming at it, one crossing
// ahead (#740).
func TestCorridorRoutes(t *testing.T) {
	route := []airport.LatLon{{Lat: 50, Lon: 10}, {Lat: 50, Lon: 12}, {Lat: 50.5, Lon: 14}, {Lat: 50.5, Lon: 16}}
	o := CorridorOptions{Route: route, At: airport.LatLon{Lat: 50, Lon: 10.5}, LevelFt: 36000, Kts: 450}
	rng := rand.New(rand.NewPCG(1, 2))
	dist := func(p airport.LatLon) float64 { return calc.HaversineNM(o.At.Lat, o.At.Lon, p.Lat, p.Lon) }
	for range 20 {
		same, ok := CorridorRoute(CorridorSame, o, rng)
		if !ok || math.Abs(math.Abs(same[0].AltFt-36000)-2000) > 1 || dist(same[0].Position) < CorridorAheadNM-1 || dist(same[0].Position) > CorridorAheadNM+21 {
			t.Fatalf("same %+v", same[0])
		}
		if dist(same[1].Position) <= dist(same[0].Position) {
			t.Errorf("same: not going the user's way")
		}
		opp, ok := CorridorRoute(CorridorOpposite, o, rng)
		if !ok || math.Abs(math.Abs(opp[0].AltFt-36000)-1000) > 1 || dist(opp[0].Position) < 60 {
			t.Fatalf("opposite %+v", opp[0])
		}
		if dist(opp[1].Position) >= dist(opp[0].Position) {
			t.Errorf("opposite: not coming at the user")
		}
		x, ok := CorridorRoute(CorridorCrossing, o, rng)
		if !ok || len(x) != 2 || x[0].AltFt == 36000 {
			t.Fatalf("crossing %+v", x)
		}
		if d := calc.HaversineNM(x[0].Position.Lat, x[0].Position.Lon, x[1].Position.Lat, x[1].Position.Lon); math.Abs(d-CorridorCrossInNM) > 1 {
			t.Errorf("crossing starts %.1f NM from the route", d)
		}
	}
	// Near the end of the route: no room ahead.
	if _, ok := CorridorRoute(CorridorSame, CorridorOptions{Route: route, At: route[3], LevelFt: 36000}, rng); ok {
		t.Error("same way past the end of the route")
	}
}
