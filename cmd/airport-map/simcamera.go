//go:build windows
// +build windows

package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The simulator's own cameras: CAMERA STATE and, within it, the view index
// (CAMERA VIEW TYPE AND INDEX:1), both settable SimVars of the user
// aircraft (MSFS 2024 SDK, Camera Variables). Our add-on camera is released
// first: the simulator's camera is the user's again, set where asked.
const (
	defSimCamState uint32 = 2020
	defSimCamView  uint32 = 2021
)

// simCameraStates are the CAMERA STATE values offered (MSFS 2024): all on
// the user's aircraft but the traffic one, "Follow Air Traffic Plane",
// which the simulator points at a traffic aircraft of its choosing.
var simCameraStates = map[string]int{
	"cockpit":  2,
	"chase":    3,
	"fixed":    4,  // fixed on plane
	"showcase": 7,  // the airport's fixed cameras
	"drone":    8,  // drone plane
	"topdown":  17, // drone top down
	"traffic":  23, // follow air traffic plane
}

// simCamera switches the simulator's camera.
type simCamera struct {
	client engine.Client

	mu         sync.Mutex
	registered bool
	state      string
	view       int
}

// register defines the two SimVars, once per connection.
func (s *simCamera) register() error {
	if s.registered {
		return nil
	}
	if err := s.client.AddToDataDefinition(defSimCamState, "CAMERA STATE", "Enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
		return err
	}
	if err := s.client.AddToDataDefinition(defSimCamView, "CAMERA VIEW TYPE AND INDEX:1", "Number", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
		return err
	}
	s.registered = true
	return nil
}

// set puts the simulator's camera in state (a simCameraStates name). Run
// on the connection's goroutine.
func (s *simCamera) set(state string) error {
	v, ok := simCameraStates[state]
	if !ok {
		names := make([]string, 0, len(simCameraStates))
		for k := range simCameraStates {
			names = append(names, k)
		}
		return fmt.Errorf("simulator camera: one of %s", strings.Join(names, ", "))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.register(); err != nil {
		return err
	}
	f := float64(v)
	if err := s.client.SetDataOnSimObject(defSimCamState, types.SIMCONNECT_OBJECT_ID_USER, 0, 0, 8, unsafe.Pointer(&f)); err != nil {
		return err
	}
	s.state, s.view = state, 0
	return nil
}

// step moves to the next (+1) or previous (-1) view of the camera in use:
// the cockpit's seats and instruments, the fixed cameras around the plane.
func (s *simCamera) step(d int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == "" {
		return errors.New("pick a simulator camera first")
	}
	if err := s.register(); err != nil {
		return err
	}
	s.view = max(0, s.view+d)
	f := float64(s.view)
	return s.client.SetDataOnSimObject(defSimCamView, types.SIMCONNECT_OBJECT_ID_USER, 0, 0, 8, unsafe.Pointer(&f))
}

// setSim gives the simulator its camera back and sets it: state (a
// simCameraStates name), or with "" the view step (+1, -1) within it.
func (m *cameraMan) setSim(state string, step int) error {
	if m.sim == nil {
		return errors.New("no simulator on this connection")
	}
	if m.dir != nil {
		m.mu.Lock()
		ours := m.mode != "off"
		m.mode, m.subject, m.err = "off", "", ""
		m.mu.Unlock()
		if ours {
			if m.frames != nil {
				m.frames(false)
			}
			if err := m.cc.do(m.dir.Release); err != nil {
				return err
			}
		}
	}
	return m.cc.do(func() error {
		if state != "" {
			return m.sim.set(state)
		}
		return m.sim.step(step)
	})
}

// current is the simulator camera last set, and its view index.
func (s *simCamera) current() (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.view
}
