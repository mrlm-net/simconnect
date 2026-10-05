package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestHostAPI: a World not connected answers its API in process, counts
// what Feed drops, and keeps the host's player clearance.
func TestHostAPI(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	var status struct {
		Connected bool `json:"connected"`
	}
	if err := w.Get("/api/status", &status); err != nil || status.Connected {
		t.Fatalf("status %+v %v", status, err)
	}
	if s := w.Snapshot(); s.Connected || len(s.Aircraft) != 0 {
		t.Errorf("snapshot %+v", s)
	}
	if _, err := w.Do("POST", "/api/control/1/taxi", nil); err == nil {
		t.Error("a clearance with nothing connected: no error")
	}
	for range DefaultQueueSize + 3 {
		w.Feed(engine.Message{})
	}
	if d := w.Snapshot().Dropped; d != 3 {
		t.Errorf("dropped %d, want 3", d)
	}
	w.ClearPlayer(PlayerClearance{ICAO: "lkpr", Runway: "24", Phase: PlayerLanding})
	if _, ok := w.st.core.playerOn("LKPR", "24"); !ok {
		t.Error("player not on 24")
	}
	if _, ok := w.st.core.playerLanding(); !ok {
		t.Error("player not landing")
	}
	// The tower asks by runway name: the host gives an end (#739).
	if _, ok := w.st.core.playerOn("LKPR", "06/24"); !ok {
		t.Error("player not on 06/24")
	}
	if _, ok := w.st.core.playerOn("LKPR", "12/30"); ok {
		t.Error("player on 12/30")
	}
	// Pushing back: ours near it wait; far off, not; taxiing, not.
	w.st.core.setUserAt(airport.LatLon{Lat: 50.1, Lon: 14.26})
	w.ClearPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerPushback})
	if !w.st.core.playerPushingNear("LKPR", airport.LatLon{Lat: 50.1005, Lon: 14.26}, playerPushClearM) {
		t.Error("55 m from the pushing player: not held")
	}
	if w.st.core.playerPushingNear("LKPR", airport.LatLon{Lat: 50.11, Lon: 14.26}, playerPushClearM) {
		t.Error("1.1 km from the pushing player: held")
	}
	if _, ok := w.st.core.playerOn("LKPR", "24"); ok {
		t.Error("pushing back: on the runway")
	}
	w.ClearPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerTaxi})
	if w.st.core.playerPushingNear("LKPR", airport.LatLon{Lat: 50.1005, Lon: 14.26}, playerPushClearM) {
		t.Error("taxiing: still held")
	}
	w.ClearPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerVacated})
	if _, ok := w.st.core.playerOn("LKPR", "24"); ok {
		t.Error("vacated: still on 24")
	}
	w.Heard(traffic.Transmission{Airport: "LKPR", Frequency: "118.100", Text: "x"}) // not connected: nothing
}

// TestIDBase: the library helpers move to IDBase, else keep the defaults.
func TestIDBase(t *testing.T) {
	if ids := New(Options{DataDir: t.TempDir(), IDBase: 0xA000}).st.core.libIDs(); ids.loaderDef != 0xA000 || ids.procReq != 0xA000+300 || ids.injEvt != 0xA000+900 {
		t.Errorf("%+v", ids)
	}
	if ids := New(Options{DataDir: t.TempDir()}).st.core.libIDs(); ids.procDef != 8400 || ids.airportList != traffic.DefaultAirportListRequestID {
		t.Errorf("defaults %+v", ids)
	}
}

// TestPushbackAPI: a drawn push is set, saved beside the settings and
// read again by a new World; DELETE clears it.
func TestPushbackAPI(t *testing.T) {
	dir := t.TempDir()
	w := New(Options{DataDir: dir})
	p := traffic.PushRoute{Points: []airport.LatLon{{Lat: 50.1, Lon: 14.26}, {Lat: 50.1005, Lon: 14.2605}}, Facing: 90}
	if _, err := w.Do("PUT", "/api/pushback?icao=LKPR&stand=S6", p); err != nil {
		t.Fatal(err)
	}
	if r, ok := traffic.CustomPush("LKPR", "s6"); !ok || r.Said != "east" {
		t.Fatalf("not set: %+v %v", r, ok)
	}
	traffic.ClearCustomPush("LKPR", "S6")
	New(Options{DataDir: dir}) // read again at start
	if _, ok := traffic.CustomPush("LKPR", "S6"); !ok {
		t.Error("not kept")
	}
	if _, err := w.Do("DELETE", "/api/pushback?icao=LKPR&stand=S6", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := traffic.CustomPush("LKPR", "S6"); ok {
		t.Error("not cleared")
	}
}

// The night: 22:00 to 06:00 sim local time (#722).
func TestNight(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	cc := &controlCenter{core: w.st.core}
	for _, c := range []struct {
		sec   float64
		night bool
	}{{0, false}, {23 * 3600, true}, {3 * 3600, true}, {12 * 3600, false}, {6 * 3600, false}} {
		w.st.core.setLocalSec(c.sec)
		if got := cc.night(); got != c.night {
			t.Errorf("%v s: night %v", c.sec, got)
		}
	}
}
