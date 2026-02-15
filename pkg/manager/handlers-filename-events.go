//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/manager/internal/handlers"
)

// OnFlightLoaded registers a callback invoked when a FlightLoaded system event arrives.
func (m *Instance) OnFlightLoaded(handler FlightLoadedHandler) string {
	return handlers.RegisterFlightLoadedHandler(&m.mu, &m.flightLoadedHandlers, handler, m.logger)
}

// RemoveFlightLoaded removes a previously registered FlightLoaded handler.
func (m *Instance) RemoveFlightLoaded(id string) error {
	return handlers.RemoveFlightLoadedHandler(&m.mu, &m.flightLoadedHandlers, id, m.logger)
}

// OnAircraftLoaded registers a callback invoked when an AircraftLoaded system event arrives.
func (m *Instance) OnAircraftLoaded(handler FlightLoadedHandler) string {
	return handlers.RegisterAircraftLoadedHandler(&m.mu, &m.aircraftLoadedHandlers, handler, m.logger)
}

// RemoveAircraftLoaded removes a previously registered AircraftLoaded handler.
func (m *Instance) RemoveAircraftLoaded(id string) error {
	return handlers.RemoveAircraftLoadedHandler(&m.mu, &m.aircraftLoadedHandlers, id, m.logger)
}

// OnFlightPlanActivated registers a callback invoked when a FlightPlanActivated system event arrives.
func (m *Instance) OnFlightPlanActivated(handler FlightLoadedHandler) string {
	return handlers.RegisterFlightPlanActivatedHandler(&m.mu, &m.flightPlanActivatedHandlers, handler, m.logger)
}

// RemoveFlightPlanActivated removes a previously registered FlightPlanActivated handler.
func (m *Instance) RemoveFlightPlanActivated(id string) error {
	return handlers.RemoveFlightPlanActivatedHandler(&m.mu, &m.flightPlanActivatedHandlers, id, m.logger)
}
