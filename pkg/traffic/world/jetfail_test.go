package world

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A gate whose dock timed out for a type has no jetway for that type: the
// next departure of the type there gets stairs, a GPU and buses as on a
// remote stand, other types still the jetway; remembered over a restart,
// tried again after JetwayTypeRetest, cleared by a dock of the type. A
// gate resting after failed docks gives every type stairs (live, LKPR C21
// 2026-11-08: SWR707 A321, CSA871 BCS3 and SWR662 A20N each waited for a
// jetway that never came).
func TestGateWithoutJetwayGetsStairs(t *testing.T) {
	dir := t.TempDir()
	w := New(Options{DataDir: dir})
	t.Cleanup(func() { gateLessons.saving.Wait(); gateLessons.load("") })
	cc := &controlCenter{core: w.st.core}
	cc.fuelTitles.stairsStock = []string{"ASO_Boarding_Stairs"}
	cc.fuelTitles.busesStock = []string{"Bus Apron 02"}
	cc.fuelTitles.gpusStock = []string{"ASO_GPU"}
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
	if gate < 0 {
		t.Fatal("no gate at LKPR")
	}
	icao := g.Layout.ICAO
	dep := func(model string) SpawnRequest {
		return SpawnRequest{Kind: "departure", Stand: gate, Model: model, StandUse: standAirline}
	}
	services := func(model string) (stairs, gpu bool, buses int) {
		prof := traffic.ProfileFor(model).Motion
		s := cc.stairs(dep(model), g, 0, prof)
		u := cc.gpu(dep(model), g, 0, prof)
		if s == nil {
			return false, u != nil, 0
		}
		return true, u != nil, len(cc.buses(g, 0, prof, s))
	}
	if s, u, b := services("A321"); s || u || b != 0 {
		t.Fatalf("at a gate before any failure: stairs %v, GPU %v, %d buses", s, u, b)
	}

	// Its dock times out (jetway.HelperOptions.OnDock): once is not enough
	// (a sim session failing every jetway a while), twice is.
	cc.dockResult(icao, gate, "A321", false)
	if s, _, _ := services("A321"); s {
		t.Fatal("stairs after one timed-out dock")
	}
	cc.dockResult(icao, gate, "A321", false)
	if s, u, b := services("A321"); !s || !u || b == 0 {
		t.Fatalf("A321 after a timed-out dock: stairs %v, GPU %v, %d buses; want all", s, u, b)
	}
	if !cc.gateWithoutJetway(icao, gate, "FSLTL_FAIB_A321_SWR-Swiss") {
		t.Error("an A321 by its title still counts the jetway")
	}
	if s, _, _ := services("A20N"); s {
		t.Error("an A20N lost the jetway too")
	}

	// A restart remembers it.
	gateLessons.saving.Wait()
	gateLessons.load(filepath.Join(dir, "jetway-gates.json"))
	if s, _, _ := services("A321"); !s {
		t.Fatal("forgotten after a restart")
	}

	// Tried again after JetwayTypeRetest.
	if gateLessons.noJetway(icao, gate, "A321", traffic.Now().Add(JetwayTypeRetest+time.Minute)) {
		t.Error("never tried again")
	}

	// One A321 docked there: its jetway counts again, over a restart too.
	cc.dockResult(icao, gate, "A321", true)
	gateLessons.saving.Wait()
	gateLessons.load(filepath.Join(dir, "jetway-gates.json"))
	if s, _, _ := services("A321"); s {
		t.Error("stairs after a dock")
	}

	// A resting gate (docks gone from docking to failed): no jetway for any type.
	k := gateKey(icao, gate)
	now := traffic.Now()
	cc.jw.mu.Lock()
	for i := 0; i < JetwayGateFails; i++ {
		cc.jw.gateResting(k, "docking", now)
		cc.jw.gateResting(k, "failed", now)
	}
	cc.jw.mu.Unlock()
	if s, _, _ := services("B738"); !s {
		t.Error("no stairs at a resting gate")
	}
}
