//go:build windows
// +build windows

package handlers

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
)

// RegisterCrashedHandler adds a Crashed handler and returns its unique ID
func RegisterCrashedHandler(mu *sync.RWMutex, handlers *[]instance.CrashedHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.CrashedHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered Crashed handler", "id", id)
	}
	return id
}

// RemoveCrashedHandler removes a Crashed handler by ID
func RemoveCrashedHandler(mu *sync.RWMutex, handlers *[]instance.CrashedHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed Crashed handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("Crashed handler not found: %s", id)
}

// RegisterCrashResetHandler adds a CrashReset handler and returns its unique ID
func RegisterCrashResetHandler(mu *sync.RWMutex, handlers *[]instance.CrashResetHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.CrashResetHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered CrashReset handler", "id", id)
	}
	return id
}

// RemoveCrashResetHandler removes a CrashReset handler by ID
func RemoveCrashResetHandler(mu *sync.RWMutex, handlers *[]instance.CrashResetHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed CrashReset handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("CrashReset handler not found: %s", id)
}

// RegisterSoundEventHandler adds a Sound event handler and returns its unique ID
func RegisterSoundEventHandler(mu *sync.RWMutex, handlers *[]instance.SoundEventHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.SoundEventHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered SoundEvent handler", "id", id)
	}
	return id
}

// RemoveSoundEventHandler removes a Sound event handler by ID
func RemoveSoundEventHandler(mu *sync.RWMutex, handlers *[]instance.SoundEventHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed SoundEvent handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("SoundEvent handler not found: %s", id)
}

// RegisterViewHandler adds a View handler and returns its unique ID
func RegisterViewHandler(mu *sync.RWMutex, handlers *[]instance.ViewHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.ViewHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered View handler", "id", id)
	}
	return id
}

// RemoveViewHandler removes a View handler by ID
func RemoveViewHandler(mu *sync.RWMutex, handlers *[]instance.ViewHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed View handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("View handler not found: %s", id)
}

// RegisterFlightPlanDeactivatedHandler adds a FlightPlanDeactivated handler and returns its unique ID
func RegisterFlightPlanDeactivatedHandler(mu *sync.RWMutex, handlers *[]instance.FlightPlanDeactivatedHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.FlightPlanDeactivatedHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered FlightPlanDeactivated handler", "id", id)
	}
	return id
}

// RemoveFlightPlanDeactivatedHandler removes a FlightPlanDeactivated handler by ID
func RemoveFlightPlanDeactivatedHandler(mu *sync.RWMutex, handlers *[]instance.FlightPlanDeactivatedHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed FlightPlanDeactivated handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightPlanDeactivated handler not found: %s", id)
}
