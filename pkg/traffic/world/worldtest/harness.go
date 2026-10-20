package worldtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// Options start a Harness.
type Options struct {
	// DataDir is the offline data's root (a simconnect-data checkout):
	// <DataDir>/<ICAO>/layout.json, procedures.json, ...
	DataDir string
	// ICAO is the airport loaded, and where the user aircraft starts.
	ICAO string
	// World is passed on to world.New, with SimlessAI on, the airport cache
	// the harness's, and Output io.Discard unless set; OnTransmission is
	// still called, after the harness has heard it.
	World world.Options
	// FrameRate is how many simulator frames a second the loopback sends;
	// 0: 30. With Manual, fewer frames make a step of time cheaper (each
	// frame moves our aircraft up to traffic.MaxFrameStepSeconds).
	FrameRate int
	// Manual runs the traffic and the World on a traffic.ManualClock (set
	// process-wide until Close) standing at Start: time moves only in
	// Advance and Until, frame by frame, each frame taken by the World
	// before the next, as fast as the machine runs them and the same every
	// run. Without it the wall clock runs and Advance waits.
	Manual bool
	// Start is the manual clock's time at the start; zero: 2026-10-20
	// 10:00 UTC (midday at LKPR).
	Start time.Time
}

// Harness is a World running on a Loopback: no simulator.
type Harness struct {
	World *world.World
	Sim   *Loopback
	ICAO  string
	// Layout is the airport loaded.
	Layout *airport.Layout

	// Clock is the manual clock (Options.Manual), nil on the wall clock.
	Clock *traffic.ManualClock

	frame  time.Duration
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
	heard  []traffic.Transmission
}

// Start runs a World on a new Loopback at o.ICAO and waits until its
// traffic is running (at most 15 s).
func Start(o Options) (*Harness, error) {
	if o.DataDir == "" || o.ICAO == "" {
		return nil, errors.New("worldtest: DataDir and ICAO are required")
	}
	if o.FrameRate <= 0 {
		o.FrameRate = 30
	}
	h := &Harness{done: make(chan struct{}), frame: time.Second / time.Duration(o.FrameRate)}
	cache := airport.NewCache()
	wo := o.World
	wo.SimlessAI, wo.Cache = true, cache
	if wo.Output == nil {
		wo.Output = io.Discard
	}
	user := wo.OnTransmission
	wo.OnTransmission = func(t traffic.Transmission) {
		h.mu.Lock()
		h.heard = append(h.heard, t)
		h.mu.Unlock()
		if user != nil {
			user(t)
		}
	}
	h.World = world.New(wo)
	icao, err := h.World.LoadAirportDir(filepath.Join(o.DataDir, strings.ToUpper(o.ICAO)))
	if err != nil {
		return nil, err
	}
	// The airports around (the simulator's facility list): the picture
	// gives each aircraft its airport from it (an emergency's rescue, the
	// player's sequence).
	if p := filepath.Join(o.DataDir, "airports.json"); fileExists(p) {
		if _, err := h.World.LoadAirportsFile(p); err != nil {
			return nil, err
		}
	}
	l, ok := cache.Layout(icao)
	if !ok {
		return nil, fmt.Errorf("worldtest: %s: no layout in %s", icao, o.DataDir)
	}
	h.ICAO, h.Layout = icao, l
	if o.Manual {
		if o.Start.IsZero() {
			o.Start = time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
		}
		h.Clock = traffic.NewManualClock(o.Start)
		traffic.SetTimeSource(h.Clock)
	}
	h.Sim = NewLoopback(l.Latitude, l.Longitude, l.Altitude/0.3048, 16384)

	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // the World on the loopback
		defer wg.Done()
		_ = h.World.RunOn(ctx, h.Sim)
	}()
	go func() { // the loopback's messages to the World, as a host feeds them
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-h.Sim.Stream():
				h.World.Feed(m)
			}
		}
	}()
	if !o.Manual {
		wg.Add(1)
		go func() { // the simulator's frames, on the wall clock
			defer wg.Done()
			t := time.NewTicker(h.frame)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					h.Sim.Step()
				}
			}
		}()
	}
	go func() { wg.Wait(); close(h.done) }()
	if !h.Until(15*time.Second, func() bool { return h.World.Snapshot().Connected }) {
		h.Close()
		return nil, errors.New("worldtest: the World did not start")
	}
	return h, nil
}

// Close stops the World and the loopback, and puts the wall clock back.
func (h *Harness) Close() {
	h.cancel()
	if h.Clock != nil {
		// Timers waiting on the manual clock (Sleep) run out.
		go func() {
			for i := 0; i < 100; i++ {
				select {
				case <-h.done:
					return
				default:
				}
				h.Clock.Advance(time.Second)
				time.Sleep(time.Millisecond)
			}
		}()
	}
	select {
	case <-h.done:
	case <-time.After(10 * time.Second):
	}
	if h.Clock != nil {
		traffic.SetTimeSource(nil)
	}
}

// Advance lets d go by: on the manual clock frame by frame (step), on the
// wall clock it waits.
func (h *Harness) Advance(d time.Duration) {
	if h.Clock == nil {
		time.Sleep(d)
		return
	}
	for left := d; left > 0; left -= h.frame {
		h.step(min(h.frame, left))
	}
}

// Until lets time go by until cond holds (true) or max has gone (false):
// on the manual clock frame by frame, cond checked after each; on the wall
// clock as WaitFor.
func (h *Harness) Until(max time.Duration, cond func() bool) bool {
	if h.Clock == nil {
		return h.WaitFor(max, cond)
	}
	for gone := time.Duration(0); ; gone += h.frame {
		if cond() {
			return true
		}
		if gone >= max {
			return false
		}
		h.step(h.frame)
	}
}

// step moves the manual clock d on and sends the frame, then waits until
// the World has taken every message of it.
func (h *Harness) step(d time.Duration) {
	h.Clock.Advance(d)
	h.Sim.Step()
	for i := 0; i < 2000; i++ {
		if len(h.Sim.out) == 0 && h.World.Queued() == 0 {
			break
		}
		time.Sleep(50 * time.Microsecond)
	}
	// The last message taken is being handled, and what it started (a
	// controller's events) runs on: a moment for them.
	time.Sleep(300 * time.Microsecond)
}

// WaitFor checks cond every 100 ms until it holds (true) or timeout passes
// (false).
func (h *Harness) WaitFor(timeout time.Duration, cond func() bool) bool {
	end := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(end) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Heard is every transmission so far, in order.
func (h *Harness) Heard() []traffic.Transmission {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]traffic.Transmission(nil), h.heard...)
}

// Said reports whether a transmission by or to callsign ("" any) had text
// containing sub (case ignored).
func (h *Harness) Said(callsign, sub string) bool {
	sub = strings.ToLower(sub)
	for _, t := range h.Heard() {
		if callsign != "" && !strings.EqualFold(t.Callsign, callsign) {
			continue
		}
		if strings.Contains(strings.ToLower(t.Text), sub) {
			return true
		}
	}
	return false
}

// Aircraft is our aircraft callsign's view now, false none.
func (h *Harness) Aircraft(callsign string) (world.ControlView, bool) {
	for _, v := range h.World.Snapshot().Aircraft {
		if strings.EqualFold(v.Tail, callsign) {
			return v, true
		}
	}
	return world.ControlView{}, false
}

// SetUser scripts the user aircraft: f sets it (position, heading,
// altitude, speed, on the ground, COM1), read by the World from the next
// second on.
func (h *Harness) SetUser(f func(o *Object)) { h.Sim.User(f) }

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
