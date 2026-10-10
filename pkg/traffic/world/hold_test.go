package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

type holdSpawner struct{ removed []string }

func (h *holdSpawner) Spawn(traffic.ManagedFlight)    {}
func (h *holdSpawner) Remove(f traffic.ManagedFlight) { h.removed = append(h.removed, f.Callsign) }

// TestHold: holding stops the schedule and removes the flights in the
// simulator (those not yet there wait); letting go gives the schedule
// back as it was; holding twice does nothing more.
func TestHold(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	cc := &controlCenter{core: w.st.core, log: w.st.core.log, clock: traffic.NewSimClock(), items: map[int]*controlled{}}
	sp := &holdSpawner{}
	mgr := traffic.NewTrafficManager(sp, traffic.ManagerOptions{})
	mgr.SetAirports("LKPR")
	mgr.SetEnabled(true)
	now := cc.clock.Now()
	mgr.Add([]traffic.Flight{
		{Callsign: "CSA1", Type: "A320", Origin: "LKPR", Destination: "EDDF", STD: now.Add(time.Minute)},
		{Callsign: "CSA2", Type: "A320", Origin: "LKPR", Destination: "EDDF", STD: now.Add(5 * time.Hour)},
	})
	mgr.Tick(now)
	s := &scheduler{cc: cc, mgr: mgr}
	res := s.hold(true)
	if !res.Held || res.Removed != 1 || mgr.Enabled() || len(sp.removed) != 1 || sp.removed[0] != "CSA1" {
		t.Fatalf("hold: %+v, enabled %v, removed %v", res, mgr.Enabled(), sp.removed)
	}
	if again := s.hold(true); again.Removed != 0 {
		t.Errorf("held twice: %+v", again)
	}
	if f, ok := mgr.Flight("departure", "CSA2"); !ok || f.Status != traffic.FlightScheduled {
		t.Errorf("a later flight touched: %+v", f)
	}
	if res := s.hold(false); res.Held || !mgr.Enabled() {
		t.Errorf("let go: %+v, enabled %v", res, mgr.Enabled())
	}
}
