package world

import (
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

type holdSpawner struct{ removed []string }

func (h *holdSpawner) Spawn(traffic.ManagedFlight)    {}
func (h *holdSpawner) Remove(f traffic.ManagedFlight) { h.removed = append(h.removed, f.Callsign) }

// TestHoldDuringTick: a hold while the manager's tick asks for the weather
// goes through. The tick takes the World's lock under the manager's, and
// the hold took them the other way round: the whole World stopped (#1318).
func TestHoldDuringTick(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	cc := &controlCenter{core: w.st.core, log: w.st.core.log, clock: traffic.NewSimClock(), items: map[int]*controlled{}}
	inTick, holding := make(chan struct{}), make(chan struct{})
	var once sync.Once
	mgr := traffic.NewTrafficManager(&holdSpawner{}, traffic.ManagerOptions{Conditions: func(string) (traffic.ApproachConditions, bool) {
		once.Do(func() {
			close(inTick)
			<-holding
			time.Sleep(100 * time.Millisecond) // the hold is at its locks
		})
		w.st.mu.Lock()
		defer w.st.mu.Unlock()
		return traffic.ApproachConditions{}, false
	}})
	mgr.SetAirports("LKPR")
	mgr.SetEnabled(true)
	now := cc.clock.Now()
	mgr.Add([]traffic.Flight{{Callsign: "CSA1", Type: "A320", Origin: "LKPR", Destination: "EDDF", STD: now.Add(time.Minute)}})
	go mgr.Tick(now)
	select {
	case <-inTick:
	case <-time.After(5 * time.Second):
		t.Fatal("the tick did not ask for the weather")
	}
	s := &scheduler{st: w.st, cc: cc, mgr: mgr}
	done := make(chan struct{})
	go func() {
		close(holding)
		s.hold(true)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: the hold and the tick wait for each other")
	}
}

// TestHoldDuringSpawn: an aircraft whose spawn finishes after the hold's
// pass over the traffic goes again, whether that pass marked its flight
// done or the manager handed it over just after (#1318).
func TestHoldDuringSpawn(t *testing.T) {
	for _, tc := range []struct {
		name string
		hold func(s *scheduler)
	}{
		{"flight seen by the hold", func(s *scheduler) { s.hold(true) }},
		{"flight handed over after it", func(s *scheduler) {
			s.st.mu.Lock()
			s.st.held = true
			s.st.mu.Unlock()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := New(Options{DataDir: t.TempDir()})
			cc := &controlCenter{core: w.st.core, log: w.st.core.log, clock: traffic.NewSimClock(), items: map[int]*controlled{}, cmds: make(chan func()), ids: traffic.NewIDBlocks(1000, 2000, controlIDBlock, 4)}
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				for {
					select {
					case f := <-cc.cmds:
						f()
					case <-stop:
						return
					}
				}
			}()
			mgr := traffic.NewTrafficManager(&holdSpawner{}, traffic.ManagerOptions{})
			mgr.SetAirports("LKPR")
			mgr.SetEnabled(true)
			now := cc.clock.Now()
			mgr.Add([]traffic.Flight{{Callsign: "CSA1", Type: "A320", Origin: "LKPR", Destination: "EDDF", STD: now.Add(time.Minute)}})
			mgr.Tick(now)
			f, ok := mgr.Flight("departure", "CSA1")
			if !ok || f.Status != traffic.FlightSpawning {
				t.Fatalf("not spawning: %+v", f)
			}
			s := &scheduler{st: w.st, cc: cc, mgr: mgr}
			tc.hold(s)
			s.runSpawn(f, func() error {
				cc.mu.Lock()
				cc.items[1] = &controlled{ID: 1, Tail: "CSA1", defBase: 1000, cc: cc, stands: traffic.NewStandAllocator(nil, &airport.Graph{Layout: &airport.Layout{}}), removed: make(chan struct{})}
				cc.mu.Unlock()
				return nil
			})
			if it := cc.byTail("CSA1"); it != nil {
				t.Errorf("aircraft still there while held")
			}
			if f, _ := mgr.Flight("departure", "CSA1"); f.Status != traffic.FlightDone {
				t.Errorf("flight %v, want done", f.Status)
			}
		})
	}
}

// TestHold: holding stops the schedule and removes the flights in the
// simulator (those not yet there wait); letting go gives the schedule
// back as it was; held again, only what came in meanwhile is removed
// (none here).
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
	s := &scheduler{st: w.st, cc: cc, mgr: mgr}
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
