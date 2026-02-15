//go:build windows
// +build windows

package manager

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// connectionOpenSubscription implements the ConnectionOpenSubscription interface
type connectionOpenSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan types.ConnectionOpenData
	done    chan struct{}
	closed  atomic.Bool
	closeMu sync.Mutex // kept for channel close coordination only
	manager *Instance
}

// connectionQuitSubscription implements the ConnectionQuitSubscription interface
type connectionQuitSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan types.ConnectionQuitData
	done    chan struct{}
	closed  atomic.Bool
	closeMu sync.Mutex // kept for channel close coordination only
	manager *Instance
}

// SubscribeOnOpen creates a new connection open subscription that delivers open events to a channel.
func (m *Instance) SubscribeOnOpen(id string, bufferSize int) ConnectionOpenSubscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &connectionOpenSubscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan types.ConnectionOpenData, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.openSubscriptions[id] = sub
	m.openSubsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created open subscription: %s", id))
	return sub
}

// watchContext monitors the open subscription's context and auto-unsubscribes when cancelled
func (s *connectionOpenSubscription) watchContext()                          { <-s.ctx.Done(); s.Unsubscribe() }
func (s *connectionOpenSubscription) ID() string                             { return s.id }
func (s *connectionOpenSubscription) Opens() <-chan types.ConnectionOpenData { return s.ch }
func (s *connectionOpenSubscription) Done() <-chan struct{}                  { return s.done }
func (s *connectionOpenSubscription) Unsubscribe() {
	if s.closed.Swap(true) {
		return // already closed
	}
	s.closeMu.Lock()
	close(s.done)
	close(s.ch)
	s.closeMu.Unlock()
	s.cancel()
	s.manager.mu.Lock()
	delete(s.manager.openSubscriptions, s.id)
	s.manager.mu.Unlock()
	s.manager.openSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] Open subscription unsubscribed: %s", s.id))
}

// GetOpenSubscription returns an existing open subscription by ID, or nil if not found.
func (m *Instance) GetOpenSubscription(id string) ConnectionOpenSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.openSubscriptions[id]; ok {
		return sub
	}
	return nil
}

// SubscribeOnQuit creates a new connection quit subscription that delivers quit events to a channel.
func (m *Instance) SubscribeOnQuit(id string, bufferSize int) ConnectionQuitSubscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &connectionQuitSubscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan types.ConnectionQuitData, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.quitSubscriptions[id] = sub
	m.quitSubsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created quit subscription: %s", id))
	return sub
}

// watchContext monitors the quit subscription's context and auto-unsubscribes when cancelled
func (s *connectionQuitSubscription) watchContext()                          { <-s.ctx.Done(); s.Unsubscribe() }
func (s *connectionQuitSubscription) ID() string                             { return s.id }
func (s *connectionQuitSubscription) Quits() <-chan types.ConnectionQuitData { return s.ch }
func (s *connectionQuitSubscription) Done() <-chan struct{}                  { return s.done }
func (s *connectionQuitSubscription) Unsubscribe() {
	if s.closed.Swap(true) {
		return // already closed
	}
	s.closeMu.Lock()
	close(s.done)
	close(s.ch)
	s.closeMu.Unlock()
	s.cancel()
	s.manager.mu.Lock()
	delete(s.manager.quitSubscriptions, s.id)
	s.manager.mu.Unlock()
	s.manager.quitSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] Quit subscription unsubscribed: %s", s.id))
}

// GetQuitSubscription returns an existing quit subscription by ID, or nil if not found.
func (m *Instance) GetQuitSubscription(id string) ConnectionQuitSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.quitSubscriptions[id]; ok {
		return sub
	}
	return nil
}
