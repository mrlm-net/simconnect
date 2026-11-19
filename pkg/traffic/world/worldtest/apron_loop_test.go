package worldtest

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// apronLoopDir writes an airport like the VECTOR.PRO builder's sample
// (XPLY, #1574): runway 09/27, a parallel taxiway A, and a single-lane
// apron loop L off A past six nose-in stands C1–C6 (west to east), all
// pushing back onto the loop. Every arrival to a stand and every
// departure from one uses the loop, both ways.
func apronLoopDir(t *testing.T) string {
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
	rwy := map[float32]int32{}
	rx := []float32{-1250, -800, -450, 0, 350, 800, 1250}
	for i, x := range rx {
		rwy[x] = pt(x, 0, 1)
		if i > 0 {
			link(rwy[rx[i-1]], rwy[x], 2, 0, 45, 9)
		}
	}
	a := map[float32]int32{}
	ax := []float32{-1250, -1100, -900, -500, -450, 0, 350, 800, 1250}
	for i, x := range ax {
		a[x] = pt(x, 180, 1)
		if i > 0 {
			link(a[ax[i-1]], a[x], 1, 1, 23, 0)
		}
	}
	for i, x := range []float32{-1250, -450, 350, 1250} {
		h := pt(x, 75, 2) // hold short
		link(rwy[x], h, 1, uint32(3+i), 23, 0)
		link(h, a[x], 1, uint32(3+i), 23, 0)
	}
	l := map[float32]int32{}
	lx := []float32{-1100, -1050, -950, -850, -750, -650, -550, -500}
	for i, x := range lx {
		l[x] = pt(x, 330, 1)
		if i > 0 {
			link(l[lx[i-1]], l[x], 1, 2, 20, 0)
		}
	}
	w, e := pt(-1100, 255, 1), pt(-500, 255, 1)
	link(a[-1100], w, 1, 2, 20, 0)
	link(w, l[-1100], 1, 2, 20, 0)
	link(a[-500], e, 1, 2, 20, 0)
	link(e, l[-500], 1, 2, 20, 0)
	var parking []airport.RawParking
	for i, x := range []float32{-1050, -950, -850, -750, -650, -550} {
		parking = append(parking, airport.RawParking{Name: 14, Number: uint32(i + 1), Type: 9, Heading: 0, Radius: 22, BiasX: x, BiasZ: 375})
		link(l[x], int32(i), 3, 0, 20, 0)
	}
	raw := airport.RawAirport{ICAO: "XPLY", Name: "Sample", Latitude: 50, Longitude: 15, Altitude: 300,
		Runways: []airport.RawRunway{{Latitude: 50, Longitude: 15, Altitude: 300, Heading: 90, Length: 2500, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking: parking, TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "L", "B", "C", "D", "E"},
		Frequencies: []airport.RawFrequency{{Type: 5, Hz: 121900000, Name: "SAMPLE"}, {Type: 6, Hz: 118100000, Name: "SAMPLE"}},
		Tower:       &airport.RawTower{Latitude: 50.0025, Longitude: 15.004, Altitude: 300}}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "XPLY"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "XPLY", "layout.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// loopRun lands arr on runway 27 to arrStand (at its exit, nil: its
// own), spawns dep on depStand to push pushIn minutes after arr lands,
// and runs until both are through: parked and departed, never both
// standing still within 150 m of each other for 3 minutes (a stand-off).
func loopRun(t *testing.T, seed uint64, arrStand, depStand string, exit *int, pushIn float64) {
	t.Helper()
	o := Options{DataDir: apronLoopDir(t), ICAO: "XPLY", Manual: true, FrameRate: 10, Seed: seed}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	wx := nav.StaticWeather(270, 8, 9999, 100, 15, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway("XPLY", "27")
	h.Advance(2 * time.Second)
	stand := func(label string) int {
		for i, p := range h.Layout.Parking {
			if p.Label() == label {
				return i
			}
		}
		t.Fatalf("no stand %s", label)
		return -1
	}
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "arrival", ICAO: "XPLY", Stand: stand(arrStand), Runway: "27", Exit: exit,
		Model: "FSLTL A320 Air France SL", Tail: "AFR919", InjectApproach: true}); err != nil {
		t.Fatal(err)
	}
	dist := func(p, q airport.LatLon) float64 {
		return math.Hypot((p.Lat-q.Lat)*111195, (p.Lon-q.Lon)*111195*math.Cos(p.Lat*math.Pi/180))
	}
	spawned := false
	last := map[string]airport.LatLon{}
	still := map[string]time.Duration{}
	const dt = 2 * time.Second
	for range 40 * 30 {
		h.Advance(dt)
		arr, _ := h.Aircraft("AFR919")
		if arr.State == "landing" && !spawned {
			spawned = true
			if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "departure", ICAO: "XPLY", Stand: stand(depStand), Runway: "27",
				Model: "FSLTL_E190_KLM", Tail: "KLM1817", PushInMin: pushIn}); err != nil {
				t.Fatal(err)
			}
		}
		dep, _ := h.Aircraft("KLM1817")
		for _, v := range []world.ControlView{arr, dep} {
			if p, ok := last[v.Tail]; ok && dist(p, v.Position) < 0.5 {
				still[v.Tail] += dt
			} else {
				still[v.Tail] = 0
			}
			last[v.Tail] = v.Position
		}
		if arr.State == "taxiing" && dep.State == "taxiing" && dist(arr.Position, dep.Position) < 150 && still["AFR919"] > 3*time.Minute && still["KLM1817"] > 3*time.Minute {
			t.Fatalf("seed %d: stand-off, AFR919 and KLM1817 still %.0f m apart for 3 minutes", seed, dist(arr.Position, dep.Position))
		}
		if arr.State == "parked" && (dep.State == "departing" || dep.State == "complete") {
			return
		}
	}
	a, _ := h.Aircraft("AFR919")
	d, _ := h.Aircraft("KLM1817")
	t.Fatalf("seed %d: not both through in 40 minutes: AFR919 %s, KLM1817 %s", seed, a.State, d.State)
}

// TestApronLoopPushIntoArrival: the issue's order of events (#1574). AFR919
// vacates at C and taxis in along A and up the loop's east leg to C2;
// KLM1817 on C5 asks to push as it comes, its push and first taxi leg on
// AFR919's way ahead. Ground holds the push until AFR919 has passed (it
// was approved, KLM1817 pushed onto the loop facing AFR919, and the two
// stood nose to nose for good).
func TestApronLoopPushIntoArrival(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	exit := 2
	for seed := uint64(2); seed <= 4; seed++ {
		loopRun(t, seed, "C2", "C5", &exit, 1.5)
	}
}

// TestApronLoopTaxiIntoArrival: KLM1817 on C2 pushes onto the loop facing
// its west leg while AFR919 lands to the end of 27, to taxi in along A
// and up that leg to C3. KLM1817's taxi goes down the leg towards it:
// AFR919 holds on A short of the junction, clear of where KLM1817 turns
// onto A, and goes once it has (it stopped in the mouth of the leg, and
// both stood nose to nose in it for good).
func TestApronLoopTaxiIntoArrival(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	for seed := uint64(20); seed <= 22; seed++ {
		loopRun(t, seed, "C3", "C2", nil, 0.5)
	}
}
