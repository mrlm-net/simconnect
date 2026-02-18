//go:build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/manager/internal/notify"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// stateSubscriptionAdapter adapts connectionStateSubscription to notify.SubscriptionTarget[interface{}]
type stateSubscriptionAdapter struct {
	sub *connectionStateSubscription
}

func (a *stateSubscriptionAdapter) Lock()          { a.sub.closeMu.Lock() }
func (a *stateSubscriptionAdapter) Unlock()        { a.sub.closeMu.Unlock() }
func (a *stateSubscriptionAdapter) IsClosed() bool { return a.sub.closed.Load() }
func (a *stateSubscriptionAdapter) TrySend(val interface{}) bool {
	// Convert interface{} struct back to ConnectionStateChange
	change := val.(struct{ OldState, NewState interface{} })
	stateChange := ConnectionStateChange{
		OldState: change.OldState.(ConnectionState),
		NewState: change.NewState.(ConnectionState),
	}
	select {
	case a.sub.ch <- stateChange:
		return true
	default:
		return false
	}
}

// simStateSubscriptionAdapter adapts simStateSubscription to notify.SubscriptionTarget[interface{}]
type simStateSubscriptionAdapter struct {
	sub *simStateSubscription
}

func (a *simStateSubscriptionAdapter) Lock()          { a.sub.closeMu.Lock() }
func (a *simStateSubscriptionAdapter) Unlock()        { a.sub.closeMu.Unlock() }
func (a *simStateSubscriptionAdapter) IsClosed() bool { return a.sub.closed.Load() }
func (a *simStateSubscriptionAdapter) TrySend(val interface{}) bool {
	// Convert interface{} struct back to SimStateChange
	change := val.(struct{ OldState, NewState interface{} })
	stateChange := SimStateChange{
		OldState: change.OldState.(SimState),
		NewState: change.NewState.(SimState),
	}
	select {
	case a.sub.ch <- stateChange:
		return true
	default:
		return false
	}
}

// setState updates the connection state and notifies handlers
func (m *Instance) setState(newState ConnectionState) {
	m.mu.Lock()
	oldState := m.state
	if oldState == newState {
		m.mu.Unlock()
		return
	}
	m.state = newState
	m.mu.Unlock()

	// Build subscription adapters
	m.mu.RLock()
	subs := make([]notify.SubscriptionTarget[interface{}], 0, len(m.connectionStateSubscriptions))
	for _, sub := range m.connectionStateSubscriptions {
		subs = append(subs, &stateSubscriptionAdapter{sub: sub})
	}
	m.mu.RUnlock()

	// Delegate to notify package
	notify.NotifyState(
		m.logger,
		&m.mu,
		oldState,
		newState,
		&m.stateHandlers,
		subs,
		safeCallHandler,
	)
}

// setSimState updates the simulator state and notifies handlers
func (m *Instance) setSimState(newState SimState) {
	m.mu.Lock()
	oldState := m.simState
	if oldState.Equal(newState) {
		m.mu.Unlock()
		return
	}
	m.simState = newState
	m.mu.Unlock()

	// Build subscription adapters
	m.mu.RLock()
	subs := make([]notify.SubscriptionTarget[interface{}], 0, len(m.simStateSubscriptions))
	for _, sub := range m.simStateSubscriptions {
		subs = append(subs, &simStateSubscriptionAdapter{sub: sub})
	}
	m.mu.RUnlock()

	// Delegate to notify package
	notify.NotifySimState(
		m.logger,
		&m.mu,
		oldState,
		newState,
		&m.simStateHandlers,
		subs,
		safeCallHandler,
	)
}

// notifySimStateChange notifies handlers and subscriptions of a SimState change.
// This is a helper used by delta update paths where state is already modified in-place.
// The caller must have already updated m.simState and must NOT hold m.mu when calling this.
func (m *Instance) notifySimStateChange(oldState, newState SimState) {
	// Build subscription adapters
	m.mu.RLock()
	subs := make([]notify.SubscriptionTarget[interface{}], 0, len(m.simStateSubscriptions))
	for _, sub := range m.simStateSubscriptions {
		subs = append(subs, &simStateSubscriptionAdapter{sub: sub})
	}
	m.mu.RUnlock()

	// Delegate to notify package
	notify.NotifySimState(
		m.logger,
		&m.mu,
		oldState,
		newState,
		&m.simStateHandlers,
		subs,
		safeCallHandler,
	)
}

// setOpen invokes all registered open handlers and sends to subscriptions
func (m *Instance) setOpen(data types.ConnectionOpenData) {
	// Build subscription adapters
	m.mu.RLock()
	subs := make([]notify.SubscriptionTarget[types.ConnectionOpenData], 0, len(m.openSubscriptions))
	for _, sub := range m.openSubscriptions {
		subs = append(subs, notify.NewSubscriptionAdapter(&sub.closeMu, sub.closed.Load, sub.ch))
	}
	m.mu.RUnlock()

	// Delegate to notify package
	notify.NotifyOpen(
		m.logger,
		&m.mu,
		data,
		&m.openHandlers,
		subs,
		safeCallHandler,
	)
}

// setQuit invokes all registered quit handlers and sends to subscriptions
func (m *Instance) setQuit(data types.ConnectionQuitData) {
	// Build subscription adapters
	m.mu.RLock()
	subs := make([]notify.SubscriptionTarget[types.ConnectionQuitData], 0, len(m.quitSubscriptions))
	for _, sub := range m.quitSubscriptions {
		subs = append(subs, notify.NewSubscriptionAdapter(&sub.closeMu, sub.closed.Load, sub.ch))
	}
	m.mu.RUnlock()

	// Delegate to notify package
	notify.NotifyQuit(
		m.logger,
		&m.mu,
		data,
		&m.quitHandlers,
		subs,
		safeCallHandler,
	)
}
