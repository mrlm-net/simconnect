package world

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// pumpCmds runs cc's commands as its connection goroutine does, until the
// test ends.
func pumpCmds(t *testing.T, cc *controlCenter) {
	t.Helper()
	cc.cmds = make(chan func())
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
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
}

// heldBed is a scheduler on a control center whose commands the test runs.
func heldBed(t *testing.T) (*scheduler, *controlCenter) {
	t.Helper()
	w := New(Options{DataDir: t.TempDir()})
	cc := &controlCenter{core: w.st.core, log: w.st.core.log, clock: traffic.NewSimClock(), items: map[int]*controlled{}, cmds: make(chan func()), ids: traffic.NewIDBlocks(1000, 2000, controlIDBlock, 4)}
	mgr := traffic.NewTrafficManager(&holdSpawner{}, traffic.ManagerOptions{})
	mgr.SetAirports("LKPR")
	return &scheduler{st: w.st, cc: cc, mgr: mgr}, cc
}

// putIn is a spawn's step on the connection goroutine: its aircraft in.
func putIn(cc *controlCenter, id int, tail string) func() error {
	return func() error {
		cc.mu.Lock()
		cc.items[id] = &controlled{ID: id, Tail: tail, defBase: 1000, cc: cc, stands: traffic.NewStandAllocator(nil, &airport.Graph{Layout: &airport.Layout{}}), removed: make(chan struct{})}
		cc.mu.Unlock()
		return nil
	}
}

// TestHoldWaitsForSpawnUnderWay: a spawn whose aircraft goes in on the
// connection goroutine while the hold comes is either in before the
// hold's pass over the traffic, and taken out by it, or not put in at all.
// Before, one put in after the pass was there while held until its spawn
// took it out again (race run, TestHoldKeepsTrafficOut: "1 aircraft
// spawned while held", #1283).
func TestHoldWaitsForSpawnUnderWay(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before bool // its step run before the hold
	}{{"in before the hold", true}, {"asked as the hold comes", false}} {
		t.Run(tc.name, func(t *testing.T) {
			s, cc := heldBed(t)
			spawned := make(chan error, 1)
			go func() { spawned <- s.spawnUnlessHeld(putIn(cc, 1, "CSA1")) }()
			var step func()
			select {
			case step = <-cc.cmds:
			case <-time.After(5 * time.Second):
				t.Fatal("the spawn asked nothing of the connection")
			}
			if tc.before {
				step()
			}
			held := make(chan HoldResult, 1)
			go func() { held <- s.hold(true) }()
			for until := time.Now().Add(5 * time.Second); !s.isHeld(); time.Sleep(time.Millisecond) {
				if time.Now().After(until) {
					t.Fatal("never held")
				}
			}
			if !tc.before {
				step()
			}
			// The connection goroutine from here on, until the hold is done.
			var res HoldResult
			for done := false; !done; {
				select {
				case f := <-cc.cmds:
					f()
				case res = <-held:
					done = true
				case <-time.After(5 * time.Second):
					t.Fatal("the hold never finished")
				}
			}
			err := <-spawned
			if it := cc.byTail("CSA1"); it != nil {
				t.Errorf("in while held, after the hold's pass (spawn: %v, hold: %+v)", err, res)
			}
			if !tc.before && !errors.Is(err, ErrHeld) {
				t.Errorf("spawn as the hold came: %v, want %v", err, ErrHeld)
			}
		})
	}
}

// TestHeldSpawnRefused: a flight the manager hands over after a hold (its
// tick collects its spawns under its lock and hands them over after it)
// is not spawned at all, and its flight is done; a spawn the hold refused
// on the way is done too, not failed and tried again (#1283).
func TestHeldSpawnRefused(t *testing.T) {
	s, cc := heldBed(t)
	pumpCmds(t, cc)
	s.mgr.SetEnabled(true)
	now := cc.clock.Now()
	s.mgr.Add([]traffic.Flight{
		{Callsign: "CSA1", Type: "A320", Origin: "LKPR", Destination: "EDDF", STD: now.Add(time.Minute)},
		{Callsign: "CSA2", Type: "A320", Origin: "LKPR", Destination: "EDDF", STD: now.Add(time.Minute)},
	})
	s.mgr.Tick(now)
	spawning := func(cs string) traffic.ManagedFlight {
		t.Helper()
		f, ok := s.mgr.Flight("departure", cs)
		if !ok || f.Status != traffic.FlightSpawning {
			t.Fatalf("%s not spawning: %+v", cs, f)
		}
		return f
	}
	f1 := spawning("CSA1")
	setHeld := func(on bool) {
		s.st.mu.Lock()
		s.st.held = on
		s.st.mu.Unlock()
	}
	setHeld(true)
	ran := false
	s.runSpawn(f1, func() error {
		ran = true
		return s.spawnUnlessHeld(putIn(cc, 1, "CSA1"))
	})
	if ran {
		t.Errorf("CSA1 spawned while held")
	}
	if it := cc.byTail("CSA1"); it != nil {
		t.Errorf("CSA1 there while held")
	}
	if f, _ := s.mgr.Flight("departure", "CSA1"); f.Status != traffic.FlightDone {
		t.Errorf("CSA1 %v, want done", f.Status)
	}
	// Held as it spawns: refused at the connection. The manager hands
	// one departure over a tick.
	setHeld(false)
	for at := now; ; at = at.Add(10 * time.Second) {
		if f, _ := s.mgr.Flight("departure", "CSA2"); f.Status != traffic.FlightScheduled || at.Sub(now) > 10*time.Minute {
			break
		}
		s.mgr.Tick(at)
	}
	f2 := spawning("CSA2")
	s.runSpawn(f2, func() error {
		setHeld(true)
		return s.spawnUnlessHeld(putIn(cc, 2, "CSA2"))
	})
	if it := cc.byTail("CSA2"); it != nil {
		t.Errorf("CSA2 there while held")
	}
	if f, _ := s.mgr.Flight("departure", "CSA2"); f.Status != traffic.FlightDone {
		t.Errorf("CSA2 %v, want done (not tried again)", f.Status)
	}
}

// pendingSim notes the enroute aircraft flown and removed.
type pendingSim struct {
	*routeSim
	mu             sync.Mutex
	flown, removed []uint32
}

func (p *pendingSim) FlyEnroute(_, objectID uint32, _ []types.SIMCONNECT_DATA_WAYPOINT) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.flown = append(p.flown, objectID)
	return nil
}

func (p *pendingSim) RemoveObject(objectID, _ uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.removed = append(p.removed, objectID)
	return nil
}

// TestHoldDropsPendingEnroute: an enroute aircraft asked for, the
// simulator not answering yet, removed (a hold, or its spawn finding the
// traffic held) is taken out once the simulator creates it. Before, it
// was found nowhere and came in as ours beside the replay, later handed
// over as an arrival while held (#1283).
func TestHoldDropsPendingEnroute(t *testing.T) {
	b := newMeetingBed(t)
	sim := &pendingSim{routeSim: b.sim}
	b.s.cc.sim = sim
	b.s.mgr = traffic.NewTrafficManager(b.s, traffic.ManagerOptions{})
	const req, obj = uint32(7001), uint32(777)
	f := traffic.ManagedFlight{Flight: traffic.Flight{Callsign: "SWR123"}, Kind: "arrival", Airport: "LKPR", Stage: "enroute"}
	e := &enrouteAC{f: f, reqID: req, pendingAt: traffic.Now(), cruiseKts: 450}
	b.s.mu.Lock()
	b.s.pending[req] = e
	b.s.mu.Unlock()
	b.s.Remove(f)
	b.s.handle(assignedObject(req, obj))
	b.s.mu.Lock()
	listed := b.s.enroute["SWR123"] != nil
	b.s.mu.Unlock()
	if listed {
		t.Errorf("SWR123 came in as ours after it was removed")
	}
	sim.mu.Lock()
	defer sim.mu.Unlock()
	if !slices.Contains(sim.removed, obj) || len(sim.flown) > 0 {
		t.Errorf("object %d: removed %v, flown %v; want it removed, not flown", obj, sim.removed, sim.flown)
	}
}
