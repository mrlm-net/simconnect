//go:build windows
// +build windows

package notify

import (
	"log/slog"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// SafeCallHandler is a function type for invoking handler callbacks with panic recovery
type SafeCallHandler func(logger *slog.Logger, handlerName string, fn func())

// SubscriptionTarget provides methods for sending to a subscription channel
type SubscriptionTarget[T any] interface {
	Lock()
	Unlock()
	IsClosed() bool
	TrySend(value T) bool // non-blocking send, returns false if channel full
}

// NotifyState updates connection state and notifies handlers and subscriptions.
// oldState and newState are interface{} to avoid import cycle — caller must ensure they are ConnectionState types.
func NotifyState(
	logger *slog.Logger,
	mu *sync.RWMutex,
	oldState, newState interface{},
	handlers *[]instance.StateHandlerEntry,
	subscriptions []SubscriptionTarget[interface{}],
	safeCall SafeCallHandler,
) {
	// Copy handlers under lock
	mu.RLock()
	handlersCopy := make([]interface{}, len(*handlers))
	for i, e := range *handlers {
		handlersCopy[i] = e.Fn
	}
	mu.RUnlock()

	logger.Debug("[manager] State changed", "old", oldState, "new", newState)

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlersCopy {
		h := handler // capture for closure
		safeCall(logger, "ConnectionStateChangeHandler", func() {
			// Handler is interface{} — caller must ensure it's func(ConnectionState, ConnectionState)
			h.(func(interface{}, interface{}))(oldState, newState)
		})
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := struct{ OldState, NewState interface{} }{oldState, newState}
	for _, sub := range subscriptions {
		sub.Lock()
		if !sub.IsClosed() {
			if !sub.TrySend(stateChange) {
				logger.Debug("[manager] State subscription channel full, dropping state change")
			}
		}
		sub.Unlock()
	}
}

// NotifySimState updates simulator state and notifies handlers and subscriptions.
// oldState and newState are interface{} to avoid import cycle — caller must ensure they are SimState types.
func NotifySimState(
	logger *slog.Logger,
	mu *sync.RWMutex,
	oldState, newState interface{},
	handlers *[]instance.SimStateHandlerEntry,
	subscriptions []SubscriptionTarget[interface{}],
	safeCall SafeCallHandler,
) {
	// Copy handlers under lock
	mu.RLock()
	handlersCopy := make([]interface{}, len(*handlers))
	for i, e := range *handlers {
		handlersCopy[i] = e.Fn
	}
	mu.RUnlock()

	logger.Debug("[manager] SimState changed")

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlersCopy {
		h := handler // capture for closure
		safeCall(logger, "SimStateChangeHandler", func() {
			// Handler is interface{} — caller must ensure it's func(SimState, SimState)
			h.(func(interface{}, interface{}))(oldState, newState)
		})
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := struct{ OldState, NewState interface{} }{oldState, newState}
	for _, sub := range subscriptions {
		sub.Lock()
		if !sub.IsClosed() {
			if !sub.TrySend(stateChange) {
				logger.Debug("[manager] SimState subscription channel full, dropping state change")
			}
		}
		sub.Unlock()
	}
}

// NotifyOpen invokes all registered open handlers and sends to subscriptions
func NotifyOpen(
	logger *slog.Logger,
	mu *sync.RWMutex,
	data types.ConnectionOpenData,
	handlers *[]instance.OpenHandlerEntry,
	subscriptions []SubscriptionTarget[types.ConnectionOpenData],
	safeCall SafeCallHandler,
) {
	// Copy handlers under lock
	mu.RLock()
	handlersCopy := make([]interface{}, len(*handlers))
	for i, e := range *handlers {
		handlersCopy[i] = e.Fn
	}
	mu.RUnlock()

	logger.Debug("[manager] Connection opened")

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlersCopy {
		h := handler // capture for closure
		d := data    // capture for closure
		safeCall(logger, "ConnectionOpenHandler", func() {
			h.(func(types.ConnectionOpenData))(d)
		})
	}

	// Forward open event to subscriptions (non-blocking)
	for _, sub := range subscriptions {
		sub.Lock()
		if !sub.IsClosed() {
			if !sub.TrySend(data) {
				logger.Debug("[manager] Open subscription channel full, dropping open event")
			}
		}
		sub.Unlock()
	}
}

// NotifyQuit invokes all registered quit handlers and sends to subscriptions
func NotifyQuit(
	logger *slog.Logger,
	mu *sync.RWMutex,
	data types.ConnectionQuitData,
	handlers *[]instance.QuitHandlerEntry,
	subscriptions []SubscriptionTarget[types.ConnectionQuitData],
	safeCall SafeCallHandler,
) {
	// Copy handlers under lock
	mu.RLock()
	handlersCopy := make([]interface{}, len(*handlers))
	for i, e := range *handlers {
		handlersCopy[i] = e.Fn
	}
	mu.RUnlock()

	logger.Debug("[manager] Connection quit")

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlersCopy {
		h := handler // capture for closure
		d := data    // capture for closure
		safeCall(logger, "ConnectionQuitHandler", func() {
			h.(func(types.ConnectionQuitData))(d)
		})
	}

	// Forward quit event to subscriptions (non-blocking)
	for _, sub := range subscriptions {
		sub.Lock()
		if !sub.IsClosed() {
			if !sub.TrySend(data) {
				logger.Debug("[manager] Quit subscription channel full, dropping quit event")
			}
		}
		sub.Unlock()
	}
}

// subscriptionAdapter implements SubscriptionTarget for generic subscription types
type subscriptionAdapter[T any] struct {
	closeMu *sync.Mutex
	closed  func() bool
	ch      chan T
}

func (s *subscriptionAdapter[T]) Lock()          { s.closeMu.Lock() }
func (s *subscriptionAdapter[T]) Unlock()        { s.closeMu.Unlock() }
func (s *subscriptionAdapter[T]) IsClosed() bool { return s.closed() }
func (s *subscriptionAdapter[T]) TrySend(val T) bool {
	select {
	case s.ch <- val:
		return true
	default:
		return false
	}
}

// NewSubscriptionAdapter creates a new SubscriptionTarget adapter
func NewSubscriptionAdapter[T any](closeMu *sync.Mutex, closedFn func() bool, ch chan T) SubscriptionTarget[T] {
	return &subscriptionAdapter[T]{
		closeMu: closeMu,
		closed:  closedFn,
		ch:      ch,
	}
}
