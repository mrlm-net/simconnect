package world

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The lock order is cc.mu, then an aircraft's it.mu (views, ownIDs,
// ownTails). Taken the other way round the two wait for each other for good,
// and the offline tiers test behind them (GET /api/traffic) on the goroutine
// that moves the clock: nothing timed out, the package hung until its 40
// minutes were up (CI, TestOverflightTiers on 621e8a89).

// waitBlockedIn waits until a goroutine waits for a mutex in fn, with
// callee also on its stack ("" whatever it called).
func waitBlockedIn(t *testing.T, fn, callee string) {
	t.Helper()
	buf := make([]byte, 1<<20)
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(time.Millisecond) {
		for _, g := range strings.Split(string(buf[:runtime.Stack(buf, true)]), "\n\n") {
			head, stack, _ := strings.Cut(g, "\n")
			if strings.Contains(head, "sync.Mutex.Lock") && strings.Contains(stack, fn) && strings.Contains(stack, callee) {
				return
			}
		}
	}
	t.Fatalf("nothing waits for a mutex in %s %s", fn, callee)
}

// returns reports whether f returns within 5 s.
func returns(f func()) bool {
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

// Departure's level in trail: worked out while the views are read.
func TestLevelInTrailLockOrder(t *testing.T) {
	cc := &controlCenter{clock: traffic.NewSimClock(), items: map[int]*controlled{}}
	dep := func(id int, tail string) *controlled {
		it := &controlled{cc: cc, ICAO: "LKPR", Tail: tail, radarFt: departureClimbFt}
		it.view.Procedure = "BALT4B"
		it.dep = &traffic.TaxiController{}
		cc.items[id] = it
		return it
	}
	ahead, it := dep(1, "RYR1427"), dep(2, "TVS1620")
	ahead.identified, ahead.identifiedAt = true, cc.clock.Now()

	ahead.mu.Lock() // levelInTrail waits here, its list of the others taken
	done := make(chan struct{})
	go func() {
		it.levelInTrail()
		close(done)
	}()
	waitBlockedIn(t, ").levelInTrail(", "")
	cc.mu.Lock() // the views begin
	ahead.mu.Unlock()
	waitBlockedIn(t, ").levelInTrail(", ").taOf(")
	ok := returns(func() {
		it.mu.Lock()
		_ = it.view
		it.mu.Unlock()
	})
	cc.mu.Unlock()
	<-done
	if !ok {
		t.Fatal("levelInTrail holds it.mu waiting for cc.mu: the views, holding cc.mu, wait for it.mu for good")
	}
	if it.radarFt != departureClimbFt-2000 {
		t.Fatalf("in trail: %.0f ft, want %.0f", it.radarFt, departureClimbFt-2000)
	}
}

// The check-in after a go-around is worded in update, under it.mu.
func TestGoAroundLevelLockOrder(t *testing.T) {
	g := &airport.Graph{Layout: &airport.Layout{ICAO: "LKPR", Altitude: 380}}
	cc := &controlCenter{clock: traffic.NewSimClock(), items: map[int]*controlled{}}
	it := &controlled{cc: cc, ICAO: "LKPR", Tail: "WZZ181", graph: g}
	cc.items[1] = it
	want := it.goAroundLevel()

	cc.mu.Lock() // the views begin
	it.mu.Lock() // update
	var got string
	ok := returns(func() { got = it.goAroundLevel() })
	it.mu.Unlock()
	cc.mu.Unlock()
	if !ok {
		t.Fatal("goAroundLevel waits for cc.mu under update's it.mu: the views, holding cc.mu, wait for it.mu for good")
	}
	if got != want {
		t.Fatalf("level said %q, want %q", got, want)
	}
}
