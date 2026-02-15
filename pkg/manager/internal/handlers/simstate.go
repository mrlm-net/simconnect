//go:build windows
// +build windows

package handlers

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
)

// RegisterSimStateHandler adds a SimState change handler and returns its unique ID
func RegisterSimStateHandler(mu *sync.RWMutex, handlers *[]instance.SimStateHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.SimStateHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered SimState handler", "id", id)
	}
	return id
}

// RemoveSimStateHandler removes a SimState change handler by ID
func RemoveSimStateHandler(mu *sync.RWMutex, handlers *[]instance.SimStateHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed SimState handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("SimState handler not found: %s", id)
}

// RegisterPauseHandler adds a Pause handler and returns its unique ID
func RegisterPauseHandler(mu *sync.RWMutex, handlers *[]instance.PauseHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.PauseHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered Pause handler", "id", id)
	}
	return id
}

// RemovePauseHandler removes a Pause handler by ID
func RemovePauseHandler(mu *sync.RWMutex, handlers *[]instance.PauseHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed Pause handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("Pause handler not found: %s", id)
}

// RegisterSimRunningHandler adds a SimRunning handler and returns its unique ID
func RegisterSimRunningHandler(mu *sync.RWMutex, handlers *[]instance.SimRunningHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.SimRunningHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered SimRunning handler", "id", id)
	}
	return id
}

// RemoveSimRunningHandler removes a SimRunning handler by ID
func RemoveSimRunningHandler(mu *sync.RWMutex, handlers *[]instance.SimRunningHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed SimRunning handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("SimRunning handler not found: %s", id)
}
