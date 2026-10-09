//go:build windows

package manager

import (
	"sync"
	"sync/atomic"
	"time"
)

// A connection that stays open while the simulator stops sending anything
// (live, the MyCrew app: MSFS died in flight, no disconnect, no state
// change, frames simply stopped for 20 minutes): after StallAfter without a
// message while Available the manager reports a stall, and a resume when
// messages come again — with the flight loaded meanwhile, if any, for a new
// session. The manager's own sim state request (SimStatePeriod) is the
// heartbeat; a paused simulator is not taken for a stalled one.

// DEFAULT_STALL_AFTER: silence this long counts as a stall.
const DEFAULT_STALL_AFTER = 5 * time.Second

// StallEvent is a stall beginning (Stalled) or ending.
type StallEvent struct {
	Stalled bool
	// Silent is how long nothing came (at a resume: the whole silence).
	Silent time.Duration
	// FlightLoaded, at a resume: the flight file loaded during the silence
	// ("" none) — a new session, not the same flight going on.
	FlightLoaded string
}

// StallHandler hears stall events; it must not block.
type StallHandler func(StallEvent)

type stallState struct {
	last     atomic.Int64 // unix nanoseconds of the last message
	stalled  atomic.Bool
	mu       sync.Mutex
	since    time.Time
	loaded   string
	handlers []StallHandler
	watching bool
	// resumedAt, silent: the last resume and its silence, for a flight
	// loaded just after it (a new session).
	resumedAt time.Time
	silent    time.Duration
}

// OnStall registers h for stall events (Stalled() for the state now).
func (m *Instance) OnStall(h StallHandler) {
	m.stall.mu.Lock()
	m.stall.handlers = append(m.stall.handlers, h)
	start := !m.stall.watching
	m.stall.watching = true
	m.stall.mu.Unlock()
	if start {
		go m.watchStall()
	}
}

// Stalled reports whether the simulator has gone silent (see OnStall).
func (m *Instance) Stalled() bool { return m.stall.stalled.Load() }

// stallSeen notes a message: a stall, if any, ends.
func (m *Instance) stallSeen() {
	now := time.Now()
	m.stall.last.Store(now.UnixNano())
	if !m.stall.stalled.Load() {
		return
	}
	m.stall.mu.Lock()
	if !m.stall.stalled.Load() {
		m.stall.mu.Unlock()
		return
	}
	m.stall.stalled.Store(false)
	ev := StallEvent{Silent: now.Sub(m.stall.since), FlightLoaded: m.stall.loaded}
	m.stall.loaded = ""
	m.stall.resumedAt, m.stall.silent = now, ev.Silent
	hs := append([]StallHandler(nil), m.stall.handlers...)
	m.stall.mu.Unlock()
	m.logger.Info("[manager] simulator data again after a stall", "silent", ev.Silent.Round(time.Second), "flightLoaded", ev.FlightLoaded)
	for _, h := range hs {
		h(ev)
	}
}

// stallFlightLoaded notes a flight loaded while stalled; loaded within
// newSessionWithin after a resume (its message is the first one again), it
// is told as a resume of its own with the flight: a new session.
func (m *Instance) stallFlightLoaded(file string) {
	m.stall.mu.Lock()
	if m.stall.stalled.Load() {
		m.stall.loaded = file
		m.stall.mu.Unlock()
		return
	}
	if m.stall.resumedAt.IsZero() || time.Since(m.stall.resumedAt) > newSessionWithin {
		m.stall.mu.Unlock()
		return
	}
	ev := StallEvent{Silent: m.stall.silent, FlightLoaded: file}
	m.stall.resumedAt = time.Time{}
	hs := append([]StallHandler(nil), m.stall.handlers...)
	m.stall.mu.Unlock()
	for _, h := range hs {
		h(ev)
	}
}

// stallPaused: the simulator paused by its "Pause" event or the consumer's
// predicate (StallPaused); the event alone has kept a pause with no resume
// in the MyCrew app, so a consumer with Pause_EX1 supplies its own.
func (m *Instance) stallPaused() bool {
	if p := m.config.StallPaused; p != nil {
		return p()
	}
	return m.SimState().Paused
}

// newSessionWithin: a flight loaded this soon after a stall ends is a new
// session.
const newSessionWithin = time.Minute

// watchStall reports a stall once nothing has come for StallAfter while
// Available and not paused.
func (m *Instance) watchStall() {
	after := m.config.StallAfter
	if after == 0 {
		after = DEFAULT_STALL_AFTER
	}
	if after < 0 {
		return
	}
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
		}
		last := m.stall.last.Load()
		if m.stall.stalled.Load() || last == 0 || m.ConnectionState() != StateAvailable || m.stallPaused() {
			continue
		}
		silent := time.Since(time.Unix(0, last))
		if silent < after {
			continue
		}
		m.stall.mu.Lock()
		m.stall.stalled.Store(true)
		m.stall.since, m.stall.loaded = time.Unix(0, last), ""
		hs := append([]StallHandler(nil), m.stall.handlers...)
		m.stall.mu.Unlock()
		m.logger.Warn("[manager] simulator silent: stalled", "silent", silent.Round(time.Second))
		for _, h := range hs {
			h(StallEvent{Stalled: true, Silent: silent})
		}
	}
}
