//go:build windows

package manager

import (
	"context"
	"log/slog"
	"sync"
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
