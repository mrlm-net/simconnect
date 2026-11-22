package worldtest

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// midExitFieldDir writes a field like the builder's sample (#1631): runway
// 09/27 of 2500 m, taxiway C leaving it at a right angle at mid-length, B
// and D at its ends, a parallel taxiway A and one stand off it. The end
// taxiways leave back towards the middle by back metres over their 75 m to
// the hold (10: about 98°, no exit for an aircraft landing towards them).
func midExitFieldDir(t *testing.T, back float32) string {
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
	xs := []float32{-1250, -625, 0, 625, 1250}
	exits := map[float32]uint32{-1250: 2, 0: 3, 1250: 4}
	rwy, a := map[float32]int32{}, map[float32]int32{}
	for i, x := range xs {
		rwy[x] = pt(x, 0, 1)
		if i > 0 {
			link(rwy[xs[i-1]], rwy[x], 2, 0, 45, 9)
		}
		name, ok := exits[x]
		if !ok {
			continue
		}
		hx := x - float32(math.Copysign(float64(back), float64(x)))
		if x == 0 {
			hx = 0
		}
		a[x] = pt(hx, 180, 1)
		h := pt(hx, 75, 2)
		link(rwy[x], h, 1, name, 23, 0)
		link(h, a[x], 1, name, 23, 0)
	}
	apron := pt(300, 180, 1)
	link(a[-1250], a[0], 1, 1, 23, 0)
	link(a[0], apron, 1, 1, 23, 0)
	link(apron, a[1250], 1, 1, 23, 0)
	lead := pt(300, 260, 1)
	link(apron, lead, 1, 5, 20, 0)
	parking := []airport.RawParking{{Name: 14, Number: 1, Type: 9, Heading: 0, Radius: 24, BiasX: 300, BiasZ: 310}}
	link(lead, 0, 3, 0, 20, 0)
	raw := airport.RawAirport{ICAO: "XMID", Name: "Mid exit", Latitude: 50.1214, Longitude: 13.8124, Altitude: 300,
		Runways: []airport.RawRunway{{Latitude: 50.1214, Longitude: 13.8124, Altitude: 300, Heading: 90, Length: 2500, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking: parking, TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "B", "C", "D", "S"},
		Frequencies: []airport.RawFrequency{{Type: 5, Hz: 121900000, Name: "MID"}, {Type: 6, Hz: 118100000, Name: "MID"}},
		Tower:       &airport.RawTower{Latitude: 50.1239, Longitude: 13.8164, Altitude: 300}}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "XMID"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "XMID", "layout.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestExitMidLength (#1631): on a runway whose one exit short of the end is
// a right-angle one at mid-length, out of reach from the touchdown, an
// arrival, normal or a MAYDAY, never turns off there at roll-out speed. With
// B at the end a right angle too it rolls on to B; with the end taxiways
// leaving back past a right angle (no exit for the landing direction) it
// rolls to the end, turns round and comes back to C. Before, it took C at
// 100 kt, its heading written round at 150°/s.
func TestExitMidLength(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	for _, back := range []float32{0, 10} {
		d := midExitFieldDir(t, back)
		for _, model := range []string{"A321", "A20N"} {
			name := fmt.Sprintf("back%.0f_%s", back, model)
			t.Run(name, func(t *testing.T) { exitTurnRunWith(t, d, "XMID", "27", model, nil, "") })
			t.Run(name+"_mayday", func(t *testing.T) { exitTurnRunWith(t, d, "XMID", "27", model, nil, "mayday") })
		}
	}
}
