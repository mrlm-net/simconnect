//go:build windows
// +build windows

package engine

import "sync"

type State struct {
	available bool
	ready     bool
	sync      sync.RWMutex
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

func (s *State) Reset() {
	s.sync.Lock()
	defer s.sync.Unlock()
	s.available = false
	s.ready = false
}
