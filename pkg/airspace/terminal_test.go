package airspace

import (
	"math"
	"testing"
)

// TestTerminalStack: the terminal airspace stacked on an airport's
// control zone, by the generic rule (#1486): LKPR to 16,500 ft, LZKZ to
// 13,500 ft; a point in it and one above it.
func TestTerminalStack(t *testing.T) {
	s := store(t, StoreSkipBroken(), StorePreferNewerSet())
	const top = 24500 * 0.3048
	for _, c := range []struct {
		icao     string
		p        Point
		groundFt float64
		layers   int
		ceilFt   float64
	}{{"LKPR", lkpr, 1247, 3, 16500}, {"LZKZ", Point{Lat: 48.6631, Lon: 21.2411}, 755, 3, 13500}} {
		stack, err := s.TerminalStack(c.p.Lat, c.p.Lon, c.groundFt*0.3048, top)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, sh := range stack {
			names = append(names, sh.Airspace.Name)
		}
		ceil := 0.0
		if len(stack) > 0 {
			ceil = stack[len(stack)-1].Airspace.Ceiling(c.groundFt*0.3048) / 0.3048
		}
		t.Logf("%s: %v to %.0f ft", c.icao, names, ceil)
		if len(stack) != c.layers || math.Abs(ceil-c.ceilFt) > 100 {
			t.Errorf("%s: %d layers to %.0f ft, want %d to %.0f ft", c.icao, len(stack), ceil, c.layers, c.ceilFt)
		}
		g := c.groundFt * 0.3048
		if !InStack(stack, c.p.Lat, c.p.Lon, 8000*0.3048, g, 0) {
			t.Errorf("%s: 8,000 ft over the field not in its terminal airspace", c.icao)
		}
		if InStack(stack, c.p.Lat, c.p.Lon, (c.ceilFt+1500)*0.3048, g, 1000*0.3048) {
			t.Errorf("%s: 1,500 ft over the top in its terminal airspace", c.icao)
		}
	}
}
