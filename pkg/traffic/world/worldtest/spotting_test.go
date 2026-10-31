package worldtest

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

func spottingStart(t *testing.T) *Harness {
	t.Helper()
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
	t.Cleanup(h.Close)
	return h
}

type spottingView struct {
	Mode, Subject, Event, Shot, Error string
}

func spottingNow(t *testing.T, h *Harness) spottingView {
	t.Helper()
	b, err := h.World.Do(http.MethodGet, "/api/camera", nil)
	if err != nil {
		t.Fatal(err)
	}
	var v spottingView
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestSpottingHoldsThroughTheEvent: left to itself the camera finds aircraft
// landing and taking off and shows each from a spot of the kind, with one
// aircraft held to the end of what it does; the filter keeps to one event (#1343).
func TestSpottingHoldsThroughTheEvent(t *testing.T) {
	h := spottingStart(t)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 6, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	if err := h.World.StartSpotting(world.SpottingRequest{ICAO: "LKPR"}); err != nil {
		t.Fatal(err)
	}
	h.Advance(time.Second)
	events := map[string]int{}
	var last spottingView
	// What it says an aircraft is doing is what that aircraft is doing, and it
	// does not leave a landing or a take-off before it is over.
	if !h.Until(40*time.Minute, func() bool {
		v := spottingNow(t, h)
		if v.Error != "" {
			t.Fatalf("camera: %s", v.Error)
		}
		if a, ok := h.Aircraft(v.Subject); ok && v.Event != "" {
			if v.Event == "takeoff" && a.Kind != "departure" || v.Event == "landing" && a.Kind != "arrival" {
				t.Errorf("%s (%s, %s) said to be on %s", v.Subject, a.Kind, a.State, v.Event)
			}
		}
		if v.Subject != last.Subject || v.Event != last.Event {
			if a, ok := h.Aircraft(last.Subject); ok && v.Subject != last.Subject && v.Subject != "" {
				if last.Event == "landing" && a.State == "landing" {
					t.Errorf("left %s still %s for %s %s", last.Subject, a.State, v.Subject, v.Event)
				}
			}
			if v.Subject != "" && v.Event != "" {
				events[v.Event]++
				t.Logf("%s %s, from %s", v.Subject, v.Event, v.Shot)
			}
			last = v
		}
		return events["landing"] >= 2 && events["takeoff"] >= 2 && events["taxi"] >= 1
	}) {
		t.Fatalf("saw only %v", events)
	}
	if h.Sim.CameraSets() == 0 {
		t.Error("no camera pose set")
	}

	if err := h.World.StartSpotting(world.SpottingRequest{ICAO: "LKPR", Events: []string{"takeoff"}}); err != nil {
		t.Fatal(err)
	}
	took := 0
	h.Until(60*time.Minute, func() bool {
		v := spottingNow(t, h)
		if v.Event != "" && v.Event != "takeoff" {
			t.Fatalf("filter on take-offs, watching %s %s", v.Subject, v.Event)
		}
		if v.Event == "takeoff" {
			took++
		}
		return took > 3
	})
	if took == 0 {
		t.Error("no take-off seen with the filter")
	}
	if err := h.World.StartSpotting(world.SpottingRequest{ICAO: "XXXX"}); err == nil {
		t.Error("an unknown airport was taken")
	}
}

// TestSpottingPlayer: the player's aircraft is picked like any other, and a filter
// for it alone leaves the rest of the traffic out (#1343).
func TestSpottingPlayer(t *testing.T) {
	h := spottingStart(t)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 3, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	if err := h.World.StartSpotting(world.SpottingRequest{ICAO: "LKPR", Kinds: []string{"player"}}); err != nil {
		t.Fatal(err)
	}
	h.Advance(30 * time.Second)
	if v := spottingNow(t, h); v.Subject != "" {
		t.Fatalf("the player is parked, watching %s %s", v.Subject, v.Event)
	}
	// Taxiing on the apron.
	g := h.Layout.Parking[0].Position
	lat := g.Lat
	h.SetUser(func(u *Object) {
		u.Lat, u.Lon, u.Heading, u.AltFt, u.OnGround, u.GroundKts = lat, g.Lon, 90, h.Layout.Altitude/0.3048, true, 15
	})
	if !h.Until(2*time.Minute, func() bool {
		v := spottingNow(t, h)
		return v.Event == "taxi" && v.Subject != ""
	}) {
		v := spottingNow(t, h)
		t.Fatalf("the player taxiing not picked: %+v", v)
	}
}

// TestSpottingFilterOurs: the kind and type filters see our traffic, under its
// call sign.
func TestSpottingFilterOurs(t *testing.T) {
	h := spottingStart(t)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 4, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	var kind string
	if !h.Until(30*time.Minute, func() bool {
		var list []world.Traffic
		if err := h.World.Get("/api/traffic", &list); err != nil {
			t.Fatal(err)
		}
		for _, a := range list {
			if a.Ours && a.Type != "" {
				kind = a.Type
				return true
			}
		}
		return false
	}) {
		t.Fatal("no aircraft of ours with a type")
	}
	if err := h.World.StartSpotting(world.SpottingRequest{ICAO: "LKPR", Kinds: []string{"ours"}, Types: []string{kind}}); err != nil {
		t.Fatal(err)
	}
	if !h.Until(40*time.Minute, func() bool {
		v := spottingNow(t, h)
		if v.Subject == "" {
			return false
		}
		if _, ok := h.Aircraft(v.Subject); !ok {
			t.Fatalf("subject %q is no call sign of ours", v.Subject)
		}
		return true
	}) {
		t.Fatalf("no %s of ours picked: %+v", kind, spottingNow(t, h))
	}
}
