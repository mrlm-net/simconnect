package worldtest

import (
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestTakeoffOnGroundWritten (#1605): a departure's written poses, the
// ones a host draws, carry the on-ground flag from lined up through the
// whole roll and rotation, and the first without it is the first with the
// wheels above the runway: its controller lifted off (at most a frame
// before, the wheels still at the runway then), and climbing above every
// write of the roll within a second. Before,
// the roll was written without it (TNCM 10, an A320: 33 s of 39). An
// airliner and a heavy, at LKPR and TNCM, with no frame jumps.
func TestTakeoffOnGroundWritten(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	d := dataDir(t)
	cases := []struct{ icao, rwy, title string }{
		{"LKPR", "24", "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery"},
		{"LKPR", "24", "FSLTL_FAIB_B744_KLM"},
		{"TNCM", "10", "FSLTL A320 Air France SL"},
		{"TNCM", "10", "FSLTL_FAIB_B744_KLM"},
	}
	for _, tc := range cases {
		t.Run(tc.icao+"_"+tc.rwy+"_"+tc.title[len(tc.title)-8:], func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(d, tc.icao, "layout.json")); err != nil {
				t.Skipf("no %s data", tc.icao)
			}
			takeoffOnGroundRun(t, d, tc.icao, tc.rwy, tc.title)
		})
	}
}

type writtenPose struct {
	state    string
	airborne bool
	onGround bool
	altFt    float64
	kts      float64
}

func takeoffOnGroundRun(t *testing.T, d, icao, rwy, title string) {
	titles := append(slices.Clone(DefaultAircraftTitles), "FSLTL_FAIB_B744_KLM")
	o := Options{DataDir: d, ICAO: icao, Manual: true, FrameRate: 30, AircraftTitles: titles}
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
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 15, 10, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway(icao, rwy)
	h.Advance(2 * time.Second)
	stand, big := -1, 0.0
	for i, p := range h.Layout.Parking {
		if p.Radius > big {
			stand, big = i, p.Radius
		}
	}
	const tail = "TOG1"
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "departure", ICAO: icao, Stand: stand, Runway: rwy, Model: title, Tail: tail, PushInMin: 1}); err != nil {
		t.Fatal(err)
	}
	var objectID uint32
	var poses []writtenPose
	var states []string
	if !h.Until(60*time.Minute, func() bool {
		v, ok := h.Aircraft(tail)
		if !ok {
			return false
		}
		if len(states) == 0 || states[len(states)-1] != v.State {
			states = append(states, v.State)
		}
		if v.ObjectID != 0 {
			objectID = v.ObjectID
		}
		if v.State != "lined up" && v.State != "departing" && len(poses) == 0 {
			return v.Done
		}
		for _, ob := range h.Sim.Objects() {
			if ob.ID == objectID {
				poses = append(poses, writtenPose{v.State, !v.OnGround, ob.OnGround, ob.AltFt, ob.GroundKts})
				return ob.AltFt > poses[0].altFt+300 || v.Done
			}
		}
		return v.Done
	}) {
		t.Fatalf("%s not airborne in 60 minutes: states %v", tail, states)
	}
	if len(poses) == 0 {
		t.Fatalf("%s never lined up or departed: states %v", tail, states)
	}
	off, rollMax := -1, math.Inf(-1)
	for k, p := range poses {
		if p.onGround {
			if off >= 0 {
				t.Errorf("write %d (%s, %.0f kt): the flag back after lift-off", k, p.state, p.kts)
				break
			}
			rollMax = math.Max(rollMax, p.altFt)
			continue
		}
		if off < 0 {
			off = k
		}
	}
	if off < 0 {
		t.Fatalf("%s never written airborne: %d writes", tail, len(poses))
	}
	first := poses[off]
	if !first.airborne {
		t.Errorf("first airborne write (%s, %.0f kt) with %s not lifted off", first.state, first.kts, tail)
	}
	late := 0
	for _, p := range poses[:off] {
		if p.airborne {
			late++
		}
	}
	if late > 2 {
		t.Errorf("%d writes with the flag after %s lifted off, want the wheels' frame at most", late, tail)
	}
	above := false
	for _, p := range poses[off:min(len(poses), off+o.FrameRate)] {
		above = above || p.altFt > rollMax
	}
	if !above {
		t.Errorf("a second after the first airborne write (%s, %.0f kt, %.2f ft), not above the roll's highest %.2f ft",
			first.state, first.kts, first.altFt, rollMax)
	}
	if off < 30 {
		t.Errorf("first airborne write after %d writes from lined up: the roll written without the flag", off)
	}
	for _, j := range h.Sim.Jumps() {
		if j.Tail == tail {
			t.Errorf("jump: %+v", j)
		}
	}
}
