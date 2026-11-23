package world

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The director's loop does not wait on the link (#1673). This file uses
// nothing #1673 added, so that it builds and fails without the fix.

// heldDep is an actuator's departure awaiting taxi whose every read, once
// held, waits to be freed. The test's server answers one call at a time:
// while one waits the whole link is quiet.
type heldDep struct {
	place   airport.Occupied
	held    atomic.Bool
	release chan struct{}
}

func (d *heldDep) wait() {
	if d.held.Load() {
		<-d.release
	}
}

func (d *heldDep) State() traffic.TaxiState { d.wait(); return traffic.TaxiAwaitingTaxi }

func (d *heldDep) Route() *airport.Route { d.wait(); return nil }

func (d *heldDep) Occupies() (airport.Occupied, bool) { d.wait(); return d.place, true }

// addHeldDep puts an actuator's departure awaiting taxi on the bed, its
// state sent as each second; hold holds its reads from then on.
func (b *splitBed) addHeldDep(t *testing.T) (it *controlled, hold func()) {
	t.Helper()
	const target = "dep/9"
	at := b.it.graph.Layout.Parking[0].Position
	d := &heldDep{place: airport.Occupied{Points: []airport.LatLon{at}, HalfSpan: 17}, release: make(chan struct{})}
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(d.release) }) })
	b.srv.add(target, d)
	it = &controlled{ID: 9, Tail: "TST9", ICAO: "XMID", cc: b.cc, dep: &remoteDep{c: b.c, t: target}, graph: b.it.graph, removed: make(chan struct{})}
	it.view.Model, it.view.Position = splitModel, at
	b.cc.mu.Lock()
	b.cc.items[9] = it
	b.cc.mu.Unlock()
	b.items = b.cc.snapshotItems()
	b.snapshot()
	for until := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		var st traffic.TaxiState
		if b.c.fromCache(target, "State", []any{&st}) {
			break
		}
		if time.Now().After(until) {
			t.Fatal("no snapshot of the departure")
		}
	}
	return it, func() { d.held.Store(true) }
}

// TestKeepClearDoesNotHoldLoop: with the link quiet, keepClear goes on
// from the actuator's snapshot, the departure awaiting taxi taken where it
// stands.
func TestKeepClearDoesNotHoldLoop(t *testing.T) {
	b := newSplitBed(t, true)
	_, hold := b.addHeldDep(t)
	hold()
	b.onLoop(t, "keep clear", b.cc.keepClear)
	if !strings.Contains(b.cc.keepClearKey, "TST9/") {
		t.Errorf("the departure awaiting taxi not taken: %q", b.cc.keepClearKey)
	}
}

// TestKeepClearStaleNotWaited: a departure whose snapshot a command on its
// way dropped is not asked over the link: keepClear goes on from the last
// one, the departure taken and looked at again once fresh.
func TestKeepClearStaleNotWaited(t *testing.T) {
	b := newSplitBed(t, true)
	_, hold := b.addHeldDep(t)
	end := b.c.commanding("dep/9", "Hold")
	hold()
	b.onLoop(t, "keep clear", b.cc.keepClear)
	stale := b.cc.keepClearKey
	if !strings.Contains(stale, "TST9/") {
		t.Errorf("the departure not taken: %q", stale)
	}
	// Answered again, the command done, a fresh snapshot.
	b.srv.mu.Lock()
	d := b.srv.targets["dep/9"].(*heldDep)
	b.srv.mu.Unlock()
	d.held.Store(false)
	end()
	b.snapshot()
	for until := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		var st traffic.TaxiState
		if b.c.fromCache("dep/9", "State", []any{&st}) {
			break
		}
		if time.Now().After(until) {
			t.Fatal("no fresh snapshot")
		}
	}
	b.onLoop(t, "keep clear again", func() {
		b.cc.keepClearAt = time.Time{}
		b.cc.keepClear()
	})
	if b.cc.keepClearKey == stale {
		t.Error("not looked at again once fresh")
	}
}

// heldTake is an arrival's way-off planning whose take waits to be freed.
type heldTake struct {
	*actuatorWayOff
	asked, release chan struct{}
}

func (h *heldTake) TakeWayOff(step uint64, parking int, occ []airport.Occupied) (bool, error) {
	select {
	case h.asked <- struct{}{}:
	default:
	}
	<-h.release
	return h.actuatorWayOff.TakeWayOff(step, parking, occ)
}

// TestWayOffTakeDoesNotHoldLoop: the take of a way off found waits on the
// link off the director's loop, and is put in place when it is answered.
func TestWayOffTakeDoesNotHoldLoop(t *testing.T) {
	b := newSplitBed(t, true)
	target := b.arr.(*remoteArr).t + wayOffSuffix
	b.srv.mu.Lock()
	real, ok := b.srv.targets[target].(*actuatorWayOff)
	b.srv.mu.Unlock()
	if !ok {
		t.Fatal("no way-off planning served")
	}
	h := &heldTake{actuatorWayOff: real, asked: make(chan struct{}, 1), release: make(chan struct{})}
	var once sync.Once
	free := func() { once.Do(func() { close(h.release) }) }
	t.Cleanup(free)
	b.srv.add(target, h)
	b.snapshot()
	b.onLoop(t, "the look", func() { b.cc.keepWayOffClear(b.items) })
	select {
	case <-h.asked:
	case <-time.After(10 * time.Second):
		t.Fatal("never taken")
	}
	b.onLoop(t, "the loop's next command", func() {})
	free()
	b.looked(t)
	got := b.arr.Plan().Route.Nodes
	if slices.Equal(got, b.first) {
		t.Fatal("way off not taken")
	}
	b.it.mu.Lock()
	view := slices.Clone(b.it.view.Nodes)
	b.it.mu.Unlock()
	if !slices.Equal(view, got) {
		t.Error("the route shown is not the one taken")
	}
}
