package worldtest

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// lineUpLandingNM: the player clears the departure with the arrival this
// close, cleared to land (inside #1688's 2 NM).
const lineUpLandingNM = 1.5

// TestNoLineUpUnderLanding (#1688): at XBAK a departure under the player's
// control holds short at D, the way in from which it backtracks towards
// the arrivals on 09, as one of ours lands, cleared to land. With that
// arrival inside 2 NM the player's "line up" and "take off" are both
// answered "hold position" and the departure stays holding short until the
// arrival is down and off the runway; then the player's line-up is taken.
// Before, both were taken and it lined up under the landing aircraft (live,
// TNCM: AAL1616 turned onto the runway 4.3 m in plan from WIA195 crossing
// the threshold at 44 ft).
func TestNoLineUpUnderLanding(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	at := meetXBAK
	title := at.title
	log := &lockedBuf{}
	o := Options{DataDir: backtrackFieldDir(t), ICAO: at.icao, Manual: true, FrameRate: 10,
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
	const dep, arr = "MTD1", "MTA1"
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
	spawn(world.SpawnRequest{Kind: "departure", Tail: dep, Stand: 4})
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
	// With the arrival cleared and close in: line up, take off.
	if !h.Until(10*time.Minute, func() bool {
		a, ok := h.Aircraft(arr)
		return ok && !a.OnGround && a.ObjectID != 0 && nm(a) <= lineUpLandingNM && h.Said(arr, "cleared to land")
	}) {
		a, _ := h.Aircraft(arr)
		t.Fatalf("nothing checked: the arrival never came within %.1f NM cleared to land (%s, %.1f NM)", lineUpLandingNM, a.State, nm(a))
	}
	give(dep, "lineup")
	give(dep, "takeoff")
	var under []string
	off := h.Until(10*time.Minute, func() bool {
		a, aok := h.Aircraft(arr)
		d, _ := h.Aircraft(dep)
		offRunway := !aok || slices.Contains([]string{"awaiting taxi", "taxiing", "parking", "parked"}, a.State)
		if d.State != "holding short" && len(under) < 5 {
			under = append(under, fmt.Sprintf("%s %s with %s %s", dep, d.State, arr, a.State))
		}
		return offRunway || h.Said(arr, "going around")
	})
	text := log.String()
	if !strings.Contains(text, "lineup: "+arr+" landing or on the runway, hold position") || !strings.Contains(text, "takeoff: "+arr+" landing or on the runway, hold position") {
		t.Errorf("the player's line-up and take-off were not held with %s landing", arr)
	}
	if !h.Said(dep, "hold position") {
		t.Errorf("%s was not told to hold position", dep)
	}
	if len(under) > 0 {
		t.Errorf("left its holding point under the landing aircraft: %q", under)
	}
	if !off {
		t.Fatalf("the arrival neither got off the runway nor went around")
	}
	if h.Said(arr, "going around") {
		t.Logf("%s went around: the line-up afterwards not checked", arr)
		return
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
			if strings.Contains(l, dep) {
				t.Log(l)
			}
		}
	}
}
