package worldtest

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// The stop bars (#1532): every departure is held at, or cleared past, its
// runway's holding point; holding short it names the point; a line-up
// releases it; the airport's bars are released exactly where an aircraft is
// cleared past them.
func TestHoldBars(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 14}); err != nil {
		t.Fatal(err)
	}
	ends := func(rwy string) []string { return strings.Split(rwy, "/") }
	protects := func(p *world.HoldPoint, rwy string) bool {
		if p == nil || rwy == "" {
			return false
		}
		for _, e := range ends(rwy) {
			if slices.Contains(ends(p.Runway), e) {
				return true
			}
		}
		return false
	}
	own := map[string]bool{}      // departure saw its runway's holding point (held at or cleared past)
	departed := map[string]bool{} // departure reached its take-off
	lineUpReleased := map[string]bool{}
	crossings, exits, checks := 0, 0, 0
	for step := 0; step < 2*60*60; step++ { // an hour, every half second
		h.Advance(500 * time.Millisecond)
		snap := h.World.Snapshot()
		for _, v := range snap.Aircraft {
			for _, p := range v.ClearedPast {
				if v.HoldAt != nil && v.HoldAt.Node == p.Node {
					t.Fatalf("%s %s: held at and cleared past node %d", v.Tail, v.State, p.Node)
				}
			}
			if v.Kind == "departure" {
				if protects(v.HoldAt, v.Runway) || slices.ContainsFunc(v.ClearedPast, func(p world.HoldPoint) bool { return protects(&p, v.Runway) }) {
					own[v.Tail] = true
				}
				if v.State == "lining up" && slices.ContainsFunc(v.ClearedPast, func(p world.HoldPoint) bool { return protects(&p, v.Runway) }) {
					lineUpReleased[v.Tail] = true
				}
				if v.State == "departing" {
					departed[v.Tail] = true
					if len(v.ClearedPast) > 0 || v.HoldAt != nil {
						t.Errorf("%s departing still holds %v / clears %v", v.Tail, v.HoldAt, v.ClearedPast)
					}
				}
			}
			if v.State == "holding short" && v.HoldAt == nil {
				t.Errorf("%s holding short of %q, no holding point", v.Tail, v.HoldingShortOf)
			}
			for _, p := range v.ClearedPast {
				switch {
				case v.Kind == "arrival" && (v.State == "rollout" || v.State == "vacating"):
					exits++
				case !protects(&p, v.Runway):
					crossings++
				}
			}
		}
		if step%120 == 0 { // the airport's bars against the aircraft, every minute
			var bars []world.HoldBar
			if err := h.World.Get("/api/holds?icao=LKPR", &bars); err != nil {
				t.Fatal(err)
			}
			cleared := map[int][]string{}
			for _, v := range h.World.Snapshot().Aircraft {
				for _, p := range v.ClearedPast {
					cleared[int(p.Node)] = append(cleared[int(p.Node)], v.Tail)
				}
			}
			for _, b := range bars {
				slices.Sort(cleared[int(b.Node)])
				if b.Released != (len(cleared[int(b.Node)]) > 0) || !slices.Equal(b.By, cleared[int(b.Node)]) {
					t.Errorf("bar %d (%s): released %v by %v, aircraft cleared past it %v", b.Node, b.Runway, b.Released, b.By, cleared[int(b.Node)])
				}
			}
			if len(bars) == 0 {
				t.Fatal("no holding points at LKPR")
			}
			checks++
		}
	}
	if len(departed) == 0 {
		t.Fatal("no departure in an hour")
	}
	for tail := range departed {
		if !own[tail] {
			t.Errorf("%s departed, never held at or cleared past its runway's holding point", tail)
		}
	}
	if len(lineUpReleased) == 0 {
		t.Error("no line-up released its holding point")
	}
	if crossings == 0 || exits == 0 {
		t.Errorf("crossings released %d frames, exits %d: want both", crossings, exits)
	}
	t.Logf("%d departed, %d released on the line-up, crossing frames %d, exit frames %d, %d bar checks", len(departed), len(lineUpReleased), crossings, exits, checks)
}
