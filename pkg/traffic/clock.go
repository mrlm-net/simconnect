package traffic

import (
	"sync"
	"time"
)

// Traffic time (#413): the simulator runs at a simulation rate (time
// acceleration, or slower) and can be paused, while MSFS AI flies its
// segments of a flight (STAR, SID, en route) in simulator time. Our own
// parts of a flight — injected motion and every timer — must run on the
// same time, or at 2× an injected final lags the STAR before it and the
// landing sequence's times are wrong.

// MaxFrameStepSeconds is the most time one frame of injected motion moves
// an aircraft on: a longer gap between frames (a stall) is not made up at
// once. At a high simulation rate with fewer frames far away (Detail), a
// frame is a quarter of a second or more.
const MaxFrameStepSeconds = 1.0

// SimClock is traffic time: it follows the wall clock at the simulation
// rate and stands still while the simulator is paused. It starts at the
// wall clock's time. Safe for concurrent use.
type SimClock struct {
	mu     sync.Mutex
	wall   func() time.Time
	at     time.Time // traffic time at wallAt
	wallAt time.Time
	rate   float64
	paused bool
	// lastFrame: the last simulator frame (Frame); read: the latest time
	// Now gave, the least it gives after a hitch.
	lastFrame time.Time
	read      time.Time
}

// NewSimClock creates a clock at the wall clock's time, rate 1, running.
func NewSimClock() *SimClock {
	return newSimClock(time.Now)
}

func newSimClock(wall func() time.Time) *SimClock {
	now := wall()
	return &SimClock{wall: wall, at: now, wallAt: now, rate: 1}
}

// Now is the traffic time now.
func (c *SimClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.nowLocked()
	if t.Before(c.read) {
		t = c.read
	}
	c.read = t
	return t
}

func (c *SimClock) nowLocked() time.Time {
	if c.paused {
		return c.at
	}
	return c.at.Add(time.Duration(float64(c.wall().Sub(c.wallAt)) * c.rate))
}

// rebase makes the time now the new base, so a change of rate or pause
// takes effect from now on without a jump.
func (c *SimClock) rebase() {
	c.at, c.wallAt = c.nowLocked(), c.wall()
}

// SetRate sets the simulation rate (the simulator's SIMULATION RATE:
// 1 real time, 2 twice as fast, 0.5 half). Rates of 0 or less are ignored.
func (c *SimClock) SetRate(rate float64) {
	if rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if rate == c.rate {
		return
	}
	c.rebase()
	c.rate = rate
}

// SetPaused stops the clock while the simulator is paused and lets it run
// on from where it stopped.
func (c *SimClock) SetPaused(paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if paused == c.paused {
		return
	}
	c.rebase()
	c.paused = paused
	c.lastFrame = time.Time{} // the frames after a pause: no hitch
}

// Frame tells the clock a simulator frame came (each EVENT_FRAME). A gap
// since the last one longer than SimHitch is the simulator standing still
// (loading a model: live, the sim time advanced 16 ms over such a frame,
// and every aircraft driven by the wall clock jumped ahead, 5–14 m at
// approach speed); the clock counts SimHitch of it, so what it drives
// pauses with the simulator. It never goes back: a time already read
// (Now during the gap) stays the least it gives.
func (c *SimClock) Frame() {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.wall()
	last := c.lastFrame
	c.lastFrame = w
	if last.IsZero() || c.paused {
		return
	}
	if gap := w.Sub(last); gap > SimHitch {
		at := c.at.Add(time.Duration(float64(last.Sub(c.wallAt)+SimHitch) * c.rate))
		if at.Before(c.read) {
			at = c.read
		}
		c.at, c.wallAt = at, w
	}
}

// SimHitch: a pause between simulator frames longer than this is the
// simulator standing still (Frame).
const SimHitch = 100 * time.Millisecond

// Rate is the simulation rate; Paused whether the clock stands still.
func (c *SimClock) Rate() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rate
}

func (c *SimClock) Paused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}
