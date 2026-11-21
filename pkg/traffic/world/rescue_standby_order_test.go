package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// rescueSharedField is a field like the issue's (#1622): runway 09/27 of
// 1900 m, a parallel taxiway A 200 m south, one fire station with 2
// vehicles south of A, and two standby points given on one taxiway, one
// behind the other on the vehicles' way: the one nearer the station
// nearer the expected stop (first to be given out), the other 12 m short
// of the taxiway node nearest it, past it on the way.
func rescueSharedField(t *testing.T) *airport.Layout {
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
	xs := []float32{-950, 200, 950}
	var prevR, prevA int32
	var m, a0 int32
	for i, x := range xs {
		r := pt(x, 0, 1)
		h := pt(x, -90, 2)
		link(r, h, 1, uint32(2+i), 23, 0)
		ax := x
		if x == 200 {
			ax = 170
			m = pt(x, -130, 1)
			link(h, m, 1, uint32(2+i), 23, 0)
		}
		a := pt(ax, -200, 1)
		if x == 200 {
			a0 = a
			link(m, a, 1, uint32(2+i), 23, 0)
		} else {
			link(h, a, 1, uint32(2+i), 23, 0)
		}
		if i > 0 {
			link(prevR, r, 2, 0, 45, 9)
			link(prevA, a, 1, 1, 23, 0)
		}
		prevR, prevA = r, a
	}
	s := pt(170, -400, 1)
	link(a0, s, 1, 5, 15, 0)
	lead := pt(170, -520, 1)
	link(s, lead, 1, 5, 15, 0)
	link(lead, 0, 3, 0, 15, 0)
	raw := airport.RawAirport{ICAO: "XRSC", Name: "Rescue", Latitude: 50.12, Longitude: 13.81, Altitude: 300,
		Runways:    []airport.RawRunway{{Latitude: 50.12, Longitude: 13.81, Altitude: 300, Heading: 90, Length: 1900, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking:    []airport.RawParking{{Name: 14, Number: 1, Type: 13, Radius: 10, BiasX: 170, BiasZ: -600, RescueVehicles: 2}},
		TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "B", "C", "D", "E"}}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	mp, ap := l.TaxiPoints[m].Position, l.TaxiPoints[a0].Position
	seg := calc.HaversineMeters(mp.Lat, mp.Lon, ap.Lat, ap.Lon)
	at := func(d float64) airport.LatLon {
		f := d / seg
		return airport.LatLon{Lat: mp.Lat + (ap.Lat-mp.Lat)*f, Lon: mp.Lon + (ap.Lon-mp.Lon)*f}
	}
	l.RescueStandby = []airport.RescueStandbyPoint{{Position: at(47), Heading: 0}, {Position: at(12), Heading: 0}}
	return l
}

// offlineLayout is icao's layout from the simconnect-data clone
// ($SIMCONNECT_DATA, else beside this checkout); the test skips without it.
func offlineLayout(t *testing.T, icao string) *airport.Layout {
	t.Helper()
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "..", "simconnect-data")
	}
	b, err := os.ReadFile(filepath.Join(dir, icao, "layout.json"))
	if err != nil {
		t.Skipf("no offline %s at %s (SIMCONNECT_DATA)", icao, dir)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// TestRescueStandbyOrder (#1622): with one fire station sending 2 and
// standby points one behind the other on their way, the first to leave
// takes the farther point along it, so each reaches its own point before
// the touchdown, none stopping behind another short of it; each way ends
// at its point (no overshoot, no reversing) and its way ahead never runs
// empty before it. The #1597 checks still hold, landing and going around.
// The issue's kind of field, and LKPR from simconnect-data with its fire
// station nearest the arrival runway sending 2.
func TestRescueStandbyOrder(t *testing.T) {
	for _, goAround := range []bool{false, true} {
		t.Run("field", func(t *testing.T) { rescueLandingOn(t, "XRSC", rescueSharedField(t), goAround) })
	}
	t.Run("LKPR", func(t *testing.T) {
		l := offlineLayout(t, "LKPR")
		depots := l.VehicleDepots()
		if len(depots) == 0 {
			t.Skip("no vehicle depot")
		}
		for _, d := range depots {
			l.Parking[d].RescueVehicles = 0
		}
		g, err := airport.BuildGraph(l)
		if err != nil {
			t.Fatal(err)
		}
		cc, _, _ := rescueBed(t, l, rescueTestTitles)
		_, end, ok := l.RunwayEnd(cc.runwayFor(g, true, -1))
		if !ok {
			t.Fatal("no arrival runway")
		}
		near, best := -1, 1e18
		for _, d := range depots {
			p := l.Parking[d].Position
			if m := calc.HaversineMeters(p.Lat, p.Lon, end.Threshold.Lat, end.Threshold.Lon); m < best {
				near, best = d, m
			}
		}
		l.Parking[near].RescueVehicles = 2
		for _, goAround := range []bool{false, true} {
			rescueLandingOn(t, "LKPR", l, goAround)
		}
	})
}

// TestRescueAssignShared (#1622): RescueAssign gives the first of two
// vehicles from one station the point farther along their shared way,
// whichever order the points come in.
func TestRescueAssignShared(t *testing.T) {
	l := rescueSharedField(t)
	depot := l.Parking[0].Position
	near, far := l.RescueStandby[0].Position, l.RescueStandby[1].Position
	for _, spots := range [][]traffic.GroundPose{
		{{Position: near}, {Position: far}},
		{{Position: far}, {Position: near}},
	} {
		got := traffic.RescueAssign(l, []*airport.LatLon{&depot, &depot}, spots)
		if len(got) != 2 || got[0].Position != far || got[1].Position != near {
			t.Errorf("assigned %v, want the farther %v first", got, far)
		}
	}
	way, ok := traffic.RescueStandbyWay(l, depot, far)
	if !ok || way[len(way)-1] != far {
		t.Fatalf("way %v, want one ending at %v", way, far)
	}
	if turn := endTurn(way, 30); turn > 135 {
		t.Errorf("way %v turns %.0f° in its last 30 m: past its point and back", way, turn)
	}
}
