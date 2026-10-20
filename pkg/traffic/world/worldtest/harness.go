package worldtest

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	// 0: 30.
	FrameRate int
}

// Harness is a World running on a Loopback: no simulator.
type Harness struct {
	World *world.World
	Sim   *Loopback
	ICAO  string
	// Layout is the airport loaded.
	Layout *airport.Layout

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
	h := &Harness{done: make(chan struct{})}
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
	l, ok := cache.Layout(icao)
	if !ok {
		return nil, fmt.Errorf("worldtest: %s: no layout in %s", icao, o.DataDir)
	}
	h.ICAO, h.Layout = icao, l
	h.Sim = NewLoopback(l.Latitude, l.Longitude, l.Altitude/0.3048, 16384)

	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	var wg sync.WaitGroup
	wg.Add(3)
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
	go func() { // the simulator's frames
		defer wg.Done()
		t := time.NewTicker(time.Second / time.Duration(o.FrameRate))
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
	go func() { wg.Wait(); close(h.done) }()
	if !h.WaitFor(15*time.Second, func() bool { return h.World.Snapshot().Connected }) {
		h.Close()
		return nil, errors.New("worldtest: the World did not start")
	}
	return h, nil
}

// Close stops the World and the loopback.
func (h *Harness) Close() {
	h.cancel()
	select {
	case <-h.done:
	case <-time.After(10 * time.Second):
	}
}

// Advance lets d go by. The World runs on the wall clock: this waits.
func (h *Harness) Advance(d time.Duration) { time.Sleep(d) }

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
