package traffic

import (
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// The time the traffic runs on: the wall clock, or a ManualClock a test
// moves on (worldtest). Every read of the time and every timer of the
// traffic and the World goes through it (Now, Since, After, NewTicker,
// AfterFunc, Sleep), so with a manual clock a scenario runs as fast as the
// test steps it, and the same every run. Process-wide, as the World is.

// TimeSource is the clock behind Now and the timers.
type TimeSource interface {
	Now() time.Time
	// After, NewTicker and AfterFunc are time.After, time.NewTicker and
	// time.AfterFunc on this clock.
	After(d time.Duration) <-chan time.Time
	NewTicker(d time.Duration) *Ticker
	AfterFunc(d time.Duration, f func()) Stopper
}

// Ticker is a time.Ticker on a TimeSource.
type Ticker struct {
	C    <-chan time.Time
	stop func()
}

// Stop stops the ticker.
func (t *Ticker) Stop() { t.stop() }

// Stopper stops a timer (AfterFunc); false when it had fired or stopped.
type Stopper interface{ Stop() bool }

var (
	sourceMu sync.RWMutex
	source   TimeSource = wallClock{}
)

// SetTimeSource makes s the clock of the traffic and the World (nil: the
// wall clock again). Set it before the World starts.
func SetTimeSource(s TimeSource) {
	if s == nil {
		s = wallClock{}
	}
	sourceMu.Lock()
	source = s
	sourceMu.Unlock()
	// The loaders' timeouts on the same clock.
	if _, wall := s.(wallClock); wall {
		airport.SetClock(nil)
		nav.SetClock(nil)
	} else {
		airport.SetClock(s.Now)
		nav.SetClock(s.Now)
	}
}

func src() TimeSource {
	sourceMu.RLock()
	defer sourceMu.RUnlock()
	return source
}

// Now is the time now on the traffic's clock.
func Now() time.Time { return src().Now() }

// Since is the time since t on the traffic's clock.
func Since(t time.Time) time.Duration { return src().Now().Sub(t) }

// Until is the time until t on the traffic's clock.
func Until(t time.Time) time.Duration { return t.Sub(src().Now()) }

// After is time.After on the traffic's clock.
func After(d time.Duration) <-chan time.Time { return src().After(d) }

// NewTicker is time.NewTicker on the traffic's clock.
func NewTicker(d time.Duration) *Ticker { return src().NewTicker(d) }

// AfterFunc is time.AfterFunc on the traffic's clock.
func AfterFunc(d time.Duration, f func()) Stopper { return src().AfterFunc(d, f) }

// Sleep is time.Sleep on the traffic's clock.
func Sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	<-src().After(d)
}

type wallClock struct{}

func (wallClock) Now() time.Time                         { return time.Now() }
func (wallClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (wallClock) NewTicker(d time.Duration) *Ticker {
	t := time.NewTicker(d)
	return &Ticker{C: t.C, stop: t.Stop}
}
func (wallClock) AfterFunc(d time.Duration, f func()) Stopper { return time.AfterFunc(d, f) }

// ManualClock is a TimeSource that moves only when Advance is called: its
// timers and tickers fire then, in time order, each at its own time.
type ManualClock struct {
	mu     sync.Mutex
	now    time.Time
	seq    int
	timers []*manualTimer
}

type manualTimer struct {
	at     time.Time
	seq    int
	period time.Duration // a ticker's; 0 a one-shot
	ch     chan time.Time
	f      func()
	dead   bool
}

// NewManualClock returns a clock standing at start.
func NewManualClock(start time.Time) *ManualClock { return &ManualClock{now: start} }

func (c *ManualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *ManualClock) add(t *manualTimer) {
	c.seq++
	t.seq = c.seq
	c.timers = append(c.timers, t)
}

func (c *ManualClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.now
		return ch
	}
	c.add(&manualTimer{at: c.now.Add(d), ch: ch})
	return ch
}

func (c *ManualClock) NewTicker(d time.Duration) *Ticker {
	if d <= 0 {
		panic("traffic: non-positive interval for NewTicker")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	t := &manualTimer{at: c.now.Add(d), period: d, ch: ch}
	c.add(t)
	return &Ticker{C: ch, stop: func() {
		c.mu.Lock()
		t.dead = true
		c.mu.Unlock()
	}}
}

func (c *ManualClock) AfterFunc(d time.Duration, f func()) Stopper {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &manualTimer{at: c.now.Add(d), f: f}
	c.add(t)
	return manualStop{c, t}
}

type manualStop struct {
	c *ManualClock
	t *manualTimer
}

func (s manualStop) Stop() bool {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	was := !s.t.dead
	s.t.dead = true
	return was
}

// Advance moves the clock d on, firing every timer and ticker due on the
// way at its own time, earliest first: a ticker's channel gets the time
// (dropped when its last tick was not taken, as time.Ticker), an
// AfterFunc's function runs in its own goroutine.
func (c *ManualClock) Advance(d time.Duration) {
	c.mu.Lock()
	end := c.now.Add(d)
	for {
		live := c.timers[:0]
		for _, t := range c.timers {
			if !t.dead {
				live = append(live, t)
			}
		}
		c.timers = live
		sort.Slice(c.timers, func(i, j int) bool {
			if !c.timers[i].at.Equal(c.timers[j].at) {
				return c.timers[i].at.Before(c.timers[j].at)
			}
			return c.timers[i].seq < c.timers[j].seq
		})
		if len(c.timers) == 0 || c.timers[0].at.After(end) {
			break
		}
		t := c.timers[0]
		c.now = t.at
		switch {
		case t.f != nil:
			t.dead = true
			go t.f()
		case t.period > 0:
			select {
			case t.ch <- t.at:
			default:
			}
			t.at = t.at.Add(t.period)
		default:
			t.dead = true
			t.ch <- t.at
		}
	}
	c.now = end
	c.mu.Unlock()
}
