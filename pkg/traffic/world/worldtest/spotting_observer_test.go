package worldtest

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type observerView struct {
	Mode, Subject, Event, Move, Shot, Error string
}

func observerNow(t *testing.T, h *Harness) observerView {
	t.Helper()
	b, err := h.World.Do(http.MethodGet, "/api/camera", nil)
	if err != nil {
		t.Fatal(err)
	}
	var v observerView
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// eyeLog follows the camera's eye frame by frame, as the simulator is told
// it: the jumps (cuts) after the first pose of the spotting, the way flown,
// the fastest it moved between them.
type eyeLog struct {
	have           bool
	lat, lon, alt  float64
	poses          int
	cuts           []float64
	flown, fastest float64
	at             time.Time
	sets           int
}

// spotJumpM: an eye moving more than this in a frame cut there.
const spotJumpM = 5.0

func (l *eyeLog) take(h *Harness) {
	n := h.Sim.CameraSets()
	if n == l.sets {
		return // not set this frame
	}
	l.sets = n
	now := h.Clock.Now()
	p := h.Sim.CameraPose()
	if p.PositionReferential != types.SIMCONNECT_POSITION_REFERENTIAL_WORLD {
		return
	}
	lat, lon, alt := p.Position.X, p.Position.Y, p.Position.Z
	if l.have {
		d := calc.HaversineMeters(l.lat, l.lon, lat, lon)
		if d > spotJumpM {
			l.cuts = append(l.cuts, d)
		} else {
			l.flown += d
			if dt := now.Sub(l.at).Seconds(); dt > 0 {
				l.fastest = max(l.fastest, d/dt)
			}
		}
	}
	l.have, l.lat, l.lon, l.alt, l.at = true, lat, lon, alt, now
	l.poses++
}

// TestSpottingObserver (#1472, #1473, #1475): the observer at LKPR for 45
// minutes. After the first shot no cut goes farther than cutM: every other
// change of place is flown, no faster than the cruise; the landings are
// caught at their touchdown; the way the drone flew is logged.
func TestSpottingObserver(t *testing.T) {
	h := spottingStart(t)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 10, MaxAircraft: 24, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var downs []world.Moment
	h.World.OnMoment(func(m world.Moment) {
		if m.Name == "touchdown" && m.Ours {
			mu.Lock()
			downs = append(downs, m)
			mu.Unlock()
		}
	})
	const cutM, travelMaxM = 300.0, 1000.0
	if err := h.World.StartSpotting(world.SpottingRequest{ICAO: "LKPR", TravelMaxM: travelMaxM}); err != nil {
		t.Fatal(err)
	}
	log := &eyeLog{}
	moves := map[string]int{}
	var last observerView
	landings, caught := 0, 0
	missed := []string{}
	h.Until(45*time.Minute, func() bool {
		log.take(h)
		v := observerNow(t, h)
		if v.Error != "" {
			t.Fatalf("camera: %s", v.Error)
		}
		if v.Move != last.Move || v.Shot != last.Shot || v.Subject != last.Subject {
			moves[v.Move]++
			t.Logf("%s %s: %s %s", v.Move, v.Shot, v.Subject, v.Event)
			last = v
		}
		mu.Lock()
		got := downs
		downs = nil
		mu.Unlock()
		for _, m := range got {
			landings++
			if v.Subject == m.Tail && v.Event == "landing" && (v.Move == "cut" || v.Move == "join") {
				caught++
			} else {
				missed = append(missed, m.Tail+" ("+v.Move+" "+v.Subject+" "+v.Event+")")
			}
		}
		return false
	})
	if log.poses == 0 {
		t.Fatal("no camera pose in the world")
	}
	t.Logf("flown %.0f m, cuts %v, fastest %.1f m/s, moves %v; landings caught %d of %d, missed %v", log.flown, log.cuts, log.fastest, moves, caught, landings, missed)
	// The first shot is a cut from wherever the camera was: the loopback's
	// camera had no pose, so it is not among the jumps.
	for _, d := range log.cuts {
		if d > cutM+1 {
			t.Errorf("a cut of %.0f m, longer than cutM %.0f", d, cutM)
		}
	}
	if moves["travel"] == 0 {
		t.Error("never travelled")
	}
	if log.fastest > 20.5 {
		t.Errorf("the drone flew %.1f m/s", log.fastest)
	}
	if landings < 5 {
		t.Fatalf("only %d landings in 45 minutes", landings)
	}
	if float64(caught) < 0.9*float64(landings) {
		t.Errorf("caught %d of %d landings at the touchdown, want 90%%: missed %v", caught, landings, missed)
	}
}

// TestSpottingNoTravelByDefault: without travelMaxM the director cuts as it
// did, anywhere: its move is always a cut, it never travels or looks round.
func TestSpottingNoTravelByDefault(t *testing.T) {
	h := spottingStart(t)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	if err := h.World.StartSpotting(world.SpottingRequest{ICAO: "LKPR", CutM: 50, TravelSpeed: 12}); err != nil {
		t.Fatal(err)
	}
	log := &eyeLog{}
	shots := 0
	var last observerView
	h.Until(25*time.Minute, func() bool {
		log.take(h)
		v := observerNow(t, h)
		if v.Move != "" && v.Move != "cut" {
			t.Fatalf("move %q without travelMaxM", v.Move)
		}
		if v.Shot != last.Shot && v.Shot != "" {
			shots++
		}
		last = v
		return false
	})
	if shots < 3 {
		t.Fatalf("only %d shots", shots)
	}
	far := 0
	for _, d := range log.cuts {
		if d > 50 {
			far++
		}
	}
	if far == 0 {
		t.Errorf("no cut past cutM 50 m (%v): cutM is not to be used without travelMaxM", log.cuts)
	}
}
