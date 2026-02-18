//go:build windows
// +build windows

package manager

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/subscriptions"
)

// connectionStateSubscription is defined in another file to avoid duplicate declarations.
type connectionStateSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan ConnectionStateChange
	done    chan struct{}
	closed  atomic.Bool
	closeMu sync.Mutex // kept for channel close coordination only
	manager *Instance
}

// simStateSubscription implements the SimStateSubscription interface
type simStateSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan SimStateChange
	done    chan struct{}
	closed  atomic.Bool
	closeMu sync.Mutex // kept for channel close coordination only
	manager *Instance
}

// SubscribeConnectionStateChange creates a new state change subscription that delivers state changes to a channel.
func (m *Instance) SubscribeConnectionStateChange(id string, bufferSize int) ConnectionStateSubscription {
	id = subscriptions.GenerateID(id)
	bufferSize = subscriptions.ValidateBufferSize(bufferSize)

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
	id = subscriptions.GenerateID(id)
	bufferSize = subscriptions.ValidateBufferSize(bufferSize)

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
	if s.closed.Swap(true) {
		return // already closed
	}
	s.closeMu.Lock()
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close state change channel
	s.closeMu.Unlock()

	s.cancel() // Cancel the subscription's context

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
	if s.closed.Swap(true) {
		return // already closed
	}
	s.closeMu.Lock()
	close(s.done)
	close(s.ch)
	s.closeMu.Unlock()
	s.cancel()
	s.manager.mu.Lock()
	delete(s.manager.simStateSubscriptions, s.id)
	s.manager.mu.Unlock()
	s.manager.simStateSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] SimState subscription unsubscribed: %s", s.id))
}
