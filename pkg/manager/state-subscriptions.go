//go:build windows
// +build windows

package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// connectionStateSubscription is defined in another file to avoid duplicate declarations.
type connectionStateSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan ConnectionStateChange
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
}

// connectionOpenSubscription implements the ConnectionOpenSubscription interface
type connectionOpenSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan types.ConnectionOpenData
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
}

// connectionQuitSubscription implements the ConnectionQuitSubscription interface
type connectionQuitSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan types.ConnectionQuitData
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
}

// simStateSubscription implements the SimStateSubscription interface
type simStateSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan SimStateChange
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
}

// SubscribeConnectionStateChange creates a new state change subscription that delivers state changes to a channel.
func (m *Instance) SubscribeConnectionStateChange(id string, bufferSize int) ConnectionStateSubscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &connectionStateSubscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan ConnectionStateChange, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.connectionStateSubscriptions[id] = sub
	m.connectionStateSubsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created state subscription: %s", id))
	return sub
}

// watchContext monitors the state subscription's context and auto-unsubscribes when cancelled
func (s *connectionStateSubscription) watchContext() {
	<-s.ctx.Done()
	s.Unsubscribe()
}

// GetConnectionStateSubscription returns an existing connection state subscription by ID, or nil if not found.
func (m *Instance) GetConnectionStateSubscription(id string) ConnectionStateSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.connectionStateSubscriptions[id]; ok {
		return sub
	}
	return nil
}

// SubscribeSimStateChange creates a new simulator state change subscription that delivers state changes to a channel.
func (m *Instance) SubscribeSimStateChange(id string, bufferSize int) SimStateSubscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &simStateSubscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan SimStateChange, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.simStateSubscriptions[id] = sub
	m.simStateSubsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created SimState subscription: %s", id))
	return sub
}

// GetSimStateSubscription returns an existing simulator state subscription by ID, or nil if not found.
func (m *Instance) GetSimStateSubscription(id string) SimStateSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.simStateSubscriptions[id]; ok {
		return sub
	}
	return nil
}

// ID returns the unique identifier of the state subscription
func (s *connectionStateSubscription) ID() string {
	return s.id
}

// StateChanges returns the channel for receiving state change events
func (s *connectionStateSubscription) ConnectionStateChanges() <-chan ConnectionStateChange {
	return s.ch
}

// Done returns a channel that is closed when the state subscription ends.
// Use this to detect when to exit your consumer goroutine.
func (s *connectionStateSubscription) Done() <-chan struct{} {
	return s.done
}

// Unsubscribe cancels the state subscription and closes the channel.
// Blocks until any pending state change delivery completes.
func (s *connectionStateSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	s.cancel()    // Cancel the subscription's context
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close state change channel

	// Remove from manager's state subscription map
	s.manager.mu.Lock()
	delete(s.manager.connectionStateSubscriptions, s.id)
	s.manager.mu.Unlock()

	// Signal WaitGroup that this subscription is done
	s.manager.connectionStateSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] State subscription unsubscribed: %s", s.id))
}

// simStateSubscription methods
func (s *simStateSubscription) ID() string                             { return s.id }
func (s *simStateSubscription) SimStateChanges() <-chan SimStateChange { return s.ch }
func (s *simStateSubscription) Done() <-chan struct{}                  { return s.done }
func (s *simStateSubscription) watchContext()                          { <-s.ctx.Done(); s.Unsubscribe() }
func (s *simStateSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.cancel()
	close(s.done)
	close(s.ch)
	s.manager.mu.Lock()
	delete(s.manager.simStateSubscriptions, s.id)
	s.manager.mu.Unlock()
	s.manager.simStateSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] SimState subscription unsubscribed: %s", s.id))
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
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.cancel()
	close(s.done)
	close(s.ch)
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
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.cancel()
	close(s.done)
	close(s.ch)
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
