package world

import (
	"testing"
	"time"
)

// onStand runs under it.mu (onRequest): it must not lock it again (live,
// the "stand B1, request taxi" call deadlocked the whole map).
func TestOnStandUnderLock(t *testing.T) {
	it := &controlled{stand: -1}
	done := make(chan string)
	go func() {
		it.mu.Lock()
		defer it.mu.Unlock()
		done <- it.onStand()
	}()
	select {
	case s := <-done:
		if s != "" {
			t.Fatalf("onStand = %q, want \"\" without a stand", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onStand deadlocked under it.mu")
	}
}
