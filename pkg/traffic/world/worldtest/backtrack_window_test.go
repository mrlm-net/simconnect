package worldtest

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// backtrackWindowNM: the player clears the backtracking departure with the
// arrival this close and farther than backtrackWindowFromNM: inside the
// landing clearance's 6 NM, outside the 2 NM a line-up at the threshold
// is held for.
const (
	backtrackWindowNM     = 4.5
	backtrackWindowFromNM = 2.5
)

// backtrackWindowField is XBKW, a made-up short field: runway 09/27 of
// 700 m with no way on near 09's threshold, so a departure on 09 enters at
// D, 450 m in, and backtracks. D leans back from 09 to the south apron; E,
// 50 m short of the far end, leads to the north apron and is the way off
// for an arrival on 09. The aprons are not joined.
//
//	                        [3] [4] [5]
//	   ---------------+---+---+---  north apron
//	                         E (hold)
//	09 ==========+===========+== 27
//	             D  (hold)
//	             |
//	   -----+---+---+---  south apron
//	       [0] [1] [2]
var backtrackWindowField = meetField{icao: "XBKW", runway: "09", title: meetXBAK.title, built: true,
	stands: map[string]int{"departure": 1, "arrival": 4}}

// backtrackWindowDir writes backtrackWindowField's layout and returns its
// data dir.
func backtrackWindowDir(t *testing.T) string {
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
	west, d, e, east := pt(-350, 0, 1), pt(100, 0, 1), pt(300, 0, 1), pt(350, 0, 1)
	link(west, d, 2, 0, 20, 9)
	link(d, e, 2, 0, 20, 9)
	link(e, east, 2, 0, 20, 9)
	var parking []airport.RawParking
	apron := func(exit int32, name uint32, holdX, inX, side float32, standX [3]float32, heading float32) {
		h := pt(holdX, side*50, 2)
		link(exit, h, 1, name, 16, 0)
		in := pt(inX, side*150, 1)
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
	apron(d, 2, 90, 90, -1, [3]float32{130, 170, 210}, 180)
	apron(e, 3, 320, 330, 1, [3]float32{200, 160, 120}, 0)
	raw := airport.RawAirport{ICAO: backtrackWindowField.icao, Name: "Backtrack window", Latitude: 45, Longitude: 15, Altitude: 10,
		Runways: []airport.RawRunway{{Latitude: 45, Longitude: 15, Altitude: 10, Heading: 90, Length: 700, Width: 20, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking: parking, TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "D", "E"},
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

// TestBacktrackHeldInLandingWindow (#1680): at XBKW a departure under the
// player's control holds short at D, the way in from which it backtracks
// along 09 towards the arrivals, as one of ours comes in on its final.
// With that arrival established inside 6 NM, cleared to land yet or not,
// the player's "line up" is answered "hold position" and the departure
// stays holding short until the arrival is down and off the runway; then
// the player's line-up is taken. Before, it entered and backtracked
// towards the arrival on its landing path.
func TestBacktrackHeldInLandingWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	at := backtrackWindowField
	title := at.title
	log := &lockedBuf{}
	o := Options{DataDir: backtrackWindowDir(t), ICAO: at.icao, Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), title)}
	o.World.Output = log
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, end, ok := h.Layout.RunwayEnd(at.runway)
	if !ok {
		t.Fatalf("no runway %s at %s", at.runway, at.icao)
	}
	thr := end.LandingThreshold()
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 100, 25, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway(at.icao, at.runway)
	h.Advance(2 * time.Second)
	const dep, arr = "MTD2", "MTA2"
	spawn := func(r world.SpawnRequest) {
		t.Helper()
		r.ICAO, r.Runway, r.Model = at.icao, at.runway, title
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			t.Fatalf("%s: %v", r.Tail, err)
		}
	}
	give := func(tail, action string) {
		t.Helper()
		v, ok := h.Aircraft(tail)
		if !ok {
			t.Fatalf("%s: no %s", action, tail)
		}
		if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/%s", v.ID, action), nil); err != nil {
			t.Fatalf("%s %s: %v", tail, action, err)
		}
	}
	// The departure, from the south apron by D: the player takes it over
	// as it taxis (its taxi clearance in force), so no tower clears it on.
	spawn(world.SpawnRequest{Kind: "departure", Tail: dep, Stand: at.standFor("departure")})
	if !h.Until(15*time.Minute, func() bool {
		v, ok := h.Aircraft(dep)
		return ok && v.State == "taxiing"
	}) {
		t.Fatal("the departure never taxied")
	}
	give(dep, "taxi")
	if !h.Until(10*time.Minute, func() bool {
		v, ok := h.Aircraft(dep)
		return ok && v.State == "holding short"
	}) {
		t.Fatal("the departure never held short")
	}
	spawn(world.SpawnRequest{Kind: "arrival", Tail: arr, Stand: at.standFor("arrival"), InjectApproach: true})
	nm := func(v world.ControlView) float64 {
		return calc.HaversineNM(v.Position.Lat, v.Position.Lon, thr.Lat, thr.Lon)
	}
	// The arrival inside the landing window, outside the line-up's 2 NM.
	if !h.Until(10*time.Minute, func() bool {
		a, ok := h.Aircraft(arr)
		d := nm(a)
		return ok && !a.OnGround && a.ObjectID != 0 && d <= backtrackWindowNM && d > backtrackWindowFromNM
	}) {
		a, _ := h.Aircraft(arr)
		t.Fatalf("nothing checked: the arrival never came within %.1f–%.1f NM (%s, %.1f NM)", backtrackWindowFromNM, backtrackWindowNM, a.State, nm(a))
	}
	a, _ := h.Aircraft(arr)
	t.Logf("%s cleared to line up with %s %.1f NM out", dep, arr, nm(a))
	give(dep, "lineup")
	var entered []string
	off := h.Until(10*time.Minute, func() bool {
		a, aok := h.Aircraft(arr)
		d, _ := h.Aircraft(dep)
		offRunway := !aok || slices.Contains([]string{"awaiting taxi", "taxiing", "parking", "parked"}, a.State)
		if d.State != "holding short" && len(entered) < 5 {
			entered = append(entered, fmt.Sprintf("%s %s with %s %s %.1f NM", dep, d.State, arr, a.State, nm(a)))
		}
		return offRunway || h.Said(arr, "going around")
	})
	text := log.String()
	if !strings.Contains(text, "lineup: "+arr+" landing or on the runway, hold position") {
		t.Errorf("the player's line-up was not held with %s inside the landing window", arr)
	}
	if !h.Said(dep, "hold position") {
		t.Errorf("%s was not told to hold position", dep)
	}
	if len(entered) > 0 {
		t.Errorf("entered to backtrack with the arrival inside the landing window: %q", entered)
	}
	if !off {
		t.Fatalf("the arrival neither got off the runway nor went around")
	}
	if h.Said(arr, "going around") {
		t.Errorf("%s went around: the runway was not kept free for it", arr)
	}
	// The runway free: the player's line-up is taken.
	give(dep, "lineup")
	if !h.Until(3*time.Minute, func() bool {
		v, _ := h.Aircraft(dep)
		return v.State == "lining up" || v.State == "lined up"
	}) {
		v, _ := h.Aircraft(dep)
		t.Errorf("not lined up once the runway was free: %s", v.State)
	}
	if t.Failed() {
		for _, l := range strings.Split(text, "\n") {
			if strings.Contains(l, dep) || strings.Contains(l, arr) {
				t.Log(l)
			}
		}
	}
}
