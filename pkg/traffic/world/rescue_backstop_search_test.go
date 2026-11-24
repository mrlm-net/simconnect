package world

import (
	"testing"
	"time"
)

// The backstop's searches (#1654): each is route searches, so they run off
// the connection's goroutine, and one that found no way is not run again
// each second.

// searchesHeld has b's searches counted on asked and held until release.
func searchesHeld(b *backstopBed) (asked chan struct{}, release func()) {
	asked = make(chan struct{}, 4096)
	held := make(chan struct{})
	released := false
	b.cc.rescue.findMoves = func(a rescueMoveAsk) []rescueMove {
		asked <- struct{}{}
		<-held
		return findRescueMoves(a)
	}
	return asked, func() {
		if !released {
			released = true
			close(held)
		}
	}
}

// TestRescueBackstopSearches: with no way off, a search as it is planned,
// then one each wayOffRetry at most, and one at once as the plan changes.
func TestRescueBackstopSearches(t *testing.T) {
	b := newBackstopBed(t)
	for _, tr := range b.crew.trucks {
		tr.Layout = nil
	}
	b.plan()
	asked, release := searchesHeld(b)
	release()

	b.now = b.now.Add(100 * time.Millisecond)
	b.step(true)
	if n := len(asked); n != 1 {
		t.Fatalf("%d searches as the way was planned, want 1", n)
	}
	const during = time.Minute
	b.wait(during, nil)
	most := 1 + int(during/wayOffRetry)
	n := len(asked)
	if n < 2 || n > most {
		t.Fatalf("%d searches in %v with no way off, want 2 to %d", n, during, most)
	}
	if len(b.standing()) != 2 {
		t.Errorf("%d reported standing while it backs off, want both", len(b.standing()))
	}

	// Planned again: at the next look, whatever is left of the back-off.
	p := *b.arr.plan
	b.arr.plan = &p
	b.now = b.now.Add(100 * time.Millisecond)
	b.step(true)
	if got := len(asked) - n; got != 1 {
		t.Errorf("%d searches as the plan changed, want 1", got)
	}
}

// TestRescueBackstopFramesDontWait: with a search still running the steps
// and frames return, it is the only one, the vehicles stay reported, and
// what it finds is put in place by a frame if the plan is still the one it
// was for.
func TestRescueBackstopFramesDontWait(t *testing.T) {
	for _, replanned := range []bool{false, true} {
		name := "found"
		if replanned {
			name = "planned again meanwhile"
		}
		t.Run(name, func(t *testing.T) {
			b := newBackstopBed(t)
			b.plan()
			asked, release := searchesHeld(b)
			defer release()
			b.unsettled = true

			done := make(chan struct{})
			go func() {
				defer close(done)
				// Three seconds of frames and steps.
				for k := 0; k < 90; k++ {
					b.now = b.now.Add(backstopFrame)
					b.step(k%30 == 0)
				}
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				release()
				<-done
				t.Fatal("the steps and frames waited for the search")
			}
			select {
			case <-asked:
			case <-time.After(5 * time.Second):
				t.Fatal("no search: nothing checked")
			}
			if n := len(asked); n != 0 {
				t.Errorf("%d searches more while one was running", n)
			}
			if n := len(b.standing()); n != 2 {
				t.Errorf("%d reported standing while it runs, want both", n)
			}
			moving := func() int {
				n := 0
				for _, tr := range b.crew.trucks {
					if _, route, ok := tr.Track(); ok && len(route) >= 2 {
						n++
					}
				}
				return n
			}
			if n := moving(); n != 0 {
				t.Fatalf("%d vehicles moving before the search has ended", n)
			}
			if replanned {
				p := *b.arr.plan
				b.arr.plan = &p
			}
			release()
			b.settle()
			b.now = b.now.Add(backstopFrame)
			b.step(false)
			switch n := moving(); {
			case replanned && n != 0:
				t.Errorf("%d vehicles sent by a search for a plan gone", n)
			case !replanned && n == 0:
				t.Error("no vehicle moves in the frame after the search ended")
			}
		})
	}
}
