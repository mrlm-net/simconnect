package airport

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// offlineCharts are icao's charts from the simconnect-data clone
// ($SIMCONNECT_DATA, else beside this checkout); the test skips without it.
func offlineCharts(t *testing.T, icao string) ChartSet {
	t.Helper()
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "simconnect-data")
	}
	dir = filepath.Join(dir, icao)
	if _, err := os.Stat(filepath.Join(dir, "procedures.json")); err != nil {
		t.Skipf("no offline %s at %s (SIMCONNECT_DATA)", icao, dir)
	}
	var raw RawAirport
	var p Procedures
	for f, v := range map[string]any{"layout.json": &raw, "procedures.json": &p} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatal(err)
		}
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return Charts(l, p)
}

// pathLoop is where a path closes a loop: a point back within 0.3 NM of
// one more than 1.5 NM earlier along it, or two segments that cross. The
// loops of #1471 bent ~3.6° a step, so a per-step reversal test misses them.
func pathLoop(pts []LatLon) (int, int, bool) {
	const nm = 1852.0
	along := make([]float64, len(pts))
	for i := 1; i < len(pts); i++ {
		along[i] = along[i-1] + calc.HaversineMeters(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
	}
	for i := range pts {
		for j := i + 2; j < len(pts); j++ {
			if along[j]-along[i] > 1.5*nm && calc.HaversineMeters(pts[i].Lat, pts[i].Lon, pts[j].Lat, pts[j].Lon) < 0.3*nm {
				return i, j, true
			}
		}
	}
	if len(pts) == 0 {
		return 0, 0, false
	}
	cosLat := math.Cos(pts[0].Lat * math.Pi / 180)
	xy := func(p LatLon) (float64, float64) { return p.Lon * cosLat * 111320, p.Lat * 111320 }
	side := func(ax, ay, bx, by, cx, cy float64) float64 { return (bx-ax)*(cy-ay) - (by-ay)*(cx-ax) }
	for i := 0; i+1 < len(pts); i++ {
		ax, ay := xy(pts[i])
		bx, by := xy(pts[i+1])
		for j := i + 2; j+1 < len(pts); j++ {
			cx, cy := xy(pts[j])
			dx, dy := xy(pts[j+1])
			if side(ax, ay, bx, by, cx, cy)*side(ax, ay, bx, by, dx, dy) < 0 &&
				side(cx, cy, dx, dy, ax, ay)*side(cx, cy, dx, dy, bx, by) < 0 {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

// TestChartsNoLoops (#1471): EIDW's SIDs off 28R and 16 drew a teardrop
// the procedures never fly. 28R: the DF to DE28R, 1.9 m south of the
// runway end, took that bearing (182°) as the heading, so the CA 277 began
// with a turn through south, and the CF 308 to DW128, inside a 2.5 km turn
// from the CA's end, was joined by a Dubins orbiting round. 16: the DF to
// OP was a Dubins arriving on the first bearing, past OP and back. No path
// at EIDW or EGKK may close a loop.
func TestChartsNoLoops(t *testing.T) {
	for _, icao := range []string{"EIDW", "EGKK"} {
		c := offlineCharts(t, icao)
		n := 0
		for _, procs := range [][]ChartProc{c.SIDs, c.STARs, c.Approaches} {
			for _, s := range procs {
				for _, p := range append(slices.Clone(s.Paths), s.Missed...) {
					n++
					if i, j, ok := pathLoop(p.Points); ok {
						t.Errorf("%s %s: a loop from point %d to %d of %d", icao, p.Label, i, j, len(p.Points))
					}
				}
			}
		}
		if n == 0 {
			t.Errorf("%s: no paths", icao)
		}
	}
}
