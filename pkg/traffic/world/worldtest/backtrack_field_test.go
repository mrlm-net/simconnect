package worldtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// meetField is where an arrival backtracking meets a departure and what
// both are: built is written by the test, its stands then given; else
// TFFJ's data, with stands assigned.
type meetField struct {
	icao, runway, title string
	built               bool
	stands              map[string]int // by kind
}

var meetTFFJ = meetField{icao: "TFFJ", runway: "10", title: "FSLTL_DHC6_WIA-Winair"}

var meetXBAK = meetField{icao: "XBAK", runway: "09", title: "FSLTL_DHC6_WIA-Winair", built: true,
	stands: map[string]int{"departure": 1, "arrival": 2}}

// standFor is where kind is spawned; -1: assigned.
func (f meetField) standFor(kind string) int {
	if i, ok := f.stands[kind]; ok {
		return i
	}
	return -1
}

// hasTFFJ skips the TFFJ case where its data is not present (the Linux
// director has LKPR's only); it is extra to the built field's.
func hasTFFJ(t *testing.T) string {
	t.Helper()
	d := dataDir(t)
	if m, _ := filepath.Glob(filepath.Join(d, "fields", "*", meetTFFJ.icao, "layout.json")); len(m) == 0 {
		t.Skip("no TFFJ data")
	}
	return d
}

// backtrackFieldDir writes XBAK, a made-up short field (#1640): runway
// 09/27 of 700 m, no taxiway beside it and two ways off, C to the apron
// north of it and D to the one south, not joined. Both lean back from 09,
// so landing 09 neither is an exit: an arrival rolls to the end, turns
// round and backtracks. To a north stand it leaves by C, where a departure
// from there holds short, its only way on. A way clear of that departure
// is by D, to a south stand.
//
//	      [0] [1] [2]
//	   ----+---+---+---  north apron
//	         |
//	         C  (hold)
//	09 ======+=======+============== 27
//	                 D  (hold)
//	                 |
//	   -----+---+---+---  south apron
//	       [3] [4] [5]
func backtrackFieldDir(t *testing.T) string {
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
	west, c, d, east := pt(-350, 0, 1), pt(-60, 0, 1), pt(100, 0, 1), pt(350, 0, 1)
	link(west, c, 2, 0, 20, 9)
	link(c, d, 2, 0, 20, 9)
	link(d, east, 2, 0, 20, 9)

	var parking []airport.RawParking
	apron := func(exit int32, name uint32, x, side float32, standX [3]float32, heading float32) {
		h := pt(x, side*50, 2) // holding point, leaning back from 09
		link(exit, h, 1, name, 16, 0)
		in := pt(x, side*150, 1)
		link(h, in, 1, name, 16, 0)
		last := in
		for _, sx := range standX {
			at := pt(sx, side*150, 1)
			link(last, at, 1, 1, 16, 0)
			parking = append(parking, airport.RawParking{Name: 1, Number: uint32(len(parking) + 1), Type: 3, Heading: heading, Radius: 13, BiasX: sx, BiasZ: side * 185})
			link(at, int32(len(parking)-1), 3, 0, 16, 0)
			last = at
		}
		link(last, pt(standX[2]+60, side*150, 1), 1, 1, 16, 0)
	}
	apron(c, 2, -70, 1, [3]float32{-30, 10, 50}, 0)
	apron(d, 3, 90, -1, [3]float32{130, 170, 210}, 180)

	raw := airport.RawAirport{ICAO: meetXBAK.icao, Name: "Backtrack", Latitude: 45, Longitude: 15, Altitude: 10,
		Runways: []airport.RawRunway{{Latitude: 45, Longitude: 15, Altitude: 10, Heading: 90, Length: 700, Width: 20, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking: parking, TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "C", "D"},
		Frequencies: []airport.RawFrequency{{Type: 5, Hz: 121900000, Name: "BACKTRACK"}, {Type: 6, Hz: 118100000, Name: "BACKTRACK"}},
		Tower:       &airport.RawTower{Latitude: 45.0022, Longitude: 15.001, Altitude: 10}}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, raw.ICAO), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, raw.ICAO, "layout.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
