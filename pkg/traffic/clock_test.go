package traffic

import (
	"math"
	"testing"
	"time"
)

// The clock runs at the simulation rate, stands still while paused, and a
// change of either takes effect from then on without a jump.
func TestSimClock(t *testing.T) {
	wall := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	c := newSimClock(func() time.Time { return wall })
	start := c.Now()
	wall = wall.Add(10 * time.Second)
	if got := c.Now().Sub(start); got != 10*time.Second {
		t.Fatalf("1×: %v after 10 s", got)
	}
	c.SetRate(2)
	wall = wall.Add(10 * time.Second)
	if got := c.Now().Sub(start); got != 30*time.Second {
		t.Fatalf("2× for 10 s after 10 s at 1×: %v", got)
	}
	c.SetPaused(true)
	before := c.Now()
	wall = wall.Add(time.Minute)
	if c.Now() != before || !c.Paused() {
		t.Fatalf("paused: moved %v", c.Now().Sub(before))
	}
	c.SetPaused(false)
	if c.Now() != before {
		t.Fatal("jumped on resuming")
	}
	wall = wall.Add(5 * time.Second)
	if got := c.Now().Sub(before); got != 10*time.Second {
		t.Fatalf("resumed at 2×: %v after 5 s", got)
	}
	c.SetRate(0) // ignored
	if c.Rate() != 2 {
		t.Errorf("rate %v after SetRate(0)", c.Rate())
	}
}

// A departure on a clock at 2× covers in 10 wall seconds what it covers
// in 20 at 1× (the same sim time), and nothing while paused.
func TestTaxiOnSimClock(t *testing.T) {
	moved := func(rate float64, paused bool, wallSeconds int) float64 {
		ctl, _, run, now := injectedDeparture(t, TaxiRequest{})
		if !run(TaxiTaxiing, 60*600) {
			t.Fatalf("state %v", ctl.State())
		}
		// From here the controller reads a sim clock started at the
		// harness time; the harness steps the wall clock under it.
		wall := *now
		clk := newSimClock(func() time.Time { return wall })
		clk.SetRate(rate)
		clk.SetPaused(paused)
		ctl.now = clk.Now
		d0 := ctl.mover.Pose().Distance
		for i := 0; i < 60*wallSeconds && ctl.State() == TaxiTaxiing; i++ {
			wall = wall.Add(time.Second / 60)
			*now = wall
			ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, ctl.req.Graph.Layout.Parking[ctl.req.Parking].Position, 0, 0, true))
		}
		return ctl.mover.Pose().Distance - d0
	}
	slow, fast, still := moved(1, false, 20), moved(2, false, 10), moved(2, true, 10)
	if slow <= 1 || math.Abs(fast-slow) > 0.15*slow {
		t.Errorf("20 s at 1×: %.1f m; 10 s at 2×: %.1f m", slow, fast)
	}
	if still > 0.01 {
		t.Errorf("paused: moved %.2f m", still)
	}
}

// TestSimClockHitch: a gap between frames longer than SimHitch counts
// SimHitch of it, the clock never goes back, and the frames after a pause
// are no hitch.
func TestSimClockHitch(t *testing.T) {
	w := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	c := newSimClock(func() time.Time { return w })
	start := c.Now()
	step := func(d time.Duration) { w = w.Add(d); c.Frame() }
	for i := 0; i < 10; i++ {
		step(16 * time.Millisecond)
	}
	if got := c.Now().Sub(start); got != 160*time.Millisecond {
		t.Fatalf("10 frames: %v", got)
	}
	// The sim stands still 400 ms: only SimHitch of it counts.
	w = w.Add(200 * time.Millisecond)
	mid := c.Now() // read during the gap
	w = w.Add(200 * time.Millisecond)
	c.Frame()
	got := c.Now()
	if got.Before(mid) {
		t.Fatalf("went back: %v before %v", got, mid)
	}
	if d := got.Sub(start); d != 160*time.Millisecond+200*time.Millisecond {
		t.Errorf("after the hitch: %v, want 360ms (the 200 ms read during it kept)", d)
	}
	// Paused 5 s: no hitch on the first frame after it.
	c.SetPaused(true)
	w = w.Add(5 * time.Second)
	c.SetPaused(false)
	before := c.Now()
	step(16 * time.Millisecond)
	if d := c.Now().Sub(before); d != 16*time.Millisecond {
		t.Errorf("first frame after a pause: %v", d)
	}
}
