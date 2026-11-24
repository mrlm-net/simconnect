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
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// Limits of TestLandingWaitsForBacktrack: when the arrival behind is
// spawned (the first this far from the threshold, the second at 5 NM:
// about a minute and a half behind), how long it all may take, and how
// often the player says "cleared to land" (the recorder's, live).
const (
	landOccupiedSpawnNM = 2.5
	landOccupiedMax     = 15 * time.Minute
	landOccupiedEvery   = 6 * time.Second
)

// TestLandingWaitsForBacktrack (#1680): at XBAK, where every arrival on 09
// rolls to the end and backtracks, an arrival under the player's control
// lands behind one of ours. The player's "cleared to land", said again and
// again while ours is still on the runway, is "continue approach", not a
// landing clearance; the arrival behind never lands while ours is on the
// runway (around at its decision point, for want of the clearance or
// with ours still on it), and the
// two never come within a span of each other. Before, the clearance was
// taken and the second rolled through the first (live, TNCM: AAL432 through
// DAL1979 at 43 kt, 0.9 m apart).
func TestLandingWaitsForBacktrack(t *testing.T) {
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
	spawn := func(tail string, stand int, gates bool) {
		t.Helper()
		r := world.SpawnRequest{Kind: "arrival", ICAO: at.icao, Tail: tail, Runway: at.runway, Stand: stand, Model: title, InjectApproach: true, Gates: gates}
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			t.Fatalf("%s: %v", tail, err)
		}
	}
	const first, second = "MTA1", "MTA2"
	spawn(first, at.standFor("arrival"), false)
	start := h.Clock.Now()
	onRunway := func(v world.ControlView) bool {
		return v.State == "landing" && v.OnGround || v.State == "rollout"
	}
	p := traffic.MotionProfileFor(title)
	var states [2][]string
	spawned := false
	var sentAt time.Time
	sent, landedOnto := 0, ""
	closest, closestAt := math.Inf(1), ""
	finished := h.Until(landOccupiedMax, func() bool {
		now := h.Clock.Now()
		a, aok := h.Aircraft(first)
		b, bok := h.Aircraft(second)
		for i, v := range []world.ControlView{a, b} {
			if (i == 0 && aok || i == 1 && bok) && (len(states[i]) == 0 || !strings.HasSuffix(states[i][len(states[i])-1], " "+v.State)) {
				states[i] = append(states[i], fmt.Sprintf("%d: %s", int(now.Sub(start).Seconds()), v.State))
			}
		}
		if !spawned {
			if aok && calc.HaversineNM(a.Position.Lat, a.Position.Lon, thr.Lat, thr.Lon) <= landOccupiedSpawnNM && a.State == "approaching" {
				spawn(second, 4, true)
				spawned = true
			}
			return false
		}
		if !bok {
			return false
		}
		// The player clears the one behind to land, again and again, while
		// ours is on the runway.
		if aok && onRunway(a) && (b.State == "approaching" || b.State == "landing" && !b.OnGround) && now.Sub(sentAt) >= landOccupiedEvery {
			sentAt = now
			if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/land", b.ID), nil); err != nil {
				t.Fatalf("land: %v", err)
			}
			sent++
		}
		if aok && bok && onRunway(a) && b.OnGround && landedOnto == "" {
			landedOnto = fmt.Sprintf("%s %s at %d s with %s %s", second, b.State, int(now.Sub(start).Seconds()), first, a.State)
		}
		if aok && bok && a.OnGround && b.OnGround && a.ObjectID != 0 && b.ObjectID != 0 {
			if d := calc.HaversineMeters(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon); d < closest {
				closest, closestAt = d, fmt.Sprintf("%s and %s at %d s", a.State, b.State, int(now.Sub(start).Seconds()))
			}
		}
		aroundOrDown := h.Said(second, "going around") || b.State == "vacating" || b.State == "awaiting taxi" || b.State == "taxiing" || b.State == "parked"
		return aroundOrDown && (!aok || !onRunway(a))
	})
	t.Logf("%s %q", first, states[0])
	t.Logf("%s %q", second, states[1])
	t.Logf("land said %d times while %s was on the runway; closest %.1f m (%s)", sent, first, closest, closestAt)
	if !spawned || sent == 0 {
		t.Fatalf("nothing checked: second spawned %v, cleared to land while the first was on the runway %d times", spawned, sent)
	}
	if !strings.Contains(log.String(), "land: "+first+" on the runway, continue approach") || !h.Said(second, "continue approach") {
		t.Errorf("%s was cleared to land with %s on the runway: not held, no \"continue approach\"", second, first)
	}
	if landedOnto != "" {
		t.Errorf("landed onto an occupied runway: %s", landedOnto)
	}
	if closest < p.SpanMeters {
		t.Errorf("came to %.1f m of each other, less than a span (%.1f m): %s", closest, p.SpanMeters, closestAt)
	}
	if !finished {
		t.Errorf("neither around nor down in %v", landOccupiedMax)
	}
	if t.Failed() {
		for _, l := range strings.Split(log.String(), "\n") {
			if strings.Contains(l, second) || strings.Contains(l, "go around") {
				t.Log(l)
			}
		}
	}
}
