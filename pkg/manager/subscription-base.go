//go:build windows

package manager

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/handlers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Default buffer size for subscriptions when invalid value is provided
const defaultSubscriptionBufferSize = 16

// subscription implements the Subscription interface
type subscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan engine.Message
	done    chan struct{}
	closed  atomic.Bool
	closeMu sync.Mutex // kept for channel close coordination only
	manager *Instance
	// Optional filter predicate. If non-nil, only messages for which
	// filter(msg) == true are forwarded to this subscription.
	filter func(engine.Message) bool
	// Optional set of allowed SIMCONNECT_RECV_ID values. If non-nil
	// and non-empty, only messages whose DwID matches one of the keys
	// will be forwarded.
	allowedTypes map[types.SIMCONNECT_RECV_ID]struct{}
	// Optional callback invoked when messages are dropped due to full buffer.
	// Called with the number of messages dropped (typically 1).
	// Must not block - called from message dispatch loop.
	onDrop func(dropped int)
	// WaitGroup entry tracked by manager for graceful shutdown
	watchWg sync.WaitGroup
}

// watchContext monitors the subscription's context and auto-unsubscribes when cancelled
func (s *subscription) watchContext() {
	defer s.watchWg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.manager.logger.Error("[subscription] watchContext panic recovered", "panic", r)
		}
	}()
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

// generateUUID delegates to handlers.GenerateUUID for backward compatibility
func generateUUID() string {
	return handlers.GenerateUUID()
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
	if s.closed.Swap(true) {
		return // already closed
	}
	s.closeMu.Lock()
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close message channel
	s.closeMu.Unlock()

	s.cancel() // Cancel the subscription's context

	// Remove from manager's subscription map
	s.manager.mu.Lock()
	delete(s.manager.subscriptions, s.id)
	s.manager.mu.Unlock()

	// Signal WaitGroup that this subscription is done
	s.manager.subsWg.Done()
	s.manager.logger.Debug("[manager] Unsubscribed: " + s.id)
}

// SubscriptionOption is a functional option for configuring subscriptions
type SubscriptionOption func(*subscription)

// WithOnDrop configures a callback to be invoked when messages are dropped
// due to a full subscription buffer. The callback receives the number of
// dropped messages (typically 1 per call). The callback must not block as
// it is called from the message dispatch loop.
func WithOnDrop(fn func(dropped int)) SubscriptionOption {
	return func(s *subscription) {
		s.onDrop = fn
	}
}

// Subscribe creates a new message subscription that delivers messages to a channel.
// The returned Subscription can be used to receive messages in an isolated goroutine.
// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
// The channel is buffered with the specified size (defaults to 16 if <= 0).
// The subscription is automatically cancelled when the manager's context is cancelled.
// Call Unsubscribe() when done to release resources.
// Optional SubscriptionOption parameters can be provided to configure drop notifications.
func (m *Instance) Subscribe(id string, bufferSize int, opts ...SubscriptionOption) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Validate buffer size
	if bufferSize <= 0 {
		bufferSize = defaultSubscriptionBufferSize
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

	// Apply options
	for _, opt := range opts {
		opt(sub)
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation with tracking
	sub.watchWg.Add(1)
	go sub.watchContext()

	m.logger.Debug("[manager] Created subscription: " + id)
	return sub
}

// SubscribeWithFilter creates a new message subscription that delivers messages
// to a channel only when the provided filter returns true for a message.
// Optional SubscriptionOption parameters can be provided to configure drop notifications.
func (m *Instance) SubscribeWithFilter(id string, bufferSize int, filter func(engine.Message) bool, opts ...SubscriptionOption) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Validate buffer size
	if bufferSize <= 0 {
		bufferSize = defaultSubscriptionBufferSize
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

	// Apply options
	for _, opt := range opts {
		opt(sub)
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation with tracking
	sub.watchWg.Add(1)
	go sub.watchContext()

	m.logger.Debug("[manager] Created filtered subscription: " + id)
	return sub
}

// SubscribeWithType creates a new message subscription that delivers messages
// only when their DwID (SIMCONNECT_RECV_ID) is one of the provided types.
// Optional SubscriptionOption parameters can be provided to configure drop notifications.
func (m *Instance) SubscribeWithType(id string, bufferSize int, recvIDs []types.SIMCONNECT_RECV_ID, opts ...SubscriptionOption) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Validate buffer size
	if bufferSize <= 0 {
		bufferSize = defaultSubscriptionBufferSize
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

	// Apply options
	for _, opt := range opts {
		opt(sub)
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation with tracking
	sub.watchWg.Add(1)
	go sub.watchContext()

	m.logger.Debug("[manager] Created type subscription: " + id)
	return sub
}
