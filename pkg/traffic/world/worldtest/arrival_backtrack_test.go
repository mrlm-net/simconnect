package worldtest

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
)

// TestArrivalBacktracksTNCM (#1563): at TNCM no taxiway runs beside 10/28,
// and from the exits an A320 can make on 10 none reaches the terminal
// stands. The arrival turns round at the far end, taxis back along the
// runway and leaves it nearer its stand. The runway is its until then: a
// departure about to go the other way along its way in is held on its
// stand meanwhile, and no second arrival is on the runway with it.
func TestArrivalBacktracksTNCM(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	d := dataDir(t)
	if _, err := os.Stat(filepath.Join(d, "TNCM", "layout.json")); err != nil {
		t.Skip("no TNCM data")
	}
	o := Options{DataDir: d, ICAO: "TNCM", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	wx := nav.StaticWeather(100, 8, 9999, 100, 27, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway("TNCM", "10")
	h.Advance(2 * time.Second)
	stand := -1
	for i, p := range h.Layout.Parking {
		if p.Label() == "A1" {
			stand = i
		}
	}
	if stand < 0 {
		t.Skip("no stand A1 at TNCM")
	}
	spawn := func(kind, tail, rwy string, stand int) {
		t.Helper()
		if _, err := h.World.Do(http.MethodPost, "/api/control", map[string]any{"kind": kind, "icao": "TNCM", "tail": tail, "runway": rwy, "stand": stand, "model": "A320"}); err != nil {
			t.Fatalf("%s: %v", tail, err)
		}
	}
	spawn("arrival", "TBA1", "10", stand)
	if !h.Until(10*time.Minute, func() bool { v, _ := h.Aircraft("TBA1"); return v.State == "rollout" }) {
		t.Fatal("TBA1 never landed")
	}
	jumps := len(h.Sim.Jumps())
	spawn("arrival", "TBA2", "10", -1)
	spawn("departure", "TBD1", "", -1)
	var backAt time.Time
	together, linedUp := false, false
	if !h.Until(30*time.Minute, func() bool {
		a, _ := h.Aircraft("TBA1")
		b, _ := h.Aircraft("TBA2")
		dep, _ := h.Aircraft("TBD1")
		onRwy := a.State == "landing" || a.State == "rollout"
		if onRwy && a.Heading > 180 && backAt.IsZero() {
			backAt = h.Clock.Now() // turned round: back along the runway
		}
		if onRwy && b.OnGround && (b.State == "landing" || b.State == "rollout") {
			together = true
		}
		if onRwy && (dep.State == "pushback" || dep.State == "taxiing" || dep.State == "lining up" || dep.State == "departing") {
			linedUp = true
		}
		return a.State == "parked"
	}) {
		v, _ := h.Aircraft("TBA1")
		t.Fatalf("TBA1 not parked: %s at %s", v.State, v.Stand)
	}
	if v, _ := h.Aircraft("TBA1"); v.Stand != "A1" {
		t.Errorf("TBA1 parked at %s, want A1", v.Stand)
	}
	if backAt.IsZero() {
		t.Error("TBA1 never backtracked on the runway")
	}
	if together {
		t.Error("TBA2 landed with TBA1 still on the runway")
	}
	if linedUp {
		t.Error("TBD1 pushed, taxied or lined up with TBA1 still on the runway")
	}
	for _, j := range h.Sim.Jumps()[jumps:] {
		if j.Tail == "TBA1" {
			t.Errorf("TBA1 jumped: %+v", j)
		}
	}
}
