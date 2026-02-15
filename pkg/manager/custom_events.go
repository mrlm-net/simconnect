//go:build windows

package manager

import (
	"errors"
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/engine"
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
		// Create filtered subscription outside lock to avoid deadlock
		// (SubscribeWithFilter also acquires mu.Lock)
		filter := func(msg engine.Message) bool {
			if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
				return false
			}
			ev := msg.AsEvent()
			return ev != nil && ev.UEventID == types.DWORD(eventID)
		}
		return m.SubscribeWithFilter(eventName+"-custom", bufferSize, filter), nil
	}

	// Allocate new event ID
	eventID, err := m.allocateCustomEventIDLocked()
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}

	// Subscribe via engine
	if m.engine == nil {
		m.mu.Unlock()
		return nil, ErrNotConnected
	}

	if err := m.engine.SubscribeToSystemEvent(eventID, eventName); err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("manager: failed to subscribe to custom system event '%s': %w", eventName, err)
	}

	// Store custom event
	m.customSystemEvents[eventName] = &instance.CustomSystemEvent{
		Name:     eventName,
		ID:       eventID,
		Handlers: []instance.CustomSystemEventHandlerEntry{},
	}

	m.logger.Debug("[manager] Subscribed to custom system event", "event", eventName, "id", eventID)
	m.mu.Unlock()

	// Create filtered subscription outside lock to avoid deadlock
	// (SubscribeWithFilter also acquires mu.Lock)
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(eventID)
	}
	return m.SubscribeWithFilter(eventName+"-custom", bufferSize, filter), nil
}

// UnsubscribeFromCustomSystemEvent unsubscribes from a custom system event.
func (m *Instance) UnsubscribeFromCustomSystemEvent(eventName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ce, exists := m.customSystemEvents[eventName]
	if !exists {
		return ErrCustomEventNotFound
	}

	// Unsubscribe via engine
	if m.engine != nil {
		if err := m.engine.UnsubscribeFromSystemEvent(ce.ID); err != nil {
			return fmt.Errorf("manager: failed to unsubscribe from custom system event '%s': %w", eventName, err)
		}
	}

	// Remove from map
	delete(m.customSystemEvents, eventName)

	m.logger.Debug("[manager] Unsubscribed from custom system event", "event", eventName, "id", ce.ID)
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
	if m.customEventIDAlloc > CustomEventIDMax {
		return 0, ErrCustomEventIDExhausted
	}
	id := m.customEventIDAlloc
	m.customEventIDAlloc++
	return id, nil
}
