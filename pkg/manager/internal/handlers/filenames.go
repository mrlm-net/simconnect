//go:build windows
// +build windows

package handlers

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
)

// RegisterFlightLoadedHandler adds a FlightLoaded handler and returns its unique ID
func RegisterFlightLoadedHandler(mu *sync.RWMutex, handlers *[]instance.FlightLoadedHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.FlightLoadedHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered FlightLoaded handler", "id", id)
	}
	return id
}

// RemoveFlightLoadedHandler removes a FlightLoaded handler by ID
func RemoveFlightLoadedHandler(mu *sync.RWMutex, handlers *[]instance.FlightLoadedHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed FlightLoaded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightLoaded handler not found: %s", id)
}

// RegisterAircraftLoadedHandler adds an AircraftLoaded handler and returns its unique ID
func RegisterAircraftLoadedHandler(mu *sync.RWMutex, handlers *[]instance.FlightLoadedHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.FlightLoadedHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered AircraftLoaded handler", "id", id)
	}
	return id
}

// RemoveAircraftLoadedHandler removes an AircraftLoaded handler by ID
func RemoveAircraftLoadedHandler(mu *sync.RWMutex, handlers *[]instance.FlightLoadedHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed AircraftLoaded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("AircraftLoaded handler not found: %s", id)
}

// RegisterFlightPlanActivatedHandler adds a FlightPlanActivated handler and returns its unique ID
func RegisterFlightPlanActivatedHandler(mu *sync.RWMutex, handlers *[]instance.FlightLoadedHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.FlightLoadedHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered FlightPlanActivated handler", "id", id)
	}
	return id
}

// RemoveFlightPlanActivatedHandler removes a FlightPlanActivated handler by ID
func RemoveFlightPlanActivatedHandler(mu *sync.RWMutex, handlers *[]instance.FlightLoadedHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed FlightPlanActivated handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightPlanActivated handler not found: %s", id)
}
