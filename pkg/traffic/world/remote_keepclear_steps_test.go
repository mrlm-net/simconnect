package world

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// TestKeepClearUnansweredNotClear (#1673): a departure whose snapshot a
// command on its way dropped is not asked over the link, and not counted
// clear: it takes a place by its last state, marked to be looked at again.
func TestKeepClearUnansweredNotClear(t *testing.T) {
	b := newSplitBed(t, true)
	it, hold := b.addHeldDep(t)
	defer b.c.commanding("dep/9", "Hold")()
	hold()
	type answer struct {
		key string
		ok  bool
	}
	done := make(chan answer, 1)
	go func() {
		_, key, ok := takenBy(it)
		done <- answer{key, ok}
	}()
	select {
	case a := <-done:
		if !a.ok || !strings.HasSuffix(a.key, "~") {
			t.Errorf("taken %v, state %q; want taken, its last state marked", a.ok, a.key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("asked over the link")
	}
}

// TestWayOffTakeNotAPlayers (#1096, #1673): a flight that became a player's
// after its way off was found over the link is not taken.
func TestWayOffTakeNotAPlayers(t *testing.T) {
	b := newSplitBed(t, true)
	b.it.gates.Store(true)
	taken := false
	b.cc.takeOverLink(b.it, 0, 0, false, splitModel, func() bool { taken = true; return true })
	if taken {
		t.Error("a player's flight taken")
	}
	if got := b.arr.Plan().Route.Nodes; !slices.Equal(got, b.first) {
		t.Error("a player's flight re-planned")
	}
}
