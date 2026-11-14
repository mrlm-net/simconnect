package worldtest

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestStaffedGroundAfterDelivery: ground and tower staffed, delivery not,
// a game's departure cleared by the AI's delivery and handed to ground asks
// the player: the AI's ground never answers its first call itself (#1538).
// The hand-off replayed the waiting request before checkStaffed had looked
// at the aircraft on its new station, so the answer was already queued:
// "request pushback and start up", "on a staffed station (Ruzyne Ground):
// the player's", "pushback and start up approved".
func TestStaffedGroundAfterDelivery(t *testing.T) {
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
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "ground"}, {ICAO: "LKPR", Position: "tower"}})
	// A game as a host plays it: its flights only, the schedule off.
	if _, err := h.World.Do(http.MethodPost, "/api/schedule", map[string]any{"enabled": false}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.World.Do(http.MethodPost, "/api/game", map[string]any{"on": true, "icao": "LKPR", "intervalSec": 60}); err != nil {
		t.Fatal(err)
	}
	// Departures cleared by delivery that have asked ground since.
	askedGround := func() map[string]bool {
		cleared, asked := map[string]bool{}, map[string]bool{}
		for _, tx := range h.Heard() {
			switch {
			case tx.Intent == traffic.IntentDepartureClearance:
				cleared[tx.Callsign] = true
			case tx.Pilot && cleared[tx.Callsign] && tx.Position == traffic.PosGround &&
				(tx.Intent == traffic.IntentRequestPushback || tx.Intent == traffic.IntentRequestStartUp || tx.Intent == traffic.IntentRequestTaxi):
				asked[tx.Callsign] = true
			}
		}
		return asked
	}
	if !h.Until(30*time.Minute, func() bool { return len(askedGround()) >= 3 }) {
		t.Fatalf("%d delivery-cleared departures asked ground in 30 simulated minutes, want 3", len(askedGround()))
	}
	h.Advance(3 * time.Minute)

	// The AI's ground clears nothing on a staffed ground.
	for _, tx := range h.Heard() {
		if tx.Pilot || tx.Position != traffic.PosGround {
			continue
		}
		switch tx.Intent {
		case traffic.IntentPushback, traffic.IntentStartUp, traffic.IntentTaxi, traffic.IntentFollowTaxi, traffic.IntentFollow:
			t.Errorf("the AI's ground on a staffed ground: %s %s %q", tx.Callsign, tx.Intent, tx.Text)
		}
	}
	if t.Failed() {
		return
	}

	// One waits for the player's push and start, and gets it.
	var dep world.ControlView
	asked := askedGround()
	for _, v := range h.World.Snapshot().Aircraft {
		if asked[v.Tail] && v.ATC == string(traffic.PosGround) && slices.Contains(v.Actions, "pushstart") {
			dep = v
			break
		}
	}
	if dep.Tail == "" {
		for _, v := range h.World.Snapshot().Aircraft {
			t.Logf("%s %s %s %s manual=%v %v", v.Tail, v.Kind, v.State, v.ATC, v.Manual, v.Actions)
		}
		t.Fatal("no departure waiting on ground with pushstart")
	}
	if _, err := h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/pushstart?as=ground", dep.ID), nil); err != nil {
		t.Fatalf("pushstart as ground: %v", err)
	}
	if !h.Until(time.Minute, func() bool {
		for _, tx := range h.Heard() {
			if tx.Callsign == dep.Tail && !tx.Pilot && tx.Position == traffic.PosGround &&
				(tx.Intent == traffic.IntentPushback || tx.Intent == traffic.IntentStartUp) {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("%s: the player's pushback and start-up never said", dep.Tail)
	}
}

// TestStaffedGroundBeforeClearance: a departure whose crew asks for its push
// while it still waits for its clearance is released to automatic when
// delivery is unstaffed, and the AI's delivery clears it; the push it asked
// for waits for the clearance and then goes to the staffed ground, the
// player's: the AI's ground never answers a departure not yet handed to it
// (#1542; live, LROP: "pushback approved" 1 s after spawn, before the
// clearance was said).
func TestStaffedGroundBeforeClearance(t *testing.T) {
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
	ground := []world.StaffedStation{{ICAO: "LKPR", Position: "ground"}, {ICAO: "LKPR", Position: "tower"}}
	h.World.SetStaffed(append([]world.StaffedStation{{ICAO: "LKPR", Position: "delivery"}}, ground...))
	if _, err := h.World.Do(http.MethodPost, "/api/schedule", map[string]any{"enabled": false, "icao": "LKPR"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.World.Do(http.MethodPost, "/api/game", map[string]any{"on": true, "icao": "LKPR", "intervalSec": 60}); err != nil {
		t.Fatal(err)
	}
	// Departures waiting for their clearance long enough for the crew to
	// be ready to push.
	if !h.Until(30*time.Minute, func() bool {
		n := 0
		for _, v := range h.World.Snapshot().Aircraft {
			if v.Kind == "departure" && slices.Contains(v.Actions, "clearance") {
				n++
			}
		}
		return n >= 2
	}) {
		t.Fatal("no two departures waiting for their clearance in 30 simulated minutes")
	}
	h.Advance(5 * time.Minute)
	h.World.SetStaffed(ground) // delivery the AI's again
	h.Advance(3 * time.Minute)

	cleared := map[string]bool{}
	for _, tx := range h.Heard() {
		if !tx.Pilot && tx.Intent == traffic.IntentDepartureClearance {
			cleared[tx.Callsign] = true
		}
		if tx.Pilot || tx.Position != traffic.PosGround {
			continue
		}
		switch tx.Intent {
		case traffic.IntentPushback, traffic.IntentStartUp, traffic.IntentTaxi, traffic.IntentFollowTaxi, traffic.IntentFollow:
			t.Errorf("the AI's ground on a staffed ground: %s %s %q", tx.Callsign, tx.Intent, tx.Text)
		}
	}
	if len(cleared) < 2 {
		t.Errorf("the AI's delivery cleared %d departures once unstaffed, want 2", len(cleared))
	}
	waiting := 0
	for _, v := range h.World.Snapshot().Aircraft {
		if cleared[v.Tail] && v.ATC == string(traffic.PosGround) && slices.Contains(v.Actions, "pushstart") {
			waiting++
		}
	}
	if waiting == 0 {
		t.Error("no cleared departure waiting on the staffed ground with pushstart")
	}
}
