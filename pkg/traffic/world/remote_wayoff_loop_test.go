package world

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The way-off check over a link is off the director's loop (#1655).

// heldCheck is an arrival's way-off planning whose check answers "clear"
// once released. The test's server answers one call at a time: until then
// the whole link is quiet.
type heldCheck struct {
	*actuatorWayOff
	asked, release chan struct{}
}

func (h *heldCheck) CheckWayOff([]airport.Occupied) (bool, error) {
	select {
	case h.asked <- struct{}{}:
	default:
	}
	<-h.release
	return true, nil
}

// holdCheck holds the bed's check; free lets it answer.
func (b *splitBed) holdCheck(t *testing.T) (asked <-chan struct{}, free func()) {
	t.Helper()
	target := b.arr.(*remoteArr).t + wayOffSuffix
	b.srv.mu.Lock()
	real, ok := b.srv.targets[target].(*actuatorWayOff)
	b.srv.mu.Unlock()
	if !ok {
		t.Fatal("no way-off planning served")
	}
	h := &heldCheck{actuatorWayOff: real, asked: make(chan struct{}, 1), release: make(chan struct{})}
	b.srv.add(target, h)
	var once sync.Once
	free = func() { once.Do(func() { close(h.release) }) }
	t.Cleanup(free)
	return h.asked, free
}

// onLoop runs f on the director's loop, which must be done with it within
// two seconds.
func (b *splitBed) onLoop(t *testing.T, what string, f func()) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- b.cc.do(func() error { f(); return nil }) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: the director's loop waits for the actuator", what)
	}
}

// looked waits for the look over the link to end, and any search of it.
func (b *splitBed) looked(t *testing.T) {
	t.Helper()
	for until := time.Now().Add(10 * time.Second); b.it.wayOffLooking.Load() || b.it.wayOffPlanning.Load(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(until) {
			t.Fatal("the look never ended")
		}
	}
}

// TestWayOffCheckDoesNotHoldLoop: with the check's reply held open the
// director's loop goes on, the unanswered check is not "no way off", and
// the answer is put in place when it comes.
func TestWayOffCheckDoesNotHoldLoop(t *testing.T) {
	b := newSplitBed(t, true)
	asked, free := b.holdCheck(t)
	look := func() { b.cc.keepWayOffClear(b.items) }
	b.onLoop(t, "the look", look)
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("never asked")
	}
	b.onLoop(t, "the loop's next command", func() {})
	b.it.wayOffBlocked.Store(true)
	b.it.apronBlocked.Store(true)
	b.onLoop(t, "the next second's look", look)
	if b.it.wayOffBlocked.Load() {
		t.Error("an unanswered check held it off the runway")
	}
	free()
	b.looked(t)
	if b.it.apronBlocked.Load() {
		t.Error("the answer was not put in place")
	}
}

// TestWayOffLateAnswerDropped: an answer that comes after the departure it
// was asked about has moved is not put in place.
func TestWayOffLateAnswerDropped(t *testing.T) {
	b := newSplitBed(t, true)
	asked, free := b.holdCheck(t)
	b.it.wayOffBlocked.Store(true)
	b.it.apronBlocked.Store(true)
	b.onLoop(t, "the look", func() { b.cc.keepWayOffClear(b.items) })
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("never asked")
	}
	for _, o := range b.items {
		if o.dep == nil {
			continue
		}
		o.mu.Lock()
		lat, lon := calc.DisplaceByHeading(o.view.Position.Lat, o.view.Position.Lon, 0, 50)
		o.view.Position = airport.LatLon{Lat: lat, Lon: lon}
		o.mu.Unlock()
	}
	free()
	b.looked(t)
	if !b.it.wayOffBlocked.Load() || !b.it.apronBlocked.Load() {
		t.Error("a stale answer put in place")
	}
}

// TestWayOffLostCallNotBlocked (#1655): an actuator that stops answering
// the planning is not "no way off", which would send the arrival round.
func TestWayOffLostCallNotBlocked(t *testing.T) {
	b := newSplitBed(t, true)
	b.srv.remove(b.arr.(*remoteArr).t + wayOffSuffix)
	b.it.wayOffBlocked.Store(true)
	b.look(t)
	b.looked(t)
	if b.it.wayOffBlocked.Load() {
		t.Error("a lost call held it off the runway")
	}
	if got := b.arr.Plan().Route.Nodes; !slices.Equal(got, b.first) {
		t.Error("re-planned though nothing answered")
	}
}
