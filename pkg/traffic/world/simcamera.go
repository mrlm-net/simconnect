//go:build windows
// +build windows

package world

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The simulator's own cameras: CAMERA STATE and, within it, the view index
// (CAMERA VIEW TYPE AND INDEX:1), settable SimVars of the user aircraft,
// written as examples/set-variables does (INT32, no unit). Our add-on
// camera is released first: the simulator's camera is the user's again,
// set where asked.
const (
	defSimCamState uint32 = 2020
	defSimCamView  uint32 = 2021
)

// simCameraStates are the CAMERA STATE values offered: the ones the
// simulator took when set, measured live (MSFS 2024, 2026-10-01): 2
// cockpit, 3 chase, 4 drone, 5 fixed on plane, 6 environment — the
// numbering of manager.CameraState. The SDK page's other numbering (drone
// 8, showcase 7, follow traffic 23) was refused: 7 and 8 fell back to the
// cockpit, 9 and above left the camera as it was.
var simCameraStates = map[string]int{
	"cockpit":     2,
	"chase":       3,
	"drone":       4,
	"fixed":       5, // fixed on plane
	"environment": 6, // a free camera
}

// simCamera switches the simulator's camera.
type simCamera struct {
	client engine.Client

	// viewNow is the view index the simulator reports (CAMERA VIEW TYPE
	// AND INDEX:1); false when not known yet.
	viewNow func() (int, bool)
	// stateNow is the CAMERA STATE the simulator reports.
	stateNow func() (int, bool)

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
	// As examples/set-variables does it: INT32, no unit, one element.
	if err := s.client.AddToDataDefinition(defSimCamState, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0); err != nil {
		return err
	}
	if err := s.client.AddToDataDefinition(defSimCamView, "CAMERA VIEW TYPE AND INDEX:1", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0); err != nil {
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
		return fmt.Errorf("simulator camera: one of cockpit, chase, drone, fixed, environment")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.register(); err != nil {
		return err
	}
	val := int32(v)
	if err := s.client.SetDataOnSimObject(defSimCamState, types.SIMCONNECT_OBJECT_ID_USER, 0, 1, uint32(unsafe.Sizeof(val)), unsafe.Pointer(&val)); err != nil {
		return err
	}
	s.state, s.view = state, 0
	return nil
}

// setRaw sets CAMERA STATE to v as it is. Run on the connection's
// goroutine.
func (s *simCamera) setRaw(v int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.register(); err != nil {
		return err
	}
	val := int32(v)
	return s.client.SetDataOnSimObject(defSimCamState, types.SIMCONNECT_OBJECT_ID_USER, 0, 1, uint32(unsafe.Sizeof(val)), unsafe.Pointer(&val))
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
	// From the view it is on: cockpit starts on 1, not 0 (live).
	if s.viewNow != nil {
		if v, ok := s.viewNow(); ok {
			s.view = v
		}
	}
	s.view = max(0, s.view+d)
	val := int32(s.view)
	return s.client.SetDataOnSimObject(defSimCamView, types.SIMCONNECT_OBJECT_ID_USER, 0, 1, uint32(unsafe.Sizeof(val)), unsafe.Pointer(&val))
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
		m.mode, m.subject, m.err, m.prevSim = "off", "", "", 0 // set where asked, not back
		m.simGen++
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
