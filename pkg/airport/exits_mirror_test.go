package airport

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// mirrorGraph is runway 09/27, 2000 m, with two stubs 60 m long: one at
// 88° from 09 at 500 m west of the middle, its mirror at 88° from 27 at
// 500 m east (#1686).
func mirrorGraph(t *testing.T) *Graph {
	t.Helper()
	raw := RawAirport{ICAO: "ZZZZ", Latitude: 50, Longitude: 14,
		Runways: []RawRunway{{Latitude: 50, Longitude: 14, Heading: 90, Length: 2000, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}}}
	point := func(x, z float64) int32 {
		raw.TaxiPoints = append(raw.TaxiPoints, RawTaxiPoint{Type: int32(types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE_NORMAL), BiasX: float32(x), BiasZ: float32(z)})
		return int32(len(raw.TaxiPoints) - 1)
	}
	path := func(kind types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE, a, b int32) {
		p := RawTaxiPath{Type: int32(kind), Width: 20, Start: a, End: b}
		if kind == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY {
			p.RunwayNumber, p.Width = 9, 45
		}
		raw.TaxiPaths = append(raw.TaxiPaths, p)
	}
	var line []int32
	for _, x := range []float64{-1000, -500, 0, 500, 1000} {
		line = append(line, point(x, 0))
	}
	for i := 1; i < len(line); i++ {
		path(types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY, line[i-1], line[i])
	}
	stub := func(from int32, x, brg float64) {
		h := brg * math.Pi / 180
		path(types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI, from, point(x+math.Sin(h)*60, math.Cos(h)*60))
	}
	stub(line[1], -500, 2)  // 88° left of 09, 92° right of 27
	stub(line[3], 500, 358) // 92° left of 09, 88° right of 27
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestExitsMirroredRunway: a right-angle stub drawn a degree or two off is
// an exit from both runway ends, the one past a right angle marked by its
// Angle (#1686: KEGE 25 had none of 07's A1-A6).
func TestExitsMirroredRunway(t *testing.T) {
	g := mirrorGraph(t)
	nodes := map[string]map[NodeID]float64{}
	for _, end := range []string{"09", "27"} {
		xs, err := g.RunwayExits(end)
		if err != nil {
			t.Fatal(err)
		}
		nodes[end] = map[NodeID]float64{}
		for _, x := range xs {
			nodes[end][x.Node] = x.Angle
			if x.Angle > 95 {
				t.Errorf("%s: exit at %.1f°, not a stub", end, x.Angle)
			}
			_, e, _ := g.Layout.RunwayEnd(end)
			if why := g.ExitTight(x, e.Heading, RouteOptions{}); why != "" {
				t.Errorf("%s: exit at %.1f° tight: %s", end, x.Angle, why)
			}
		}
		if len(xs) != 2 {
			t.Errorf("%s: %d exits %v, want both stubs", end, len(xs), nodes[end])
		}
	}
	for n, a := range nodes["09"] {
		b, ok := nodes["27"][n]
		if !ok {
			t.Errorf("stub %d at %.1f° from 09 is no exit from 27", n, a)
			continue
		}
		if math.Abs(a+b-180) > 0.5 {
			t.Errorf("stub %d: %.1f° and %.1f°, not mirrored", n, a, b)
		}
	}
}

// TestExitsKEGEBothWays: KEGE's right-angle stubs off 07 are exits from 25
// too (#1686); the counts per direction are logged. Skipped without the
// simconnect-data clone.
func TestExitsKEGEBothWays(t *testing.T) {
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "simconnect-data")
	}
	b, err := os.ReadFile(filepath.Join(dir, "KEGE", "layout.json"))
	if err != nil {
		t.Skip("no KEGE layout (SIMCONNECT_DATA)")
	}
	var raw RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	ends := map[string]map[NodeID]RunwayExit{}
	for _, end := range []string{"07", "25"} {
		xs, err := g.RunwayExits(end)
		if err != nil {
			t.Fatal(err)
		}
		ends[end] = map[NodeID]RunwayExit{}
		for _, x := range xs {
			ends[end][x.Node] = x
			t.Logf("%s: %-4s at %4.0f m, %5.1f°", end, x.Taxiway, x.Along, x.Angle)
		}
		t.Logf("%s: %d exits", end, len(xs))
	}
	for _, pair := range [][2]string{{"07", "25"}, {"25", "07"}} {
		for n, x := range ends[pair[0]] {
			if x.Angle < 85 {
				continue
			}
			if _, ok := ends[pair[1]][n]; !ok {
				t.Errorf("%s at %.1f° from %s is no exit from %s", x.Taxiway, x.Angle, pair[0], pair[1])
			}
		}
	}
}
