package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/jetway"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A departure on a gate resting after failed docks (no jetway will come)
// gets stairs and buses as on a remote stand (live, LKPR C21 2026-11-08:
// "resting after 3 failed docks, no jetway for SWR662", nothing else came).
func TestRestingGateGetsStairs(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	cc := &controlCenter{core: w.st.core}
	cc.fuelTitles.stairsStock = []string{"ASO_Boarding_Stairs"}
	cc.fuelTitles.busesStock = []string{"Bus Apron 02"}
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	gate := -1
	for i, p := range g.Layout.Parking {
		if p.IsGate() && p.Radius >= 20 {
			gate = i
			break
		}
	}
	r := SpawnRequest{Kind: "departure", Stand: gate, Model: "A20N", StandUse: standAirline}
	prof := traffic.ProfileFor("A20N").Motion
	if s := cc.stairs(r, g, 0, prof); s != nil {
		t.Fatal("stairs at a gate whose jetway works")
	}
	k := gateKey(g.Layout.ICAO, gate)
	now := traffic.Now()
	cc.jw.mu.Lock()
	for i := 0; i < JetwayGateFails; i++ {
		cc.jw.gateResting(k, jetway.StatusDocking, now)
		cc.jw.gateResting(k, jetway.StatusFailed, now)
	}
	cc.jw.mu.Unlock()
	s := cc.stairs(r, g, 0, prof)
	if s == nil {
		t.Fatal("no stairs at a gate resting after failed docks")
	}
	if b := cc.buses(g, 0, prof, s); len(b) == 0 {
		t.Error("no bus with the stairs")
	}
}
