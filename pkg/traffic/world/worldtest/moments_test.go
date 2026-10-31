package worldtest

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// heardMoments collects the moments by call sign.
type heardMoments struct {
	mu sync.Mutex
	by map[string][]string
}

func listenMoments(h *Harness) *heardMoments {
	l := &heardMoments{by: map[string][]string{}}
	h.World.OnMoment(func(m world.Moment) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.by[m.Tail] = append(l.by[m.Tail], m.Name)
	})
	return l
}

// has: some aircraft went through want in that order; which one it was.
func (l *heardMoments) has(want ...string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	for tail, got := range l.by {
		i := 0
		for _, n := range got {
			if i < len(want) && n == want[i] {
				i++
			}
		}
		if i == len(want) {
			return tail
		}
	}
	return ""
}

// TestMomentsOfOurs: our traffic says each moment of its take-offs and landings,
// in order, to the host and on the API.
func TestMomentsOfOurs(t *testing.T) {
	h := spottingStart(t)
	l := listenMoments(h)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	up := []string{world.MomentRotate, world.MomentLiftOff, world.MomentGearUp}
	down := []string{world.MomentFlare, world.MomentTouchdown, world.MomentReversers, world.MomentVacated}
	if !h.Until(60*time.Minute, func() bool { return l.has(up...) != "" && l.has(down...) != "" }) {
		l.mu.Lock()
		defer l.mu.Unlock()
		t.Fatalf("heard only %v", l.by)
	}
	var list []world.Moment
	if err := h.World.Get("/api/camera/moments", &list); err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 || !list[0].Ours || list[0].ObjectID == 0 || list[0].Tail == "" {
		t.Fatalf("moments on the API: %+v", list)
	}
	var after []world.Moment
	if err := h.World.Get("/api/camera/moments?since="+strconv.FormatUint(list[0].Seq, 10), &after); err != nil {
		t.Fatal(err)
	}
	if len(after) == 0 || after[0].Seq != list[0].Seq+1 {
		t.Fatalf("after %d: %+v", list[0].Seq, after)
	}
}

// TestMomentsOfThePlayer: what the simulator shows of the player's aircraft is
// told too, and a bounce is no lift-off.
func TestMomentsOfThePlayer(t *testing.T) {
	h := spottingStart(t)
	l := listenMoments(h)
	field := h.Layout.Altitude / 0.3048
	p := h.Layout.Parking[0].Position
	step := func(f func(u *Object)) {
		h.SetUser(f)
		h.Advance(5 * time.Second)
	}
	step(func(u *Object) { u.Lat, u.Lon, u.Heading, u.AltFt, u.OnGround = p.Lat, p.Lon, 90, field+500, false })
	step(func(u *Object) { u.AltFt, u.OnGround = field, true })
	waitHeard := func(want ...string) {
		t.Helper()
		if !h.WaitFor(5*time.Second, func() bool { return l.has(want...) != "" }) {
			l.mu.Lock()
			defer l.mu.Unlock()
			t.Fatalf("want %v, heard %v", want, l.by)
		}
	}
	waitHeard(world.MomentTouchdown)
	// A bounce.
	step(func(u *Object) { u.AltFt, u.OnGround = field+5, false })
	step(func(u *Object) { u.AltFt, u.OnGround = field, true })
	// Along the line it touched down on, east: still the runway.
	step(func(u *Object) { u.Lon = p.Lon + 0.01 })
	// Off to the north of it.
	step(func(u *Object) { u.Lat = p.Lat + 0.002 })
	waitHeard(world.MomentTouchdown, world.MomentVacated)
	step(func(u *Object) { u.AltFt, u.OnGround = field+100, false })
	waitHeard(world.MomentTouchdown, world.MomentVacated, world.MomentLiftOff)
	step(func(u *Object) { u.GearUp = true })
	waitHeard(world.MomentTouchdown, world.MomentVacated, world.MomentLiftOff, world.MomentGearUp)
	l.mu.Lock()
	defer l.mu.Unlock()
	got := l.by["PLAYER"]
	want := []string{world.MomentTouchdown, world.MomentVacated, world.MomentLiftOff, world.MomentGearUp}
	if !slices.Equal(got, want) {
		t.Errorf("player: %v, want %v", got, want)
	}
}

// TestSceneCuesOnMoments: beats wait for moments of the cast, the player and an
// aircraft outside the cast, and for an arrival's height.
func TestSceneCuesOnMoments(t *testing.T) {
	h := spottingStart(t)
	shot := []world.SceneShot{{Move: "topOrbit", Who: "arr", Sec: 5}}
	bad := world.Scene{Name: "bad", Cast: []world.SceneCast{{Role: "arr", Kind: "arrival", Tail: "MOM1", Models: "a320"}},
		Beats: []world.SceneBeat{{Cue: "start", Shots: shot}, {Cue: "nobody:flare", Shots: shot}}}
	if err := h.World.PlayScene("LKPR", bad); err == nil || !strings.Contains(err.Error(), "nobody:flare") {
		t.Fatalf("a typed-wrong role: %v", err)
	}
	var mu sync.Mutex
	var cues []string
	h.World.OnSceneEvent(func(e world.SceneEvent) {
		if e.Kind == "beat" {
			mu.Lock()
			cues = append(cues, e.Cue)
			mu.Unlock()
		}
	})
	want := []string{"start", "arr:below:1000", "arr:flare", "arr:touchdown", "arr:vacated", "player:liftoff"}
	sc := world.Scene{Name: "moments", LimitSec: 3000, Cast: bad.Cast}
	for _, c := range want {
		sc.Beats = append(sc.Beats, world.SceneBeat{Cue: c, Shots: shot})
	}
	if err := h.World.PlayScene("LKPR", sc); err != nil {
		t.Fatal(err)
	}
	beats := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(cues)
	}
	if !h.Until(30*time.Minute, func() bool { return len(beats()) >= len(want)-1 }) {
		t.Fatalf("beats reached: %v", beats())
	}
	if got := beats(); !slices.Equal(got, want[:len(want)-1]) {
		t.Fatalf("beats: %v", got)
	}
	field := h.Layout.Altitude / 0.3048
	h.SetUser(func(u *Object) { u.AltFt, u.OnGround = field, true })
	h.Advance(5 * time.Second)
	h.SetUser(func(u *Object) { u.AltFt, u.OnGround = field+100, false })
	if !h.Until(5*time.Minute, func() bool { return len(beats()) == len(want) }) {
		t.Fatalf("the player's lift-off: beats %v", beats())
	}
}
