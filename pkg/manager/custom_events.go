//go:build windows

package manager

import (
	"errors"
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/handlers"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
	"github.com/mrlm-net/simconnect/pkg/types"
)

var (
	// ErrReservedEventName is returned when attempting to register a custom event with a reserved name
	ErrReservedEventName = errors.New("manager: event name is reserved for internal use")

	// ErrCustomEventNotFound is returned when attempting to unsubscribe from a non-existent custom event
	ErrCustomEventNotFound = errors.New("manager: custom event not found")

	// ErrCustomEventIDExhausted is returned when the custom event ID pool is exhausted
	ErrCustomEventIDExhausted = errors.New("manager: custom event ID pool exhausted")

	// ErrCustomEventNotSubscribed is returned when attempting to register a handler for an unsubscribed event
	ErrCustomEventNotSubscribed = errors.New("manager: custom event not subscribed")

	// ErrCustomEventHandlerNotFound is returned when attempting to remove a non-existent handler
	ErrCustomEventHandlerNotFound = errors.New("manager: custom event handler not found")
)

// reservedSystemEvents contains all internal event names that cannot be used as custom events
var reservedSystemEvents = map[string]bool{
	"Pause":                 true,
	"Sim":                   true,
	"FlightLoaded":          true,
	"AircraftLoaded":        true,
	"ObjectAdded":           true,
	"ObjectRemoved":         true,
	"FlightPlanActivated":   true,
	"Crashed":               true,
	"CrashReset":            true,
	"Sound":                 true,
	"View":                  true,
	"FlightPlanDeactivated": true,
}

// SubscribeToCustomSystemEvent subscribes to a custom system event by name.
// The event must not conflict with reserved internal event names.
// Returns a filtered Subscription that delivers only messages for this event.
func (m *Instance) SubscribeToCustomSystemEvent(eventName string, bufferSize int) (Subscription, error) {
	if reservedSystemEvents[eventName] {
		return nil, ErrReservedEventName
	}

	m.mu.Lock()

	// Check if already subscribed
	if ce, exists := m.customSystemEvents[eventName]; exists {
		eventID := ce.ID
		m.mu.Unlock()
		return m.customEventSubscription(eventName, eventID, bufferSize)
	}

	// Connected first: an ID allocated while disconnected would be lost (#405)
	if m.engine == nil {
		m.mu.Unlock()
		return nil, ErrNotConnected
	}

	// Allocate new event ID
	eventID, err := m.allocateCustomEventIDLocked()
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}

	// Subscribe via engine

	if err := m.engine.SubscribeToSystemEvent(eventID, eventName); err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("manager: failed to subscribe to custom system event '%s': %w", eventName, err)
	}

	// Store custom event
	m.customSystemEvents[eventName] = &instance.CustomSystemEvent{
		Name:     eventName,
		ID:       eventID,
		Handlers: []instance.CustomSystemEventHandlerEntry{},
		Conn:     m.connGen,
	}

	m.logger.Debug("[manager] Subscribed to custom system event", "event", eventName, "id", eventID)
	m.mu.Unlock()

	return m.customEventSubscription(eventName, eventID, bufferSize)
}

// customEventSubscription is a filtered subscription for the custom event
// eventName with eventID, recorded with the event so that
// UnsubscribeFromCustomSystemEvent closes it (review #30). It is created
// outside m.mu (SubscribeWithFilter takes it); an event unsubscribed in the
// meantime closes it at once.
func (m *Instance) customEventSubscription(eventName string, eventID uint32, bufferSize int) (Subscription, error) {
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(eventID)
	}
	sub := m.SubscribeWithFilter(customSubscriptionID(eventName), bufferSize, filter)
	m.mu.Lock()
	ce, exists := m.customSystemEvents[eventName]
	if !exists || ce.ID != eventID {
		m.mu.Unlock()
		sub.Unsubscribe()
		return nil, ErrCustomEventNotFound
	}
	// drop the ones already closed, so the list does not grow
	live := m.customEventSubs[eventName][:0]
	for _, s := range m.customEventSubs[eventName] {
		if !s.closed.Load() {
			live = append(live, s)
		}
	}
	m.customEventSubs[eventName] = append(live, sub.(*subscription))
	m.mu.Unlock()
	return sub, nil
}

// UnsubscribeFromCustomSystemEvent unsubscribes from a custom system event.
// Its ID is free for another event, and the subscriptions
// SubscribeToCustomSystemEvent returned for it are closed (their Done
// channels close): they would never get a message again, or worse, get the
// events of the next custom event given the same ID (review #30).
func (m *Instance) UnsubscribeFromCustomSystemEvent(eventName string) error {
	m.mu.Lock()

	ce, exists := m.customSystemEvents[eventName]
	if !exists {
		m.mu.Unlock()
		return ErrCustomEventNotFound
	}

	// Unsubscribe via engine
	if m.engine != nil {
		if err := m.engine.UnsubscribeFromSystemEvent(ce.ID); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("manager: failed to unsubscribe from custom system event '%s': %w", eventName, err)
		}
	}

	// Remove from map: its ID is free again
	delete(m.customSystemEvents, eventName)
	subs := m.customEventSubs[eventName]
	delete(m.customEventSubs, eventName)

	m.logger.Debug("[manager] Unsubscribed from custom system event", "event", eventName, "id", ce.ID)
	m.mu.Unlock()

	// Closed outside the lock: Unsubscribe takes m.mu
	for _, s := range subs {
		s.Unsubscribe()
	}
	return nil
}

// OnCustomSystemEvent registers a callback handler for a custom system event.
// The event must be subscribed first via SubscribeToCustomSystemEvent.
func (m *Instance) OnCustomSystemEvent(eventName string, handler CustomSystemEventHandler) (string, error) {
	if reservedSystemEvents[eventName] {
		return "", ErrReservedEventName
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ce, exists := m.customSystemEvents[eventName]
	if !exists {
		return "", ErrCustomEventNotSubscribed
	}

	id := generateUUID()
	ce.Handlers = append(ce.Handlers, instance.CustomSystemEventHandlerEntry{ID: id, Fn: handler})

	m.logger.Debug("[manager] Registered custom system event handler", "event", eventName, "id", id)
	return id, nil
}

// RemoveCustomSystemEvent removes a callback handler for a custom system event.
func (m *Instance) RemoveCustomSystemEvent(eventName string, handlerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ce, exists := m.customSystemEvents[eventName]
	if !exists {
		return ErrCustomEventNotFound
	}

	for i, e := range ce.Handlers {
		if e.ID == handlerID {
			ce.Handlers = append(ce.Handlers[:i], ce.Handlers[i+1:]...)
			m.logger.Debug("[manager] Removed custom system event handler", "event", eventName, "handlerID", handlerID)
			return nil
		}
	}

	return ErrCustomEventHandlerNotFound
}

// allocateCustomEventIDLocked allocates the next available custom event ID.
// Must be called with m.mu held.
func (m *Instance) allocateCustomEventIDLocked() (uint32, error) {
	// The next ID no custom event holds, from customEventIDAlloc round the
	// range: IDs of unsubscribed events are used again (review #30, they
	// were never freed). Round the range, not lowest first: a freed ID comes
	// back late, so a late event of the old one hardly reaches a new one.
	used := make(map[uint32]bool, len(m.customSystemEvents))
	for _, ce := range m.customSystemEvents {
		used[ce.ID] = true
	}
	n := CustomEventIDMax - CustomEventIDMin + 1
	next := m.customEventIDAlloc
	for i := uint32(0); i < n; i++ {
		if next < CustomEventIDMin || next > CustomEventIDMax {
			next = CustomEventIDMin
		}
		id := next
		next++
		if !used[id] {
			m.customEventIDAlloc = next
			return id, nil
		}
	}
	return 0, ErrCustomEventIDExhausted
}

// customSubscriptionID is a new subscription ID for eventName: each
// SubscribeToCustomSystemEvent call its own subscription, not one replacing
// the last (an app subscribing again after a reconnect orphaned its first,
// #405).
func customSubscriptionID(eventName string) string {
	return eventName + "-custom-" + handlers.GenerateUUID()
}
