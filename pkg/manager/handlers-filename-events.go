//go:build windows
// +build windows

package manager

import "fmt"

// OnFlightLoaded registers a callback invoked when a FlightLoaded system event arrives.
func (m *Instance) OnFlightLoaded(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.flightLoadedHandlers = append(m.flightLoadedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered FlightLoaded handler", "id", id)
	}
	return id
}

// RemoveFlightLoaded removes a previously registered FlightLoaded handler.
func (m *Instance) RemoveFlightLoaded(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.flightLoadedHandlers {
		if e.id == id {
			m.flightLoadedHandlers = append(m.flightLoadedHandlers[:i], m.flightLoadedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed FlightLoaded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightLoaded handler not found: %s", id)
}

// OnAircraftLoaded registers a callback invoked when an AircraftLoaded system event arrives.
func (m *Instance) OnAircraftLoaded(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.aircraftLoadedHandlers = append(m.aircraftLoadedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered AircraftLoaded handler", "id", id)
	}
	return id
}

// RemoveAircraftLoaded removes a previously registered AircraftLoaded handler.
func (m *Instance) RemoveAircraftLoaded(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.aircraftLoadedHandlers {
		if e.id == id {
			m.aircraftLoadedHandlers = append(m.aircraftLoadedHandlers[:i], m.aircraftLoadedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed AircraftLoaded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("AircraftLoaded handler not found: %s", id)
}

// OnFlightPlanActivated registers a callback invoked when a FlightPlanActivated system event arrives.
func (m *Instance) OnFlightPlanActivated(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.flightPlanActivatedHandlers = append(m.flightPlanActivatedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered FlightPlanActivated handler", "id", id)
	}
	return id
}

// RemoveFlightPlanActivated removes a previously registered FlightPlanActivated handler.
func (m *Instance) RemoveFlightPlanActivated(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.flightPlanActivatedHandlers {
		if e.id == id {
			m.flightPlanActivatedHandlers = append(m.flightPlanActivatedHandlers[:i], m.flightPlanActivatedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed FlightPlanActivated handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightPlanActivated handler not found: %s", id)
}
