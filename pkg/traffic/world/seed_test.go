package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestSeed: a snapshot's parked aircraft depart after a turnaround, an
// inbound one arrives, a taxiing one and one at no airport of ours are
// skipped with why; each flight is marked seeded (not real); the budget
// keeps the nearest.
func TestSeed(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	l := lkprLayout(t)
	w.st.cache.Put(l)
	cc := &controlCenter{core: w.st.core, log: w.st.core.log, clock: traffic.NewSimClock(), items: map[int]*controlled{}}
	mgr := traffic.NewTrafficManager(&holdSpawner{}, traffic.ManagerOptions{})
	mgr.SetAirports("LKPR")
	s := &scheduler{st: w.st, cc: cc, mgr: mgr}
	at := time.Now()
	stand := l.Parking[0].Position
	obs := []traffic.Observed{
		{ID: "a1", Callsign: "CSA123", Type: "A320", Lat: stand.Lat, Lon: stand.Lon, OnGround: true, Destination: "EDDF"},
		{ID: "a2", Callsign: "DLH1402", Type: "A320", Lat: 50.30, Lon: 14.26, AltFt: 6000, GroundKts: 220, TrackDeg: 180, VSFpm: -800},
		{ID: "a3", Callsign: "EZY1", Type: "A320", Lat: l.Latitude, Lon: l.Longitude, OnGround: true, GroundKts: 15},
		{ID: "a4", Callsign: "UAL1", Type: "B77W", Lat: 40, Lon: -74, AltFt: 35000, GroundKts: 480},
	}
	res := s.seedSnapshot(obs, at)
	if res.Placed != 2 || res.Skipped != 2 {
		t.Fatalf("placed %d skipped %d: %+v", res.Placed, res.Skipped, res.Results)
	}
	dep, ok := mgr.Flight("departure", "CSA123")
	if !ok || dep.Destination != "EDDF" || dep.Observed == nil || !dep.Observed.Seeded || dep.STD.Before(cc.clock.Now().Add(30*time.Minute)) {
		t.Errorf("parked: %+v", dep)
	}
	if arr, ok := mgr.Flight("arrival", "DLH1402"); !ok || !arr.Observed.Seeded {
		t.Errorf("inbound: %+v", arr)
	}
	for _, r := range res.Results {
		if r.Callsign == "EZY1" && r.Reason == "" {
			t.Error("taxiing skipped without why")
		}
	}
	// The budget: room for one.
	mgr2 := traffic.NewTrafficManager(&holdSpawner{}, traffic.ManagerOptions{MaxAircraft: 1})
	mgr2.SetAirports("LKPR")
	s2 := &scheduler{st: w.st, cc: cc, mgr: mgr2}
	if res := s2.seedSnapshot(obs[:2], at); res.Placed != 1 || res.Results[0].Callsign != "CSA123" {
		t.Errorf("budget: %+v", res)
	}
}
