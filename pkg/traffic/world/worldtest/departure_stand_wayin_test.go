package worldtest

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// wayInFieldDir writes XAPR, a made-up field (#1641): runway 09/27 of
// 2400 m, no taxiway beside it. An A320 landing 09 makes the turning loop
// at the far end and backtracks to C, the one way to the apron, and in
// along the apron's lane. Stand [0] stands with its tail on that lane (the
// departure's at TFFJ stood so); the arrival's stand [1] and [2] next to it
// lie beyond, out of the way in.
//
//	        [0]          [1] [2]
//	   ----+-+-----------+---+----  lane
//	       |
//	       C (hold)                         loop
//	09 ====+==============================+===+= 27
func wayInFieldDir(t *testing.T) string {
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
	west, c, out, back, east := pt(-1200, 0, 1), pt(-820, 0, 1), pt(1000, 0, 1), pt(1190, 0, 1), pt(1200, 0, 1)
	link(west, c, 2, 0, 45, 9)
	link(c, out, 2, 0, 45, 9)
	link(out, back, 2, 0, 45, 9)
	link(back, east, 2, 0, 45, 9)
	a, b := pt(1070, 70, 1), pt(1150, 70, 1)
	link(out, a, 1, 3, 23, 0)
	link(a, b, 1, 3, 23, 0)
	link(b, back, 1, 3, 23, 0)

	hold := pt(-830, 75, 2)
	link(c, hold, 1, 2, 23, 0)
	in := pt(-830, 220, 1)
	link(hold, in, 1, 2, 23, 0)

	var parking []airport.RawParking
	last := in
	for _, s := range []struct{ x, z float32 }{{-700, 245}, {-540, 290}, {-490, 290}} {
		at := pt(s.x, 220, 1)
		link(last, at, 1, 1, 23, 0)
		parking = append(parking, airport.RawParking{Name: 1, Number: uint32(len(parking) + 1), Type: 9, Heading: 0, Radius: 25, BiasX: s.x, BiasZ: s.z})
		link(at, int32(len(parking)-1), 3, 0, 23, 0)
		last = at
	}
	link(last, pt(-440, 220, 1), 1, 1, 23, 0)

	raw := airport.RawAirport{ICAO: "XAPR", Name: "Way in", Latitude: 45, Longitude: 15, Altitude: 10,
		Runways: []airport.RawRunway{{Latitude: 45, Longitude: 15, Altitude: 10, Heading: 90, Length: 2400, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking: parking, TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "C", "E"},
		Frequencies: []airport.RawFrequency{{Type: 5, Hz: 121900000, Name: "WAY IN"}, {Type: 6, Hz: 118100000, Name: "WAY IN"}},
		Tower:       &airport.RawTower{Latitude: 45.0032, Longitude: 14.99, Altitude: 10}}
	j, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, raw.ICAO), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, raw.ICAO, "layout.json"), j, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDepartureStandOutOfWayIn (#1641): a departure spawned while an
// arrival is on approach to backtrack to the one way to the apron is not
// given a stand where it would stand in that arrival's way in, though that
// one ranks first by taxi-in (the stand beyond is next to the arrival's).
func TestDepartureStandOutOfWayIn(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	const title = "FSLTL A320 Air France SL"
	o := Options{DataDir: wayInFieldDir(t), ICAO: "XAPR", Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), title)}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, end, ok := h.Layout.RunwayEnd("09")
	if !ok {
		t.Fatal("no runway 09")
	}
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 100, 25, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway("XAPR", "09")
	h.Advance(2 * time.Second)
	spawn := func(kind, tail string, stand int) {
		t.Helper()
		r := world.SpawnRequest{Kind: kind, ICAO: "XAPR", Tail: tail, Runway: "09", Stand: stand, Model: title, InjectApproach: kind == "arrival"}
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	spawn("arrival", "MTA1", 1)
	h.Advance(2 * time.Second)
	if a, _ := h.Aircraft("MTA1"); a.State != "approaching" {
		t.Fatalf("arrival %q, want approaching", a.State)
	}
	spawn("departure", "MTD1", -1)
	h.Advance(2 * time.Second)
	d, _ := h.Aircraft("MTD1")
	if d.Stand == "" {
		t.Fatal("departure without a stand")
	}
	if in := h.Layout.Parking[0].Label(); d.Stand == in {
		t.Errorf("departure on %s, in the arrival's way in", in)
	}
}
