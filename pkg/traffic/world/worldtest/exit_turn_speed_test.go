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

// TestExitTurnSpeed (#1586): an arrival slows to its exit's turn speed
// before it turns off. A right-angle exit is never entered faster than
// about 15 kt, and on the ground the written heading rate stays within
// what the speed allows at a lateral acceleration of about 0.15 g (with a
// margin for the trailing main gear): never a 90° snap at roll-out speed.
// Both directions at LKPR, an airliner and a heavy, and LKTB.
func TestExitTurnSpeed(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	d := dataDir(t)
	cases := []struct{ icao, rwy, model, wind string }{
		{"LKPR", "24", "A320", "24"},
		{"LKPR", "06", "A320", "06"},
		{"LKPR", "24", "B77W", "24"},
		{"LKPR", "06", "B77W", "06"},
		{"LKTB", "27", "A320", "27"},
		{"LKTB", "09", "E190", "09"},
		{"TNCM", "28", "E190", "28"},
		{"TNCM", "10", "E190", "10"},
	}
	for _, tc := range cases {
		t.Run(tc.icao+"_"+tc.rwy+"_"+tc.model, func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(d, tc.icao, "layout.json")); err != nil {
				t.Skipf("no %s data", tc.icao)
			}
			exitTurnRun(t, d, tc.icao, tc.rwy, tc.model, nil)
		})
	}
}

const (
	exitRightAngleKts = 15.0
	exitLateralLimit  = 0.15 * 9.81 * 1.5
	// exitStraightRate (°/s): slower heading changes are the roll-out
	// keeping the centreline, not a turn.
	exitStraightRate = 3.0
)

func exitTurnRun(t *testing.T, d, icao, rwy, model string, exit *int) {
	titles := append(slices.Clone(DefaultAircraftTitles), "FSLTL_FAIB_B77W_UAE-Emirates")
	o := Options{DataDir: d, ICAO: icao, Manual: true, FrameRate: 20, AircraftTitles: titles}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, end, ok := h.Layout.RunwayEnd(rwy)
	if !ok {
		t.Fatalf("no runway %s at %s", rwy, icao)
	}
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 100, 20, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway(icao, rwy)
	h.Advance(2 * time.Second)
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "arrival", ICAO: icao, Tail: "TEX1", Runway: rwy, Stand: -1, Model: model, Exit: exit, InjectApproach: true}); err != nil {
		t.Fatal(err)
	}
	if !h.Until(15*time.Minute, func() bool { v, _ := h.Aircraft("TEX1"); return v.State == "rollout" }) {
		t.Fatal("TEX1 never landed")
	}
	rwyHdg := end.Heading
	start := h.Clock.Now()
	type sample struct {
		at       time.Time
		hdg, kts float64
	}
	var prev *sample
	entered, worstLat, worstKts, worstRate := -1.0, 0.0, 0.0, 0.0
	worstAt, worstState, worstDev := 0.0, "", 0.0
	h.Until(5*time.Minute, func() bool {
		v, _ := h.Aircraft("TEX1")
		if v.State != "rollout" && v.State != "vacating" {
			return true
		}
		var o Object
		for _, x := range h.Sim.Objects() {
			if x.ATCID == "TEX1" {
				o = x
			}
		}
		if o.ID == 0 || !o.OnGround {
			return false
		}
		s := sample{h.Clock.Now(), o.Heading, math.Max(o.GroundKts, v.GroundSpeed)}
		dev := math.Abs(math.Mod(s.hdg-rwyHdg+540, 360) - 180)
		if dev >= 60 && entered < 0 {
			entered = s.kts
		}
		if prev != nil {
			dt := s.at.Sub(prev.at).Seconds()
			if dt > 0 {
				rate := math.Abs(math.Mod(s.hdg-prev.hdg+540, 360)-180) / dt
				ms := (s.kts + prev.kts) / 2 * 0.514444
				lat := ms * rate * math.Pi / 180
				if rate > exitStraightRate && lat > worstLat {
					worstLat, worstKts, worstRate = lat, (s.kts+prev.kts)/2, rate
					worstAt, worstState, worstDev = s.at.Sub(start).Seconds(), v.State, dev
				}
			}
		}
		prev = &s
		return false
	})
	occ := h.Clock.Now().Sub(start)
	t.Logf("%s %s %s: right-angle entry %.1f kt, worst lateral %.2f m/s² (%.0f°/s at %.0f kt, %s %.1f s in, %.0f° off the runway), rollout to vacating done %.0f s", icao, rwy, model, entered, worstLat, worstRate, worstKts, worstState, worstAt, worstDev, occ.Seconds())
	if entered > exitRightAngleKts {
		t.Errorf("entered a right-angle exit at %.1f kt, want ≤ %.0f", entered, exitRightAngleKts)
	}
	if worstLat > exitLateralLimit {
		t.Errorf("heading rate %.0f°/s at %.0f kt: %.2f m/s² lateral, want ≤ %.2f", worstRate, worstKts, worstLat, exitLateralLimit)
	}
}

// exitFieldDir writes a small field like the issue's generated one
// (#1586): runway 09/27 of 1900 m, a parallel taxiway A and right-angle
// exits only, the first just past the touchdown zone on 27; one stand off A.
func exitFieldDir(t *testing.T) string {
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
	xs := []float32{-950, -400, 50, 450, 950}
	rwy, a := map[float32]int32{}, map[float32]int32{}
	for i, x := range xs {
		rwy[x] = pt(x, 0, 1)
		a[x] = pt(x, 180, 1)
		if i > 0 {
			link(rwy[xs[i-1]], rwy[x], 2, 0, 45, 9)
			link(a[xs[i-1]], a[x], 1, 1, 23, 0)
		}
		h := pt(x, 75, 2)
		link(rwy[x], h, 1, uint32(2+i), 23, 0)
		link(h, a[x], 1, uint32(2+i), 23, 0)
	}
	apron := pt(-200, 180, 1)
	paths = slices.DeleteFunc(paths, func(p airport.RawTaxiPath) bool { return p.Start == a[-400] && p.End == a[50] })
	link(a[-400], apron, 1, 1, 23, 0)
	link(apron, a[50], 1, 1, 23, 0)
	lead := pt(-200, 260, 1)
	link(apron, lead, 1, 7, 20, 0)
	parking := []airport.RawParking{{Name: 14, Number: 1, Type: 9, Heading: 0, Radius: 22, BiasX: -200, BiasZ: 310}}
	link(lead, 0, 3, 0, 20, 0)
	raw := airport.RawAirport{ICAO: "XEXT", Name: "Exits", Latitude: 50, Longitude: 15, Altitude: 300,
		Runways: []airport.RawRunway{{Latitude: 50, Longitude: 15, Altitude: 300, Heading: 90, Length: 1900, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking: parking, TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "B", "C", "D", "E", "F", "G"},
		Frequencies: []airport.RawFrequency{{Type: 5, Hz: 121900000, Name: "EXITS"}, {Type: 6, Hz: 118100000, Name: "EXITS"}},
		Tower:       &airport.RawTower{Latitude: 50.0025, Longitude: 15.004, Altitude: 300}}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "XEXT"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "XEXT", "layout.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestExitTurnSpeedSmallField (#1586): the issue's field. An E190 and an
// A320 planned onto the first right-angle exit past the touchdown zone,
// too close to slow for, roll on to a later exit rather than turn off at
// roll-out speed; left to choose, they slow for theirs.
func TestExitTurnSpeedSmallField(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	d := exitFieldDir(t)
	b, err := os.ReadFile(filepath.Join(d, "XEXT", "layout.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	layout, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(layout)
	if err != nil {
		t.Fatal(err)
	}
	exits, err := g.RunwayExits("27")
	if err != nil {
		t.Fatal(err)
	}
	first := -1
	for i, x := range exits {
		t.Logf("27 exit %d: %s at %.0f m, %.0f°", i, x.Taxiway, x.Along, x.Angle)
		if first < 0 && x.Along > 300 {
			first = i
		}
	}
	if first < 0 {
		t.Fatal("no exit past the touchdown zone")
	}
	for _, model := range []string{"E190", "A320"} {
		t.Run(model+"_first", func(t *testing.T) { exitTurnRun(t, d, "XEXT", "27", model, &first) })
		t.Run(model+"_own", func(t *testing.T) { exitTurnRun(t, d, "XEXT", "27", model, nil) })
	}
}
