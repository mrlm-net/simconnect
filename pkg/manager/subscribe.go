//go:build windows
// +build windows

package manager

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// subscription implements the Subscription interface
type subscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan engine.Message
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
	// Optional filter predicate. If non-nil, only messages for which
	// filter(msg) == true are forwarded to this subscription.
	filter func(engine.Message) bool
	// Optional set of allowed SIMCONNECT_RECV_ID values. If non-nil
	// and non-empty, only messages whose DwID matches one of the keys
	// will be forwarded.
	allowedTypes map[types.SIMCONNECT_RECV_ID]struct{}
}

// stateSubscription implements the StateSubscription interface
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

// Subscribe creates a new message subscription that delivers messages to a channel.
// The returned Subscription can be used to receive messages in an isolated goroutine.
// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
// The channel is buffered with the specified size.
// The subscription is automatically cancelled when the manager's context is cancelled.
// Call Unsubscribe() when done to release resources.
func (m *Instance) Subscribe(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &subscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan engine.Message, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created subscription: %s", id))
	return sub
}

// SubscribeWithFilter creates a new message subscription that delivers messages
// to a channel only when the provided filter returns true for a message.
func (m *Instance) SubscribeWithFilter(id string, bufferSize int, filter func(engine.Message) bool) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &subscription{
		id:           id,
		ctx:          subCtx,
		cancel:       subCancel,
		ch:           make(chan engine.Message, bufferSize),
		done:         make(chan struct{}),
		manager:      m,
		filter:       filter,
		allowedTypes: nil,
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created filtered subscription: %s", id))
	return sub
}

// SubscribeWithType creates a new message subscription that delivers messages
// only when their DwID (SIMCONNECT_RECV_ID) is one of the provided types.
func (m *Instance) SubscribeWithType(id string, bufferSize int, recvIDs ...types.SIMCONNECT_RECV_ID) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	allowed := make(map[types.SIMCONNECT_RECV_ID]struct{}, len(recvIDs))
	for _, r := range recvIDs {
		allowed[r] = struct{}{}
	}

	sub := &subscription{
		id:           id,
		ctx:          subCtx,
		cancel:       subCancel,
		ch:           make(chan engine.Message, bufferSize),
		done:         make(chan struct{}),
		manager:      m,
		filter:       nil,
		allowedTypes: allowed,
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created type subscription: %s", id))
	return sub
}

// watchContext monitors the subscription's context and auto-unsubscribes when cancelled
func (s *subscription) watchContext() {
	<-s.ctx.Done()
	s.Unsubscribe()
}

// GetSubscription returns an existing subscription by ID, or nil if not found.
func (m *Instance) GetSubscription(id string) Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.subscriptions[id]; ok {
		return sub
	}
	return nil
}

// generateUUID generates a simple UUID v4
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ID returns the unique identifier of the subscription
func (s *subscription) ID() string {
	return s.id
}

// Messages returns the channel for receiving messages
func (s *subscription) Messages() <-chan engine.Message {
	return s.ch
}

// Done returns a channel that is closed when the subscription ends.
// Use this to detect when to exit your consumer goroutine.
func (s *subscription) Done() <-chan struct{} {
	return s.done
}

// Unsubscribe cancels the subscription and closes the channel.
// Blocks until any pending message delivery completes.
func (s *subscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	s.cancel()    // Cancel the subscription's context
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close message channel

	// Remove from manager's subscription map
	s.manager.mu.Lock()
	delete(s.manager.subscriptions, s.id)
	s.manager.mu.Unlock()

	// Signal WaitGroup that this subscription is done
	s.manager.subsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] Unsubscribed: %s", s.id))
}

// SubscribeStateChange creates a new state change subscription that delivers state changes to a channel.
// The returned StateSubscription can be used to receive state changes in an isolated goroutine.
// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
// The channel is buffered with the specified size.
// The subscription is automatically cancelled when the manager's context is cancelled.
// Call Unsubscribe() when done to release resources.
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
