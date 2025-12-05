//go:build windows
// +build windows

package engine

import "sync"

type State struct {
	available  bool
	ready      bool
	paused     bool
	simRunning bool
	soundOn    bool
	sync       sync.RWMutex
}

func (s *State) IsAvailable() bool {
	s.sync.RLock()
	defer s.sync.RUnlock()
	return s.available
}

func (s *State) SetAvailable(available bool) {
	s.sync.Lock()
	defer s.sync.Unlock()
	s.available = available
}

func (s *State) IsReady() bool {
	s.sync.RLock()
	defer s.sync.RUnlock()
	return s.ready
}

func (s *State) SetReady(ready bool) {
	s.sync.Lock()
	defer s.sync.Unlock()
	s.ready = ready
}

func (s *State) IsPaused() bool {
	s.sync.RLock()
	defer s.sync.RUnlock()
	return s.paused
}

func (s *State) SetPaused(paused bool) {
	s.sync.Lock()
	defer s.sync.Unlock()
	s.paused = paused
}

func (s *State) IsSimRunning() bool {
	s.sync.RLock()
	defer s.sync.RUnlock()
	return s.simRunning
}

func (s *State) SetSimRunning(running bool) {
	s.sync.Lock()
	defer s.sync.Unlock()
	s.simRunning = running
}

func (s *State) IsSoundOn() bool {
	s.sync.RLock()
	defer s.sync.RUnlock()
	return s.soundOn
}

func (s *State) SetSoundOn(on bool) {
	s.sync.Lock()
	defer s.sync.Unlock()
	s.soundOn = on
}

func (s *State) Reset() {
	s.sync.Lock()
	defer s.sync.Unlock()
	s.available = false
	s.ready = false
	s.paused = false
	s.simRunning = false
	s.soundOn = false
}
