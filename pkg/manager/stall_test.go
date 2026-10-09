//go:build windows

package manager

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStall: silent while Available → a stall; a message → its end; a
// flight loaded just after → a new session.
func TestStall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &Instance{config: &Config{StallAfter: 1500 * time.Millisecond}, logger: slog.Default(), ctx: ctx}
	m.state = StateAvailable
	var mu sync.Mutex
	var got []StallEvent
	m.OnStall(func(e StallEvent) { mu.Lock(); got = append(got, e); mu.Unlock() })
	m.stallSeen()
	time.Sleep(3 * time.Second)
	if !m.Stalled() {
		t.Fatal("silent 3 s: not stalled")
	}
	m.stallSeen()
	m.stallFlightLoaded(`flights\LKKB.FLT`)
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 || !got[0].Stalled || got[1].Stalled || got[1].Silent < 2*time.Second || got[2].FlightLoaded != `flights\LKKB.FLT` {
		t.Fatalf("events %+v, want stall, resume, new session", got)
	}
}

// TestStallPaused: the consumer's paused predicate (WithStallPaused) holds
// a stall back; once it says running, the silence is a stall.
func TestStallPaused(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var paused atomic.Bool
	paused.Store(true)
	cfg := &Config{StallAfter: time.Second}
	WithStallPaused(paused.Load)(cfg)
	m := &Instance{config: cfg, logger: slog.Default(), ctx: ctx}
	m.state = StateAvailable
	m.OnStall(func(StallEvent) {})
	m.stallSeen()
	time.Sleep(2500 * time.Millisecond)
	if m.Stalled() {
		t.Fatal("paused: stalled")
	}
	paused.Store(false)
	time.Sleep(1500 * time.Millisecond)
	if !m.Stalled() {
		t.Fatal("running and silent: not stalled")
	}
}

// TestStallReset: a stall ends silently with its connection; the next
// connection's first message tells no resume.
func TestStallReset(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &Instance{config: &Config{StallAfter: time.Second}, logger: slog.Default(), ctx: ctx}
	m.state = StateAvailable
	var mu sync.Mutex
	var got []StallEvent
	m.OnStall(func(e StallEvent) { mu.Lock(); got = append(got, e); mu.Unlock() })
	m.stallSeen()
	time.Sleep(2500 * time.Millisecond)
	if !m.Stalled() {
		t.Fatal("not stalled")
	}
	m.setState(StateDisconnected)
	if m.Stalled() {
		t.Error("stalled after the connection ended")
	}
	m.setState(StateAvailable)
	m.stallSeen()
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || !got[0].Stalled {
		t.Errorf("events %+v, want the stall only", got)
	}
}
